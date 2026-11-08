package accounts

import (
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	// SignInLifetime bounds how long a provider CLI waits for the person; the runner stops the CLI at the same deadline.
	SignInLifetime       = 10 * time.Minute
	MaxSignInCodeLength  = 512
	MaxSignInURLLength   = 4096
	MaxUserCodeLength    = 64
	MaxSignInFailureRune = 500

	// SignInExpiredError is the account's last error when nobody finished a sign-in in time.
	SignInExpiredError = "sign_in_expired"
)

var (
	ErrSignInNotFound  = httpapi.NewAPIError(http.StatusNotFound, "sign_in_not_found")
	ErrSignInNotActive = httpapi.NewAPIError(http.StatusConflict, "sign_in_not_active")
	ErrSignInNoCode    = httpapi.NewAPIError(http.StatusConflict, "sign_in_not_awaiting_code")
	ErrSignInInFlight  = httpapi.NewAPIError(http.StatusConflict, "sign_in_in_flight")
)

type SignInState string

const (
	SignInRequested     SignInState = "requested"
	SignInAwaitingUser  SignInState = "awaiting_user"
	SignInCodeSubmitted SignInState = "code_submitted"
	SignInVerifying     SignInState = "verifying"
	SignInSucceeded     SignInState = "succeeded"
	SignInFailed        SignInState = "failed"
	SignInCancelled     SignInState = "cancelled"
	SignInExpired       SignInState = "expired"
)

// IsActive says whether the provider CLI may still be waiting on this sign-in.
func (state SignInState) IsActive() bool {
	return state == SignInRequested || state == SignInAwaitingUser || state == SignInCodeSubmitted || state == SignInVerifying
}

// AccountSignIn is one attempt to sign a subscription account in through its provider CLI on the runner. The code a
// person pastes is kept only encrypted and only until the runner takes it (rule 67).
type AccountSignIn struct {
	ID             int64
	Token          string
	AccountID      int64
	State          SignInState
	SignInURL      *string
	UserCode       *string
	ExpectsCode    bool
	CodeCiphertext []byte
	Failure        *string
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SignInURLHost is what the screens show next to the link, so a person sees which site they are about to sign in to.
func (signIn AccountSignIn) SignInURLHost() *string {
	if signIn.SignInURL == nil {
		return nil
	}
	parsed, err := url.Parse(*signIn.SignInURL)
	if err != nil {
		return nil
	}
	host := parsed.Host
	return &host
}

// IsValidSignInURL accepts only https links, so a sandbox cannot point the person at a plain-text page.
func IsValidSignInURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && len(raw) <= MaxSignInURLLength
}

// IsValidSignInCode accepts what provider pages show: printable characters without spaces.
func IsValidSignInCode(code string) bool {
	if code == "" || len(code) > MaxSignInCodeLength {
		return false
	}
	for _, character := range code {
		if unicode.IsSpace(character) || !unicode.IsPrint(character) {
			return false
		}
	}
	return true
}

func IsValidUserCode(code string) bool {
	return len(code) <= MaxUserCodeLength && (code == "" || IsValidSignInCode(code))
}

// TruncateFailure keeps a runner's failure message short enough to show on the account.
func TruncateFailure(message string) string {
	runes := []rune(strings.TrimSpace(message))
	if len(runes) > MaxSignInFailureRune {
		runes = runes[:MaxSignInFailureRune]
	}
	return string(runes)
}

// RunnerSignIn is what the runner holding an account needs to drive its sign-in; Code is set only once a person pasted it.
type RunnerSignIn struct {
	Token        string
	AccountToken string
	Provider     Provider
	State        SignInState
	Code         string
	ExpiresAt    time.Time
}

// SignInProgress is what a runner reports while the CLI runs.
type SignInProgress struct {
	State       SignInState
	URL         string
	UserCode    string
	ExpectsCode bool
	Message     string
}
