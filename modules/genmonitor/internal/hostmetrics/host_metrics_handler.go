package hostmetrics

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type HostMetricsHandler struct {
	snapshots HostSnapshotSource
}

func NewHostMetricsHandler(snapshots HostSnapshotSource) *HostMetricsHandler {
	return &HostMetricsHandler{snapshots: snapshots}
}

func (handler *HostMetricsHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/genmonitor/v1/host", httpapi.RequirePermission(access.PermissionHostRead), handler.readHost)
}

func (handler *HostMetricsHandler) readHost(w http.ResponseWriter, r *http.Request) {
	snapshot, found := handler.snapshots.LatestSnapshot()
	if !found {
		httpapi.WriteError(w, r, httpapi.ErrServiceUnavailable)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewHostSnapshotResponse(snapshot, SampleInterval))
}
