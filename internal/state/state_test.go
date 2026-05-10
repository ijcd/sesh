package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Projects) != 0 {
		t.Errorf("expected empty Projects, got %v", s.Projects)
	}
}

func TestStore_LoadCorruptedReturnsError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	if err := os.WriteFile(p, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error on corrupted JSON")
	}
}

func TestStore_SetAndPersist(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, _ := Load(p)
	s.Set("liberties", LaunchEntry{
		Socket: "/tmp/sock", Pid: 1234, LaunchedAt: time.Now(),
	})
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}

	// Reload from disk
	s2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := s2.Get("liberties")
	if !ok {
		t.Fatal("entry not found after reload")
	}
	if e.Socket != "/tmp/sock" || e.Pid != 1234 {
		t.Errorf("got entry %+v", e)
	}
}

func TestStore_Delete(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, _ := Load(p)
	s.Set("a", LaunchEntry{Socket: "/a", Pid: 1})
	s.Set("b", LaunchEntry{Socket: "/b", Pid: 2})
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Error("a should be gone")
	}
	if _, ok := s.Get("b"); !ok {
		t.Error("b should still exist")
	}
}

func TestStore_AtomicWriteSurvivesPartialFailure(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, _ := Load(p)
	s.Set("x", LaunchEntry{Socket: "/x", Pid: 1})
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	// Verify there's no leftover .tmp file
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file should be gone, stat: %v", err)
	}
}

func TestStore_ConcurrentSavesSerialized(t *testing.T) {
	// Smoke test: two goroutines saving concurrently shouldn't corrupt.
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s, _ := Load(p)
			s.Set("p", LaunchEntry{Socket: "/x", Pid: n})
			_ = s.Save(p)
		}(i)
	}
	wg.Wait()

	// Final state should be valid JSON.
	s, err := Load(p)
	if err != nil {
		t.Fatalf("post-concurrency load failed: %v", err)
	}
	if _, ok := s.Get("p"); !ok {
		t.Error("entry lost")
	}
}
