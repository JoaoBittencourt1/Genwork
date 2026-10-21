package modules

import (
	"bytes"
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
)

//go:embed templates/Caddyfile.tmpl
var caddyfileTemplate string

var caddyfile = template.Must(template.New("Caddyfile").Parse(caddyfileTemplate))

type gatewayRoute struct {
	APIPrefix        string
	UpstreamVariable string
}

// RenderCaddyfile routes core plus every installed module; upstream addresses stay in .env (rule 1).
func RenderCaddyfile(installed []manifest.ModuleManifest) ([]byte, error) {
	sorted := slices.Clone(installed)
	slices.SortFunc(sorted, func(left, right manifest.ModuleManifest) int { return strings.Compare(left.ID, right.ID) })
	routes := make([]gatewayRoute, 0, len(sorted))
	for _, moduleManifest := range sorted {
		routes = append(routes, gatewayRoute{APIPrefix: moduleManifest.APIPrefix, UpstreamVariable: ConventionsFor(moduleManifest.ID).UpstreamVariable})
	}
	var rendered bytes.Buffer
	if err := caddyfile.Execute(&rendered, routes); err != nil {
		return nil, fmt.Errorf("render Caddyfile: %w", err)
	}
	return rendered.Bytes(), nil
}
