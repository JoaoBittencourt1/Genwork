package commands

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
	"github.com/spf13/cobra"
)

var (
	ErrAlreadyInstalled    = errors.New("this directory already holds a Genwork installation; update it with `genwork update`")
	ErrInstallInRepository = errors.New("`genwork install` sets up a release installation; a repository checkout runs with `genwork up`")
	errInvalidPublicURL    = errors.New("the public URL must start with http:// or https://, e.g. https://genwork.example.com")
)

var smtpTLSModes = []string{"none", "starttls", "tls"}

type installOptions struct {
	publicURL    string
	instanceName string
	modules      []string
	fromCheckout string
}

func newInstallCommand(dependencies Dependencies) *cobra.Command {
	var options installOptions
	command := &cobra.Command{
		Use:   "install",
		Short: "Set up a server from the release in this directory: configuration, modules, and the published images",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runInstall(command, dependencies, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.publicURL, "public-url", "", "address people open in the browser, e.g. https://genwork.example.com (asked when omitted)")
	flags.StringVar(&options.instanceName, "instance-name", "", "name shown in the web app and e-mails (asked when omitted)")
	flags.StringSliceVar(&options.modules, "modules", nil, "modules to install, comma-separated (a checklist is shown when omitted)")
	flags.StringVar(&options.fromCheckout, "from-checkout", "", "repository checkout whose deploy/.env and module choice this installation takes over")
	return command
}

// runInstall never builds and never runs Node: the stack is the published images of this directory's VERSION (spec
// "Installing a server from a release").
func runInstall(command *cobra.Command, dependencies Dependencies, options installOptions) error {
	workspace, err := locateWorkspace(dependencies)
	if err != nil {
		return err
	}
	if workspace.Kind() == WorkspaceRepository {
		return ErrInstallInRepository
	}
	if isInstalled(workspace) {
		return ErrAlreadyInstalled
	}
	if options.fromCheckout != "" {
		if err := copyCheckoutConfiguration(options.fromCheckout, workspace); err != nil {
			return err
		}
	}
	if err := workspace.EnsureEnvFiles(command.ErrOrStderr()); err != nil {
		return err
	}
	if err := configureInstance(command, dependencies, workspace, options); err != nil {
		return err
	}
	setup, err := loadModuleSetup(command, dependencies, workspace)
	if err != nil {
		return err
	}
	if err := chooseInstalledModules(setup, options.modules); err != nil {
		return err
	}
	return startInstallation(setup)
}

func isInstalled(workspace Workspace) bool {
	for _, path := range []string{workspace.StackEnvFile(), workspace.InstallationFile()} {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

// copyCheckoutConfiguration takes over the secrets and module choice, so the release reuses the checkout's database volume.
func copyCheckoutConfiguration(checkoutRoot string, workspace Workspace) error {
	checkout := Workspace{Root: checkoutRoot}
	files := []struct {
		source, target string
		required       bool
	}{
		{checkout.StackEnvFile(), workspace.StackEnvFile(), true},
		{checkout.InstallationFile(), workspace.InstallationFile(), false},
	}
	for _, file := range files {
		content, err := os.ReadFile(file.source)
		if errors.Is(err, os.ErrNotExist) && !file.required {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", file.source, err)
		}
		if err := os.WriteFile(file.target, content, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", file.target, err)
		}
	}
	return nil
}

// configureInstance asks only for what a fresh installation lacks; a checkout's values are kept unless a flag replaces them.
func configureInstance(command *cobra.Command, dependencies Dependencies, workspace Workspace, options installOptions) error {
	values := map[string]string{}
	if options.publicURL != "" {
		publicURL, err := normalizePublicURL(options.publicURL)
		if err != nil {
			return err
		}
		values["GENWORK_PUBLIC_URL"] = publicURL
	}
	if options.instanceName != "" {
		values["GENWORK_INSTANCE_NAME"] = strings.TrimSpace(options.instanceName)
	}
	askable := options.fromCheckout == "" && dependencies.IsInteractive()
	if values["GENWORK_PUBLIC_URL"] == "" && options.fromCheckout == "" {
		if !dependencies.IsInteractive() {
			return errors.New("--public-url is required without a terminal")
		}
		publicURL, err := askPublicURL(command, dependencies.Prompter)
		if err != nil {
			return err
		}
		values["GENWORK_PUBLIC_URL"] = publicURL
	}
	if askable && options.instanceName == "" {
		current := workspace.StackEnvValue("GENWORK_INSTANCE_NAME")
		name, err := dependencies.Prompter.PromptLine(fmt.Sprintf("Instance name [%s]: ", current))
		if err != nil {
			return err
		}
		if name != "" {
			values["GENWORK_INSTANCE_NAME"] = name
		}
	}
	if askable {
		smtp, err := askSMTP(dependencies.Prompter)
		if err != nil {
			return err
		}
		for name, value := range smtp {
			values[name] = value
		}
	}
	if err := workspace.SetStackEnvValues(values); err != nil {
		return err
	}
	if workspace.StackEnvValue("SMTP_HOST") == "mailpit" {
		fmt.Fprintln(command.ErrOrStderr(), "Warning: SMTP_HOST is still mailpit, the development mail catcher, so no e-mail will be delivered. "+
			"Set the SMTP_* variables in deploy/.env and run `genwork up`.")
	}
	return nil
}

func askPublicURL(command *cobra.Command, prompter Prompter) (string, error) {
	for {
		answer, err := prompter.PromptLine("Public URL (e.g. https://genwork.example.com): ")
		if err != nil {
			return "", err
		}
		publicURL, err := normalizePublicURL(answer)
		if err == nil {
			return publicURL, nil
		}
		fmt.Fprintln(command.ErrOrStderr(), err)
	}
}

// normalizePublicURL accepts an absolute http(s) address and drops the trailing slash, since the token issuer must match exactly.
func normalizePublicURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errInvalidPublicURL
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

// askSMTP is optional: an empty host keeps the example's values and the warning that follows.
func askSMTP(prompter Prompter) (map[string]string, error) {
	host, err := prompter.PromptLine("SMTP host (leave empty to set it later in deploy/.env): ")
	if err != nil || host == "" {
		return nil, err
	}
	values := map[string]string{"SMTP_HOST": host}
	questions := []struct{ variable, label string }{
		{"SMTP_PORT", "SMTP port: "},
		{"SMTP_FROM", "Sender (e.g. Genwork <no-reply@example.com>): "},
	}
	for _, question := range questions {
		answer, err := askUntilFilled(prompter, question.label, "")
		if err != nil {
			return nil, err
		}
		values[question.variable] = answer
	}
	tls, err := askChoice(prompter, "SMTP TLS (none, starttls, tls): ", smtpTLSModes)
	if err != nil {
		return nil, err
	}
	values["SMTP_TLS"] = tls
	username, err := prompter.PromptLine("SMTP username (empty when the server needs none): ")
	if err != nil {
		return nil, err
	}
	values["SMTP_USERNAME"] = username
	if username != "" {
		password, err := prompter.PromptPassword("SMTP password: ")
		if err != nil {
			return nil, err
		}
		values["SMTP_PASSWORD"] = password
	}
	return values, nil
}

func askChoice(prompter Prompter, label string, choices []string) (string, error) {
	for {
		answer, err := askUntilFilled(prompter, label, "")
		if err != nil {
			return "", err
		}
		if slices.Contains(choices, answer) {
			return answer, nil
		}
	}
}

// chooseInstalledModules takes --modules when given; otherwise it shows the checklist once, as `genwork up` does.
func chooseInstalledModules(setup *moduleSetup, chosen []string) error {
	if setup.config.Chosen {
		return nil
	}
	if chosen == nil {
		return chooseModulesOnFirstRun(setup)
	}
	plan, err := modules.PlanSelection(setup.catalog, nil, chosen)
	if err != nil {
		return err
	}
	if !plan.IsEmpty() {
		fmt.Fprint(setup.command.ErrOrStderr(), setup.summarize(plan))
	}
	return setup.saveChoice(plan.Install)
}

// startInstallation pulls before starting anything, so a registry problem stops the install with nothing running.
func startInstallation(setup *moduleSetup) error {
	output := setup.command.ErrOrStderr()
	if err := setup.compose("pull"); err != nil {
		fmt.Fprintln(output, "Pulling the images failed. If the registry answered `denied`, the images of this release are not public yet.")
		return err
	}
	if err := setup.startDatabase(); err != nil {
		return err
	}
	if err := setup.writeGatewayConfig(); err != nil {
		return err
	}
	if err := setup.compose("up", "-d", "--wait"); err != nil {
		return err
	}
	for _, id := range setup.config.Modules {
		if err := setup.setModuleState(id, "enable"); err != nil {
			return err
		}
	}
	printStackEndpoints(setup.command, setup.workspace)
	fmt.Fprintf(output, "Genwork is running at %s. Create the first administrator with `genwork auth`.\n",
		setup.workspace.StackEnvValue("GENWORK_PUBLIC_URL"))
	fmt.Fprintf(output, "Run genwork from any directory with %s=%s.\n", genworkHomeVariable, setup.workspace.Root)
	return nil
}
