package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/commands"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(execute(ctx, os.Args[1:], commands.DefaultDependencies(), os.Stdout, os.Stderr))
}

func execute(ctx context.Context, args []string, dependencies commands.Dependencies, stdout, stderr io.Writer) int {
	root := commands.NewRootCommand(dependencies)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		if !errors.Is(err, commands.ErrReported) {
			fmt.Fprintln(stderr, "error:", err)
		}
		return 1
	}
	return 0
}
