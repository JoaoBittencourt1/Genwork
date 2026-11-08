package signin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

// States a runner reports; genagents knows the same set.
const (
	stateAwaitingUser = "awaiting_user"
	stateVerifying    = "verifying"
	stateSucceeded    = "succeeded"
	stateFailed       = "failed"
)

// signInSession is one CLI sign-in in progress.
type signInSession struct {
	loop    *Loop
	signIn  api.SignIn
	spec    *adapters.SignInSpec
	codes   chan string
	cancel  context.CancelFunc
	scanner *PromptScanner
	changed chan struct{}
	code    string
}

func newSignInSession(loop *Loop, signIn api.SignIn, spec *adapters.SignInSpec) *signInSession {
	return &signInSession{
		loop: loop, signIn: signIn, spec: spec, codes: make(chan string, 1), scanner: NewPromptScanner(spec.UserCodePattern), changed: make(chan struct{}, 1),
	}
}

// deliverCode hands the session the code a person pasted; a second delivery of the same sign-in is ignored.
func (session *signInSession) deliverCode(code string) {
	select {
	case session.codes <- code:
	default:
	}
}

// run drives the CLI until it ends, the sign-in expires, or genagents cancels it, and always reports the outcome.
func (session *signInSession) run(ctx context.Context) {
	ctx, cancel := context.WithDeadline(ctx, session.signIn.ExpiresAt)
	defer cancel()
	interactive, scratch, err := session.start(ctx)
	if err != nil {
		session.report(ctx, api.SignInProgress{State: stateFailed, Message: "the sign-in could not start: " + err.Error()})
		return
	}
	defer os.RemoveAll(scratch)
	defer interactive.Close()
	outcome := session.converse(ctx, interactive)
	if outcome != nil {
		session.report(ctx, *outcome)
	}
}

func (session *signInSession) start(ctx context.Context) (sandbox.InteractiveSession, string, error) {
	profileDir := session.loop.settings.ProfileDir(session.signIn.AccountToken)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, "", fmt.Errorf("create the account profile: %w", err)
	}
	scratch, err := os.MkdirTemp(session.loop.settings.ScratchDir, "signin-")
	if err != nil {
		return nil, "", fmt.Errorf("create the sign-in directory: %w", err)
	}
	adapter, _ := adapters.For(session.signIn.Provider)
	spec := sandbox.Spec{
		SignInToken: session.signIn.Token, Image: session.loop.settings.Image, Workspace: scratch,
		Profile: &sandbox.Profile{Dir: profileDir, Mount: adapter.ProfileMount()}, Env: session.spec.Env, Command: session.spec.Command,
		Limits: session.loop.settings.Limits,
	}
	interactive, err := session.loop.launcher.StartInteractive(ctx, spec)
	if err != nil {
		_ = os.RemoveAll(scratch)
		return nil, "", err
	}
	return interactive, scratch, nil
}

// converse relays the prompt, types the code, and waits for the CLI; it returns the final report, or nil when the sign-in
// ended on genagents' side and there is nothing left to say.
func (session *signInSession) converse(ctx context.Context, interactive sandbox.InteractiveSession) *api.SignInProgress {
	go session.readOutput(interactive.Output())
	exited := make(chan sandbox.Result, 1)
	waitErr := make(chan error, 1)
	go func() {
		result, err := interactive.Wait()
		waitErr <- err
		exited <- result
	}()
	reported := Prompt{}
	urlDeadline := time.NewTimer(session.loop.settings.URLTimeout)
	defer urlDeadline.Stop()
	var verifyDeadline <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return session.endedOnGenagentsSide(ctx)
		case <-session.changed:
			if prompt := session.scanner.Prompt(); prompt != reported && prompt.URL != "" {
				reported = prompt
				urlDeadline.Stop()
				session.report(ctx, api.SignInProgress{State: stateAwaitingUser, URL: prompt.URL, UserCode: prompt.UserCode, ExpectsCode: session.spec.ExpectsCode})
			}
		case <-urlDeadline.C:
			if reported.URL == "" {
				return &api.SignInProgress{State: stateFailed, Message: "the CLI printed no sign-in URL: " + session.scanner.Tail()}
			}
		case code := <-session.codes:
			session.code = code
			if _, err := io.WriteString(interactive.Input(), code+"\r"); err != nil {
				return &api.SignInProgress{State: stateFailed, Message: "the code could not be typed into the CLI"}
			}
			session.report(ctx, api.SignInProgress{State: stateVerifying})
			verifyDeadline = time.After(session.loop.settings.VerifyTimeout)
		case <-verifyDeadline:
			return &api.SignInProgress{State: stateFailed, Message: "the CLI did not accept the code: " + session.scanner.Tail(session.code)}
		case result := <-exited:
			return session.exitOutcome(result, <-waitErr)
		}
	}
}

func (session *signInSession) exitOutcome(result sandbox.Result, err error) *api.SignInProgress {
	session.scanner.Flush()
	switch {
	case err != nil:
		return &api.SignInProgress{State: stateFailed, Message: "the sign-in sandbox failed: " + err.Error()}
	case result.ExitCode != 0:
		return &api.SignInProgress{State: stateFailed, Message: fmt.Sprintf("the CLI exited with code %d: %s", result.ExitCode, session.scanner.Tail(session.code))}
	}
	return &api.SignInProgress{State: stateSucceeded}
}

// endedOnGenagentsSide reports an expiry the runner noticed first; a cancellation needs no report.
func (session *signInSession) endedOnGenagentsSide(ctx context.Context) *api.SignInProgress {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &api.SignInProgress{State: stateFailed, Message: "the sign-in expired before it was completed"}
	}
	return nil
}

func (session *signInSession) readOutput(output io.Reader) {
	buffer := make([]byte, 4096)
	for {
		read, err := output.Read(buffer)
		if read > 0 {
			_, _ = session.scanner.Write(buffer[:read])
			session.scanner.Flush()
			select {
			case session.changed <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

func (session *signInSession) report(ctx context.Context, progress api.SignInProgress) {
	if err := session.loop.api.ReportSignInProgress(context.WithoutCancel(ctx), session.signIn.Token, progress); err != nil {
		slog.Warn("report a sign-in", "signIn", session.signIn.Token, "state", progress.State, "err", err)
	}
}
