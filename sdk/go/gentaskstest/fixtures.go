// Package gentaskstest holds gentasks responses recorded for other modules' test suites: genagents stubs gentasks'
// public API with them, and gentasks' own tests compare its live responses against them (rules 25 and 62).
package gentaskstest

import (
	"embed"
	"fmt"
)

//go:embed fixtures/*.json
var fixtures embed.FS

// FixtureNames lists the recorded gentasks responses.
var FixtureNames = []string{"task_context.json", "assigned_tasks.json"}

func Fixture(name string) []byte {
	data, err := fixtures.ReadFile("fixtures/" + name)
	if err != nil {
		panic(fmt.Sprintf("unknown gentasks fixture %q: %v", name, err))
	}
	return data
}
