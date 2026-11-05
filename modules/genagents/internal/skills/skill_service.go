package skills

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SkillService struct {
	db     *pgxpool.Pool
	skills SkillRepository
	now    func() time.Time
}

func NewSkillService(db *pgxpool.Pool, now func() time.Time) *SkillService {
	return &SkillService{db: db, now: now}
}

func (service *SkillService) ListSkills(ctx context.Context) ([]Skill, error) {
	return service.skills.List(ctx, service.db)
}

func (service *SkillService) FindSkill(ctx context.Context, token string) (Skill, error) {
	return service.skills.FindByToken(ctx, service.db, token)
}

func (service *SkillService) CreateSkill(ctx context.Context, skill Skill) (Skill, error) {
	return service.skills.Create(ctx, service.db, skill, service.now())
}

func (service *SkillService) UpdateSkill(ctx context.Context, token string, request UpdateSkillRequest) (Skill, error) {
	var updated Skill
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		current, err := service.skills.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		changed, err := request.ApplyTo(current)
		if err != nil {
			return err
		}
		updated, err = service.skills.Update(ctx, tx, changed, service.now())
		return err
	})
	return updated, err
}

func (service *SkillService) DeleteSkill(ctx context.Context, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		skill, err := service.skills.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		return service.skills.SoftDelete(ctx, tx, skill.ID, service.now())
	})
}
