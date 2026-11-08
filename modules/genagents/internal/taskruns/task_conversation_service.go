package taskruns

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskConversationView is a task's conversation as clients poll it: entries after a sequence, plus the active run.
type TaskConversationView struct {
	Conversation *runs.Conversation
	Entries      []runs.Entry
	LastSequence int64
	ActiveRun    *ActiveRunView
}

type ActiveRunView struct {
	Run     runs.Run
	BotName string
}

// TaskConversationService is the conversation people read, write to, and stop runs from. Every call first reads the task
// as the caller, so seeing a conversation needs the right to see its task (gentasks.board.read).
type TaskConversationService struct {
	db            *pgxpool.Pool
	gentasks      GentasksClient
	bots          bots.BotRepository
	conversations runs.ConversationRepository
	runs          runs.RunRepository
	entries       runs.EntryRepository
	lifecycle     *runs.RunLifecycleService
	now           func() time.Time
}

func NewTaskConversationService(db *pgxpool.Pool, gentasks GentasksClient, lifecycle *runs.RunLifecycleService, now func() time.Time) *TaskConversationService {
	return &TaskConversationService{db: db, gentasks: gentasks, lifecycle: lifecycle, now: now}
}

func (service *TaskConversationService) ReadConversation(ctx context.Context, caller Caller, taskToken string, afterSequence int64) (TaskConversationView, error) {
	if _, err := service.gentasks.FindTaskContext(ctx, caller.AccessToken, taskToken); err != nil {
		return TaskConversationView{}, err
	}
	conversation, err := service.conversations.FindByTaskToken(ctx, service.db, taskToken)
	if errors.Is(err, runs.ErrConversationNotFound) {
		return TaskConversationView{Entries: []runs.Entry{}}, nil
	}
	if err != nil {
		return TaskConversationView{}, err
	}
	entries, err := service.entries.ListAfter(ctx, service.db, conversation.ID, afterSequence)
	if err != nil {
		return TaskConversationView{}, err
	}
	last, err := service.entries.LastSequence(ctx, service.db, conversation.ID)
	if err != nil {
		return TaskConversationView{}, err
	}
	view := TaskConversationView{Conversation: &conversation, Entries: entries, LastSequence: last}
	active, err := service.runs.ListActiveForConversation(ctx, service.db, conversation.ID)
	if err != nil {
		return TaskConversationView{}, err
	}
	if len(active) > 0 {
		bot, err := service.bots.FindByID(ctx, service.db, active[0].BotID)
		if err != nil && !errors.Is(err, bots.ErrBotNotFound) {
			return TaskConversationView{}, err
		}
		view.ActiveRun = &ActiveRunView{Run: active[0], BotName: bot.Name}
	}
	return view, nil
}

// PostMessage adds the caller's message. A runner holding an active run picks it up at its next turn; when nothing is
// active, the message starts a follow-up run on the same CLI session when its bot still exists.
func (service *TaskConversationService) PostMessage(ctx context.Context, caller Caller, taskToken, text string) error {
	text = strings.TrimSpace(text)
	if !runs.IsValidMessage(text) {
		return runs.ErrMessageInvalid
	}
	if _, err := service.gentasks.FindTaskContext(ctx, caller.AccessToken, taskToken); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		conversation, err := service.conversations.FindOrCreateTask(ctx, tx, taskToken, service.now())
		if err != nil {
			return err
		}
		author := caller.UserToken
		if _, err := service.entries.Append(ctx, tx, runs.NewEntry{
			ConversationID: conversation.ID, Type: runs.EntryHumanMessage, AuthorToken: &author, Payload: map[string]any{"text": text}, At: service.now(),
		}); err != nil {
			return err
		}
		active, err := service.runs.ListActiveForConversation(ctx, tx, conversation.ID)
		if err != nil || len(active) > 0 {
			return err
		}
		return service.startFollowUp(ctx, tx, conversation, caller, text)
	})
}

