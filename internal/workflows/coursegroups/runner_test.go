package coursegroups

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/api"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/jobs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/people"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/runs"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/settings"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp/demo"
	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/workflows"
)

// recorder wraps the demo client to observe and disturb the run.
type recorder struct {
	whatsapp.Client
	mu       sync.Mutex
	batches  [][]string
	failures int
	onAdd    func(call int) // called before each AddParticipants
}

func (r *recorder) AddParticipants(ctx context.Context, group string, ps []whatsapp.Participant) ([]whatsapp.AddResult, error) {
	r.mu.Lock()
	call := len(r.batches) + r.failures + 1
	hook := r.onAdd
	r.mu.Unlock()
	if hook != nil {
		hook(call)
	}
	res, err := r.Client.AddParticipants(ctx, group, ps)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.failures++
		return nil, err
	}
	phones := make([]string, len(ps))
	for i, p := range ps {
		phones[i] = p.Phone
	}
	r.batches = append(r.batches, phones)
	return res, nil
}

type fixture struct {
	wf   *Workflow
	rec  *recorder
	demo *demo.Client
	deps workflows.Deps
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo-session.json"), []byte(`{"Linked":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := demo.New(dir, demo.Options{Latency: 0.002, OfflineFor: 300 * time.Millisecond})
	if err := d.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for d.Status().State != whatsapp.StateConnected {
		if time.Now().After(deadline) {
			t.Fatal("demo client never connected")
		}
		time.Sleep(5 * time.Millisecond)
	}
	rec := &recorder{Client: d}
	pacing := whatsapp.DefaultPacing.Scaled(0.0005) // milliseconds instead of seconds
	rs, err := runs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := settings.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	deps := workflows.Deps{WA: rec, Jobs: jobs.NewManager(), Runs: rs, Settings: st,
		Checker: people.NewChecker(rec, pacing), Pacing: pacing, Log: slog.New(slog.DiscardHandler)}
	return &fixture{wf: New(deps), rec: rec, demo: d, deps: deps}
}

func phones(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("+1212555%04d", 100+i)
	}
	return out
}

func (f *fixture) start(t *testing.T, req map[string]any) (runID string, status int, body string) {
	t.Helper()
	data, _ := json.Marshal(req)
	rw := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(data))
	if err := f.wf.handleRun(rw, r); err != nil {
		return "", errStatus(err), err.Error()
	}
	var res struct {
		RunID string `json:"runId"`
	}
	_ = json.Unmarshal(rw.Body.Bytes(), &res)
	return res.RunID, rw.Code, rw.Body.String()
}

func errStatus(err error) int {
	var ae *api.Error
	if errors.As(err, &ae) {
		return ae.Status
	}
	return http.StatusInternalServerError
}

func createRequest(ps []string) map[string]any {
	people := make([]map[string]any, len(ps))
	for i, p := range ps {
		people[i] = map[string]any{"row": i + 2, "name": fmt.Sprintf("Person %d", i+1), "phone": p, "display": p}
	}
	return map[string]any{
		"mode": "create", "communityId": "demo-community-1", "communityName": "Sabeel Institute",
		"groupName": "Tafsir Circle Fall 2026", "description": "Weekly class",
		"settings": map[string]bool{"onlyAdminsEdit": true, "approvalRequired": true},
		"people":   people,
	}
}

