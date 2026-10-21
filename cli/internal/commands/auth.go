package commands

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const coreService = "core"

var (
	ErrPasswordsDoNotMatch = errors.New("passwords do not match")
	ErrStackNotRunning     = errors.New("the core API is not running; start it first with `genwork api` or `genwork up`")
)

type authOptions struct {
	email string
	name  string
}

func newAuthCommand(dependencies Dependencies) *cobra.Command {
	var options authOptions
	command := &cobra.Command{
		Use:   "auth",
		Short: "Create an administrator (asks for e-mail, name, and password)",
		RunE: func(command *cobra.Command, _ []string) error {
			return runAuth(command, dependencies, options)
		},
	}
	command.Flags().StringVar(&options.email, "email", "", "administrator e-mail (asked when omitted)")
	command.Flags().StringVar(&options.name, "name", "", "administrator name (asked when omitted)")
	return command
}

func runAuth(command *cobra.Command, dependencies Dependencies, options authOptions) error {
	workspace, err := locateWorkspace(dependencies)
	if err != nil {
		return err
	}
	if err := ensureCoreIsRunning(command, dependencies, workspace); err != nil {
		return err
	}
	email, err := askUntilFilled(dependencies.Prompter, "E-mail: ", options.email)
	if err != nil {
		return err
	}
	name, err := askUntilFilled(dependencies.Prompter, "Name: ", options.name)
	if err != nil {
		return err
	}
	password, err := promptNewPassword(dependencies.Prompter)
	if err != nil {
		return err
	}
	err = dependencies.Runner.Run(command.Context(), RunRequest{
		Name: "docker",
		Args: []string{
			"compose", "-f", workspace.ComposeFile(), "exec", "-T", coreService,
			"/core", "admin", "create", "--email", email, "--name", name, "--password-stdin",
		},
		Dir:    workspace.Root,
		Stdin:  strings.NewReader(password + "\n"),
		Stdout: command.OutOrStdout(),
		Stderr: command.ErrOrStderr(),
	})
	if err != nil {
		return ErrReported
	}
	return nil
}

func ensureCoreIsRunning(command *cobra.Command, dependencies Dependencies, workspace Workspace) error {
	running, err := runningServices(command, dependencies, workspace)
	if err != nil || !slices.Contains(running, coreService) {
		return ErrStackNotRunning
	}
	return nil
}

func askUntilFilled(prompter Prompter, label, preset string) (string, error) {
	value := strings.TrimSpace(preset)
	for value == "" {
		answer, err := prompter.PromptLine(label)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(answer)
	}
	return value, nil
}

func promptNewPassword(prompter Prompter) (string, error) {
	password, err := prompter.PromptPassword("Password: ")
	if err != nil {
		return "", err
	}
	confirmation, err := prompter.PromptPassword("Confirm password: ")
	if err != nil {
		return "", err
	}
	if password != confirmation {
		return "", ErrPasswordsDoNotMatch
	}
	if strings.ContainsAny(password, "\r\n") {
		return "", errors.New("password must not contain line breaks")
	}
	return password, nil
}
