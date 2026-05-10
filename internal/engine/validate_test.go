package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/drivers/mock"
	"github.com/ijcd/sesh/internal/spec"
)

type validatingMock struct {
	*mock.Driver
	validateErrs []error
}

func (v *validatingMock) Validate(p *spec.Project) []error { return v.validateErrs }

func TestValidate_DriverErrorsBubble(t *testing.T) {
	md := mock.New("tmux")
	vm := &validatingMock{Driver: md, validateErrs: []error{errors.New("bad layout: tall is not a tmux layout")}}
	e := New()
	e.Register(vm)
	p := &spec.Project{Name: "x", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}}
	err := e.Validate(context.Background(), p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad layout") {
		t.Errorf("got %v", err)
	}
}

func TestValidate_NoErrorsWhenDriverHappy(t *testing.T) {
	md := mock.New("tmux")
	e := New()
	e.Register(md)
	p := &spec.Project{Name: "x", Driver: "tmux", Tabs: []spec.Tab{{Title: "shell"}}}
	if err := e.Validate(context.Background(), p); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ContainmentErrorBubbles(t *testing.T) {
	md := mock.New("tmux")
	e := New()
	e.Register(md)
	p := &spec.Project{Name: "x", Driver: "tmux", Tabs: []spec.Tab{{
		Title: "x", Driver: "kitty", Panes: []spec.Pane{{Title: "p", Cmd: "y"}},
	}}}
	if err := e.Validate(context.Background(), p); err == nil {
		t.Fatal("expected containment error")
	}
}

// Make sure mock satisfies drivers.Driver after adding Validate.
var _ drivers.Driver = (*validatingMock)(nil)
