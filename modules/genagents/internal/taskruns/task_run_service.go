// Package taskruns starts, cancels, and talks to the runs of gentasks tasks: dev runs on assignment, bot reviews, and
// the task conversation people follow them through.
package taskruns

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/gentasksclient"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoOpenPullRequests = httpapi.NewAPIError(http.StatusConflict, "no_open_pull_requests")

// GentasksClient is what task runs need from gentasks (rule 9).
type GentasksClient interface {
	FindTaskContext(ctx context.Context, accessToken, taskToken string) (gentasksclient.TaskContext, error)
	ListAssignedTasks(ctx context.Context, accessToken string, limit int) ([]gentasksclient.AssignedTask, error)
}

// Caller is who asked: the access token is forwarded to gentasks, which applies the caller's own rights.
type Caller struct {
	UserToken   string
	AccessToken string
}

type TaskRunService struct {
	db            *pgxpool.Pool
	gentasks      GentasksClient
	bots          bots.BotRepository
	conversations runs.ConversationRepository
	runs          runs.RunRepository
	lifecycle     *runs.RunLifecycleService
	prompts       PromptComposer
	now           func() time.Time
}

func NewTaskRunService(db *pgxpool.Pool, gentasks GentasksClient, lifecycle *runs.RunLifecycleService, now func() time.Time) *TaskRunService {
	return &TaskRunService{db: db, gentasks: gentasks, lifecycle: lifecycle, now: now}
}

// HandleAssignment treats an assignment notification only as a hint: it reads the task as the caller and acts on what
// gentasks says is true, so a forged notification starts nothing.
func (service *TaskRunService) HandleAssignment(ctx context.Context, caller Caller, taskToken string) error {
	task, err := service.gentasks.FindTaskContext(ctx, caller.AccessToken, taskToken)
	if err != nil {
		return err
	}
	return service.ApplyAssignment(ctx, caller.UserToken, task)
}

// ApplyAssignment makes the runs of a task match its current assignee: it starts a dev run for a dev bot and cancels the
// active run of a bot that no longer holds the task.
func (service *TaskRunService) ApplyAssignment(ctx context.Context, requesterToken string, task gentasksclient.TaskContext) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		assignee, err := service.findAssigneeBot(ctx, tx, task)
		if err != nil {
			return err
		}
		conversation, err := service.conversations.FindByTaskToken(ctx, tx, task.Token)
		if errors.Is(err, runs.ErrConversationNotFound) {
			if assignee == nil {
				return nil
			}
			if conversation, err = service.conversations.FindOrCreateTask(ctx, tx, task.Token, service.now()); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		active, err := service.runs.FindActive(ctx, tx, conversation.ID, runs.RunKindTask)
		if err != nil && !errors.Is(err, runs.ErrRunNotFound) {
			return err
		}
		if err == nil && (assignee == nil || assignee.ID != active.BotID) {
			if err := service.lifecycle.Cancel(ctx, tx, active, runs.CauseReassigned, map[string]any{"by": requesterToken}); err != nil {
				return err
			}
			active = runs.Run{}
		}
		if assignee == nil || active.ID != 0 {
			return nil
		}
		return service.startTaskRun(ctx, tx, conversation, *assignee, requesterToken, task)
	})
}

// findAssigneeBot returns the enabled bot whose member holds the task, or nil when a person (or nobody) does.
func (service *TaskRunService) findAssigneeBot(ctx context.Context, tx pgx.Tx, task gentasksclient.TaskContext) (*bots.Bot, error) {
	if task.AssigneeToken == nil || task.AssigneeKind == nil || *task.AssigneeKind != "bot" {
		return nil, nil
	}
	bot, err := service.bots.FindByMemberToken(ctx, tx, *task.AssigneeToken)
	if errors.Is(err, bots.ErrBotNotFound) || (err == nil && !bot.IsEnabled()) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bot, nil
}

