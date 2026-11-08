package accounts

import (
	"strings"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// SubmitSignInCodeRequest is the code a person copied from the provider's page after authorizing.
type SubmitSignInCodeRequest struct {
	Code string `json:"code"`
}

func (request SubmitSignInCodeRequest) Validate() (string, error) {
	code := strings.TrimSpace(request.Code)
	if !IsValidSignInCode(code) {
		return "", httpapi.NewValidationError(map[string]string{"code": "invalid"})
	}
	return code, nil
}

// ReportSignInProgressRequest is a runner's report on the CLI it runs for a sign-in.
type ReportSignInProgressRequest struct {
	State       string `json:"state"`
	URL         string `json:"url"`
	UserCode    string `json:"userCode"`
	ExpectsCode bool   `json:"expectsCode"`
	Message     string `json:"message"`
}

func (request ReportSignInProgressRequest) Validate() (SignInProgress, error) {
	progress := SignInProgress{
		State: SignInState(request.State), URL: strings.TrimSpace(request.URL), UserCode: strings.TrimSpace(request.UserCode),
		ExpectsCode: request.ExpectsCode, Message: request.Message,
	}
	switch progress.State {
	case SignInAwaitingUser, SignInVerifying, SignInSucceeded, SignInFailed:
		return progress, nil
	}
	return SignInProgress{}, httpapi.NewValidationError(map[string]string{"state": "invalid"})
}
