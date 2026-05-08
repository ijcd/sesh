package engine

import (
	"context"
	"strings"
	"testing"
)

func TestRunHooks_Empty(t *testing.T) {
	if err := RunHooks(context.Background(), "pre", nil, "/tmp"); err != nil {
		t.Errorf("empty hooks should be no-op, got %v", err)
	}
}

func TestRunHooks_Success(t *testing.T) {
	ctx := context.Background()
	err := RunHooks(ctx, "pre", []string{"true", "echo ok"}, "/tmp")
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestRunHooks_FailureAborts(t *testing.T) {
	ctx := context.Background()
	err := RunHooks(ctx, "pre", []string{"true", "false", "true"}, "/tmp")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pre") || !strings.Contains(err.Error(), "false") {
		t.Errorf("error should name hook + line: %v", err)
	}
}
