package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/app"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetricstest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
)

const hostPath = "/api/genmonitor/v1/host"

type testApp struct {
	t       *testing.T
	handler http.Handler
	core    *coretest.FakeCore
}

func newTestApp(t *testing.T, snapshots hostmetrics.HostSnapshotSource) *testApp {
	t.Helper()
	core := coretest.NewFakeCore(t)
	handler := app.New(app.Dependencies{
		TokenVerifier: tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: core.JWKSURL(), Issuer: coretest.Issuer}),
		HostSnapshots: snapshots,
	})
	return &testApp{t: t, handler: handler, core: core}
}

// newSampledTestApp samples the baseline and busy readings once; the hour-long interval keeps that snapshot for the whole test.
func newSampledTestApp(t *testing.T) *testApp {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	sampler := hostmetrics.NewHostSampler(hostmetrics.HostSamplerConfig{
		Reader:           hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading(), hostmetricstest.BusyReading()),
		Interval:         time.Hour,
		FirstSampleDelay: time.Millisecond,
	})
	if err := sampler.Start(ctx); err != nil {
		t.Fatalf("start sampler: %v", err)
	}
	return newTestApp(t, sampler)
}

func (testApp *testApp) get(path string, authorization string) *httptest.ResponseRecorder {
	testApp.t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	testApp.handler.ServeHTTP(recorder, request)
	return recorder
}

func (testApp *testApp) bearer(permissions ...string) string {
	return "Bearer " + testApp.core.MintAccessToken(testApp.t, "ana-user-token", permissions...)
}

func decodeJSON(t *testing.T, data []byte) any {
	t.Helper()
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return decoded
}

func expectErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || recorder.Code != status || body.Error.Code != code {
		t.Errorf("status = %d, body = %s; want %d %s", recorder.Code, recorder.Body.Bytes(), status, code)
	}
}

func TestReadHostReturnsTheLatestSnapshot(t *testing.T) {
	testApp := newSampledTestApp(t)

	recorder := testApp.get(hostPath, testApp.bearer(access.PermissionHostRead))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.Bytes())
	}
	want, err := os.ReadFile("testdata/host_snapshot.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if got := decodeJSON(t, recorder.Body.Bytes()); !reflect.DeepEqual(got, decodeJSON(t, want)) {
		t.Errorf("body = %s", recorder.Body.Bytes())
	}
}

func TestReadHostRequiresAnAccessToken(t *testing.T) {
	expectErrorCode(t, newSampledTestApp(t).get(hostPath, ""), http.StatusUnauthorized, "invalid_token")
}

func TestReadHostRequiresTheHostReadPermission(t *testing.T) {
	testApp := newSampledTestApp(t)

	expectErrorCode(t, testApp.get(hostPath, testApp.bearer("gentasks.board.read")), http.StatusForbidden, "forbidden")
}

func TestReadHostIsUnavailableBeforeTheFirstSnapshot(t *testing.T) {
	reader := hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading())
	testApp := newTestApp(t, hostmetrics.NewHostSampler(hostmetrics.HostSamplerConfig{Reader: reader, Interval: time.Hour}))

	expectErrorCode(t, testApp.get(hostPath, testApp.bearer(access.PermissionHostRead)), http.StatusServiceUnavailable, "service_unavailable")
}

func TestHealthzIsPublic(t *testing.T) {
	if recorder := newSampledTestApp(t).get("/healthz", ""); recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Errorf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
}
