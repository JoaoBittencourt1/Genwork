package mail_test

import (
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/mail"
)

func TestRenderPasswordResetInEachLocale(t *testing.T) {
	expiresAt := time.Date(2026, 10, 20, 13, 0, 0, 0, time.UTC)
	for locale, wantSubject := range map[string]string{"pt-BR": "Redefina sua senha", "en": "Reset your"} {
		message, err := mail.RenderPasswordReset(mail.PasswordResetEmail{
			Locale: locale, To: "ana@example.com", InstanceName: "Genwork", Link: "https://x.test/reset-password/abc", ExpiresAt: expiresAt,
		})
		if err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
		if !strings.Contains(message.Subject, wantSubject) || !strings.Contains(message.Text, "https://x.test/reset-password/abc") || !strings.Contains(message.HTML, "https://x.test/reset-password/abc") {
			t.Errorf("%s: message = %+v", locale, message)
		}
	}
}

func TestRenderPasswordResetRejectsUnsupportedLocale(t *testing.T) {
	if _, err := mail.RenderPasswordReset(mail.PasswordResetEmail{Locale: "fr"}); err == nil {
		t.Error("expected an error for an unsupported locale")
	}
}