// startFollowUp continues the last run's session with the message, if its bot is still enabled.
func (service *TaskConversationService) startFollowUp(ctx context.Context, tx pgx.Tx, conversation runs.Conversation, caller Caller, text string) error {
	last, err := service.lastRun(ctx, tx, conversation.ID)
	if errors.Is(err, runs.ErrRunNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	bot, err := service.bots.FindByID(ctx, tx, last.BotID)
	if errors.Is(err, bots.ErrBotNotFound) || (err == nil && !bot.IsEnabled()) {
		return nil
	}
	if err != nil {
		return err
	}
	spec := last.Spec
	spec.Prompt = text
	run, err := service.runs.Create(ctx, tx, runs.NewRun{
		ConversationID: conversation.ID, BotID: bot.ID, Kind: last.Kind, RequesterUserToken: caller.UserToken, CLISessionID: last.CLISessionID, Spec: spec, At: service.now(),
	})
	if err != nil {
		return err
	}
	return service.lifecycle.Queued(ctx, tx, run)
}

func (service *TaskConversationService) lastRun(ctx context.Context, tx pgx.Tx, conversationID int64) (runs.Run, error) {
	task, taskErr := service.runs.FindLast(ctx, tx, conversationID, runs.RunKindTask)
	review, reviewErr := service.runs.FindLast(ctx, tx, conversationID, runs.RunKindReview)
	switch {
	case taskErr == nil && reviewErr == nil:
		if review.ID > task.ID {
			return review, nil
		}
		return task, nil
	case taskErr == nil:
		return task, nil
	case reviewErr == nil:
		return review, nil
	case errors.Is(taskErr, runs.ErrRunNotFound) && errors.Is(reviewErr, runs.ErrRunNotFound):
		return runs.Run{}, runs.ErrRunNotFound
	case taskErr != nil && !errors.Is(taskErr, runs.ErrRunNotFound):
		return runs.Run{}, taskErr
	default:
		return runs.Run{}, reviewErr
	}
}

// StopRuns cancels the active runs of the task's conversation, saying who stopped them.
func (service *TaskConversationService) StopRuns(ctx context.Context, caller Caller, taskToken string) error {
	if _, err := service.gentasks.FindTaskContext(ctx, caller.AccessToken, taskToken); err != nil {
		return err
	}
	conversation, err := service.conversations.FindByTaskToken(ctx, service.db, taskToken)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		active, err := service.runs.ListActiveForConversation(ctx, tx, conversation.ID)
		if err != nil {
			return err
		}
		if len(active) == 0 {
			return runs.ErrRunNotActive
		}
		for _, run := range active {
			if err := service.lifecycle.Cancel(ctx, tx, run, runs.CauseUser, map[string]any{"by": caller.UserToken}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ClearConversation starts a task's bot conversation over: it cancels the active run and deletes every entry and run,
// physically, because the person asked for the history to go (a documented rule 22 exception). A cancelled run still
// held by a runner stays, without entries, until that runner sees the cancellation; a later run starts a new conversation.
func (service *TaskConversationService) ClearConversation(ctx context.Context, caller Caller, taskToken string) error {
	if _, err := service.gentasks.FindTaskContext(ctx, caller.AccessToken, taskToken); err != nil {
		return err
	}
	conversation, err := service.conversations.FindByTaskToken(ctx, service.db, taskToken)
	if errors.Is(err, runs.ErrConversationNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		active, err := service.runs.ListActiveForConversation(ctx, tx, conversation.ID)
		if err != nil {
			return err
		}
		held := make([]int64, 0, len(active))
		for _, run := range active {
			if err := service.lifecycle.Cancel(ctx, tx, run, runs.CauseUser, map[string]any{"by": caller.UserToken}); err != nil {
				return err
			}
			if run.RunnerID != nil {
				held = append(held, run.ID)
			}
		}
		if err := service.entries.DeleteByConversation(ctx, tx, conversation.ID); err != nil {
			return err
		}
		remaining, err := service.runs.DeleteByConversationExcept(ctx, tx, conversation.ID, held)
		if err != nil || remaining > 0 {
			return err
		}
		return service.conversations.Delete(ctx, tx, conversation.ID)
	})
}