func (service *TaskRunService) startTaskRun(ctx context.Context, tx pgx.Tx, conversation runs.Conversation, bot bots.Bot, requesterToken string, task gentasksclient.TaskContext) error {
	if bot.Kind != bots.BotKindDev {
		_, err := service.postSystemStatus(ctx, tx, conversation, runs.StatusPayload{Code: "only_dev_bots_work_tasks", Params: map[string]any{"bot": bot.Name, "taskKey": task.Key}})
		return err
	}
	spec := runs.RunSpec{TaskKey: task.Key, TaskTitle: task.Title}
	if task.Repository != nil {
		spec.Repository = *task.Repository
		spec.Branch = BranchName(task.Key)
		spec.Prompt = service.prompts.ComposeTaskPrompt(task, spec.Repository, spec.Branch)
	}
	run, err := service.runs.Create(ctx, tx, runs.NewRun{
		ConversationID: conversation.ID, BotID: bot.ID, Kind: runs.RunKindTask, RequesterUserToken: requesterToken, Spec: spec, At: service.now(),
	})
	if errors.Is(err, runs.ErrRunAlreadyActive) {
		return nil
	}
	if err != nil {
		return err
	}
	if task.Repository == nil {
		return service.lifecycle.Fail(ctx, tx, run, runs.FailureNoRepository, map[string]any{"taskKey": task.Key, "board": task.Board.Name})
	}
	return service.lifecycle.Queued(ctx, tx, run)
}

// BranchName names the branch of a dev run after the task key, with a random suffix so reruns never collide.
func BranchName(taskKey string) string {
	suffix := strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(platform.NewPublicToken()))[:6]
	return "bot/" + strings.ToLower(taskKey) + "-" + suffix
}

func (service *TaskRunService) postSystemStatus(ctx context.Context, tx pgx.Tx, conversation runs.Conversation, payload runs.StatusPayload) (bool, error) {
	var entries runs.EntryRepository
	return entries.Append(ctx, tx, runs.NewEntry{ConversationID: conversation.ID, Type: runs.EntryStatus, Payload: payload, At: service.now()})
}

// RequestReview queues a review run by a reviewer bot over the task's open and draft pull requests.
func (service *TaskRunService) RequestReview(ctx context.Context, caller Caller, taskToken, botToken string) (runs.Run, error) {
	task, err := service.gentasks.FindTaskContext(ctx, caller.AccessToken, taskToken)
	if err != nil {
		return runs.Run{}, err
	}
	var reviewable []gentasksclient.PullRequest
	for _, pullRequest := range task.PullRequests {
		if pullRequest.IsReviewable() {
			reviewable = append(reviewable, pullRequest)
		}
	}
	if len(reviewable) == 0 {
		return runs.Run{}, ErrNoOpenPullRequests
	}
	var created runs.Run
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		reviewer, err := service.chooseReviewer(ctx, tx, botToken)
		if err != nil {
			return err
		}
		conversation, err := service.conversations.FindOrCreateTask(ctx, tx, task.Token, service.now())
		if err != nil {
			return err
		}
		refs := make([]runs.PullRequestRef, 0, len(reviewable))
		for _, pullRequest := range reviewable {
			ref := runs.PullRequestRef{Repository: pullRequest.Repository, Number: pullRequest.Number, URL: pullRequest.URL}
			if pullRequest.Title != nil {
				ref.Title = *pullRequest.Title
			}
			refs = append(refs, ref)
		}
		created, err = service.runs.Create(ctx, tx, runs.NewRun{
			ConversationID: conversation.ID, BotID: reviewer.ID, Kind: runs.RunKindReview, RequesterUserToken: caller.UserToken,
			Spec: runs.RunSpec{Prompt: service.prompts.ComposeReviewPrompt(task, reviewable), TaskKey: task.Key, TaskTitle: task.Title, PullRequests: refs}, At: service.now(),
		})
		if err != nil {
			return err
		}
		return service.lifecycle.Queued(ctx, tx, created)
	})
	return created, err
}

// chooseReviewer honors a named reviewer bot; unnamed, it takes the only enabled one and refuses to guess among several.
func (service *TaskRunService) chooseReviewer(ctx context.Context, tx pgx.Tx, botToken string) (bots.Bot, error) {
	if botToken != "" {
		bot, err := service.bots.FindByToken(ctx, tx, botToken)
		if errors.Is(err, bots.ErrBotNotFound) || (err == nil && (bot.Kind != bots.BotKindReviewer || !bot.IsEnabled())) {
			return bots.Bot{}, httpapi.NewValidationError(map[string]string{"botToken": "not_a_reviewer"})
		}
		return bot, err
	}
	kind := bots.BotKindReviewer
	listed, err := service.bots.List(ctx, tx, &kind)
	if err != nil {
		return bots.Bot{}, err
	}
	var enabled []bots.Bot
	for _, bot := range listed {
		if bot.IsEnabled() {
			enabled = append(enabled, bot)
		}
	}
	switch len(enabled) {
	case 0:
		return bots.Bot{}, httpapi.NewValidationError(map[string]string{"botToken": "no_reviewer_bot"})
	case 1:
		return enabled[0], nil
	default:
		return bots.Bot{}, httpapi.NewValidationError(map[string]string{"botToken": "reviewer_required"})
	}
}
