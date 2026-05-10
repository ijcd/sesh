package launch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWaitForSocket_AppearsInTime(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")
	go func() {
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(sock, []byte{}, 0o644)
	}()
	if err := WaitForSocket(context.Background(), sock, 1*time.Second); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWaitForSocket_TimesOut(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "missing.sock")
	err := WaitForSocket(context.Background(), sock, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestWaitForSocket_PreExisting(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "exists.sock")
	if err := os.WriteFile(sock, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WaitForSocket(context.Background(), sock, 50*time.Millisecond); err != nil {
		t.Errorf("expected pre-existing socket to succeed, got %v", err)
	}
}

func TestSocketPathFor(t *testing.T) {
	dir := t.TempDir()
	p, err := SocketPathFor("liberties", dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "sockets", "liberties.sock")
	if p != want {
		t.Errorf("got %q, want %q", p, want)
	}
}
