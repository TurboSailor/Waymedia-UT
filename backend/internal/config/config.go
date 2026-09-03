// Package config holds the daemon's settings on disk and hands changes to
// whoever is running.
//
// The file is the only channel between the QML front end (through the HTTP
// API) and the running daemon, so a write has to be atomic: the daemon may be
// reading it at any moment and a half-written file is indistinguishable from a
// corrupted one on the next start.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Bounds on the poll intervals. Faster than twice a second only burns battery
// in the container; slower than a minute stops being a live bridge.
const (
	MinPollMS = 500
	MaxPollMS = 60000
)

// Settings is the persisted configuration.
type Settings struct {
	// Enabled owns the whole bridge: with it off the daemon releases the MPRIS
	// bus name (the player disappears from the sound menu) and stops polling
	// the container, which is exactly what a user turning it off asks for.
	Enabled bool `json:"enabled"`
	// PollMS is the interval while something is playing; IdlePollMS is used
	// otherwise, when only "did playback start" needs noticing.
	PollMS     int `json:"poll_ms"`
	IdlePollMS int `json:"idle_poll_ms"`
}

func Defaults() Settings {
	return Settings{Enabled: true, PollMS: 1500, IdlePollMS: 5000}
}

// Normalize clamps instead of rejecting: the UI is the only writer, and a
// rejected save would leave it out of sync with what the daemon uses.
func (s *Settings) Normalize() {
	s.PollMS = clamp(s.PollMS, MinPollMS, MaxPollMS)
	s.IdlePollMS = clamp(s.IdlePollMS, MinPollMS, MaxPollMS)
	if s.IdlePollMS < s.PollMS {
		s.IdlePollMS = s.PollMS
	}
}

func (s Settings) Poll() time.Duration     { return time.Duration(s.PollMS) * time.Millisecond }
func (s Settings) IdlePoll() time.Duration { return time.Duration(s.IdlePollMS) * time.Millisecond }

func clamp(v, lo, hi int) int {
	switch {
	case v < lo:
		return lo
	case v > hi:
		return hi
	default:
		return v
	}
}

// Store is the settings file. Changes are applied by the caller that writes
// them (the API handler), so there is no subscription machinery here.
type Store struct {
	path string

	mu  sync.Mutex
	cur Settings
}

// Load reads the settings file, creating it with the defaults if absent.
//
// An unparseable file is replaced by the defaults rather than being fatal: the
// daemon exists to bridge playback, and refusing to start over a stray byte
// would need a terminal to fix.
func Load(path string) (*Store, error) {
	s := &Store{path: path, cur: Defaults()}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		v := Defaults()
		if json.Unmarshal(raw, &v) == nil {
			v.Normalize()
			s.cur = v
		}
	case !os.IsNotExist(err):
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("config: create %s: %w", filepath.Dir(path), err)
	}
	if err := s.write(s.cur); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

// Set normalizes and persists new settings.
func (s *Store) Set(v Settings) error {
	v.Normalize()

	s.mu.Lock()
	defer s.mu.Unlock()
	if v == s.cur {
		return nil
	}
	if err := s.write(v); err != nil {
		return err
	}
	s.cur = v
	return nil
}

// write persists settings atomically. The temporary file has to live in the
// target's directory, or the rename crosses a filesystem boundary and fails.
func (s *Store) write(v Settings) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".config-*.json")
	if err != nil {
		return fmt.Errorf("config: temp file: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("config: write %s: %w", name, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("config: sync %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: close %s: %w", name, err)
	}
	if err := os.Rename(name, s.path); err != nil {
		return fmt.Errorf("config: rename onto %s: %w", s.path, err)
	}
	return nil
}
