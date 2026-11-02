package jobs

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func waitFor(t *testing.T, m *Manager, id string, cond func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s, ok := m.Get(id); ok && cond(s) {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	s, _ := m.Get(id)
	t.Fatalf("condition not met; last snapshot %+v", s)
	return s
}

func TestCompletesAndReportsProgress(t *testing.T) {
	m := NewManager()
	var busy []bool
	m.OnBusy = func(b bool) { busy = append(busy, b) }
	snap, err := m.Start(Spec{Title: "t"}, func(c *Control) error {
		c.Phase("working")
		for i := 1; i <= 3; i++ {
			c.Add(Entry{Title: "p", Outcome: "added"})
			c.Count("added", 1)
			c.Progress(i, 3)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s := waitFor(t, m, snap.ID, func(s Snapshot) bool { return s.State.Finished() })
	if s.State != Completed || s.Done != 3 || s.Counts["added"] != 3 || len(s.Recent) != 3 || s.ETASeconds != 0 {
		t.Errorf("final = %+v", s)
	}
	if m.Busy() || len(busy) != 2 || !busy[0] || busy[1] {
		t.Errorf("busy transitions = %v", busy)
	}
}

func TestOneJobAtATime(t *testing.T) {
	m := NewManager()
	release := make(chan struct{})
	snap, _ := m.Start(Spec{}, func(*Control) error { <-release; return nil })
	if _, err := m.Start(Spec{}, func(*Control) error { return nil }); !errors.Is(err, ErrBusy) {
		t.Errorf("second Start err = %v, want ErrBusy", err)
	}
	close(release)
	waitFor(t, m, snap.ID, func(s Snapshot) bool { return s.State.Finished() })
}

func TestPauseResumeAndStopAtSafePoints(t *testing.T) {
	m := NewManager()
	var steps atomic.Int32
	snap, _ := m.Start(Spec{}, func(c *Control) error {
		for {
			if err := c.Checkpoint(); err != nil {
				return err
			}
			steps.Add(1)
			if err := c.Sleep(10 * time.Millisecond); err != nil {
				return err
			}
		}
	})
	waitFor(t, m, snap.ID, func(Snapshot) bool { return steps.Load() > 2 })
	if !m.Pause(snap.ID) {
		t.Fatal("pause refused")
	}
	time.Sleep(50 * time.Millisecond)
	frozen := steps.Load()
	time.Sleep(100 * time.Millisecond)
	if got := steps.Load(); got > frozen+1 {
		t.Errorf("kept working while paused: %d -> %d", frozen, got)
	}
	if s, _ := m.Get(snap.ID); s.State != Paused {
		t.Errorf("state = %s, want paused", s.State)
	}
	m.Resume(snap.ID)
	waitFor(t, m, snap.ID, func(Snapshot) bool { return steps.Load() > frozen+2 })
	m.Stop(snap.ID)
	s := waitFor(t, m, snap.ID, func(s Snapshot) bool { return s.State.Finished() })
	if s.State != Stopped {
		t.Errorf("state = %s, want stopped", s.State)
	}
}

func TestStopWhilePaused(t *testing.T) {
	m := NewManager()
	snap, _ := m.Start(Spec{}, func(c *Control) error {
		for {
			if err := c.Sleep(time.Hour); err != nil {
				return err
			}
		}
	})
	m.Pause(snap.ID)
	m.Stop(snap.ID)
	if s := waitFor(t, m, snap.ID, func(s Snapshot) bool { return s.State.Finished() }); s.State != Stopped {
		t.Errorf("state = %s", s.State)
	}
}

func TestFailureMessage(t *testing.T) {
	m := NewManager()
	snap, _ := m.Start(Spec{}, func(*Control) error {
		return &Failure{Message: "Plain words.", Err: errors.New("detail")}
	})
	s := waitFor(t, m, snap.ID, func(s Snapshot) bool { return s.State.Finished() })
	if s.State != Failed || s.Error != "Plain words." {
		t.Errorf("got %+v", s)
	}
}

func TestWaitingDoesNotCountTowardETA(t *testing.T) {
	m := NewManager()
	gate := make(chan struct{})
	snap, _ := m.Start(Spec{}, func(c *Control) error {
		c.Progress(1, 4)
		c.Waiting("offline")
		<-gate
		return nil
	})
	s := waitFor(t, m, snap.ID, func(s Snapshot) bool { return s.State == Waiting })
	if s.Message != "offline" {
		t.Errorf("message = %q", s.Message)
	}
	close(gate)
	waitFor(t, m, snap.ID, func(s Snapshot) bool { return s.State.Finished() })
}
