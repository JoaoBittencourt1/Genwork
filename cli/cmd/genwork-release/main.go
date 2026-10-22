// Command genwork-release builds the deploy bundle and install.sh of a release; `make release-artifacts` runs it.
package main

import (
	"fmt"
	"os"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/packaging"
)

func main() {
	if err := packaging.Run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "genwork-release:", err)
		os.Exit(1)
	}
}
