package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	sandboxUser     = "10001:10001"
	toolLabelPrefix = "genwork.tool."
)

// ContainerSandbox runs each invocation in a disposable container: non-root, no capabilities, no privilege escalation,
// a read-only root file system with a temporary home and /tmp, resource limits, outbound-only networking, and only the
// run's workspace and its own account profile mounted.
type ContainerSandbox struct {
	engine   *engineClient
	capacity engineCapacity
	mutex    sync.Mutex
	// containers maps a run token to the container id it currently runs in.
	containers map[string]string
}

func NewContainerSandbox(socketPath string) *ContainerSandbox {
	return &ContainerSandbox{engine: newEngineClient(socketPath), containers: map[string]string{}}
}

// ContainerRequest is the body of POST /containers/create.
type ContainerRequest struct {
	Image        string            `json:"Image"`
	Cmd          []string          `json:"Cmd"`
	Env          []string          `json:"Env"`
	User         string            `json:"User"`
	WorkingDir   string            `json:"WorkingDir"`
	Labels       map[string]string `json:"Labels"`
	Tty          bool              `json:"Tty"`
	OpenStdin    bool              `json:"OpenStdin"`
	StdinOnce    bool              `json:"StdinOnce"`
	AttachStdin  bool              `json:"AttachStdin"`
	AttachStdout bool              `json:"AttachStdout"`
	AttachStderr bool              `json:"AttachStderr"`
	HostConfig   HostConfig        `json:"HostConfig"`
}

type HostConfig struct {
	Binds          []string          `json:"Binds"`
	CapDrop        []string          `json:"CapDrop"`
	SecurityOpt    []string          `json:"SecurityOpt"`
	ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
	Tmpfs          map[string]string `json:"Tmpfs"`
	Memory         int64             `json:"Memory"`
	MemorySwap     int64             `json:"MemorySwap"`
	NanoCPUs       int64             `json:"NanoCpus"`
	PidsLimit      int64             `json:"PidsLimit"`
	NetworkMode    string            `json:"NetworkMode"`
	Privileged     bool              `json:"Privileged"`
}

// BuildContainerRequest turns a spec into the container's definition; it is the one place the isolation settings live.
func BuildContainerRequest(spec Spec) ContainerRequest {
	binds := []string{spec.Workspace + ":" + WorkspaceMount + ":rw"}
	if spec.Profile != nil {
		binds = append(binds, spec.Profile.Dir+":"+spec.Profile.Mount+":rw")
	}
	env := []string{"HOME=" + HomeMount}
	keys := make([]string, 0, len(spec.Env))
	for key := range spec.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = append(env, key+"="+spec.Env[key])
	}
	labels := map[string]string{RunLabel: spec.RunToken}
	if spec.SignInToken != "" {
		labels = map[string]string{SignInLabel: spec.SignInToken}
	}
	return ContainerRequest{
		Image: spec.Image, Cmd: spec.Command, Env: env, User: containerUser(), WorkingDir: WorkspaceMount,
		Labels: labels, Tty: true, AttachStdout: true, AttachStderr: true,
		HostConfig: HostConfig{
			Binds: binds, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges"}, ReadonlyRootfs: true,
			Tmpfs:  map[string]string{"/tmp": "rw,nosuid,size=1g", HomeMount: "rw,nosuid,size=1g"},
			Memory: spec.Limits.Memory, MemorySwap: spec.Limits.Memory, NanoCPUs: int64(spec.Limits.CPUs * 1e9), PidsLimit: spec.Limits.PIDs,
			NetworkMode: "bridge",
		},
	}
}

// containerUser runs the sandbox as the runner's own unprivileged user, so the workspace files it writes belong to the
// runner; a runner started as root falls back to a fixed unprivileged user.
func containerUser() string {
	if uid := os.Getuid(); uid != 0 {
		return strconv.Itoa(uid) + ":" + strconv.Itoa(os.Getgid())
	}
	return sandboxUser
}

func (sandbox *ContainerSandbox) Run(ctx context.Context, spec Spec, output io.Writer) (Result, error) {
	if err := sandbox.ensureImage(ctx, spec.Image); err != nil {
		return Result{}, err
	}
	spec = sandbox.fitLimits(ctx, spec)
	var created struct {
		ID string `json:"Id"`
	}
	query := url.Values{"name": {"genwork-run-" + spec.RunToken + "-" + strconv.FormatInt(nextContainerNumber(), 10)}}
	if _, err := sandbox.engine.do(ctx, "POST", "/containers/create", query, BuildContainerRequest(spec), &created); err != nil {
		return Result{}, fmt.Errorf("create the sandbox: %w", err)
	}
	sandbox.remember(spec.RunToken, created.ID)
	defer sandbox.forget(spec.RunToken)
	defer sandbox.remove(created.ID)

	if _, err := sandbox.engine.do(ctx, "POST", "/containers/"+created.ID+"/start", nil, nil, nil); err != nil {
		return Result{}, fmt.Errorf("start the sandbox: %w", err)
	}
	streamed := make(chan error, 1)
	go func() {
		streamed <- sandbox.engine.stream(ctx, "GET", "/containers/"+created.ID+"/logs", url.Values{"follow": {"1"}, "stdout": {"1"}, "stderr": {"1"}}, output)
	}()
	var waited struct {
		StatusCode int `json:"StatusCode"`
	}
	if _, err := sandbox.engine.do(ctx, "POST", "/containers/"+created.ID+"/wait", nil, nil, &waited); err != nil {
		return Result{}, fmt.Errorf("wait for the sandbox: %w", err)
	}
	if err := <-streamed; err != nil && ctx.Err() == nil {
		return Result{}, err
	}
	return sandbox.collect(ctx, created.ID, waited.StatusCode)
}

