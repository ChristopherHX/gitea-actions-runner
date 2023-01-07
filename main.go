package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"gitea.com/gitea/act_runner/cmd"
)

func withContextFunc(ctx context.Context, f func()) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(c)

		select {
		case <-ctx.Done():
		case <-c:
			cancel()
			f()
		}
	}()

	return ctx
}

func main() {
	ctx := withContextFunc(context.Background(), func() {})
	// run the command
	cmd.Execute(ctx)
}
