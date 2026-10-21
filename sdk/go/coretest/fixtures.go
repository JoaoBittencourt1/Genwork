// Package coretest stubs core for module test suites, built from recorded contract fixtures (rules 25, 62).
package coretest

import (
	"embed"
	"fmt"
)

//go:embed fixtures/*.json
var fixtures embed.FS

// FixtureNames lists the recorded core responses; core's own tests compare its live responses against them.
var FixtureNames = []string{
	"jwks.json",
	"directory_user.json",
	"user_not_found.json",
	"capabilities.json",
	"registered_module.json",
	"incompatible_core_version.json",
}

func Fixture(name string) []byte {
	data, err := fixtures.ReadFile("fixtures/" + name)
	if err != nil {
		panic(fmt.Sprintf("unknown core fixture %q: %v", name, err))
	}
	return data
}
