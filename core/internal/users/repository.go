package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var ErrUserNotFound = errors.New("user not found")

type Repository struct{}

type NewUser struct {
	Email        string
	Name         string
	PasswordHash string
	RoleID       int64
}

const userColumns = `id, token, email, name, password_hash, role_id, failed_login_count, locked_until, last_login_at, created_at`

func (Repository) Create(ctx context.Context, db platform.DBTX, newUser NewUser) (User, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash, role_id)
		VALUES ($1, $2, $3, $4)
		RETURNING `+userColumns,
		newUser.Email, newUser.Name, newUser.PasswordHash, newUser.RoleID)
	user, err := scanUser(row)
	if platform.IsUniqueViolation(err) {
		return User{}, ErrEmailAlreadyRegistered
	}
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (Repository) FindByEmail(ctx context.Context, db platform.DBTX, normalizedEmail string) (User, error) {
	return scanSingleUser(db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1 AND deleted_at IS NULL`, normalizedEmail))
}

func (Repository) FindByID(ctx context.Context, db platform.DBTX, id int64) (User, error) {
	return scanSingleUser(db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id))
}

func (Repository) FindByToken(ctx context.Context, db platform.DBTX, token string) (User, error) {
	return scanSingleUser(db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE token = $1 AND deleted_at IS NULL`, token))
}

func (Repository) ListActive(ctx context.Context, db platform.DBTX) ([]User, error) {
	rows, err := db.Query(ctx, `SELECT `+userColumns+` FROM users WHERE deleted_at IS NULL ORDER BY name, email`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	activeUsers, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (User, error) {
		return scanUser(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return activeUsers, nil
}

func (Repository) ExistsByEmail(ctx context.Context, db platform.DBTX, normalizedEmail string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`, normalizedEmail).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user e-mail: %w", err)
	}
	return exists, nil
}

// RecordFailedLogin locks the account once the threshold is reached and restarts the count for the next window.
func (Repository) RecordFailedLogin(ctx context.Context, db platform.DBTX, id int64, threshold int, lockUntil time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE users SET
			failed_login_count = CASE WHEN failed_login_count + 1 >= $2 THEN 0 ELSE failed_login_count + 1 END,
			locked_until = CASE WHEN failed_login_count + 1 >= $2 THEN $3 ELSE locked_until END,
			updated_at = now()
		WHERE id = $1`, id, threshold, lockUntil)
	if err != nil {
		return fmt.Errorf("record failed login: %w", err)
	}
	return nil
}

func (Repository) RecordSuccessfulLogin(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = $2, updated_at = now()
		WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("record successful login: %w", err)
	}
	return nil
}

func (Repository) UpdatePasswordHash(ctx context.Context, db platform.DBTX, id int64, passwordHash string) error {
	_, err := db.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	return nil
}

func scanSingleUser(row pgx.Row) (User, error) {
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Token, &user.Email, &user.Name, &user.PasswordHash, &user.RoleID,
		&user.FailedLoginCount, &user.LockedUntil, &user.LastLoginAt, &user.CreatedAt)
	return user, err
}
