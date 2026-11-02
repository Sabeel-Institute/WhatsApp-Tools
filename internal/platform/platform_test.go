package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSingleInstanceLock(t *testing.T) {
	dir := t.TempDir()
	first, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Publish(RunningInstance{URL: "http://127.0.0.1:1234/#token=abc", PID: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireInstanceLock(dir); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second lock: err = %v, want ErrAlreadyRunning", err)
	}
	info, err := ReadRunningInstance(dir)
	if err != nil || info.URL != "http://127.0.0.1:1234/#token=abc" {
		t.Fatalf("info = %+v, %v", info, err)
	}
	st, err := os.Stat(filepath.Join(dir, instanceFileName))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("instance file mode = %v, %v", st.Mode().Perm(), err)
	}
	first.Release()
	again, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again.Release()
	if _, err := ReadRunningInstance(dir); err == nil {
		t.Error("address still published after release")
	}
}

func TestDataDirIsPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	got, err := DataDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(got)
	if err != nil || st.Mode().Perm() != 0o700 {
		t.Errorf("mode = %v, %v", st.Mode().Perm(), err)
	}
}
