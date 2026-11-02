// Package jobs runs long WhatsApp tasks in the background, one at a time,
// with pause, stop, progress, an estimated time left and a feed of recent
// results. Workflows supply the work as a function of a *Control.
//
// Pausing and stopping take effect at safe points (Checkpoint and Sleep), never
// in the middle of a WhatsApp request, so work already done is always known.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// State of a job.
type State string

const (
	Running   State = "running"
	Paused    State = "paused"
	Waiting   State = "waiting"  // paused automatically, e.g. offline
	Stopping  State = "stopping" // stop requested; finishing the current step
	Completed State = "completed"
	Stopped   State = "stopped"
	Failed    State = "failed"
)

// Finished reports whether s is terminal.
func (s State) Finished() bool { return s == Completed || s == Stopped || s == Failed }

// Entry is one line in the live results feed.
type Entry struct {
	At      time.Time `json:"at"`
	Title   string    `json:"title"`
	Detail  string    `json:"detail,omitempty"`
	Outcome string    `json:"outcome"`
}

// Snapshot is the public view of a job.
type Snapshot struct {
	ID         string         `json:"id"`
	Workflow   string         `json:"workflow"`
	Kind       string         `json:"kind"`
	Title      string         `json:"title"`
	RunID      string         `json:"runId,omitempty"`
	State      State          `json:"state"`
	Phase      string         `json:"phase"`
	Message    string         `json:"message,omitempty"`
	Done       int            `json:"done"`
	Total      int            `json:"total"`
	Counts     map[string]int `json:"counts"`
	ETASeconds int            `json:"etaSeconds"` // -1 when unknown
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	Recent     []Entry        `json:"recent"`
	Error      string         `json:"error,omitempty"`
}

// Spec describes a job to start.
type Spec struct {
	Workflow string
	Kind     string
	Title    string
	RunID    string
}

// ErrStopped is returned by Control methods once the user asks to stop.
var ErrStopped = errors.New("stopped by the user")

// ErrBusy is returned by Start while another job is active.
var ErrBusy = errors.New("another job is running")

const recentLimit = 40

// Manager owns the jobs.
type Manager struct {
	mu     sync.Mutex
	jobs   map[string]*Job
	active *Job
	subs   map[int]chan Snapshot
	nextID int
	// OnBusy is called with true when a job starts and false when it ends.
	OnBusy func(busy bool)
}

// NewManager returns an empty manager.
func NewManager() *Manager {
	return &Manager{jobs: map[string]*Job{}, subs: map[int]chan Snapshot{}}
}

// Job is a running or finished job.
type Job struct {
	m        *Manager
	mu       sync.Mutex
	snap     Snapshot
	paused   bool
	stopping bool
	wake     chan struct{} // closed and replaced on every pause/resume/stop
	// active time accounting for the ETA
	activeSince time.Time
	activeTotal time.Duration
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Start runs fn in the background. fn's error decides the final state:
// nil → Completed, ErrStopped → Stopped, anything else → Failed with
// Control.Fail's message (or a generic one).
func (m *Manager) Start(spec Spec, fn func(*Control) error) (Snapshot, error) {
	m.mu.Lock()
	if m.active != nil {
		m.mu.Unlock()
		return Snapshot{}, ErrBusy
	}
	now := time.Now()
	j := &Job{m: m, wake: make(chan struct{}), activeSince: now}
	j.snap = Snapshot{ID: newID(), Workflow: spec.Workflow, Kind: spec.Kind, Title: spec.Title, RunID: spec.RunID,
		State: Running, Counts: map[string]int{}, ETASeconds: -1, StartedAt: now, Recent: []Entry{}}
	m.jobs[j.snap.ID] = j
	m.active = j
	onBusy := m.OnBusy
	m.mu.Unlock()
	if onBusy != nil {
		onBusy(true)
	}

	snap := j.snapshot()
	m.publish(snap)
	go func() {
		ctl := &Control{job: j}
		err := fn(ctl)
		j.finish(err)
		m.mu.Lock()
		m.active = nil
		m.mu.Unlock()
		if onBusy != nil {
			onBusy(false)
		}
		m.publish(j.snapshot())
	}()
	return snap, nil
}

// Busy reports whether a job is running (including paused or waiting).
func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active != nil
}

