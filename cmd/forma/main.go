package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hossein1376/forma/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	code := cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
