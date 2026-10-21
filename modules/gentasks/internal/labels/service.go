package labels

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LabelService struct {
	db         *pgxpool.Pool
	boards     boards.BoardRepository
	labels     LabelRepository
	taskLabels TaskLabelRepository
	now        func() time.Time
}

func NewLabelService(db *pgxpool.Pool, now func() time.Time) *LabelService {
	return &LabelService{db: db, now: now}
}

func (service *LabelService) CreateLabel(ctx context.Context, boardToken string, input LabelInput) (Label, error) {
	board, err := service.boards.FindByToken(ctx, service.db, boardToken)
	if err != nil {
		return Label{}, err
	}
	return service.labels.Create(ctx, service.db, NewLabel{BoardID: board.ID, Name: input.Name, Color: input.Color, At: service.now()})
}

func (service *LabelService) ListLabels(ctx context.Context, boardToken string) ([]Label, error) {
	board, err := service.boards.FindByToken(ctx, service.db, boardToken)
	if err != nil {
		return nil, err
	}
	return service.labels.ListByBoard(ctx, service.db, board.ID)
}

func (service *LabelService) UpdateLabel(ctx context.Context, token string, changes LabelChanges) (Label, error) {
	label, err := service.labels.FindByToken(ctx, service.db, token)
	if err != nil {
		return Label{}, err
	}
	if changes == (LabelChanges{}) {
		return label, nil
	}
	return service.labels.Update(ctx, service.db, changes.ApplyTo(label), service.now())
}

func (service *LabelService) DeleteLabel(ctx context.Context, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		label, err := service.labels.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if err := service.taskLabels.DetachLabel(ctx, tx, label.ID); err != nil {
			return err
		}
		return service.labels.SoftDelete(ctx, tx, label.ID, service.now())
	})
}
