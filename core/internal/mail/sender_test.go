package mail

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

func testMessage() Message {
	return Message{
		To:      "bruno@example.com",
		Subject: "Ana convidou você",
		Text:    "Abra o link: http://localhost:8485/invite/abc",
		HTML:    `<p>Abra o <a href="http://localhost:8485/invite/abc">link</a></p>`,
	}
}

func sendWithTimeout(t *testing.T, sender *SMTPSender) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return sender.Send(ctx, testMessage())
}

func TestSMTPSenderDeliversPlainAndHTMLParts(t *testing.T) {
	server := startFakeSMTPServer(t, false)
	sender := NewSMTPSender(platform.SMTPConfig{Host: "127.0.0.1", Port: server.port(), From: "Genwork <no-reply@genwork.test>", TLS: "none"})

	if err := sendWithTimeout(t, sender); err != nil {
		t.Fatalf("send: %v", err)
	}
	messages := server.receivedMessages()
	if len(messages) != 1 {
		t.Fatalf("messages = %d", len(messages))
	}
	for _, fragment := range []string{"To: <bruno@example.com>", "no-reply@genwork.test", "text/plain", "text/html"} {
		if !strings.Contains(messages[0], fragment) {
			t.Errorf("message is missing %q:\n%s", fragment, messages[0])
		}
	}
}

func TestSMTPSenderAuthenticatesWhenCredentialsAreSet(t *testing.T) {
	server := startFakeSMTPServer(t, true)
	sender := NewSMTPSender(platform.SMTPConfig{
		Host: "127.0.0.1", Port: server.port(), From: "no-reply@genwork.test", TLS: "none",
		Username: "mailer", Password: "s3cret",
	})

	if err := sendWithTimeout(t, sender); err != nil {
		t.Fatalf("send: %v", err)
	}
	authLines := server.receivedAuth()
	if len(authLines) != 1 {
		t.Fatalf("auth lines = %v", authLines)
	}
	credentials := strings.TrimPrefix(authLines[0], "AUTH PLAIN ")
	decoded, _ := base64.StdEncoding.DecodeString(credentials)
	if string(decoded) != "\x00mailer\x00s3cret" {
		t.Errorf("credentials = %q", decoded)
	}
}

func TestSMTPSenderRefusesPlaintextWhenTLSIsRequired(t *testing.T) {
	for _, mode := range []string{"starttls", "tls"} {
		t.Run(mode, func(t *testing.T) {
			server := startFakeSMTPServer(t, false)
			sender := NewSMTPSender(platform.SMTPConfig{Host: "127.0.0.1", Port: server.port(), From: "no-reply@genwork.test", TLS: mode})
			if err := sendWithTimeout(t, sender); err == nil {
				t.Fatal("expected delivery to fail without TLS")
			}
			if len(server.receivedMessages()) != 0 {
				t.Error("no message may be delivered over plaintext")
			}
		})
	}
}

func TestSMTPSenderRejectsInvalidAddresses(t *testing.T) {
	sender := NewSMTPSender(platform.SMTPConfig{Host: "127.0.0.1", Port: 1, From: "not an address", TLS: "none"})
	if err := sendWithTimeout(t, sender); err == nil || !strings.Contains(err.Error(), "sender") {
		t.Errorf("error = %v", err)
	}
}
