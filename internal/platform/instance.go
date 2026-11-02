package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	lockFileName     = "instance.lock"
	instanceFileName = "instance.json"
)

// InstanceLock is the single-instance lock held by the running app.
type InstanceLock struct {
	file *os.File
	dir  string
}

// RunningInstance describes how to reach the app that holds the lock.
type RunningInstance struct {
	URL string `json:"url"`
	PID int    `json:"pid"`
}

// ErrAlreadyRunning is returned by AcquireInstanceLock when another copy of
// the app holds the lock.
var ErrAlreadyRunning = errors.New("another copy of the app is running")

// AcquireInstanceLock takes the exclusive per-user lock in dir. It returns
// ErrAlreadyRunning when another process holds it. The OS releases the lock
// automatically if the process dies, so a crash never leaves a stale lock.
func AcquireInstanceLock(dir string) (*InstanceLock, error) {
	f, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening instance lock: %w", err)
	}
	held, err := tryLock(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("locking instance lock: %w", err)
	}
	if !held {
		_ = f.Close()
		return nil, ErrAlreadyRunning
	}
	return &InstanceLock{file: f, dir: dir}, nil
}

// Publish records how other launches can reach this instance.
func (l *InstanceLock) Publish(info RunningInstance) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(l.dir, instanceFileName), data, 0o600)
}

// Release removes the published address and releases the lock.
func (l *InstanceLock) Release() {
	_ = os.Remove(filepath.Join(l.dir, instanceFileName))
	_ = unlock(l.file)
	_ = l.file.Close()
}

// ReadRunningInstance returns the address published by the lock holder.
func ReadRunningInstance(dir string) (RunningInstance, error) {
	var info RunningInstance
	data, err := os.ReadFile(filepath.Join(dir, instanceFileName))
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("reading %s: %w", instanceFileName, err)
	}
	if info.URL == "" {
		return info, fmt.Errorf("%s has no address", instanceFileName)
	}
	return info, nil
}

// WriteFileAtomic writes data to a temporary file in the same directory and
// renames it into place, so readers never see a half-written file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
