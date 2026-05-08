package main

import (
	"github.com/spf13/cobra"

	"github.com/ijcd/sesh/internal/engine"
)

func newValidateCmd(*engine.Engine) *cobra.Command {
	return &cobra.Command{Use: "validate", Hidden: true}
}
