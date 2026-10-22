package app_test

import (
	"context"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

func (app *testApp) restrictModules(user users.User, moduleNames ...string) {
	app.t.Helper()
	err := access.ModuleRestrictionRepository{}.Replace(context.Background(), app.db, user.ID, moduleNames, app.clock.Now())
	if err != nil {
		app.t.Fatalf("restrict modules %v: %v", moduleNames, err)
	}
}

func (app *testApp) deactivateDirectly(user users.User) {
	app.t.Helper()
	deactivatedAt := app.clock.Now()
	if err := (users.Repository{}).UpdateDeactivatedAt(context.Background(), app.db, user.ID, &deactivatedAt); err != nil {
		app.t.Fatalf("deactivate user: %v", err)
	}
}

func (app *testApp) countSessions(user users.User) int {
	app.t.Helper()
	var count int
	if err := app.db.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE user_id = $1`, user.ID).Scan(&count); err != nil {
		app.t.Fatalf("count sessions: %v", err)
	}
	return count
}

func (app *testApp) meBody(accessToken string) meBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: accessToken})
	expectStatus(app.t, resp, http.StatusOK)
	var body meBody
	resp.decode(app.t, &body)
	return body
}

type meBody struct {
	Permissions []string `json:"permissions"`
}

func (app *testApp) countUnrevokedSessions(user users.User) int {
	app.t.Helper()
	var count int
	err := app.db.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, user.ID).Scan(&count)
	if err != nil {
		app.t.Fatalf("count unrevoked sessions: %v", err)
	}
	return count
}

func (app *testApp) removeDirectly(user users.User) {
	app.t.Helper()
	if err := (users.Repository{}).SoftDelete(context.Background(), app.db, user.ID, app.clock.Now()); err != nil {
		app.t.Fatalf("remove user: %v", err)
	}
}

func (app *testApp) storedUser(user users.User) users.User {
	app.t.Helper()
	stored, err := users.Repository{}.FindByID(context.Background(), app.db, user.ID)
	if err != nil {
		app.t.Fatalf("find user: %v", err)
	}
	return stored
}

type memberBody struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	AvatarColor string `json:"avatarColor"`
	Role        struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"role"`
	Status            string     `json:"status"`
	JoinedAt          time.Time  `json:"joinedAt"`
	LastSignInAt      *time.Time `json:"lastSignInAt"`
	RestrictedModules []string   `json:"restrictedModules"`
}

func (app *testApp) listMembers(accessToken string) []memberBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: "/api/core/v1/members", accessToken: accessToken})
	expectStatus(app.t, resp, http.StatusOK)
	var body struct {
		Members []memberBody `json:"members"`
	}
	resp.decode(app.t, &body)
	return body.Members
}

func (app *testApp) findMember(accessToken, memberToken string) memberBody {
	app.t.Helper()
	for _, member := range app.listMembers(accessToken) {
		if member.Token == memberToken {
			return member
		}
	}
	app.t.Fatalf("member %s not listed", memberToken)
	return memberBody{}
}

func (app *testApp) patchMember(accessToken, memberToken string, body any) response {
	app.t.Helper()
	return app.do(request{method: "PATCH", path: "/api/core/v1/members/" + memberToken, accessToken: accessToken, body: body})
}

func (app *testApp) removeMember(accessToken, memberToken string) response {
	app.t.Helper()
	return app.do(request{method: "DELETE", path: "/api/core/v1/members/" + memberToken, accessToken: accessToken})
}
