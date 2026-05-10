// Package state persists per-project runtime state to a JSON file.
// Currently used by the kitty driver's --launch flow to track which
// projects have a sesh-spawned kitty so we can clean up on `sesh down`.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const Version = 1

type LaunchEntry struct {
	Socket     string    `json:"socket"`
	Pid        int       `json:"pid"`
	LaunchedAt time.Time `json:"launched_at"`
}

type Store struct {
	Version  int                    `json:"version"`
	Projects map[string]LaunchEntry `json:"projects"`

	mu sync.Mutex
}

// Load reads state from path. Missing file → empty Store. Corrupted → error.
func Load(path string) (*Store, error) {
	s := &Store{Version: Version, Projects: map[string]LaunchEntry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.Projects == nil {
		s.Projects = map[string]LaunchEntry{}
	}
	return s, nil
}

func (s *Store) Get(name string) (LaunchEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.Projects[name]
	return e, ok
}

func (s *Store) Set(name string, e LaunchEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Projects[name] = e
}

func (s *Store) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Projects, name)
}

// Save writes the state to path atomically (via tmp + rename) under a
// flock held for the duration of write.
func (s *Store) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock %s: %w", lockPath, err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("flock %s: %w", lockPath, err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck

	s.mu.Lock()
	s.Version = Version
	data, err := json.MarshalIndent(s, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s → %s: %w", tmp, path, err)
	}
	return nil
}

// DefaultPath returns the standard state.json location for sesh.
func DefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "sesh", "state.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "sesh", "state.json"), nil
}
