package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	cases := map[string]string{
		"sent to 15551234567@s.whatsapp.net":      "sent to [redacted]@s.whatsapp.net",
		"lid 123456789012345:12@lid":              "lid [redacted]:12@lid",
		"phone +1 (212) 555-0123 failed":          "phone [redacted] failed",
		"time=2026-11-01T21:16:00.123-05:00":      "time=2026-11-01T21:16:00.123-05:00",
		"port 41234 ready":                        "port 41234 ready",
		"batch 3 of 12 took 4.5s":                 "batch 3 of 12 took 4.5s",
		"2026-11-01 time=12:30:01 level=INFO msg": "2026-11-01 time=12:30:01 level=INFO msg",
	}
	for in, want := range cases {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRotatingFileRedactsAndRotates(t *testing.T) {
	dir := t.TempDir()
	r, err := OpenRotatingFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	line := "number 447911123456 " + strings.Repeat("x", 1000) + "\n"
	for i := 0; i < 2*maxBytes/len(line); i++ {
		if _, err := r.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "logs", fileName+".1"))
	if err != nil {
		t.Fatalf("expected a rotated file: %v", err)
	}
	if strings.Contains(string(data), "447911123456") {
		t.Fatal("phone number reached the log file")
	}
	st, err := os.Stat(filepath.Join(dir, "logs", fileName))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("log mode = %v, want 0600", st.Mode().Perm())
	}
}
