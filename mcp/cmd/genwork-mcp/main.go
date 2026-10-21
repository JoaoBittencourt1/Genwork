package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/command"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := command.Run(ctx, os.Args[1:], command.NewSystemEnvironment()); err != nil {
		fmt.Fprintln(os.Stderr, "genwork-mcp:", err)
		os.Exit(1)
	}
}
