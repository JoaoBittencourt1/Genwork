package app

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardview"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	DB            *pgxpool.Pool
	TokenVerifier httpapi.AccessTokenVerifier
	UserDirectory tasks.UserDirectory
	Now           func() time.Time
}

// New is the composition root of the gentasks HTTP API.
func New(dependencies Dependencies) http.Handler {
	router := httpapi.NewRouter(dependencies.TokenVerifier)
	router.HandleFunc("GET /healthz", httpapi.Public, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	boards.NewBoardHandler(boards.NewBoardService(dependencies.DB, tasks.TaskRepository{}, dependencies.Now)).Register(router)
	boardview.NewBoardViewHandler(boardview.NewBoardViewService(dependencies.DB)).Register(router)
	labels.NewLabelHandler(labels.NewLabelService(dependencies.DB, dependencies.Now)).Register(router)
	tasks.NewTaskHandler(tasks.NewTaskService(dependencies.DB, dependencies.UserDirectory, dependencies.Now)).Register(router)
	return router
}
