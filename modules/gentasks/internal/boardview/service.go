// Package boardview assembles the full board a client renders: columns, their active tasks with their linked pull
// requests, and the board's labels.
package boardview

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
)

// BoardView.PullRequests holds each task's active links, keyed by task id; tasks without links have no entry.
type BoardView struct {
	Board        boards.Board
	Columns      []boards.Column
	Tasks        []tasks.TaskWithLabels
	Labels       []labels.Label
	PullRequests map[int64][]githubsync.LinkedPullRequest
}

// BoardViewService reads a board with a fixed number of queries whatever its size (rule 21).
type BoardViewService struct {
	db         platform.DBTX
	boards     boards.BoardRepository
	columns    boards.ColumnRepository
	tasks      tasks.TaskRepository
	labels     labels.LabelRepository
	taskLabels labels.TaskLabelRepository
	links      githubsync.TaskPullRequestRepository
}

func NewBoardViewService(db platform.DBTX) *BoardViewService {
	return &BoardViewService{db: db}
}

func (service *BoardViewService) ReadBoard(ctx context.Context, token string) (BoardView, error) {
	board, err := service.boards.FindByToken(ctx, service.db, token)
	if err != nil {
		return BoardView{}, err
	}
	columns, err := service.columns.ListByBoard(ctx, service.db, board.ID)
	if err != nil {
		return BoardView{}, err
	}
	active, err := service.tasks.ListActiveByBoard(ctx, service.db, board.ID)
	if err != nil {
		return BoardView{}, err
	}
	taskIDs := make([]int64, 0, len(active))
	for _, task := range active {
		taskIDs = append(taskIDs, task.ID)
	}
	labelsByTask, err := service.taskLabels.ListForTasks(ctx, service.db, taskIDs)
	if err != nil {
		return BoardView{}, err
	}
	pullRequestsByTask, err := service.links.ListActiveForTasks(ctx, service.db, taskIDs)
	if err != nil {
		return BoardView{}, err
	}
	boardLabels, err := service.labels.ListByBoard(ctx, service.db, board.ID)
	if err != nil {
		return BoardView{}, err
	}
	view := BoardView{
		Board: board, Columns: columns, Labels: boardLabels, PullRequests: pullRequestsByTask,
		Tasks: make([]tasks.TaskWithLabels, 0, len(active)),
	}
	for _, task := range active {
		view.Tasks = append(view.Tasks, tasks.TaskWithLabels{Task: task, Labels: labelsByTask[task.ID]})
	}
	return view, nil
}
