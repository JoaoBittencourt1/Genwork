package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "0.1.0"

const usage = `genwork-mcp connects AI assistants to a Genwork instance through the Model Context Protocol.

Usage:
  genwork-mcp login [--email <e-mail>]   sign in and keep the session in the OS secure store
  genwork-mcp logout                     end the session and forget it
  genwork-mcp status                     show who is signed in
  genwork-mcp serve                      run the MCP server on stdin/stdout (what assistants launch)
  genwork-mcp version                    print the version

Every command reads ` + platform.InstanceURLVariable + `, the address of the Genwork gateway.
`

var ErrUnknownCommand = errors.New("unknown command; run `genwork-mcp help`")

type Prompter interface {
	PromptLine(label string) (string, error)
	PromptPassword(label string) (string, error)
}

// Environment holds everything a command touches outside the process, so tests replace it whole.
type Environment struct {
	Getenv          platform.Getenv
	Stdout          io.Writer
	Stderr          io.Writer
	Prompter        Prompter
	CredentialStore auth.CredentialStore
	LockDirectory   string
	DeviceName      string
	HTTPClient      *http.Client
	ServerTransport mcp.Transport
}

func Run(ctx context.Context, args []string, environment Environment) error {
	if len(args) == 0 {
		fmt.Fprint(environment.Stderr, usage)
		return ErrUnknownCommand
	}
	switch args[0] {
	case "login":
		return RunLogin(ctx, args[1:], environment)
	case "logout":
		return RunLogout(ctx, environment)
	case "status":
		return RunStatus(environment)
	case "serve":
		return RunServe(ctx, environment)
	case "version":
		fmt.Fprintln(environment.Stdout, Version)
		return nil
	case "help", "-h", "--help":
		fmt.Fprint(environment.Stdout, usage)
		return nil
	default:
		fmt.Fprint(environment.Stderr, usage)
		return ErrUnknownCommand
	}
}

func newSessionService(config platform.Config, environment Environment) (*auth.SessionService, error) {
	if environment.LockDirectory == "" {
		return nil, errors.New("cannot locate the user cache directory for the refresh lock")
	}
	return auth.NewSessionService(auth.SessionServiceConfig{
		InstanceURL: config.InstanceURL,
		Client:      auth.NewSessionClient(platform.NewAPIClient(config.InstanceURL, environment.HTTPClient), environment.DeviceName),
		Store:       environment.CredentialStore,
		Lock:        auth.NewFileRefreshLock(refreshLockPath(environment.LockDirectory, config.InstanceURL)),
	}), nil
}

// refreshLockPath gives each instance its own lock, so sessions of different instances never wait on each other.
func refreshLockPath(lockDirectory, instanceURL string) string {
	digest := sha256.Sum256([]byte(instanceURL))
	return filepath.Join(lockDirectory, hex.EncodeToString(digest[:8])+".lock")
}
