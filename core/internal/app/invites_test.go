package app

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

type deliveryBody struct {
	Invite struct {
		Token  string `json:"token"`
		Email  string `json:"email"`
		Status string `json:"status"`
		Role   struct {
			Name string `json:"name"`
		} `json:"role"`
		InvitedBy struct {
			Name string `json:"name"`
		} `json:"invitedBy"`
		ExpiresAt time.Time `json:"expiresAt"`
	} `json:"invite"`
	Link      string `json:"link"`
	EmailSent bool   `json:"emailSent"`
}

func (delivery deliveryBody) secret() string {
	return strings.TrimPrefix(delivery.Link, testPublicURL+"/invite/")
}

func (app *testApp) signedInAdmin() signInBody {
	app.t.Helper()
	app.createUser("ana@example.com", "Ana", "admin")
	return app.loginNative("ana@example.com")
}

func (app *testApp) invite(accessToken, email, roleName string) (response, deliveryBody) {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/core/v1/invites", accessToken: accessToken, body: map[string]any{
		"email": email, "role": app.roleToken(roleName),
	}})
	var delivery deliveryBody
	if resp.status == http.StatusCreated {
		resp.decode(app.t, &delivery)
	}
	return resp, delivery
}

func TestCreateInviteSendsEmailAndReturnsLink(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()

	resp, delivery := app.invite(admin.AccessToken, " Bruno@Example.com ", "member")
	expectStatus(t, resp, http.StatusCreated)
	if !delivery.EmailSent || !strings.HasPrefix(delivery.Link, testPublicURL+"/invite/") || len(delivery.secret()) < 40 {
		t.Fatalf("delivery = %+v", delivery)
	}
	if delivery.Invite.Email != "bruno@example.com" || delivery.Invite.Status != "pending" || delivery.Invite.Role.Name != "member" || delivery.Invite.InvitedBy.Name != "Ana" {
		t.Errorf("invite = %+v", delivery.Invite)
	}
	if !delivery.Invite.ExpiresAt.Equal(app.clock.Now().Add(7 * 24 * time.Hour)) {
		t.Errorf("expiresAt = %s", delivery.Invite.ExpiresAt)
	}
	message, sent := app.mailer.Last()
	if !sent || message.To != "bruno@example.com" || !strings.Contains(message.Text, delivery.Link) || !strings.Contains(message.Subject, "convidou") {
		t.Errorf("e-mail = %+v", message)
	}

	var storedHash []byte
	if err := app.db.QueryRow(context.Background(), `SELECT secret_hash FROM invites`).Scan(&storedHash); err != nil {
		t.Fatalf("read invite: %v", err)
	}
	if strings.Contains(string(storedHash), delivery.secret()) {
		t.Error("the invite secret must be stored only as a hash")
	}
}

func TestInviteIsCreatedEvenWhenEmailFails(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	app.mailer.FailDeliveries(true)

	resp, delivery := app.invite(admin.AccessToken, "bruno@example.com", "member")
	expectStatus(t, resp, http.StatusCreated)
	if delivery.EmailSent || delivery.Link == "" {
		t.Errorf("delivery = %+v", delivery)
	}
	inspect := app.do(request{method: "GET", path: "/api/core/v1/invitations/" + delivery.secret()})
	expectStatus(t, inspect, http.StatusOK)
}

func TestInviteConflicts(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()

	registered, _ := app.invite(admin.AccessToken, "ana@example.com", "member")
	expectError(t, registered, http.StatusConflict, "email_already_registered")

	first, _ := app.invite(admin.AccessToken, "bruno@example.com", "member")
	expectStatus(t, first, http.StatusCreated)
	duplicate, _ := app.invite(admin.AccessToken, "BRUNO@example.com", "member")
	expectError(t, duplicate, http.StatusConflict, "invite_already_pending")

	app.clock.Advance(7*24*time.Hour + time.Minute)
	admin = app.loginNative("ana@example.com")
	afterExpiry, _ := app.invite(admin.AccessToken, "bruno@example.com", "member")
	expectStatus(t, afterExpiry, http.StatusCreated)
}

func TestInviteValidatesInput(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()

	badEmail := app.do(request{method: "POST", path: "/api/core/v1/invites", accessToken: admin.AccessToken, body: map[string]any{
		"email": "nope", "role": app.roleToken("member"),
	}})
	expectError(t, badEmail, http.StatusBadRequest, "validation_failed")

	badRole := app.do(request{method: "POST", path: "/api/core/v1/invites", accessToken: admin.AccessToken, body: map[string]any{
		"email": "bruno@example.com", "role": "unknown-role",
	}})
	expectError(t, badRole, http.StatusBadRequest, "validation_failed")
}

