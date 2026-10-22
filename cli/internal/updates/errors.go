package updates

import (
	"errors"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
)

// Error codes of the update engine; the updater API returns them and the web app translates them (spec "platform-updates").
const (
	CodeNoNewerRelease    = "no_newer_release"
	CodeVersionMismatch   = "version_mismatch"
	CodeUpdateInProgress  = "update_in_progress"
	CodeUpdateInterrupted = "update_interrupted"
	CodeDownloadFailed    = "download_failed"
	CodePullFailed        = "pull_failed"
	CodeBackupFailed      = "backup_failed"
	CodeSwapFailed        = "swap_failed"
	CodeMigrationFailed   = "migration_failed"
	CodeVerifyFailed      = "verification_failed"
	// ReasonDevelopmentInstallation is why a repository checkout is not updatable: it updates through git.
	ReasonDevelopmentInstallation = "development_installation"
)

type UpdateError struct {
	Code  string
	cause error
}

func newUpdateError(code string, cause error) *UpdateError {
	return &UpdateError{Code: code, cause: cause}
}

func (err *UpdateError) Error() string {
	if err.cause == nil {
		return err.Code
	}
	return err.Code + ": " + err.cause.Error()
}

func (err *UpdateError) Unwrap() error { return err.cause }

// ErrorCode returns the stable code of an update or release error, or "" for any other error.
func ErrorCode(err error) string {
	var updateError *UpdateError
	if errors.As(err, &updateError) {
		return updateError.Code
	}
	return releases.ErrorCode(err)
}
