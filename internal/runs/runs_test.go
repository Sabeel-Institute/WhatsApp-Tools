package runs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveListAndInterrupted(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	older := Run{ID: "20260901-100000-0001", Workflow: "w", Title: "Old", Status: StatusCompleted,
		StartedAt: time.Now().Add(-time.Hour), Detail: []byte(`{"x":1}`)}
	newer := Run{ID: "20260901-110000-0002", Workflow: "w", Title: "New", Status: StatusRunning, StartedAt: time.Now()}
	for _, r := range []Run{older, newer} {
		if err := s.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.List(10)
	if err != nil || len(list) != 2 || list[0].Title != "New" || list[1].Detail != nil {
		t.Fatalf("list = %+v, %v", list, err)
	}
	st, _ := os.Stat(filepath.Join(dir, "runs", older.ID+".json"))
	if st.Mode().Perm() != 0o600 {
		t.Errorf("run file mode = %v", st.Mode().Perm())
	}

	// Reopening (the app restarted) marks the unfinished run as interrupted.
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Get(newer.ID)
	if err != nil || r.Status != StatusInterrupted || r.Error == "" {
		t.Errorf("after restart: %+v, %v", r, err)
	}
	if _, err := s.Get("../../etc/passwd"); err != ErrNotFound {
		t.Errorf("path traversal: err = %v", err)
	}
}