func TestInvitePermissions(t *testing.T) {
	app := newTestApp(t)
	app.createUser("bruno@example.com", "Bruno", "member")
	member := app.loginNative("bruno@example.com")

	resp, _ := app.invite(member.AccessToken, "carla@example.com", "member")
	expectError(t, resp, http.StatusForbidden, "forbidden")
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/invites", accessToken: member.AccessToken}), http.StatusForbidden, "forbidden")
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/roles", accessToken: member.AccessToken}), http.StatusForbidden, "forbidden")
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/invites"}), http.StatusUnauthorized, "invalid_token")
}

func TestCannotInviteWithMorePowerfulRole(t *testing.T) {
	app := newTestApp(t)
	app.createRole("inviter", access.PermissionInviteCreate)
	app.createUser("dora@example.com", "Dora", "inviter")
	inviter := app.loginNative("dora@example.com")

	escalation, _ := app.invite(inviter.AccessToken, "eva@example.com", "admin")
	expectError(t, escalation, http.StatusForbidden, "role_not_assignable")
	allowed, _ := app.invite(inviter.AccessToken, "eva@example.com", "member")
	expectStatus(t, allowed, http.StatusCreated)
}

func TestListRolesForInviters(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	resp := app.do(request{method: "GET", path: "/api/core/v1/roles", accessToken: admin.AccessToken})
	expectStatus(t, resp, http.StatusOK)
	var body struct {
		Roles []struct {
			Name string `json:"name"`
		} `json:"roles"`
	}
	resp.decode(t, &body)
	if len(body.Roles) != 2 || body.Roles[0].Name != "admin" || body.Roles[1].Name != "member" {
		t.Errorf("roles = %+v", body.Roles)
	}
}

func TestListInvitesFiltersByDerivedStatus(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	app.invite(admin.AccessToken, "old@example.com", "member")
	app.clock.Advance(7*24*time.Hour + time.Minute)
	admin = app.loginNative("ana@example.com")
	app.invite(admin.AccessToken, "new@example.com", "member")

	list := func(status string) []string {
		path := "/api/core/v1/invites"
		if status != "" {
			path += "?status=" + status
		}
		resp := app.do(request{method: "GET", path: path, accessToken: admin.AccessToken})
		expectStatus(t, resp, http.StatusOK)
		var body struct {
			Invites []struct {
				Email string `json:"email"`
			} `json:"invites"`
		}
		resp.decode(t, &body)
		emails := []string{}
		for _, invite := range body.Invites {
			emails = append(emails, invite.Email)
		}
		return emails
	}
	if all := list(""); len(all) != 2 {
		t.Errorf("all = %v", all)
	}
	if pending := list("pending"); len(pending) != 1 || pending[0] != "new@example.com" {
		t.Errorf("pending = %v", pending)
	}
	if expired := list("expired"); len(expired) != 1 || expired[0] != "old@example.com" {
		t.Errorf("expired = %v", expired)
	}
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/invites?status=weird", accessToken: admin.AccessToken}), http.StatusBadRequest, "validation_failed")
}

func TestRevokeInvite(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	_, delivery := app.invite(admin.AccessToken, "bruno@example.com", "member")

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/core/v1/invites/" + delivery.Invite.Token, accessToken: admin.AccessToken}), http.StatusNoContent)
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/invitations/" + delivery.secret()}), http.StatusNotFound, "invite_not_found")
	expectError(t, app.do(request{method: "DELETE", path: "/api/core/v1/invites/" + delivery.Invite.Token, accessToken: admin.AccessToken}), http.StatusConflict, "invite_not_pending")
	expectError(t, app.do(request{method: "DELETE", path: "/api/core/v1/invites/unknown", accessToken: admin.AccessToken}), http.StatusNotFound, "invite_not_found")
}

func TestResendReplacesLink(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	_, original := app.invite(admin.AccessToken, "bruno@example.com", "member")
	app.clock.Advance(8 * 24 * time.Hour)
	admin = app.loginNative("ana@example.com")

	resp := app.do(request{method: "POST", path: "/api/core/v1/invites/" + original.Invite.Token + "/resend", accessToken: admin.AccessToken})
	expectStatus(t, resp, http.StatusOK)
	var resent deliveryBody
	resp.decode(t, &resent)
	if resent.Link == original.Link || !resent.EmailSent || resent.Invite.Status != "pending" {
		t.Fatalf("resent = %+v", resent)
	}
	if len(app.mailer.Messages()) != 2 {
		t.Errorf("e-mails sent = %d, want 2", len(app.mailer.Messages()))
	}
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/invitations/" + original.secret()}), http.StatusNotFound, "invite_not_found")
	expectStatus(t, app.do(request{method: "GET", path: "/api/core/v1/invitations/" + resent.secret()}), http.StatusOK)
}

