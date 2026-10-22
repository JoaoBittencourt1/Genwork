package sessions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users/userstest"
)

var sessionTime = time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)

func startSession(t *testing.T, db platform.DBTX, userID int64) sessions.Session {
	t.Helper()
	session, err := sessions.Repository{}.Create(context.Background(), db, sessions.NewSession{
		UserID: userID, ClientType: sessions.ClientNative, Now: sessionTime,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session
}

func TestRevokeAllForUserRevokesOnlyThatUsersSessions(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	ana := userstest.CreateUser(t, db, "ana@example.com", "member")
	bruno := userstest.CreateUser(t, db, "bruno@example.com", "member")
	anaPhone, anaLaptop := startSession(t, db, ana.ID), startSession(t, db, ana.ID)
	brunoPhone := startSession(t, db, bruno.ID)
	repository := sessions.Repository{}

	if err := repository.RevokeAllForUser(ctx, db, ana.ID, sessionTime); err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	for _, revoked := range []sessions.Session{anaPhone, anaLaptop} {
		if _, err := repository.FindActiveByToken(ctx, db, revoked.Token, sessionTime); !errors.Is(err, sessions.ErrSessionNotFound) {
			t.Errorf("revoked session %s error = %v", revoked.Token, err)
		}
	}
	if _, err := repository.FindActiveByToken(ctx, db, brunoPhone.Token, sessionTime); err != nil {
		t.Errorf("other user's session should stay active: %v", err)
	}
}

func TestFindActiveByTokenIgnoresDeactivatedUsers(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	ana := userstest.CreateUser(t, db, "ana@example.com", "member")
	session := startSession(t, db, ana.ID)
	if err := (users.Repository{}).UpdateDeactivatedAt(ctx, db, ana.ID, &sessionTime); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	_, err := sessions.Repository{}.FindActiveByToken(ctx, db, session.Token, sessionTime)

	if !errors.Is(err, sessions.ErrSessionNotFound) {
		t.Errorf("session of a deactivated user error = %v", err)
	}
}
