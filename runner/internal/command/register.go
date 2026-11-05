package command

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/registration"
)

func runRegister(ctx context.Context, args []string, getenv config.Getenv, stdout io.Writer) error {
	flags := flag.NewFlagSet("register", flag.ContinueOnError)
	instance := flags.String("instance", getenv("GENWORK_INSTANCE_URL"), "address of the Genwork instance")
	token := flags.String("token", getenv("GENWORK_REGISTRATION_TOKEN"), "registration token shown when the runner was created")
	flags.SetOutput(io.Discard)
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	home, err := config.Home(getenv)
	if err != nil {
		return err
	}
	if err := registration.Register(ctx, home, *instance, *token, nil); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	_, err = fmt.Fprintf(stdout, "registered; the runner's secret is stored in %s with owner-only permissions\n", home)
	return err
}
