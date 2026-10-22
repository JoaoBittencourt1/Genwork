package packaging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/versionfile"
)

const usage = `usage: genwork-release <command> <repository root> <output directory>

commands:
  bundle          write genwork-bundle_<version>.tar.gz
  install-script  write install.sh from deploy/install.sh.tmpl`

// Run is what `make release-artifacts` calls; the version is the repository's VERSION (rule 50).
func Run(args []string, getenv func(string) string, stdout io.Writer) error {
	if len(args) != 3 {
		return errors.New(usage)
	}
	repositoryRoot, outputDirectory := args[1], args[2]
	switch args[0] {
	case "bundle":
		version, err := versionfile.Read(filepath.Join(repositoryRoot, "VERSION"))
		if err != nil {
			return err
		}
		destination := filepath.Join(outputDirectory, releases.BundleAsset(version))
		if err := WriteDeployBundle(repositoryRoot, destination); err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, destination)
		return err
	case "install-script":
		return writeInstallScript(repositoryRoot, outputDirectory, getenv, stdout)
	default:
		return errors.New(usage)
	}
}

func writeInstallScript(repositoryRoot, outputDirectory string, getenv func(string) string, stdout io.Writer) error {
	version, err := versionfile.Read(filepath.Join(repositoryRoot, "VERSION"))
	if err != nil {
		return err
	}
	scriptTemplate, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "install.sh.tmpl"))
	if err != nil {
		return fmt.Errorf("read the install.sh template: %w", err)
	}
	rendered, err := RenderInstallScript(scriptTemplate, version, getenv)
	if err != nil {
		return err
	}
	destination := filepath.Join(outputDirectory, "install.sh")
	if err := os.WriteFile(destination, rendered, 0o755); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	_, err = fmt.Fprintln(stdout, destination)
	return err
}
