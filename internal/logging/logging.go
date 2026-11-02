// Package logging writes the app's log to a size-rotated file in the data
// directory, with every long run of digits redacted so phone numbers (and the
// WhatsApp IDs built from them) never reach the disk.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const (
	fileName = "app.log"
	maxBytes = 5 << 20 // rotate at 5 MB
	keep     = 3       // app.log.1 .. app.log.3
)

// digits matches any run of 7 or more digits (WhatsApp IDs embed the phone
// number as one such run) and anything written in international format with
// separators. Seven digits is shorter than any real phone number and longer
// than ports, counts and durations; dates and times have shorter runs.
var digits = regexp.MustCompile(`\+\d[\d\s().-]{5,}\d|\d{7,}`)

// Redact replaces anything that could be a phone number.
func Redact(s string) string {
	return digits.ReplaceAllStringFunc(s, func(m string) string {
		n := 0
		for _, r := range m {
			if r >= '0' && r <= '9' {
				n++
			}
		}
		if n < 7 {
			return m
		}
		return "[redacted]"
	})
}

// RotatingFile is an io.Writer that rotates the log at maxBytes.
type RotatingFile struct {
	mu   sync.Mutex
	dir  string
	f    *os.File
	size int64
}

// OpenRotatingFile opens (or creates) the log in dir/logs.
func OpenRotatingFile(dir string) (*RotatingFile, error) {
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, err
	}
	r := &RotatingFile{dir: logDir}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RotatingFile) open() error {
	f, err := os.OpenFile(filepath.Join(r.dir, fileName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	r.f, r.size = f, st.Size()
	return nil
}

func (r *RotatingFile) rotate() error {
	_ = r.f.Close()
	base := filepath.Join(r.dir, fileName)
	_ = os.Remove(fmt.Sprintf("%s.%d", base, keep))
	for i := keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", base, i), fmt.Sprintf("%s.%d", base, i+1))
	}
	_ = os.Rename(base, base+".1")
	return r.open()
}

// Write redacts p and appends it to the log, rotating first when full.
func (r *RotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	clean := []byte(Redact(string(p)))
	if r.size+int64(len(clean)) > maxBytes {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := r.f.Write(clean)
	r.size += int64(n)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the current file.
func (r *RotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}

// redactingWriter redacts before forwarding; used for stderr in development.
type redactingWriter struct{ w io.Writer }

func (rw redactingWriter) Write(p []byte) (int, error) {
	if _, err := rw.w.Write([]byte(Redact(string(p)))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// New returns a logger writing to file and, when alsoStderr is set, to the
// terminal as well. Both outputs are redacted.
func New(file *RotatingFile, alsoStderr bool, level slog.Level) *slog.Logger {
	var w io.Writer = file
	if alsoStderr {
		w = io.MultiWriter(file, redactingWriter{os.Stderr})
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
