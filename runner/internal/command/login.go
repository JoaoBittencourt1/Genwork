package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

func runAccountLogin(ctx context.Context, args []string, getenv config.Getenv, stdout io.Writer) error {
	flags := flag.NewFlagSet("account login", flag.ContinueOnError)
	directoryMode := flags.Bool("directory-mode", false, "run the sign-in on the host without a sandbox (development only)")
	flags.SetOutput(io.Discard)
	if err := flags.Parse(args); err != nil || flags.NArg() != 1 {
		return errors.New("usage: genwork-runner account login [--directory-mode] <account>")
	}
	settings, err := config.Load(getenv, Version, getenv("GENWORK_IMAGE_REGISTRY"))
	if err != nil {
		return err
	}
	client := api.NewClient(settings.InstanceURL, settings.RunnerSecret, nil)
	accountToken := flags.Arg(0)
	provider, err := findAssignedProvider(ctx, client, accountToken)
	if err != nil {
		return err
	}
	adapter, err := adapters.For(provider)
	if err != nil {
		return err
	}
	if adapter.SignIn() == nil {
		return fmt.Errorf("%s has no subscription sign-in", provider)
	}
	profileDir := settings.ProfileDir(accountToken)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("create the account profile: %w", err)
	}
	command, err := BuildLoginCommand(adapter, profileDir, settings, getenv, *directoryMode)
	if err != nil {
		return err
	}
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, stdout, os.Stderr
	runErr := command.Run()
	message := ""
	if runErr != nil {
		message = "the sign-in did not finish: " + runErr.Error()
	}
	if err := client.ReportAccountStatus(ctx, accountToken, runErr == nil, message); err != nil {
		return fmt.Errorf("report the sign-in: %w", err)
	}
	if runErr != nil {
		return errors.New(message)
	}
	_, err = fmt.Fprintln(stdout, "signed in; the account is ready")
	return err
}

func findAssignedProvider(ctx context.Context, client *api.Client, accountToken string) (string, error) {
	answer, err := client.SendHeartbeat(ctx, api.Heartbeat{MaxConcurrency: config.DefaultConcurrency})
	if err != nil {
		return "", err
	}
	for _, account := range answer.Accounts {
		if account.Token == accountToken {
			return account.Provider, nil
		}
	}
	return "", fmt.Errorf("this runner holds no account %q: link the account to this runner first", accountToken)
}

// BuildLoginCommand is the CLI's own sign-in, run interactively in a throwaway container that mounts only this account's
// profile, with the same isolation settings a run gets; directory mode runs it on the host instead. It is the terminal
// fallback to the sign-in started from the web app.
func BuildLoginCommand(adapter adapters.Adapter, profileDir string, settings config.Config, getenv config.Getenv, directoryMode bool) (*exec.Cmd, error) {
	signIn := adapter.SignIn()
	environment := signIn.Env
	if directoryMode {
		command := exec.Command(signIn.Command[0], signIn.Command[1:]...)
		command.Env = append(os.Environ(), flattenEnvironment(rebase(environment, adapter.ProfileMount(), profileDir))...)
		return command, nil
	}
	socket, err := sandbox.FindSocket(settings.EngineSocket, getenv)
	if err != nil {
		return nil, err
	}
	engine := "docker"
	if strings.Contains(socket, "podman") {
		engine = "podman"
	}
	if override := getenv("GENWORK_CONTAINER_CLI"); override != "" {
		engine = override
	}
	arguments := []string{"run", "--rm", "-it", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only",
		"--tmpfs", sandbox.HomeMount, "--tmpfs", "/tmp", "--user", fmt.Sprint(os.Getuid()) + ":" + fmt.Sprint(os.Getgid()),
		"-v", filepath.Clean(profileDir) + ":" + adapter.ProfileMount() + ":rw", "-e", "HOME=" + sandbox.HomeMount}
	for _, pair := range flattenEnvironment(environment) {
		arguments = append(arguments, "-e", pair)
	}
	arguments = append(arguments, settings.SandboxImage)
	arguments = append(arguments, signIn.Command...)
	return exec.Command(engine, arguments...), nil
}

func flattenEnvironment(environment map[string]string) []string {
	pairs := make([]string, 0, len(environment))
	for key, value := range environment {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}

// rebase points the CLI's profile variables at the host directory instead of the container path.
func rebase(environment map[string]string, mount, hostDir string) map[string]string {
	rebased := make(map[string]string, len(environment))
	for key, value := range environment {
		rebased[key] = strings.ReplaceAll(value, mount, hostDir)
	}
	return rebased
}
