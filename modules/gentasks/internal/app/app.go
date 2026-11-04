package app

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardview"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	DB            *pgxpool.Pool
	TokenVerifier httpapi.AccessTokenVerifier
	UserDirectory tasks.UserDirectory
	Github        GithubConfig
	Now           func() time.Time
}

type GithubConfig struct {
	EncryptionKey []byte
	PublicURL     string
	WebURL        string
	APIURL        string
}

// New is the composition root of the gentasks HTTP API.
func New(dependencies Dependencies) (http.Handler, error) {
	credentialCipher, err := githubsync.NewCredentialCipher(dependencies.Github.EncryptionKey)
	if err != nil {
		return nil, err
	}
	manifestStates, err := githubsync.NewManifestStateSigner(dependencies.Github.EncryptionKey)
	if err != nil {
		return nil, err
	}
	githubConnections := githubsync.NewGithubConnectionService(dependencies.DB, credentialCipher, manifestStates,
		githubsync.NewGithubManifestConversionClient(dependencies.Github.APIURL),
		githubsync.GithubConnectionConfig{PublicURL: dependencies.Github.PublicURL, GithubWebURL: dependencies.Github.WebURL}, dependencies.Now)

	router := httpapi.NewRouter(dependencies.TokenVerifier)
	router.HandleFunc("GET /healthz", httpapi.Public, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	boards.NewBoardHandler(boards.NewBoardService(dependencies.DB, tasks.TaskRepository{}, dependencies.Now)).Register(router)
	boardview.NewBoardViewHandler(boardview.NewBoardViewService(dependencies.DB), dependencies.Github.WebURL).Register(router)
	labels.NewLabelHandler(labels.NewLabelService(dependencies.DB, dependencies.Now)).Register(router)
	taskService := tasks.NewTaskService(dependencies.DB, dependencies.UserDirectory, dependencies.Now)
	tasks.NewTaskHandler(taskService).Register(router)
	githubsync.NewGithubConnectionHandler(githubConnections).Register(router)
	githubsync.NewGithubWebhookHandler(githubConnections, githubsync.NewPullRequestSyncService(dependencies.DB, taskService, dependencies.Now)).Register(router)
	githubsync.NewBoardGithubSettingsHandler(githubsync.NewBoardGithubSettingsService(dependencies.DB, dependencies.Now)).Register(router)
	return router, nil
}
