package discovery

import (
	"net/http"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

const (
	ProductID = "genwork"
	// CoreAPIVersion is the semantic version of the contracts modules and clients consume (rule 61).
	CoreAPIVersion = "1.0.0"
)

func APIMajorVersion() string {
	major, _, _ := strings.Cut(CoreAPIVersion, ".")
	return "v" + major
}

type AuthInfo struct {
	Methods []string `json:"methods"`
}

type Document struct {
	Product string   `json:"product"`
	Name    string   `json:"name"`
	Version string   `json:"version"`
	API     string   `json:"api"`
	Auth    AuthInfo `json:"auth"`
}

type Handler struct {
	document Document
}

func NewHandler(instanceName, version string) *Handler {
	return &Handler{document: Document{
		Product: ProductID,
		Name:    instanceName,
		Version: version,
		API:     APIMajorVersion(),
		Auth:    AuthInfo{Methods: []string{"password"}},
	}}
}

func (handler *Handler) Register(router *platform.Router) {
	router.HandleFunc("GET /.well-known/genwork", platform.Public, handler.serveDocument)
}

func (handler *Handler) serveDocument(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	platform.WriteJSON(w, http.StatusOK, handler.document)
}
