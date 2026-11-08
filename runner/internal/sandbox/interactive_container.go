package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Terminal size of a sign-in: wide enough that a CLI never wraps a long authorization URL across lines.
const (
	interactiveColumns = 500
	interactiveRows    = 50
)

// StartInteractive runs the spec in a container with a terminal and an open stdin. It attaches before starting, so no
// output is lost, through the engine's raw-stream upgrade; the container gets the same isolation as a run.
func (sandbox *ContainerSandbox) StartInteractive(ctx context.Context, spec Spec) (InteractiveSession, error) {
	if err := sandbox.ensureImage(ctx, spec.Image); err != nil {
		return nil, err
	}
	request := BuildContainerRequest(sandbox.fitLimits(ctx, spec))
	request.OpenStdin, request.StdinOnce, request.AttachStdin = true, true, true
	var created struct {
		ID string `json:"Id"`
	}
	query := url.Values{"name": {"genwork-signin-" + spec.SignInToken + "-" + strconv.FormatInt(nextContainerNumber(), 10)}}
	if _, err := sandbox.engine.do(ctx, "POST", "/containers/create", query, request, &created); err != nil {
		return nil, fmt.Errorf("create the sign-in sandbox: %w", err)
	}
	stream, err := sandbox.engine.attach(ctx, created.ID)
	if err != nil {
		sandbox.remove(created.ID)
		return nil, err
	}
	if _, err := sandbox.engine.do(ctx, "POST", "/containers/"+created.ID+"/start", nil, nil, nil); err != nil {
		_ = stream.Close()
		sandbox.remove(created.ID)
		return nil, fmt.Errorf("start the sign-in sandbox: %w", err)
	}
	resize := url.Values{"h": {strconv.Itoa(interactiveRows)}, "w": {strconv.Itoa(interactiveColumns)}}
	_, _ = sandbox.engine.do(ctx, "POST", "/containers/"+created.ID+"/resize", resize, nil, nil)
	return &containerSession{sandbox: sandbox, id: created.ID, stream: stream, ctx: ctx}, nil
}

type containerSession struct {
	sandbox *ContainerSandbox
	id      string
	stream  io.ReadWriteCloser
	ctx     context.Context
}

func (session *containerSession) Output() io.Reader { return session.stream }
func (session *containerSession) Input() io.Writer  { return session.stream }

func (session *containerSession) Wait() (Result, error) {
	var waited struct {
		StatusCode int `json:"StatusCode"`
	}
	if _, err := session.sandbox.engine.do(session.ctx, "POST", "/containers/"+session.id+"/wait", nil, nil, &waited); err != nil {
		return Result{}, fmt.Errorf("wait for the sign-in sandbox: %w", err)
	}
	return session.sandbox.collect(session.ctx, session.id, waited.StatusCode)
}

func (session *containerSession) Close() error {
	err := session.stream.Close()
	session.sandbox.remove(session.id)
	return err
}

// attach opens the container's raw terminal stream; with a TTY the engine sends output unframed.
func (client *engineClient) attach(ctx context.Context, id string) (io.ReadWriteCloser, error) {
	query := url.Values{"stream": {"1"}, "stdin": {"1"}, "stdout": {"1"}, "stderr": {"1"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, engineBaseURL+"/containers/"+id+"/attach?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build engine request: %w", err)
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "tcp")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoEngine, err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		defer response.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, &EngineError{Status: response.StatusCode, Message: string(message)}
	}
	stream, ok := response.Body.(io.ReadWriteCloser)
	if !ok {
		_ = response.Body.Close()
		return nil, errors.New("the container engine did not hand over the terminal stream")
	}
	return stream, nil
}
