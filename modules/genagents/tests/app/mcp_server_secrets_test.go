package app_test

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/mcpservers"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
)

// readStoredSecrets decrypts the stored secrets of a server the way run claiming does, to prove what is kept.
func readStoredSecrets(app *testApp, token string) (string, error) {
	secretCipher, err := platform.NewSecretCipher(testEncryptionKey)
	if err != nil {
		return "", err
	}
	service := mcpservers.NewMcpServerService(app.db, secretCipher, app.clock.Now)
	var id int64
	if err := app.db.QueryRow(context.Background(), `SELECT id FROM mcp_servers WHERE token = $1`, token).Scan(&id); err != nil {
		return "", err
	}
	resolved, err := service.ResolveForRun(context.Background(), app.db, []int64{id})
	if err != nil {
		return "", err
	}
	text := ""
	for key, value := range resolved[0].Secrets {
		text += key + "=" + value + ";"
	}
	return text, nil
}
