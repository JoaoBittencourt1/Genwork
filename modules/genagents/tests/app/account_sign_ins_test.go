package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
)

const (
	anthropicSignInURL = "https://claude.ai/oauth/authorize?code=true&client_id=fixture&state=fixture-state"
	pastedSignInCode   = "anthropic-pasted-code#fixture-state"
)

type signInBody struct {
	Token       string    `json:"token"`
	State       string    `json:"state"`
	URL         *string   `json:"url"`
	URLHost     *string   `json:"urlHost"`
	UserCode    *string   `json:"userCode"`
	ExpectsCode bool      `json:"expectsCode"`
	Failure     *string   `json:"failure"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type runnerSignInBody struct {
	Token        string `json:"token"`
	AccountToken string `json:"accountToken"`
	Provider     string `json:"provider"`
	State        string `json:"state"`
	Code         string `json:"code"`
}

// subscriptionAccountOn creates a Claude subscription account held by a registered runner.
func (app *testApp) subscriptionAccountOn(runner testRunner) accountBody {
	app.t.Helper()
	resp := app.createAccount(map[string]any{"name": "Claude Max", "provider": "anthropic", "mode": "subscription", "runnerToken": runner.Token})
	expectStatus(app.t, resp, http.StatusCreated)
	var account accountBody
	resp.decode(app.t, &account)
	return account
}

func (app *testApp) startSignIn(accountToken string) signInBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/genagents/v1/accounts/" + accountToken + "/sign-ins"})
	expectStatus(app.t, resp, http.StatusCreated)
	var signIn signInBody
	resp.decode(app.t, &signIn)
	return signIn
}

func (app *testApp) runnerSignIns(runner testRunner) []runnerSignInBody {
	app.t.Helper()
	resp := app.runnerDo(runner.Secret, "GET", "/api/genagents/v1/runner/sign-ins", nil)
	expectStatus(app.t, resp, http.StatusOK)
	if resp.header("Cache-Control") != "no-store" {
		app.t.Errorf("the runner's sign-ins can be cached: %q", resp.header("Cache-Control"))
	}
	var body struct {
		SignIns []runnerSignInBody `json:"signIns"`
	}
	resp.decode(app.t, &body)
	return body.SignIns
}

func (app *testApp) reportSignIn(runner testRunner, signInToken string, body map[string]any) response {
	app.t.Helper()
	return app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/sign-ins/"+signInToken+"/progress", body)
}

func (app *testApp) submitSignInCode(accountToken, signInToken, code string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: "/api/genagents/v1/accounts/" + accountToken + "/sign-ins/" + signInToken + "/code", body: map[string]any{"code": code}})
}

// awaitingCode starts a sign-in and has the runner relay Anthropic's URL, as the CLI prints it.
func (app *testApp) awaitingCode() (testRunner, accountBody, signInBody) {
	app.t.Helper()
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	account := app.subscriptionAccountOn(runner)
	signIn := app.startSignIn(account.Token)
	expectNoContent(app.t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "awaiting_user", "url": anthropicSignInURL, "expectsCode": true}))
	return runner, account, signIn
}

func (app *testApp) storedCodeCiphertext(signInToken string) []byte {
	app.t.Helper()
	var ciphertext []byte
	if err := app.db.QueryRow(context.Background(), `SELECT code_ciphertext FROM account_sign_ins WHERE token = $1`, signInToken).Scan(&ciphertext); err != nil {
		app.t.Fatalf("read code: %v", err)
	}
	return ciphertext
}

func TestStartingASignInRequestsItFromTheAccountsRunner(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	account := app.subscriptionAccountOn(runner)

	signIn := app.startSignIn(account.Token)

	if signIn.State != "requested" || signIn.URL != nil || !signIn.ExpiresAt.Equal(app.clock.Now().Add(10*time.Minute)) {
		t.Errorf("sign-in = %+v", signIn)
	}
	listed := app.runnerSignIns(runner)
	if len(listed) != 1 || listed[0].Token != signIn.Token || listed[0].AccountToken != account.Token || listed[0].Provider != "anthropic" || listed[0].Code != "" {
		t.Errorf("runner sign-ins = %+v", listed)
	}
	got := app.findAccount(account.Token)
	if got.Attempt == nil || got.Attempt.Token != signIn.Token || got.SignIn == nil {
		t.Errorf("the account does not show the attempt and the fallback command: %+v", got)
	}
}

func TestAPIKeyAccountsHaveNoSignIn(t *testing.T) {
	app := newTestApp(t)
	account := app.createAPIKeyAccount("Claude API", "anthropic", "sk-ant-secret-key")

	resp := app.do(request{method: "POST", path: "/api/genagents/v1/accounts/" + account.Token + "/sign-ins"})

	expectValidationError(t, resp, "mode")
	if got := app.findAccount(account.Token); got.Status != "ready" || got.Attempt != nil {
		t.Errorf("the API-key account changed: %+v", got)
	}
}

func TestSignInRoutesNeedTheAccountPermission(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	account := app.subscriptionAccountOn(runner)
	member := app.tokenWith(access.PermissionBotRead)

	resp := app.do(request{method: "POST", path: "/api/genagents/v1/accounts/" + account.Token + "/sign-ins", accessToken: member})

	expectStatus(t, resp, http.StatusForbidden)
	expectStatus(t, app.runnerDo("not-a-runner-secret", "GET", "/api/genagents/v1/runner/sign-ins", nil), http.StatusUnauthorized)
}

func TestTheRunnerRelaysTheURLAndThePageShowsItsHost(t *testing.T) {
	app := newTestApp(t)

	_, account, _ := app.awaitingCode()

	got := app.findAccount(account.Token).Attempt
	if got.State != "awaiting_user" || got.URL == nil || *got.URL != anthropicSignInURL || got.URLHost == nil || *got.URLHost != "claude.ai" || !got.ExpectsCode {
		t.Errorf("sign-in = %+v", got)
	}
}

func TestOnlyHTTPSURLsAreRelayed(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"claude"}})
	signIn := app.startSignIn(app.subscriptionAccountOn(runner).Token)

	resp := app.reportSignIn(runner, signIn.Token, map[string]any{"state": "awaiting_user", "url": "http://phishing.example/login"})

	expectValidationError(t, resp, "url")
}

func TestThePastedCodeReachesOnlyTheRunnerAndIsDroppedOnceTaken(t *testing.T) {
	app := newTestApp(t)
	runner, account, signIn := app.awaitingCode()

	resp := app.submitSignInCode(account.Token, signIn.Token, "  "+pastedSignInCode+"  ")

	expectStatus(t, resp, http.StatusOK)
	if strings.Contains(string(resp.body), pastedSignInCode) {
		t.Error("the submit response echoes the code")
	}
	stored := app.storedCodeCiphertext(signIn.Token)
	if len(stored) == 0 || strings.Contains(string(stored), pastedSignInCode) {
		t.Error("the code is not stored encrypted")
	}
	listing := app.do(request{method: "GET", path: "/api/genagents/v1/accounts"})
	if strings.Contains(string(listing.body), pastedSignInCode) {
		t.Error("the account list carries the code")
	}
	listed := app.runnerSignIns(runner)
	if len(listed) != 1 || listed[0].State != "code_submitted" || listed[0].Code != pastedSignInCode {
		t.Fatalf("runner sign-ins = %+v", listed)
	}
	expectNoContent(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "verifying"}))
	if app.storedCodeCiphertext(signIn.Token) != nil {
		t.Error("the code outlived the runner taking it")
	}
	if listed := app.runnerSignIns(runner); len(listed) != 1 || listed[0].Code != "" {
		t.Errorf("runner sign-ins after verifying = %+v", listed)
	}
}

func TestASuccessfulSignInMakesTheAccountReady(t *testing.T) {
	app := newTestApp(t)
	runner, account, signIn := app.awaitingCode()
	expectStatus(t, app.submitSignInCode(account.Token, signIn.Token, pastedSignInCode), http.StatusOK)
	expectNoContent(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "verifying"}))

	expectNoContent(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "succeeded"}))

	got := app.findAccount(account.Token)
	if got.Status != "ready" || got.LastError != nil || got.Attempt.State != "succeeded" || got.SignIn != nil {
		t.Errorf("account = %+v", got)
	}
	if len(app.runnerSignIns(runner)) != 0 {
		t.Error("a finished sign-in is still listed for the runner")
	}
}

func TestAFailedSignInShowsTheErrorOnTheAccount(t *testing.T) {
	app := newTestApp(t)
	runner, account, signIn := app.awaitingCode()

	expectNoContent(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "failed", "message": "OAuth error: Invalid code"}))

	got := app.findAccount(account.Token)
	if got.Status != "error" || got.LastError == nil || *got.LastError != "OAuth error: Invalid code" || got.Attempt.State != "failed" {
		t.Errorf("account = %+v", got)
	}
	if got.SignIn == nil {
		t.Error("the fallback command is gone from an account in error")
	}
}

func TestACancelledSignInKeepsTheAccountStatus(t *testing.T) {
	app := newTestApp(t)
	runner, account, signIn := app.awaitingCode()

	expectNoContent(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/accounts/" + account.Token + "/sign-ins/" + signIn.Token}))

	got := app.findAccount(account.Token)
	if got.Status != "pending_sign_in" || got.Attempt.State != "cancelled" {
		t.Errorf("account = %+v", got)
	}
	if len(app.runnerSignIns(runner)) != 0 {
		t.Error("the runner still sees a cancelled sign-in")
	}
	expectError(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "succeeded"}), http.StatusConflict, "sign_in_not_active")
	if got := app.findAccount(account.Token); got.Status != "pending_sign_in" {
		t.Errorf("a late report changed the account: %+v", got)
	}
}

func TestAnUnfinishedSignInExpiresAndMarksTheAccount(t *testing.T) {
	app := newTestApp(t)
	runner, account, signIn := app.awaitingCode()

	app.clock.Advance(10*time.Minute + time.Second)

	got := app.findAccount(account.Token)
	if got.Attempt.State != "expired" || got.Status != "error" || got.LastError == nil || *got.LastError != "sign_in_expired" {
		t.Errorf("account = %+v", got)
	}
	expectError(t, app.submitSignInCode(account.Token, signIn.Token, pastedSignInCode), http.StatusConflict, "sign_in_not_active")
	if len(app.runnerSignIns(runner)) != 0 {
		t.Error("the runner still sees an expired sign-in")
	}
}

func TestStartingAgainReplacesTheActiveSignIn(t *testing.T) {
	app := newTestApp(t)
	runner, account, first := app.awaitingCode()

	second := app.startSignIn(account.Token)

	listed := app.runnerSignIns(runner)
	if len(listed) != 1 || listed[0].Token != second.Token {
		t.Errorf("runner sign-ins = %+v", listed)
	}
	expectError(t, app.reportSignIn(runner, first.Token, map[string]any{"state": "succeeded"}), http.StatusConflict, "sign_in_not_active")
}

func TestACodeIsOnlyAcceptedWhileTheCLIWaitsForOne(t *testing.T) {
	app := newTestApp(t)
	runner := app.registerRunner("build-box", heartbeatOptions{tools: []string{"codex"}})
	resp := app.createAccount(map[string]any{"name": "Codex", "provider": "openai", "mode": "subscription", "runnerToken": runner.Token})
	var account accountBody
	resp.decode(t, &account)
	signIn := app.startSignIn(account.Token)

	expectError(t, app.submitSignInCode(account.Token, signIn.Token, "early-code"), http.StatusConflict, "sign_in_not_awaiting_code")
	expectNoContent(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "awaiting_user", "url": "https://auth.openai.com/codex/device", "userCode": "ABCD-12345"}))
	expectError(t, app.submitSignInCode(account.Token, signIn.Token, "device-flows-take-no-code"), http.StatusConflict, "sign_in_not_awaiting_code")
	expectValidationError(t, app.submitSignInCode(account.Token, signIn.Token, "has spaces"), "code")
	if got := app.findAccount(account.Token).Attempt; got.UserCode == nil || *got.UserCode != "ABCD-12345" || got.ExpectsCode {
		t.Errorf("sign-in = %+v", got)
	}
}

func TestARunnerCannotDriveAnotherRunnersSignIn(t *testing.T) {
	app := newTestApp(t)
	_, _, signIn := app.awaitingCode()
	other := app.registerRunner("other-box", heartbeatOptions{tools: []string{"claude"}})

	if listed := app.runnerSignIns(other); len(listed) != 0 {
		t.Errorf("another runner sees the sign-in: %+v", listed)
	}
	expectError(t, app.reportSignIn(other, signIn.Token, map[string]any{"state": "succeeded"}), http.StatusNotFound, "sign_in_not_found")
}

func TestInvalidProgressStatesAreRejected(t *testing.T) {
	app := newTestApp(t)
	runner, _, signIn := app.awaitingCode()

	expectValidationError(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "cancelled"}), "state")
	expectError(t, app.reportSignIn(runner, signIn.Token, map[string]any{"state": "verifying"}), http.StatusConflict, "sign_in_not_awaiting_code")
}
