package releases

import (
	"errors"
	"fmt"
)

// Error codes the CLI prints and the updater returns; the web app translates them (spec "platform-updates").
const (
	CodeChannelUnreachable = "release_channel_unreachable"
	CodeChannelRateLimited = "release_channel_rate_limited"
	CodeChecksumMismatch   = "checksum_mismatch"
	CodeNoPublishedRelease = "no_published_release"
)

type ChannelError struct {
	Code  string
	cause error
}

func newChannelError(code string, cause error) *ChannelError {
	return &ChannelError{Code: code, cause: cause}
}

func (err *ChannelError) Error() string { return err.Code + ": " + err.cause.Error() }
func (err *ChannelError) Unwrap() error { return err.cause }

// ChecksumMismatchError names the asset that does not match checksums.txt, or that the file does not list.
type ChecksumMismatchError struct {
	Asset  string
	Reason string
}

func (err *ChecksumMismatchError) Error() string {
	return fmt.Sprintf("%s: %s %s", CodeChecksumMismatch, err.Asset, err.Reason)
}

// ErrorCode returns the stable code of a release error, or "" for any other error.
func ErrorCode(err error) string {
	var channelError *ChannelError
	var mismatch *ChecksumMismatchError
	switch {
	case errors.As(err, &channelError):
		return channelError.Code
	case errors.As(err, &mismatch):
		return CodeChecksumMismatch
	case errors.Is(err, ErrNoPublishedRelease):
		return CodeNoPublishedRelease
	default:
		return ""
	}
}
