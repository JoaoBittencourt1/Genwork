// Package gentasks embeds the module manifest, the single source of truth for its registration facts (rule 51).
package gentasks

import _ "embed"

//go:embed module.yaml
var ManifestYAML []byte
