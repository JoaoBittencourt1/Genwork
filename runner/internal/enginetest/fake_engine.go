// Package enginetest stubs the Docker Engine API on a unix socket for the runner's sandbox tests (rules 25 and 62).
package enginetest

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Container is what the stub remembers of one created container.
type Container struct {
	ID      string
	Request map[string]any
	Started bool
	Removed bool
	Stopped bool
}

type FakeEngine struct {
	socket     string
	server     *httptest.Server
	mutex      sync.Mutex
	containers map[string]*Container
	counter    int
	// Logs is what a container prints; ExitCode and OOMKilled shape how it ends.
	Logs      string
	ExitCode  int
	OOMKilled bool
	// HasImage says whether the image is already present; Pulled records pulls.
	HasImage bool
	Pulled   []string
	// Calls lists "METHOD /path" in order.
	Calls []string
	// Leftovers are containers a previous runner left behind, listed by run token.
	Leftovers map[string]string
	// SignInLeftovers are sign-in containers a previous runner left behind, listed by sign-in token.
	SignInLeftovers map[string]string
	// TerminalOutput is what an attached container prints at once; after it reads a line ending in a carriage return,
	// it prints TerminalReply and exits. TerminalInput records what was typed.
	TerminalOutput string
	TerminalReply  string
	TerminalInput  string
	// Resizes records each "WxH" a container's terminal was resized to.
	Resizes  []string
	attached chan struct{}
}

func NewFakeEngine(t testing.TB) *FakeEngine {
	t.Helper()
	directory, err := os.MkdirTemp("", "engine")
	if err != nil {
		t.Fatalf("create the socket directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	engine := &FakeEngine{
		socket: filepath.Join(directory, "engine.sock"), containers: map[string]*Container{}, HasImage: true, Leftovers: map[string]string{},
		SignInLeftovers: map[string]string{}, attached: make(chan struct{}),
	}
	listener, err := net.Listen("unix", engine.socket)
	if err != nil {
		t.Fatalf("listen on the engine socket: %v", err)
	}
	engine.server = httptest.NewUnstartedServer(http.HandlerFunc(engine.serve))
	engine.server.Listener = listener
	engine.server.Start()
	t.Cleanup(engine.server.Close)
	return engine
}

func (engine *FakeEngine) Socket() string { return engine.socket }

func (engine *FakeEngine) Container(id string) Container {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	return *engine.containers[id]
}

func (engine *FakeEngine) Containers() []Container {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	listed := make([]Container, 0, len(engine.containers))
	for _, container := range engine.containers {
		listed = append(listed, *container)
	}
	return listed
}

func (engine *FakeEngine) CallsSnapshot() []string {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	return append([]string(nil), engine.Calls...)
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func (engine *FakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	engine.mutex.Lock()
	engine.Calls = append(engine.Calls, r.Method+" "+r.URL.Path)
	engine.mutex.Unlock()
	path := r.URL.Path
	switch {
	case r.Method == "GET" && path == "/version":
		write(w, http.StatusOK, map[string]any{"Platform": map[string]any{"Name": "Docker Engine - Community"}, "Version": "27.0.1"})
	case r.Method == "GET" && strings.HasPrefix(path, "/images/") && strings.HasSuffix(path, "/json"):
		engine.serveImage(w)
	case r.Method == "POST" && path == "/images/create":
		engine.mutex.Lock()
		engine.Pulled = append(engine.Pulled, r.URL.Query().Get("fromImage"))
		engine.HasImage = true
		engine.mutex.Unlock()
		write(w, http.StatusOK, map[string]any{"status": "pulled"})
	case r.Method == "POST" && path == "/containers/create":
		engine.createContainer(w, r)
	case r.Method == "GET" && path == "/containers/json":
		engine.listContainers(w, r)
	case strings.HasPrefix(path, "/containers/"):
		engine.serveContainer(w, r)
	default:
		write(w, http.StatusNotFound, map[string]any{"message": "no such route " + path})
	}
}

func (engine *FakeEngine) serveImage(w http.ResponseWriter) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	if !engine.HasImage {
		write(w, http.StatusNotFound, map[string]any{"message": "no such image"})
		return
	}
	write(w, http.StatusOK, map[string]any{
		"Id": "sha256:image", "RepoDigests": []string{"registry.example/genwork-sandbox@sha256:abc123"},
		"Config": map[string]any{"Labels": map[string]string{"genwork.tool.claude": "2.1.0", "genwork.tool.codex": "0.30.0", "other": "x"}},
	})
}

func (engine *FakeEngine) createContainer(w http.ResponseWriter, r *http.Request) {
	var request map[string]any
	payload, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(payload, &request)
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	engine.counter++
	id := "container-" + string(rune('a'+engine.counter))
	engine.containers[id] = &Container{ID: id, Request: request}
	write(w, http.StatusCreated, map[string]any{"Id": id})
}

// listContainers honours the label filter, as the engine does.
func (engine *FakeEngine) listContainers(w http.ResponseWriter, r *http.Request) {
	var filters map[string][]string
	_ = json.Unmarshal([]byte(r.URL.Query().Get("filters")), &filters)
	wanted := map[string]bool{}
	for _, label := range filters["label"] {
		wanted[label] = true
	}
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	listed := []map[string]any{}
	for label, containers := range map[string]map[string]string{"genwork.run": engine.Leftovers, "genwork.signin": engine.SignInLeftovers} {
		if len(wanted) > 0 && !wanted[label] {
			continue
		}
		for token, id := range containers {
			listed = append(listed, map[string]any{"Id": id, "Labels": map[string]string{label: token}})
		}
	}
	write(w, http.StatusOK, listed)
}

func (engine *FakeEngine) serveContainer(w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/containers/"), "/")
	id := segments[0]
	engine.mutex.Lock()
	container, found := engine.containers[id]
	if !found {
		if _, leftover := engine.leftoverID(id); leftover && r.Method == "DELETE" {
			engine.mutex.Unlock()
			engine.removeLeftover(id)
			write(w, http.StatusNoContent, nil)
			return
		}
		engine.mutex.Unlock()
		write(w, http.StatusNotFound, map[string]any{"message": "no such container"})
		return
	}
	if len(segments) == 2 && segments[1] == "attach" {
		engine.mutex.Unlock()
		engine.attach(w)
		return
	}
	if len(segments) == 2 && segments[1] == "wait" && engine.TerminalOutput != "" {
		engine.mutex.Unlock()
		<-engine.attached
		engine.mutex.Lock()
	}
	defer engine.mutex.Unlock()
	switch {
	case r.Method == "DELETE":
		container.Removed = true
		w.WriteHeader(http.StatusNoContent)
	case len(segments) == 2 && segments[1] == "resize":
		engine.Resizes = append(engine.Resizes, r.URL.Query().Get("w")+"x"+r.URL.Query().Get("h"))
		w.WriteHeader(http.StatusOK)
	case len(segments) == 2 && segments[1] == "start":
		container.Started = true
		w.WriteHeader(http.StatusNoContent)
	case len(segments) == 2 && segments[1] == "stop":
		container.Stopped = true
		w.WriteHeader(http.StatusNoContent)
	case len(segments) == 2 && segments[1] == "logs":
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, engine.Logs)
	case len(segments) == 2 && segments[1] == "wait":
		write(w, http.StatusOK, map[string]any{"StatusCode": engine.ExitCode})
	case len(segments) == 2 && segments[1] == "json":
		write(w, http.StatusOK, map[string]any{"State": map[string]any{"OOMKilled": engine.OOMKilled}})
	default:
		write(w, http.StatusNotFound, map[string]any{"message": "no such route"})
	}
}

