package app

import (
	"context"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/mcpservers"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/skills"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	DB            *pgxpool.Pool
	TokenVerifier httpapi.AccessTokenVerifier
	EncryptionKey []byte
	Core          *corelink.Client
	Now           func() time.Time
}

// New is the composition root of the genagents HTTP API.
func New(dependencies Dependencies) (http.Handler, error) {
	secretCipher, err := platform.NewSecretCipher(dependencies.EncryptionKey)
	if err != nil {
		return nil, err
	}
	router := httpapi.NewRouter(dependencies.TokenVerifier)
	router.HandleFunc("GET /healthz", httpapi.Public, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	accountService := accounts.NewAiAccountService(dependencies.DB, secretCipher, bots.BotRepository{}, dependencies.Now)
	accounts.NewAiAccountHandler(accountService).Register(router)
	bots.NewBotHandler(bots.NewBotService(dependencies.DB, secretCipher, dependencies.Core, noRunCanceller{}, accountService, dependencies.Now)).Register(router)
	skills.NewSkillHandler(skills.NewSkillService(dependencies.DB, dependencies.Now)).Register(router)
	mcpservers.NewMcpServerHandler(mcpservers.NewMcpServerService(dependencies.DB, secretCipher, dependencies.Now)).Register(router)
	return router, nil
}

// noRunCanceller stands in until runs exist; it is replaced when the run services are wired.
type noRunCanceller struct{}

func (noRunCanceller) CancelActiveRunsForBot(_ context.Context, _ platform.DBTX, _ int64, _ string) error {
	return nil
}
