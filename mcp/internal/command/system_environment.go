package command

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/term"
)

func NewSystemEnvironment() Environment {
	return Environment{
		Getenv:          os.Getenv,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		Prompter:        NewTerminalPrompter(os.Stdin, os.Stderr),
		CredentialStore: auth.NewKeyringCredentialStore(),
		LockDirectory:   userLockDirectory(),
		DeviceName:      deviceName(),
		HTTPClient:      &http.Client{Timeout: platform.DefaultRequestTimeout},
		ServerTransport: &mcp.StdioTransport{},
	}
}

func userLockDirectory() string {
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDirectory, "genwork-mcp")
}

func deviceName() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "genwork-mcp"
	}
	return "genwork-mcp on " + hostname
}

// TerminalPrompter writes prompts to stderr, so stdout stays clean for scripts.
type TerminalPrompter struct {
	input  *os.File
	reader *bufio.Reader
	output io.Writer
}

func NewTerminalPrompter(input *os.File, output io.Writer) *TerminalPrompter {
	return &TerminalPrompter{input: input, reader: bufio.NewReader(input), output: output}
}

func (prompter *TerminalPrompter) PromptLine(label string) (string, error) {
	fmt.Fprint(prompter.output, label)
	line, err := prompter.reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read %s: %w", strings.TrimSuffix(label, ": "), err)
	}
	return strings.TrimSpace(line), nil
}

// PromptPassword hides the typed password on a terminal and reads one line when the password is piped in.
func (prompter *TerminalPrompter) PromptPassword(label string) (string, error) {
	descriptor := int(prompter.input.Fd())
	if !term.IsTerminal(descriptor) {
		line, err := prompter.reader.ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("read password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(prompter.output, label)
	password, err := term.ReadPassword(descriptor)
	fmt.Fprintln(prompter.output)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(password), nil
}
