package app

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type Dependencies struct {
	TokenVerifier httpapi.AccessTokenVerifier
	HostSnapshots hostmetrics.HostSnapshotSource
}

// New is the composition root of the genmonitor HTTP API.
func New(dependencies Dependencies) http.Handler {
	router := httpapi.NewRouter(dependencies.TokenVerifier)
	router.HandleFunc("GET /healthz", httpapi.Public, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	hostmetrics.NewHostMetricsHandler(dependencies.HostSnapshots).Register(router)
	return router
}
