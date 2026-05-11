package drivers

import (
	"context"
	"testing"
)

func TestSocketHint_RoundTrip(t *testing.T) {
	ctx := WithSocketHint(context.Background(), "unix:/tmp/kitty.sock")
	got, ok := SocketHintFromContext(ctx)
	if !ok {
		t.Fatal("SocketHintFromContext: ok = false, want true")
	}
	if got != "unix:/tmp/kitty.sock" {
		t.Errorf("got %q, want %q", got, "unix:/tmp/kitty.sock")
	}
}

func TestSocketHint_EmptyStringNotFound(t *testing.T) {
	ctx := WithSocketHint(context.Background(), "")
	_, ok := SocketHintFromContext(ctx)
	if ok {
		t.Error("SocketHintFromContext: ok = true for empty string, want false")
	}
}

func TestSocketHint_MissingKey(t *testing.T) {
	_, ok := SocketHintFromContext(context.Background())
	if ok {
		t.Error("SocketHintFromContext: ok = true on bare context, want false")
	}
}
