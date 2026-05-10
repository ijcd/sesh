// Package mock is a recording-driver used by engine tests.
package mock

import (
	"context"

	"github.com/ijcd/sesh/internal/drivers"
	"github.com/ijcd/sesh/internal/spec"
)

type Driver struct {
	NameVal          string
	UpCalls          []*spec.Project
	DownCalls        []string
	StatusVal        drivers.Status
	StatusErr        error
	UpErr            error
	DownErr          error
	DryRunVal        []string
	AttachCommandVal string
	AttachCommandErr error
}

func New(name string) *Driver {
	return &Driver{NameVal: name, StatusVal: drivers.StatusNotExists}
}

func (d *Driver) Name() string { return d.NameVal }

func (d *Driver) Up(ctx context.Context, p *spec.Project) error {
	d.UpCalls = append(d.UpCalls, p)
	return d.UpErr
}

func (d *Driver) Down(ctx context.Context, name string) error {
	d.DownCalls = append(d.DownCalls, name)
	return d.DownErr
}

func (d *Driver) Status(ctx context.Context, name string) (drivers.Status, error) {
	return d.StatusVal, d.StatusErr
}

func (d *Driver) Capture(ctx context.Context) (*spec.Project, error) { return nil, nil }
func (d *Driver) DryRun(p *spec.Project) ([]string, error)           { return d.DryRunVal, nil }

// AttachCommandVal is what AttachCommand returns. AttachCommandErr is the error.
// Set these in tests as needed; defaults are empty (returns "" and nil).
func (d *Driver) AttachCommand(p *spec.Project) (string, error) {
	return d.AttachCommandVal, d.AttachCommandErr
}
