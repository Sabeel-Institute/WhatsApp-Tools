// Package settings stores the user's preferences: the default country for
// phone numbers and the column choices remembered per spreadsheet layout.
package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/platform"
)

// Mapping is a remembered choice of columns, by header text so it survives
// columns being reordered.
type Mapping struct {
	Phone    string `json:"phone"`
	NameMode string `json:"nameMode"` // "none", "full" or "split"
	Name     string `json:"name,omitempty"`
	First    string `json:"first,omitempty"`
	Last     string `json:"last,omitempty"`
}

// Settings is everything persisted.
type Settings struct {
	DefaultCountry string             `json:"defaultCountry"`
	Mappings       map[string]Mapping `json:"mappings"`
	// InviteTemplate is the last message used for invites; empty means the
	// workflow's default.
	InviteTemplate string `json:"inviteTemplate,omitempty"`
}

// Store reads and writes settings.json.
type Store struct {
	path string
	mu   sync.Mutex
	s    Settings
}

// Open loads dataDir/settings.json, starting with defaults if it is absent
// or unreadable.
func Open(dataDir string) (*Store, error) {
	st := &Store{path: filepath.Join(dataDir, "settings.json"), s: defaults()}
	data, err := os.ReadFile(st.path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	} else if err != nil {
		return nil, err
	}
	var s Settings
	if json.Unmarshal(data, &s) == nil {
		if s.DefaultCountry == "" {
			s.DefaultCountry = "US"
		}
		if s.Mappings == nil {
			s.Mappings = map[string]Mapping{}
		}
		st.s = s
	}
	return st, nil
}

func defaults() Settings {
	return Settings{DefaultCountry: "US", Mappings: map[string]Mapping{}}
}

// Get returns a copy of the settings.
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.s
	s.Mappings = make(map[string]Mapping, len(st.s.Mappings))
	for k, v := range st.s.Mappings {
		s.Mappings[k] = v
	}
	return s
}

// Update applies f and saves.
func (st *Store) Update(f func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.s
	next.Mappings = make(map[string]Mapping, len(st.s.Mappings))
	for k, v := range st.s.Mappings {
		next.Mappings[k] = v
	}
	f(&next)
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := platform.WriteFileAtomic(st.path, data, 0o600); err != nil {
		return err
	}
	st.s = next
	return nil
}

// HeaderSignature identifies a spreadsheet layout by its set of headers,
// ignoring order, case and punctuation.
func HeaderSignature(headers []string) string {
	norm := make([]string, 0, len(headers))
	for _, h := range headers {
		var b strings.Builder
		for _, r := range strings.ToLower(h) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			}
		}
		norm = append(norm, b.String())
	}
	slices.Sort(norm)
	sum := sha256.Sum256([]byte(strings.Join(norm, "\x00")))
	return hex.EncodeToString(sum[:12])
}
