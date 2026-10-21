package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
)

func RunLogin(ctx context.Context, args []string, environment Environment) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	email := flags.String("email", "", "e-mail to sign in with (asked when omitted)")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("usage: genwork-mcp login [--email <e-mail>]: %w", err)
	}
	config, err := platform.LoadConfig(environment.Getenv)
	if err != nil {
		return err
	}
	sessions, err := newSessionService(config, environment)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*email) == "" {
		if *email, err = environment.Prompter.PromptLine("E-mail: "); err != nil {
			return err
		}
	}
	password, err := environment.Prompter.PromptPassword("Password: ")
	if err != nil {
		return err
	}
	if err := sessions.Login(ctx, *email, password); err != nil {
		return err
	}
	fmt.Fprintf(environment.Stdout, "Signed in to %s as %s.\n", config.InstanceURL, strings.TrimSpace(*email))
	return nil
}

func RunLogout(ctx context.Context, environment Environment) error {
	config, err := platform.LoadConfig(environment.Getenv)
	if err != nil {
		return err
	}
	sessions, err := newSessionService(config, environment)
	if err != nil {
		return err
	}
	if err := sessions.Logout(ctx); err != nil {
		return err
	}
	fmt.Fprintf(environment.Stdout, "Signed out of %s.\n", config.InstanceURL)
	return nil
}

func RunStatus(environment Environment) error {
	config, err := platform.LoadConfig(environment.Getenv)
	if err != nil {
		return err
	}
	sessions, err := newSessionService(config, environment)
	if err != nil {
		return err
	}
	email, err := sessions.SignedInEmail()
	if err != nil {
		return err
	}
	fmt.Fprintf(environment.Stdout, "Signed in to %s as %s.\n", config.InstanceURL, email)
	return nil
}
