package skills

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/jackc/pgx/v5"
)

type SkillRepository struct{}

const skillColumns = `id, token, name, description, body, version, created_at, updated_at`

func (repository SkillRepository) Create(ctx context.Context, db platform.DBTX, skill Skill, at time.Time) (Skill, error) {
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO skills (token, name, description, body, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5) RETURNING id`,
		platform.NewPublicToken(), skill.Name, skill.Description, skill.Body, at).Scan(&id)
	if platform.IsUniqueViolation(err) {
		return Skill{}, ErrSkillNameTaken
	}
	if err != nil {
		return Skill{}, fmt.Errorf("create skill: %w", err)
	}
	if err := repository.replaceFiles(ctx, db, id, skill.Files, at); err != nil {
		return Skill{}, err
	}
	return repository.FindByID(ctx, db, id)
}

// Update bumps the version, so runs started later see the edit and runs in progress keep the version they claimed.
func (repository SkillRepository) Update(ctx context.Context, db platform.DBTX, skill Skill, at time.Time) (Skill, error) {
	tag, err := db.Exec(ctx, `
		UPDATE skills SET name = $2, description = $3, body = $4, version = version + 1, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL`, skill.ID, skill.Name, skill.Description, skill.Body, at)
	if platform.IsUniqueViolation(err) {
		return Skill{}, ErrSkillNameTaken
	}
	if err != nil {
		return Skill{}, fmt.Errorf("update skill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Skill{}, ErrSkillNotFound
	}
	if err := repository.replaceFiles(ctx, db, skill.ID, skill.Files, at); err != nil {
		return Skill{}, err
	}
	return repository.FindByID(ctx, db, skill.ID)
}

func (repository SkillRepository) replaceFiles(ctx context.Context, db platform.DBTX, skillID int64, files []SkillFile, at time.Time) error {
	if _, err := db.Exec(ctx, `DELETE FROM skill_files WHERE skill_id = $1`, skillID); err != nil {
		return fmt.Errorf("clear skill files: %w", err)
	}
	for _, file := range files {
		_, err := db.Exec(ctx, `INSERT INTO skill_files (token, skill_id, path, content, created_at) VALUES ($1, $2, $3, $4, $5)`,
			platform.NewPublicToken(), skillID, file.Path, file.Content, at)
		if err != nil {
			return fmt.Errorf("create skill file: %w", err)
		}
	}
	return nil
}

func (repository SkillRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Skill, error) {
	return repository.findOne(ctx, db, `id = $1`, id)
}

func (repository SkillRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Skill, error) {
	return repository.findOne(ctx, db, `token = $1`, token)
}

func (repository SkillRepository) findOne(ctx context.Context, db platform.DBTX, condition string, argument any) (Skill, error) {
	skill, err := scanSkill(db.QueryRow(ctx, `SELECT `+skillColumns+` FROM skills WHERE `+condition+` AND deleted_at IS NULL`, argument))
	if errors.Is(err, pgx.ErrNoRows) {
		return Skill{}, ErrSkillNotFound
	}
	if err != nil {
		return Skill{}, fmt.Errorf("find skill: %w", err)
	}
	files, err := repository.listFiles(ctx, db, []int64{skill.ID})
	if err != nil {
		return Skill{}, err
	}
	skill.Files = files[skill.ID]
	return skill, nil
}

func (repository SkillRepository) List(ctx context.Context, db platform.DBTX) ([]Skill, error) {
	rows, err := db.Query(ctx, `SELECT `+skillColumns+` FROM skills WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Skill, error) { return scanSkill(row) })
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	return repository.attachFiles(ctx, db, listed)
}

// ListByIDs loads the skills of a bot with their files in two queries (rule 21).
func (repository SkillRepository) ListByIDs(ctx context.Context, db platform.DBTX, ids []int64) ([]Skill, error) {
	rows, err := db.Query(ctx, `SELECT `+skillColumns+` FROM skills WHERE id = ANY($1) AND deleted_at IS NULL ORDER BY name`, ids)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Skill, error) { return scanSkill(row) })
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	return repository.attachFiles(ctx, db, listed)
}

func (repository SkillRepository) attachFiles(ctx context.Context, db platform.DBTX, listed []Skill) ([]Skill, error) {
	ids := make([]int64, 0, len(listed))
	for _, skill := range listed {
		ids = append(ids, skill.ID)
	}
	files, err := repository.listFiles(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for index := range listed {
		listed[index].Files = files[listed[index].ID]
	}
	return listed, nil
}

func (SkillRepository) listFiles(ctx context.Context, db platform.DBTX, skillIDs []int64) (map[int64][]SkillFile, error) {
	rows, err := db.Query(ctx, `SELECT skill_id, path, content FROM skill_files WHERE skill_id = ANY($1) ORDER BY path`, skillIDs)
	if err != nil {
		return nil, fmt.Errorf("list skill files: %w", err)
	}
	defer rows.Close()
	byskill := map[int64][]SkillFile{}
	for rows.Next() {
		var skillID int64
		var file SkillFile
		if err := rows.Scan(&skillID, &file.Path, &file.Content); err != nil {
			return nil, fmt.Errorf("scan skill file: %w", err)
		}
		byskill[skillID] = append(byskill[skillID], file)
	}
	return byskill, rows.Err()
}

// SoftDelete also detaches the skill from every bot; the join rows carry no history, so they are deleted.
func (SkillRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	if _, err := db.Exec(ctx, `DELETE FROM bot_skills WHERE skill_id = $1`, id); err != nil {
		return fmt.Errorf("detach skill: %w", err)
	}
	tag, err := db.Exec(ctx, `UPDATE skills SET deleted_at = $2, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSkillNotFound
	}
	return nil
}

func scanSkill(row pgx.Row) (Skill, error) {
	var skill Skill
	err := row.Scan(&skill.ID, &skill.Token, &skill.Name, &skill.Description, &skill.Body, &skill.Version, &skill.CreatedAt, &skill.UpdatedAt)
	return skill, err
}
