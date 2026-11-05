// Package taskcontext assembles what a bot needs to work a task: the task, its board's repository, and its pull requests.
package taskcontext

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
)

type TaskContext struct {
	Task         tasks.Task
	Board        boards.Board
	Labels       []labels.Label
	PullRequests []githubsync.LinkedPullRequest
	Repository   *string
}

// Key is the identifier people use for the task: the board's prefix and the task number.
func (taskContext TaskContext) Key() string {
	return fmt.Sprintf("%s-%d", taskContext.Board.TaskPrefix, taskContext.Task.Number)
}

type TaskContextService struct {
	db         platform.DBTX
	tasks      tasks.TaskRepository
	boards     boards.BoardRepository
	taskLabels labels.TaskLabelRepository
	links      githubsync.TaskPullRequestRepository
	settings   githubsync.BoardGithubSettingsRepository
}

func NewTaskContextService(db platform.DBTX) *TaskContextService {
	return &TaskContextService{db: db}
}

func (service *TaskContextService) FindContext(ctx context.Context, taskToken string) (TaskContext, error) {
	task, err := service.tasks.FindByToken(ctx, service.db, taskToken)
	if err != nil {
		return TaskContext{}, err
	}
	board, err := service.boards.FindByID(ctx, service.db, task.BoardID)
	if err != nil {
		return TaskContext{}, err
	}
	taskLabels, err := service.taskLabels.ListForTasks(ctx, service.db, []int64{task.ID})
	if err != nil {
		return TaskContext{}, err
	}
	pullRequests, err := service.links.ListActiveForTasks(ctx, service.db, []int64{task.ID})
	if err != nil {
		return TaskContext{}, err
	}
	settings, err := service.settings.FindByBoard(ctx, service.db, board.ID)
	if err != nil {
		return TaskContext{}, err
	}
	return TaskContext{
		Task: task, Board: board, Labels: taskLabels[task.ID], PullRequests: pullRequests[task.ID], Repository: settings.Repository,
	}, nil
}
