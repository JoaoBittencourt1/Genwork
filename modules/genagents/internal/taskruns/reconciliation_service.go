package taskruns

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/gentasksclient"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ReconciliationInterval = 60 * time.Second
	assignedTasksLimit     = 200
)

// BotSignIn is how reconciliation reads gentasks as a bot (rule 9).
type BotSignIn interface {
	IssueBotAccessToken(ctx context.Context, botToken, clientSecret string) (corelink.BotAccessToken, error)
}

// ReconciliationService makes the runs match the assignments even when a notification was lost: it reads each enabled
// dev bot's assigned tasks as that bot, starts runs for new assignments and cancels runs for tasks it no longer holds.
type ReconciliationService struct {
	db            *pgxpool.Pool
	gentasks      GentasksClient
	signIn        BotSignIn
	botService    *bots.BotService
	bots          bots.BotRepository
	conversations runs.ConversationRepository
	runs          runs.RunRepository
	taskRuns      *TaskRunService
}

func NewReconciliationService(db *pgxpool.Pool, gentasks GentasksClient, signIn BotSignIn, botService *bots.BotService, taskRuns *TaskRunService) *ReconciliationService {
	return &ReconciliationService{db: db, gentasks: gentasks, signIn: signIn, botService: botService, taskRuns: taskRuns}
}

// ReconcileOnce reconciles every enabled dev bot; a bot that fails does not keep the others from being reconciled.
func (service *ReconciliationService) ReconcileOnce(ctx context.Context) error {
	kind := bots.BotKindDev
	listed, err := service.bots.List(ctx, service.db, &kind)
	if err != nil {
		return err
	}
	var failures []error
	for _, bot := range listed {
		if !bot.IsEnabled() {
			continue
		}
		if err := service.reconcileBot(ctx, bot); err != nil {
			failures = append(failures, err)
			slog.Warn("reconcile bot", "bot", bot.Token, "err", err)
		}
	}
	return errors.Join(failures...)
}

func (service *ReconciliationService) reconcileBot(ctx context.Context, bot bots.Bot) error {
	secret, err := service.botService.DecryptMemberSecret(bot)
	if err != nil {
		return err
	}
	token, err := service.signIn.IssueBotAccessToken(ctx, bot.MemberToken, secret)
	if err != nil {
		return err
	}
	assigned, err := service.gentasks.ListAssignedTasks(ctx, token.AccessToken, assignedTasksLimit)
	if err != nil {
		return err
	}
	held := map[string]bool{}
	for _, task := range assigned {
		held[task.Token] = true
		if err := service.startForNewAssignment(ctx, bot, token.AccessToken, task); err != nil {
			return err
		}
	}
	return service.cancelForReleasedTasks(ctx, bot, token.AccessToken, held)
}

// startForNewAssignment starts a run unless one was already created since the task was assigned to the bot.
func (service *ReconciliationService) startForNewAssignment(ctx context.Context, bot bots.Bot, accessToken string, assigned gentasksclient.AssignedTask) error {
	conversation, err := service.conversations.FindByTaskToken(ctx, service.db, assigned.Token)
	if err == nil {
		last, lastErr := service.runs.FindLast(ctx, service.db, conversation.ID, runs.RunKindTask)
		if lastErr == nil && (last.IsActive() || assigned.AssignedAt == nil || !last.CreatedAt.Before(*assigned.AssignedAt)) {
			return nil
		}
		if lastErr != nil && !errors.Is(lastErr, runs.ErrRunNotFound) {
			return lastErr
		}
	} else if !errors.Is(err, runs.ErrConversationNotFound) {
		return err
	}
	task, err := service.gentasks.FindTaskContext(ctx, accessToken, assigned.Token)
	if err != nil {
		return err
	}
	return service.taskRuns.ApplyAssignment(ctx, bot.MemberToken, task)
}

// cancelForReleasedTasks cancels the bot's active runs for tasks missing from its assigned list. The list leaves out
// tasks in a done column, so the task is read first: only a changed assignee cancels.
func (service *ReconciliationService) cancelForReleasedTasks(ctx context.Context, bot bots.Bot, accessToken string, held map[string]bool) error {
	active, err := service.runs.ListActiveForBot(ctx, service.db, bot.ID)
	if err != nil {
		return err
	}
	for _, run := range active {
		if run.Kind != runs.RunKindTask {
			continue
		}
		conversation, err := service.conversations.FindByID(ctx, service.db, run.ConversationID)
		if err != nil {
			return err
		}
		if conversation.TaskToken == nil || held[*conversation.TaskToken] {
			continue
		}
		task, err := service.gentasks.FindTaskContext(ctx, accessToken, *conversation.TaskToken)
		if errors.Is(err, gentasksclient.ErrTaskNotFound) {
			task = gentasksclient.TaskContext{Token: *conversation.TaskToken}
		} else if err != nil {
			return err
		}
		if err := service.taskRuns.ApplyAssignment(ctx, bot.MemberToken, task); err != nil {
			return err
		}
	}
	return nil
}

// Run reconciles on a fixed interval until the context ends.
func (service *ReconciliationService) Run(ctx context.Context) {
	ticker := time.NewTicker(ReconciliationInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := service.ReconcileOnce(ctx); err != nil && ctx.Err() == nil {
				slog.Error("reconcile task runs", "err", err)
			}
		}
	}
}
