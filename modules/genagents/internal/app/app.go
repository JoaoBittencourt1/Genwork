package app

import (
	"context"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/adminchat"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/mcpservers"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runexec"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/skills"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/storagecleanup"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/taskruns"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	DB            *pgxpool.Pool
	TokenVerifier httpapi.AccessTokenVerifier
	EncryptionKey []byte
	Core          *corelink.Client
	Gentasks      taskruns.GentasksClient
	// RepositoryTokens issues the per-run GitHub tokens; nil when gentasks is not reachable from this module.
	RepositoryTokens runexec.RepositoryTokenIssuer
	// GenworkMCPURL is the instance's Genwork MCP server, which every run is configured with.
	GenworkMCPURL string
	// PublicURL is where people open Genwork; OAuth callbacks come back to it.
	PublicURL string
	// MCPOAuthClient is how genagents reaches MCP authorization servers.
	MCPOAuthClient mcpservers.OAuthProviderClient
	// ClaimLongPoll bounds how long a runner's claim waits for work; zero keeps the default.
	ClaimLongPoll time.Duration
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
	db, now := dependencies.DB, dependencies.Now
	lifecycle := runs.NewRunLifecycleService(now)
	accountService := accounts.NewAiAccountService(db, secretCipher, bots.BotRepository{}, now)
	accounts.NewAiAccountHandler(accountService).Register(router)
	botService := bots.NewBotService(db, secretCipher, dependencies.Core, lifecycle, accountService, now)
	bots.NewBotHandler(botService).Register(router)
	skills.NewSkillHandler(skills.NewSkillService(db, now)).Register(router)
	mcpOAuthService := mcpservers.NewMcpServerOAuthService(db, secretCipher, dependencies.MCPOAuthClient, dependencies.PublicURL, now)
	mcpServerService := mcpservers.NewMcpServerService(db, secretCipher, now).WithOAuth(mcpOAuthService)
	mcpservers.NewMcpServerHandler(mcpServerService).Register(router)
	mcpservers.NewMcpServerOAuthHandler(mcpOAuthService).Register(router)

	taskRunService := taskruns.NewTaskRunService(db, dependencies.Gentasks, lifecycle, now)
	taskruns.NewTaskRunHandler(taskRunService, taskruns.NewTaskConversationService(db, dependencies.Gentasks, lifecycle, now)).Register(router)

	adminchat.NewAdminChatHandler(adminchat.NewAdminChatService(db, dependencies.Core, secretCipher, lifecycle, now)).Register(router)

	runnerService := runexec.NewRunnerService(db, lifecycle, now)
	runexec.NewRunnerHandler(runnerService, now).Register(router)
	storagecleanup.NewStorageCleanupHandler(storagecleanup.NewStorageCleanupService(db, now)).Register(router)
	accounts.NewAccountSignInHandler(accounts.NewAccountSignInService(db, secretCipher, now), runnerService).Register(router)
	genworkTokens := runexec.NewGenworkTokenService(db, dependencies.Core, secretCipher, botService, lifecycle, now)
	runnerAPI := runexec.NewRunnerAPIHandler(db, runnerService,
		runexec.NewRunClaimService(db, accountService, mcpServerService, lifecycle, now),
		runexec.NewRunEventService(db, lifecycle, now),
		genworkTokens,
		runexec.NewGithubTokenService(db, genworkTokens, dependencies.RepositoryTokens),
		dependencies.GenworkMCPURL, now,
	)
	if dependencies.ClaimLongPoll > 0 {
		runnerAPI.WithLongPoll(dependencies.ClaimLongPoll, 50*time.Millisecond)
	}
	runnerAPI.Register(router)
	return router, nil
}

// Worker is a background loop the module runs next to its HTTP server.
type Worker interface {
	Run(ctx context.Context)
}

// NewWorkers builds the background loops: the sweeper that ends runs nobody is working on, and the reconciliation that
// catches assignments whose notification was lost.
func NewWorkers(dependencies Dependencies) ([]Worker, error) {
	secretCipher, err := platform.NewSecretCipher(dependencies.EncryptionKey)
	if err != nil {
		return nil, err
	}
	db, now := dependencies.DB, dependencies.Now
	lifecycle := runs.NewRunLifecycleService(now)
	accountService := accounts.NewAiAccountService(db, secretCipher, bots.BotRepository{}, now)
	botService := bots.NewBotService(db, secretCipher, dependencies.Core, lifecycle, accountService, now)
	taskRunService := taskruns.NewTaskRunService(db, dependencies.Gentasks, lifecycle, now)
	return []Worker{
		runexec.NewRunSweeper(db, lifecycle, now),
		taskruns.NewReconciliationService(db, dependencies.Gentasks, dependencies.Core, botService, taskRunService),
	}, nil
}