// Active returns the active job's snapshot.
func (m *Manager) Active() (Snapshot, bool) {
	m.mu.Lock()
	j := m.active
	m.mu.Unlock()
	if j == nil {
		return Snapshot{}, false
	}
	return j.snapshot(), true
}

// Get returns a job's snapshot.
func (m *Manager) Get(id string) (Snapshot, bool) {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return Snapshot{}, false
	}
	return j.snapshot(), true
}

func (m *Manager) job(id string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[id]
}

// Pause asks the job to pause at its next safe point.
func (m *Manager) Pause(id string) bool {
	j := m.job(id)
	if j == nil {
		return false
	}
	j.mu.Lock()
	if j.snap.State.Finished() || j.stopping {
		j.mu.Unlock()
		return false
	}
	j.paused = true
	j.pauseClock()
	j.snap.State = Paused
	j.kick()
	j.mu.Unlock()
	m.publish(j.snapshot())
	return true
}

// Resume continues a paused job.
func (m *Manager) Resume(id string) bool {
	j := m.job(id)
	if j == nil {
		return false
	}
	j.mu.Lock()
	if !j.paused || j.stopping {
		j.mu.Unlock()
		return false
	}
	j.paused = false
	j.activeSince = time.Now()
	j.snap.State = Running
	if j.snap.Message != "" {
		j.snap.State = Waiting
	}
	j.kick()
	j.mu.Unlock()
	m.publish(j.snapshot())
	return true
}

// Stop asks the job to stop at its next safe point.
func (m *Manager) Stop(id string) bool {
	j := m.job(id)
	if j == nil {
		return false
	}
	j.mu.Lock()
	if j.snap.State.Finished() {
		j.mu.Unlock()
		return false
	}
	j.stopping = true
	j.paused = false
	j.snap.State = Stopping
	j.kick()
	j.mu.Unlock()
	m.publish(j.snapshot())
	return true
}

// Subscribe receives a snapshot after every change.
func (m *Manager) Subscribe() (<-chan Snapshot, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	ch := make(chan Snapshot, 64)
	m.subs[id] = ch
	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if c, ok := m.subs[id]; ok {
			delete(m.subs, id)
			close(c)
		}
	}
}

func (m *Manager) publish(s Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.subs {
		select {
		case ch <- s:
		default:
		}
	}
}

// ---- Job internals (j.mu held where noted) ----

// kick wakes anything blocked in Checkpoint or Sleep. Caller holds j.mu.
func (j *Job) kick() {
	close(j.wake)
	j.wake = make(chan struct{})
}

// pauseClock stops the active-time clock. Caller holds j.mu.
func (j *Job) pauseClock() {
	if !j.activeSince.IsZero() {
		j.activeTotal += time.Since(j.activeSince)
		j.activeSince = time.Time{}
	}
}

func (j *Job) snapshot() Snapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	s := j.snap
	s.Counts = make(map[string]int, len(j.snap.Counts))
	for k, v := range j.snap.Counts {
		s.Counts[k] = v
	}
	s.Recent = make([]Entry, len(j.snap.Recent)) // never nil: the UI reads .length
	copy(s.Recent, j.snap.Recent)
	s.ETASeconds = j.eta()
	return s
}

// eta estimates the seconds left from active time per unit done. Caller
// holds j.mu.
func (j *Job) eta() int {
	if j.snap.Done <= 0 || j.snap.Total <= j.snap.Done || j.snap.State.Finished() {
		if j.snap.State.Finished() {
			return 0
		}
		return -1
	}
	active := j.activeTotal
	if !j.activeSince.IsZero() {
		active += time.Since(j.activeSince)
	}
	per := active / time.Duration(j.snap.Done)
	return int((per * time.Duration(j.snap.Total-j.snap.Done)).Seconds() + 0.5)
}

