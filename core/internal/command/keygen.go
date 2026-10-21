package command

import (
	"fmt"
	"io"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
)

func RunKeygen(stdout io.Writer) error {
	_, err := fmt.Fprintln(stdout, auth.GenerateSigningKeySeed())
	return err
}
