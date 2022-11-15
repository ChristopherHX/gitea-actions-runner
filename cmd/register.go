package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// runRegister registers a runner to the server
func runRegister(ctx context.Context, regArgs *registerArgs) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Infoln("Starting register to gitea instance")
		return nil
	}
}

// registerArgs represents the arguments for register command
type registerArgs struct {
	NoInteractive bool
	InstanceAddr  string
	Token         string
}