func (j *Job) finish(err error) {
	j.mu.Lock()
	now := time.Now()
	j.pauseClock()
	j.snap.FinishedAt = &now
	j.snap.Message = ""
	var failure *Failure
	switch {
	case err == nil:
		j.snap.State = Completed
	case errors.Is(err, ErrStopped):
		j.snap.State = Stopped
	case errors.As(err, &failure):
		j.snap.State = Failed
		j.snap.Error = failure.Message
	default:
		j.snap.State = Failed
		j.snap.Error = "Something unexpected went wrong. The results so far have been saved."
	}
	j.mu.Unlock()
}

// Failure is an error whose Message is shown to the user.
type Failure struct {
	Message string
	Err     error
}

func (f *Failure) Error() string {
	if f.Err != nil {
		return f.Message + ": " + f.Err.Error()
	}
	return f.Message
}

func (f *Failure) Unwrap() error { return f.Err }

// Control is the job's handle on its own progress.
type Control struct {
	job *Job
}

func (c *Control) update(f func(s *Snapshot)) {
	j := c.job
	j.mu.Lock()
	f(&j.snap)
	j.mu.Unlock()
	j.m.publish(j.snapshot())
}

// ID returns the job ID.
func (c *Control) ID() string { return c.job.snap.ID }

// Phase sets the plain-language description of the current step.
func (c *Control) Phase(text string) { c.update(func(s *Snapshot) { s.Phase = text }) }

// Progress records how many of total units are done.
func (c *Control) Progress(done, total int) {
	c.update(func(s *Snapshot) { s.Done, s.Total = done, total })
}

// Count adds delta to a named counter.
func (c *Control) Count(key string, delta int) {
	c.update(func(s *Snapshot) { s.Counts[key] += delta })
}

// Add records results in the live feed (newest first).
func (c *Control) Add(entries ...Entry) {
	c.update(func(s *Snapshot) {
		for _, e := range entries {
			if e.At.IsZero() {
				e.At = time.Now()
			}
			s.Recent = append([]Entry{e}, s.Recent...)
		}
		if len(s.Recent) > recentLimit {
			s.Recent = s.Recent[:recentLimit]
		}
	})
}

// Waiting shows that the job is waiting for something outside its control
// (empty msg clears it). Waiting time does not count toward the ETA.
func (c *Control) Waiting(msg string) {
	j := c.job
	j.mu.Lock()
	j.snap.Message = msg
	if !j.paused && !j.stopping {
		if msg != "" {
			j.snap.State = Waiting
			j.pauseClock()
		} else {
			j.snap.State = Running
			if j.activeSince.IsZero() {
				j.activeSince = time.Now()
			}
		}
	}
	j.mu.Unlock()
	j.m.publish(j.snapshot())
}

// Stopping reports whether the user asked to stop.
func (c *Control) Stopping() bool {
	c.job.mu.Lock()
	defer c.job.mu.Unlock()
	return c.job.stopping
}

// Checkpoint returns ErrStopped if a stop was requested and blocks while the
// job is paused. Call it between units of work.
func (c *Control) Checkpoint() error {
	j := c.job
	for {
		j.mu.Lock()
		stopping, paused, wake := j.stopping, j.paused, j.wake
		j.mu.Unlock()
		if stopping {
			return ErrStopped
		}
		if !paused {
			return nil
		}
		<-wake
	}
}

// Sleep waits d, honouring pause (the remaining wait resumes afterwards) and
// stop (returns ErrStopped immediately).
func (c *Control) Sleep(d time.Duration) error {
	deadline := time.Now().Add(d)
	for {
		if err := c.Checkpoint(); err != nil {
			return err
		}
		left := time.Until(deadline)
		if left <= 0 {
			return nil
		}
		j := c.job
		j.mu.Lock()
		wake := j.wake
		j.mu.Unlock()
		t := time.NewTimer(left)
		select {
		case <-t.C:
		case <-wake:
			t.Stop()
			// A pause extends the deadline by however long it lasts.
			pauseStart := time.Now()
			if err := c.Checkpoint(); err != nil {
				return err
			}
			deadline = deadline.Add(time.Since(pauseStart))
		}
	}
}

// Context returns a context for one WhatsApp request. It is deliberately not
// cancelled by Stop: a request in flight is allowed to finish so its result
// is known. The timeout bounds a request that never answers.
func (c *Control) Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 75*time.Second)
}