func TestInspectInvite(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	_, delivery := app.invite(admin.AccessToken, "bruno@example.com", "member")

	resp := app.do(request{method: "GET", path: "/api/core/v1/invitations/" + delivery.secret()})
	expectStatus(t, resp, http.StatusOK)
	var body struct {
		InstanceName string    `json:"instanceName"`
		Email        string    `json:"email"`
		ExpiresAt    time.Time `json:"expiresAt"`
	}
	resp.decode(t, &body)
	if body.InstanceName != testInstanceName || body.Email != "bruno@example.com" || !body.ExpiresAt.Equal(delivery.Invite.ExpiresAt) {
		t.Errorf("inspection = %+v", body)
	}

	app.clock.Advance(7*24*time.Hour + time.Minute)
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/invitations/" + delivery.secret()}), http.StatusNotFound, "invite_not_found")
	expectError(t, app.do(request{method: "GET", path: "/api/core/v1/invitations/made-up"}), http.StatusNotFound, "invite_not_found")
}

func TestAcceptInviteCreatesAccountAndSignsIn(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	_, delivery := app.invite(admin.AccessToken, "bruno@example.com", "member")

	resp := app.do(request{method: "POST", path: "/api/core/v1/invitations/" + delivery.secret() + "/accept", body: map[string]any{
		"name": "Bruno", "password": testPassword, "client": "native", "deviceName": "Bruno's laptop",
	}})
	expectStatus(t, resp, http.StatusOK)
	var signIn signInBody
	resp.decode(t, &signIn)
	if signIn.RefreshToken == "" {
		t.Fatal("native acceptance must return the refresh token")
	}

	me := app.do(request{method: "GET", path: "/api/core/v1/me", accessToken: signIn.AccessToken})
	expectStatus(t, me, http.StatusOK)
	if !strings.Contains(string(me.body), `"email":"bruno@example.com"`) || !strings.Contains(string(me.body), `"name":"member"`) {
		t.Errorf("me = %s", me.body)
	}
	app.loginNative("bruno@example.com")

	listed := app.do(request{method: "GET", path: "/api/core/v1/invites?status=accepted", accessToken: admin.AccessToken})
	if !strings.Contains(string(listed.body), "bruno@example.com") {
		t.Errorf("accepted invites = %s", listed.body)
	}
	again := app.do(request{method: "POST", path: "/api/core/v1/invitations/" + delivery.secret() + "/accept", body: map[string]any{
		"name": "Mallory", "password": testPassword, "client": "native",
	}})
	expectError(t, again, http.StatusNotFound, "invite_not_found")
}

func TestAcceptInviteOnWebSetsCookie(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	_, delivery := app.invite(admin.AccessToken, "bruno@example.com", "member")

	resp := app.do(request{method: "POST", path: "/api/core/v1/invitations/" + delivery.secret() + "/accept", body: map[string]any{
		"name": "Bruno", "password": testPassword, "client": "web",
	}})
	expectStatus(t, resp, http.StatusOK)
	if resp.cookie(auth.RefreshCookieName) == nil || strings.Contains(string(resp.body), "refreshToken") {
		t.Errorf("web acceptance must use the cookie: %s", resp.body)
	}
}

func TestAcceptInviteValidatesPasswordWithoutConsumingInvite(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	_, delivery := app.invite(admin.AccessToken, "bruno@example.com", "member")

	short := app.do(request{method: "POST", path: "/api/core/v1/invitations/" + delivery.secret() + "/accept", body: map[string]any{
		"name": "Bruno", "password": "short", "client": "native",
	}})
	expectError(t, short, http.StatusBadRequest, "password_length_invalid")
	expectStatus(t, app.do(request{method: "GET", path: "/api/core/v1/invitations/" + delivery.secret()}), http.StatusOK)

	exists, _ := users.Repository{}.ExistsByEmail(context.Background(), app.db, "bruno@example.com")
	if exists {
		t.Error("no user must be created when validation fails")
	}
}

func TestConcurrentAcceptanceSucceedsOnce(t *testing.T) {
	app := newTestApp(t)
	admin := app.signedInAdmin()
	_, delivery := app.invite(admin.AccessToken, "bruno@example.com", "member")

	var waitGroup sync.WaitGroup
	results := make(chan response, 2)
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			results <- app.do(request{method: "POST", path: "/api/core/v1/invitations/" + delivery.secret() + "/accept", body: map[string]any{
				"name": "Bruno", "password": testPassword, "client": "native",
			}})
		}()
	}
	waitGroup.Wait()
	close(results)
	var succeeded, notFound int
	for result := range results {
		switch {
		case result.status == http.StatusOK:
			succeeded++
		case result.status == http.StatusNotFound && result.errorCode(t) == "invite_not_found":
			notFound++
		}
	}
	if succeeded != 1 || notFound != 1 {
		t.Errorf("succeeded=%d notFound=%d, want 1 and 1", succeeded, notFound)
	}
}

func TestNoHTTPRouteCreatesUsersWithoutInvite(t *testing.T) {
	app := newTestApp(t)
	for _, path := range []string{"/api/core/v1/users", "/api/core/v1/auth/register", "/api/core/v1/auth/signup", "/api/core/v1/setup"} {
		resp := app.do(request{method: "POST", path: path, body: map[string]any{"email": "x@example.com", "password": testPassword}})
		if resp.status != http.StatusNotFound && resp.status != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d", path, resp.status)
		}
	}
}