func (sandbox *ContainerSandbox) collect(ctx context.Context, id string, exitCode int) (Result, error) {
	var inspected struct {
		State struct {
			OOMKilled bool `json:"OOMKilled"`
		} `json:"State"`
	}
	if _, err := sandbox.engine.do(context.WithoutCancel(ctx), "GET", "/containers/"+id+"/json", nil, nil, &inspected); err != nil {
		return Result{}, fmt.Errorf("inspect the sandbox: %w", err)
	}
	return Result{ExitCode: exitCode, OOMKilled: inspected.State.OOMKilled}, nil
}

var (
	containerCounter   int64
	containerCounterMu sync.Mutex
)

// nextContainerNumber keeps container names unique when one run starts several turns.
func nextContainerNumber() int64 {
	containerCounterMu.Lock()
	defer containerCounterMu.Unlock()
	containerCounter++
	return containerCounter
}

func (sandbox *ContainerSandbox) remember(runToken, id string) {
	sandbox.mutex.Lock()
	defer sandbox.mutex.Unlock()
	sandbox.containers[runToken] = id
}

func (sandbox *ContainerSandbox) forget(runToken string) {
	sandbox.mutex.Lock()
	defer sandbox.mutex.Unlock()
	delete(sandbox.containers, runToken)
}

// remove deletes the container even when the run's context is gone, so a cancelled run leaves nothing behind.
func (sandbox *ContainerSandbox) remove(id string) {
	_, _ = sandbox.engine.do(context.Background(), "DELETE", "/containers/"+id, url.Values{"force": {"1"}, "v": {"1"}}, nil, nil)
}

// Stop asks the container to stop and kills it after the grace period.
func (sandbox *ContainerSandbox) Stop(ctx context.Context, runToken string) error {
	sandbox.mutex.Lock()
	id, found := sandbox.containers[runToken]
	sandbox.mutex.Unlock()
	if !found {
		return nil
	}
	_, err := sandbox.engine.do(ctx, "POST", "/containers/"+id+"/stop", url.Values{"t": {strconv.Itoa(int(stopGracePeriod.Seconds()))}}, nil, nil)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("stop the sandbox: %w", err)
	}
	return nil
}

// RemoveLeftovers also removes every sign-in container: a sign-in never survives the runner, genagents expires it.
func (sandbox *ContainerSandbox) RemoveLeftovers(ctx context.Context, keep map[string]bool) ([]string, error) {
	signIns, err := sandbox.listLabelled(ctx, SignInLabel)
	if err != nil {
		return nil, err
	}
	for _, container := range signIns {
		sandbox.remove(container.ID)
	}
	listed, err := sandbox.listLabelled(ctx, RunLabel)
	if err != nil {
		return nil, err
	}
	var removedRuns []string
	for _, container := range listed {
		runToken := container.Labels[RunLabel]
		if keep[runToken] {
			continue
		}
		sandbox.remove(container.ID)
		removedRuns = append(removedRuns, runToken)
	}
	return removedRuns, nil
}

type labelledContainer struct {
	ID     string            `json:"Id"`
	Labels map[string]string `json:"Labels"`
}

func (sandbox *ContainerSandbox) listLabelled(ctx context.Context, label string) ([]labelledContainer, error) {
	filters, err := json.Marshal(map[string][]string{"label": {label}})
	if err != nil {
		return nil, err
	}
	var listed []labelledContainer
	if _, err := sandbox.engine.do(ctx, "GET", "/containers/json", url.Values{"all": {"1"}, "filters": {string(filters)}}, nil, &listed); err != nil {
		return nil, fmt.Errorf("list sandboxes: %w", err)
	}
	return listed, nil
}

// Describe reads the image the runner uses: its digest, and the provider CLIs and versions its labels declare.
func (sandbox *ContainerSandbox) Describe(ctx context.Context, image string) (Info, error) {
	info := Info{Engine: "container engine", Tools: map[string]string{}}
	var versionBody struct {
		Platform struct {
			Name string `json:"Name"`
		} `json:"Platform"`
		Version string `json:"Version"`
	}
	if _, err := sandbox.engine.do(ctx, "GET", "/version", nil, nil, &versionBody); err != nil {
		return Info{}, err
	}
	info.Engine = strings.TrimSpace(versionBody.Platform.Name + " " + versionBody.Version)
	if err := sandbox.ensureImage(ctx, image); err != nil {
		return Info{}, err
	}
	var inspected struct {
		ID          string   `json:"Id"`
		RepoDigests []string `json:"RepoDigests"`
		Config      struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if _, err := sandbox.engine.do(ctx, "GET", "/images/"+url.PathEscape(image)+"/json", nil, nil, &inspected); err != nil {
		return Info{}, fmt.Errorf("inspect the sandbox image: %w", err)
	}
	info.ImageDigest = inspected.ID
	if len(inspected.RepoDigests) > 0 {
		info.ImageDigest = inspected.RepoDigests[0]
	}
	for label, value := range inspected.Config.Labels {
		if tool, found := strings.CutPrefix(label, toolLabelPrefix); found {
			info.Tools[tool] = value
		}
	}
	return info, nil
}

// ensureImage pulls the image when the engine does not have it yet.
func (sandbox *ContainerSandbox) ensureImage(ctx context.Context, image string) error {
	_, err := sandbox.engine.do(ctx, "GET", "/images/"+url.PathEscape(image)+"/json", nil, nil, nil)
	if err == nil || !isNotFound(err) {
		return err
	}
	if _, err := sandbox.engine.do(ctx, "POST", "/images/create", url.Values{"fromImage": {image}}, nil, nil); err != nil {
		return fmt.Errorf("pull the sandbox image: %w", err)
	}
	return nil
}
