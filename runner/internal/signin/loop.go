package signin

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const (
	DefaultPollInterval  = 3 * time.Second
	DefaultURLTimeout    = 90 * time.Second
	DefaultVerifyTimeout = 90 * time.Second
	requestedState       = "requested"
)

// SignInAPI is what the loop needs from genagents (rule 9).
type SignInAPI interface {
	ListSignIns(ctx context.Context) ([]api.SignIn, error)
	ReportSignInProgress(ctx context.Context, signInToken string, progress api.SignInProgress) error
}

type Settings struct {
	Image  string
	Limits sandbox.Limits
	// ProfileDir resolves an account's isolated sign-in profile directory.
	ProfileDir func(accountToken string) string
	// ScratchDir holds the empty working directory each sign-in sandbox gets.
	ScratchDir    string
	PollInterval  time.Duration
	URLTimeout    time.Duration
	VerifyTimeout time.Duration
}

// Loop polls genagents for the sign-ins of the accounts this runner holds and runs one session per sign-in.
type Loop struct {
	api      SignInAPI
	launcher sandbox.InteractiveLauncher
	settings Settings
	mutex    sync.Mutex
	sessions map[string]*signInSession
	wait     sync.WaitGroup
}

func NewLoop(signInAPI SignInAPI, launcher sandbox.InteractiveLauncher, settings Settings) *Loop {
	if settings.PollInterval == 0 {
		settings.PollInterval = DefaultPollInterval
	}
	if settings.URLTimeout == 0 {
		settings.URLTimeout = DefaultURLTimeout
	}
	if settings.VerifyTimeout == 0 {
		settings.VerifyTimeout = DefaultVerifyTimeout
	}
	return &Loop{api: signInAPI, launcher: launcher, settings: settings, sessions: map[string]*signInSession{}}
}

// Run polls until the context ends, then stops every session.
func (loop *Loop) Run(ctx context.Context) error {
	ticker := time.NewTicker(loop.settings.PollInterval)
	defer ticker.Stop()
	defer loop.wait.Wait()
	for {
		if err := loop.Poll(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("poll sign-ins", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Poll starts the requested sign-ins, hands pasted codes to their sessions, and stops the sessions genagents ended.
func (loop *Loop) Poll(ctx context.Context) error {
	listed, err := loop.api.ListSignIns(ctx)
	if err != nil {
		return err
	}
	active := make(map[string]bool, len(listed))
	for _, signIn := range listed {
		active[signIn.Token] = true
		loop.follow(ctx, signIn)
	}
	loop.mutex.Lock()
	defer loop.mutex.Unlock()
	for token, session := range loop.sessions {
		if !active[token] {
			session.cancel()
		}
	}
	return nil
}

func (loop *Loop) follow(ctx context.Context, signIn api.SignIn) {
	loop.mutex.Lock()
	session, running := loop.sessions[signIn.Token]
	loop.mutex.Unlock()
	switch {
	case running && signIn.Code != "":
		session.deliverCode(signIn.Code)
	case running:
	case signIn.State == requestedState:
		loop.start(ctx, signIn)
	default:
		message := "the runner restarted during the sign-in; start it again"
		if err := loop.api.ReportSignInProgress(ctx, signIn.Token, api.SignInProgress{State: stateFailed, Message: message}); err != nil {
			slog.Warn("report an orphaned sign-in", "signIn", signIn.Token, "err", err)
		}
	}
}

func (loop *Loop) start(ctx context.Context, signIn api.SignIn) {
	adapter, err := adapters.For(signIn.Provider)
	if err != nil || adapter.SignIn() == nil {
		message := "this provider has no subscription sign-in"
		if reportErr := loop.api.ReportSignInProgress(ctx, signIn.Token, api.SignInProgress{State: stateFailed, Message: message}); reportErr != nil {
			slog.Warn("report a sign-in", "signIn", signIn.Token, "err", reportErr)
		}
		return
	}
	session := newSignInSession(loop, signIn, adapter.SignIn())
	sessionCtx, cancel := context.WithCancel(ctx)
	session.cancel = cancel
	loop.mutex.Lock()
	loop.sessions[signIn.Token] = session
	loop.mutex.Unlock()
	loop.wait.Add(1)
	go func() {
		defer loop.wait.Done()
		defer cancel()
		session.run(sessionCtx)
		loop.mutex.Lock()
		delete(loop.sessions, signIn.Token)
		loop.mutex.Unlock()
	}()
}
