package app_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

const (
	passwordResetPath   = "/api/core/v1/auth/password-reset"
	newTestPassword     = "another battery staple"
	passwordResetLinkRe = `/reset-password/([A-Za-z0-9_-]+)`
)

func (app *testApp) requestPasswordReset(email string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: passwordResetPath, body: map[string]any{"email": email}})
}

func (app *testApp) confirmPasswordReset(secret, password string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: passwordResetPath + "/" + secret, body: map[string]any{"password": password}})
}

// waitForResetSecret polls because the e-mail is delivered in the background.
func (app *testApp) waitForResetSecret() string {
	app.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if message, sent := app.mailer.Last(); sent {
			match := regexp.MustCompile(passwordResetLinkRe).FindStringSubmatch(message.Text)
			if match == nil {
				app.t.Fatalf("e-mail without reset link: %s", message.Text)
			}
			return match[1]
		}
		time.Sleep(10 * time.Millisecond)
	}
	app.t.Fatal("password reset e-mail was not sent")
	return ""
}

func (app *testApp) expectNoEmail() {
	app.t.Helper()
	time.Sleep(100 * time.Millisecond)
	if messages := app.mailer.Messages(); len(messages) != 0 {
		app.t.Fatalf("unexpected e-mail: %+v", messages)
	}
}

func (app *testApp) loginWith(email, password string) response {
	return app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": email, "password": password, "client": "native",
	}})
}

func TestPasswordResetEmailsLinkAndChangesPassword(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	session := app.loginNative("ana@example.com")

	expectStatus(t, app.requestPasswordReset(" Ana@Example.com "), http.StatusNoContent)
	secret := app.waitForResetSecret()
	message, _ := app.mailer.Last()
	if message.To != "ana@example.com" || !strings.Contains(message.Text, testPublicURL+"/reset-password/"+secret) || !strings.Contains(message.Subject, testInstanceName) {
		t.Errorf("e-mail = %+v", message)
	}
	var storedHash []byte
	if err := app.db.QueryRow(context.Background(), `SELECT secret_hash FROM password_resets`).Scan(&storedHash); err != nil {
		t.Fatalf("read reset: %v", err)
	}
	if strings.Contains(string(storedHash), secret) {
		t.Error("the reset secret must be stored only as a hash")
	}

	expectStatus(t, app.confirmPasswordReset(secret, newTestPassword), http.StatusNoContent)
	expectStatus(t, app.loginWith("ana@example.com", newTestPassword), http.StatusOK)
	expectError(t, app.loginWith("ana@example.com", testPassword), http.StatusUnauthorized, "invalid_credentials")
	expectError(t, app.do(request{method: "POST", path: "/api/core/v1/auth/refresh", body: map[string]any{"refreshToken": session.RefreshToken}}),
		http.StatusUnauthorized, "session_revoked")
}

func TestPasswordResetLinkWorksOnlyOnce(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	app.requestPasswordReset("ana@example.com")
	secret := app.waitForResetSecret()

	expectStatus(t, app.confirmPasswordReset(secret, newTestPassword), http.StatusNoContent)
	expectError(t, app.confirmPasswordReset(secret, "yet another password"), http.StatusNotFound, "password_reset_not_found")
}

func TestPasswordResetLinkExpires(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	app.requestPasswordReset("ana@example.com")
	secret := app.waitForResetSecret()

	app.clock.Advance(time.Hour + time.Minute)
	expectError(t, app.confirmPasswordReset(secret, newTestPassword), http.StatusNotFound, "password_reset_not_found")
}

func TestPasswordResetRejectsUnknownSecretAndShortPassword(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	app.requestPasswordReset("ana@example.com")
	secret := app.waitForResetSecret()

	expectError(t, app.confirmPasswordReset("unknown", newTestPassword), http.StatusNotFound, "password_reset_not_found")
	expectError(t, app.confirmPasswordReset(secret, "short"), http.StatusBadRequest, "password_length_invalid")
	expectStatus(t, app.confirmPasswordReset(secret, newTestPassword), http.StatusNoContent)
}

func TestPasswordResetRequestHidesWhetherAccountExists(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")

	expectStatus(t, app.requestPasswordReset("nobody@example.com"), http.StatusNoContent)
	app.expectNoEmail()
	expectError(t, app.requestPasswordReset("not-an-email"), http.StatusBadRequest, "validation_failed")
}

func TestPasswordResetIsNotSentToDeactivatedAccounts(t *testing.T) {
	app := newTestApp(t)
	user := app.createUser("ana@example.com", "Ana", "member")
	if _, err := app.db.Exec(context.Background(), `UPDATE users SET deactivated_at = now() WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	expectStatus(t, app.requestPasswordReset("ana@example.com"), http.StatusNoContent)
	app.expectNoEmail()
}

func TestPasswordResetThrottlesRepeatedRequestsAndInvalidatesTheOlderLink(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	app.requestPasswordReset("ana@example.com")
	first := app.waitForResetSecret()

	app.requestPasswordReset("ana@example.com")
	app.expectCount(t, 1)

	app.clock.Advance(2 * time.Minute)
	app.requestPasswordReset("ana@example.com")
	app.expectCount(t, 2)
	expectError(t, app.confirmPasswordReset(first, newTestPassword), http.StatusNotFound, "password_reset_not_found")
}

func (app *testApp) expectCount(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(app.mailer.Messages()) < want {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if got := len(app.mailer.Messages()); got != want {
		t.Fatalf("e-mails sent = %d, want %d", got, want)
	}
}

func TestPasswordResetClearsLockout(t *testing.T) {
	app := newTestApp(t)
	app.createUser("ana@example.com", "Ana", "member")
	for range 10 {
		app.loginWith("ana@example.com", "wrong password")
	}
	expectError(t, app.loginWith("ana@example.com", testPassword), http.StatusUnauthorized, "invalid_credentials")

	app.requestPasswordReset("ana@example.com")
	expectStatus(t, app.confirmPasswordReset(app.waitForResetSecret(), newTestPassword), http.StatusNoContent)
	expectStatus(t, app.loginWith("ana@example.com", newTestPassword), http.StatusOK)
}
