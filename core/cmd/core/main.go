package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"

const usage = `usage: core <command>

commands:
  serve          run the HTTP API
  worker         run background jobs
  migrate        apply database migrations
  admin create   create an administrator (--email, --name, --password-stdin)
  keygen         print a new JWT signing key`

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] {
	case "serve":
		return runServe(ctx, os.Getenv)
	case "worker":
		return runWorker(ctx)
	case "migrate":
		return runMigrate(ctx, os.Getenv)
	case "admin":
		if len(args) < 2 || args[1] != "create" {
			return fmt.Errorf("%s", usage)
		}
		return runAdminCreate(ctx, os.Getenv, args[2:], stdin, stdout)
	case "keygen":
		return runKeygen(stdout)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}