func (engine *FakeEngine) leftoverID(id string) (string, bool) {
	for _, containers := range []map[string]string{engine.Leftovers, engine.SignInLeftovers} {
		for token, leftover := range containers {
			if leftover == id {
				return token, true
			}
		}
	}
	return "", false
}

func (engine *FakeEngine) removeLeftover(id string) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	for _, containers := range []map[string]string{engine.Leftovers, engine.SignInLeftovers} {
		for token, leftover := range containers {
			if leftover == id {
				delete(containers, token)
			}
		}
	}
}

// attach upgrades to the raw terminal stream the way the engine does, plays TerminalOutput, reads one line, answers
// with TerminalReply, and ends the container.
func (engine *FakeEngine) attach(w http.ResponseWriter) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		write(w, http.StatusInternalServerError, map[string]any{"message": "no hijack"})
		return
	}
	connection, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer connection.Close()
	defer close(engine.attached)
	_, _ = buffered.WriteString("HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
	_, _ = buffered.WriteString(engine.TerminalOutput)
	_ = buffered.Flush()
	typed, err := buffered.ReadString('\r')
	engine.mutex.Lock()
	engine.TerminalInput = typed
	engine.mutex.Unlock()
	if err != nil {
		return
	}
	_, _ = buffered.WriteString(engine.TerminalReply)
	_ = buffered.Flush()
}

// Typed returns what was typed into the attached terminal.
func (engine *FakeEngine) Typed() string {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	return engine.TerminalInput
}
