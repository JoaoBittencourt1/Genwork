package mail_test

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/mail"
	"strings"
	"testing"
	"time"
)

func TestRenderInviteInEachLocale(t *testing.T) {
	expiresAt := time.Date(2026, 10, 27, 15, 30, 0, 0, time.UTC)
	cases := map[string]struct{ subjectWord, expiry string }{
		"pt-BR": {"convidou", "27/10/2026 15:30 UTC"},
		"en":    {"invited", "Oct 27, 2026 15:30 UTC"},
	}
	for locale, expected := range cases {
		t.Run(locale, func(t *testing.T) {
			message, err := mail.RenderInvite(mail.InviteEmail{
				Locale: locale, To: "bruno@example.com", InstanceName: "Genwork Test",
				InviterName: "Ana", Link: "http://localhost:8485/invite/abc", ExpiresAt: expiresAt,
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if message.To != "bruno@example.com" || !strings.Contains(message.Subject, expected.subjectWord) {
				t.Errorf("unexpected header: to=%q subject=%q", message.To, message.Subject)
			}
			for _, body := range []string{message.Text, message.HTML} {
				for _, fragment := range []string{"Genwork Test", "Ana", "http://localhost:8485/invite/abc", expected.expiry} {
					if !strings.Contains(body, fragment) {
						t.Errorf("body is missing %q", fragment)
					}
				}
			}
		})
	}
}

func TestRenderInviteEscapesHTML(t *testing.T) {
	message, err := mail.RenderInvite(mail.InviteEmail{
		Locale: "en", InstanceName: "<script>x</script>", InviterName: "Ana", Link: "http://localhost/invite/a", ExpiresAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(message.HTML, "<script>") {
		t.Error("instance name must be escaped in HTML")
	}
}

func TestRenderInviteRejectsUnknownLocale(t *testing.T) {
	if _, err := mail.RenderInvite(mail.InviteEmail{Locale: "fr"}); err == nil {
		t.Fatal("expected unsupported locale to fail")
	}
}
