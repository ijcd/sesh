package main

import (
	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/engine"
)

func newDebugCmd(*engine.Engine) *cobra.Command { return &cobra.Command{Use: "debug", Hidden: true} }
func newCaptureCmd(*engine.Engine) *cobra.Command {
	return &cobra.Command{Use: "capture", Hidden: true}
}
func newLocalCmd(*engine.Engine) *cobra.Command { return &cobra.Command{Use: "local", Hidden: true} }
func newValidateCmd(*engine.Engine) *cobra.Command {
	return &cobra.Command{Use: "validate", Hidden: true}
}
