package platform

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// ErrCoreUnavailable is core's user directory not answering; every feature that reads it reports this one error.
var ErrCoreUnavailable = httpapi.NewAPIError(http.StatusServiceUnavailable, "core_unavailable")
