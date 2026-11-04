package passwordreset

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

const (
	Lifetime = time.Hour
	// RequestInterval bounds how often one account can be sent a reset e-mail.
	RequestInterval = time.Minute
)

var ErrResetNotFound = platform.NewAPIError(http.StatusNotFound, "password_reset_not_found")

type Reset struct {
	ID        int64
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}
