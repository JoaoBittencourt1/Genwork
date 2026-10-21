package main

import (
	"fmt"
	"io"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
)

func runKeygen(stdout io.Writer) error {
	_, err := fmt.Fprintln(stdout, auth.GenerateSigningKeySeed())
	return err
}
