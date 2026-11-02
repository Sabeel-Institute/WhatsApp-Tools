// Package runs keeps the history shown under "Recent activity": one JSON
// file per run in the data directory, readable only by the user. A run has a
// common header every workflow fills in and a workflow-specific Detail.
package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/platform"
)

// Status of a run.
type Status string

const (
	StatusRunning     Status = "running"
	StatusCompleted   Status = "completed"
	StatusStopped     Status = "stopped"
	StatusFailed      Status = "failed"
	StatusInterrupted Status = "interrupted" // the app closed during the run
)

// Run is one saved activity.
type Run struct {
	ID         string     `json:"id"`
	Workflow   string     `json:"workflow"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Subtitle   string     `json:"subtitle,omitempty"`
	Status     Status     `json:"status"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	// Headline is the short result shown in Recent activity, e.g. "40 added".
	Headline string         `json:"headline"`
	Counts   map[string]int `json:"counts"`
	Error    string         `json:"error,omitempty"`
	// Detail is owned by the workflow.
	Detail json.RawMessage `json:"detail,omitempty"`
}

// Store reads and writes runs.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open prepares dir/runs and marks runs left "running" by a previous app
// session as interrupted.
func Open(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "runs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	all, err := s.all()
	if err != nil {
		return nil, err
	}
	for _, r := range all {
		if r.Status == StatusRunning {
			r.Status = StatusInterrupted
			if r.Error == "" {
				r.Error = "The app was closed before this finished. The results up to that point are below."
			}
			if err := s.Save(r); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

var idPattern = regexp.MustCompile(`^[a-z0-9-]{6,64}$`)

// NewID returns a sortable, unique run ID.
func NewID() string {
	return fmt.Sprintf("%s-%04x", time.Now().UTC().Format("20060102-150405"), time.Now().UnixNano()&0xffff)
}

func (s *Store) path(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("invalid run id %q", id)
	}
	return filepath.Join(s.dir, id+".json"), nil
}

// Save writes r atomically.
func (s *Store) Save(r Run) error {
	p, err := s.path(r.ID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return platform.WriteFileAtomic(p, data, 0o600)
}

// ErrNotFound is returned by Get for an unknown run.
var ErrNotFound = errors.New("run not found")

// Get loads one run.
func (s *Store) Get(id string) (Run, error) {
	var r Run
	p, err := s.path(id)
	if err != nil {
		return r, ErrNotFound
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return r, ErrNotFound
	} else if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, fmt.Errorf("reading run %s: %w", id, err)
	}
	return r, nil
}

func (s *Store) all() ([]Run, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []Run
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue
		}
		r, err := s.Get(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue // a damaged file must not hide the rest of the history
		}
		out = append(out, r)
	}
	return out, nil
}

// List returns up to limit runs without their Detail, newest first.
func (s *Store) List(limit int) ([]Run, error) {
	all, err := s.all()
	if err != nil {
		return nil, err
	}
	slices.SortFunc(all, func(a, b Run) int { return b.StartedAt.Compare(a.StartedAt) })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	for i := range all {
		all[i].Detail = nil
	}
	return all, nil
}