func (f *fixture) wait(t *testing.T, runID string) (runs.Run, Detail) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if !f.deps.Jobs.Busy() {
			run, err := f.deps.Runs.Get(runID)
			if err != nil {
				t.Fatal(err)
			}
			d, err := loadDetail(run)
			if err != nil {
				t.Fatal(err)
			}
			if run.Status != runs.StatusRunning {
				return run, d
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run did not finish")
	return runs.Run{}, Detail{}
}

func TestCreateRunBatchesRetriesAndRecordsEveryone(t *testing.T) {
	f := newFixture(t)
	ps := phones(40)
	runID, code, body := f.start(t, createRequest(ps))
	if code != http.StatusOK {
		t.Fatalf("start: %d %s", code, body)
	}
	run, d := f.wait(t, runID)
	if run.Status != runs.StatusCompleted {
		t.Fatalf("status = %s (%s)", run.Status, run.Error)
	}

	// Batches are small, and nobody is sent twice once they've succeeded.
	sent := map[string]int{}
	for _, b := range f.rec.batches {
		if len(b) == 0 || len(b) > f.deps.Pacing.AddBatchSize {
			t.Errorf("batch of %d", len(b))
		}
		for _, p := range b {
			sent[p]++
		}
	}
	for p, n := range sent {
		if n != 1 {
			t.Errorf("%s added %d times", p, n)
		}
	}
	// The demo fails some batches once with a temporary error: they must
	// have been retried, not reported as failures.
	if f.rec.failures == 0 {
		t.Error("expected the demo to produce at least one temporary failure")
	}

	counts := map[Outcome]int{}
	for _, p := range d.People {
		counts[p.Outcome]++
		if p.Outcome == OutcomePending {
			t.Errorf("%s left pending in a completed run", p.Phone)
		}
		if p.Outcome == Outcome(whatsapp.AddNotOnWhatsApp) && sent[p.Phone] > 0 {
			t.Errorf("%s isn't on WhatsApp but was sent to WhatsApp to add", p.Phone)
		}
		if p.Outcome == Outcome(whatsapp.AddPrivacy) && p.Invite == nil {
			t.Errorf("%s needs an invite but has no invite token", p.Phone)
		}
	}
	if counts[Outcome(whatsapp.AddAdded)] < 25 || counts[Outcome(whatsapp.AddFailed)] != 0 {
		t.Errorf("outcomes = %v", counts)
	}
	if run.Counts[countAdded] != counts[Outcome(whatsapp.AddAdded)] {
		t.Errorf("run counts %v disagree with people %v", run.Counts, counts)
	}
	total := 0
	for _, n := range run.Counts {
		total += n
	}
	if total != len(ps) {
		t.Errorf("counts add up to %d, want %d", total, len(ps))
	}
	if d.GroupID == "" || !strings.HasPrefix(d.InviteLink, "https://chat.whatsapp.com/") {
		t.Errorf("group %q link %q", d.GroupID, d.InviteLink)
	}
	if !strings.HasPrefix(d.InviteMessage, "Assalamu alaikum!") || !strings.Contains(d.InviteMessage, d.InviteLink) {
		t.Errorf("invite message = %q", d.InviteMessage)
	}
}

func TestStopKeepsWorkDone(t *testing.T) {
	f := newFixture(t)
	f.rec.onAdd = func(call int) {
		if call == 3 {
			if snap, ok := f.deps.Jobs.Active(); ok {
				f.deps.Jobs.Stop(snap.ID)
			}
		}
	}
	runID, code, body := f.start(t, createRequest(phones(40)))
	if code != http.StatusOK {
		t.Fatalf("start: %d %s", code, body)
	}
	run, d := f.wait(t, runID)
	if run.Status != runs.StatusStopped || run.Error == "" {
		t.Fatalf("status = %s, error %q", run.Status, run.Error)
	}
	added, pending := 0, 0
	for _, p := range d.People {
		switch p.Outcome {
		case Outcome(whatsapp.AddAdded):
			added++
		case OutcomePending:
			pending++
		}
	}
	if added == 0 || pending == 0 {
		t.Errorf("added %d, pending %d: want some of each", added, pending)
	}
	if d.InviteLink == "" {
		t.Error("a stopped run should still record the invite link")
	}
}

func TestWaitsForTheConnectionAndCarriesOn(t *testing.T) {
	f := newFixture(t)
	var waited bool
	events, stop := f.deps.Jobs.Subscribe()
	defer stop()
	go func() {
		for s := range events {
			if s.State == jobs.Waiting {
				waited = true
			}
		}
	}()
	f.rec.onAdd = func(call int) {
		if call == 2 {
			_ = f.demo.Simulate("offline")
		}
	}
	runID, code, body := f.start(t, createRequest(phones(20)))
	if code != http.StatusOK {
		t.Fatalf("start: %d %s", code, body)
	}
	run, d := f.wait(t, runID)
	if run.Status != runs.StatusCompleted {
		t.Fatalf("status = %s (%s)", run.Status, run.Error)
	}
	for _, p := range d.People {
		if p.Outcome == OutcomePending || p.Outcome == Outcome(whatsapp.AddFailed) {
			t.Errorf("%s: %s after the connection came back", p.Phone, p.Outcome)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if !waited {
		t.Error("the job never showed that it was waiting for the connection")
	}
}

func TestAddToExistingGroup(t *testing.T) {
	f := newFixture(t)
	req := createRequest(phones(7))
	req["mode"] = "add"
	req["groupId"] = "demo-g-arabic"
	req["groupName"] = "Arabic Level 1"
	runID, code, body := f.start(t, req)
	if code != http.StatusOK {
		t.Fatalf("start: %d %s", code, body)
	}
	run, d := f.wait(t, runID)
	if run.Status != runs.StatusCompleted || d.GroupID != "demo-g-arabic" || d.Description != "" {
		t.Fatalf("run %s %+v", run.Status, d)
	}
}

func TestValidation(t *testing.T) {
	f := newFixture(t)
	bad := []map[string]any{
		func() map[string]any {
			r := createRequest(phones(2))
			r["groupName"] = "This name is far too long for WhatsApp"
			return r
		}(),
		func() map[string]any { r := createRequest(phones(2)); r["groupName"] = "  "; return r }(),
		func() map[string]any { r := createRequest(phones(2)); r["communityId"] = ""; return r }(),
		func() map[string]any { r := createRequest(nil); return r }(),
		func() map[string]any { r := createRequest([]string{"+12125550100", "+12125550100"}); return r }(),
		func() map[string]any { r := createRequest([]string{"2125550100"}); return r }(),
		func() map[string]any { r := createRequest(phones(1024)); return r }(),
		func() map[string]any { r := createRequest(phones(2)); r["mode"] = "add"; return r }(),
		func() map[string]any { r := createRequest(phones(2)); r["mode"] = "delete"; return r }(),
	}
	for i, req := range bad {
		if _, code, body := f.start(t, req); code == http.StatusOK {
			t.Errorf("case %d accepted: %s", i, body)
		}
	}
	if f.deps.Jobs.Busy() {
		t.Error("an invalid request started a job")
	}
}
