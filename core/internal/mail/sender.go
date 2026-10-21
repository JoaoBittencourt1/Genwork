package mail

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	gomail "github.com/wneessen/go-mail"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Sender interface {
	Send(ctx context.Context, message Message) error
}

type SMTPSender struct {
	config platform.SMTPConfig
}

func NewSMTPSender(config platform.SMTPConfig) *SMTPSender {
	return &SMTPSender{config: config}
}

func (sender *SMTPSender) Send(ctx context.Context, message Message) error {
	msg := gomail.NewMsg()
	if err := msg.From(sender.config.From); err != nil {
		return fmt.Errorf("set sender: %w", err)
	}
	if err := msg.To(message.To); err != nil {
		return fmt.Errorf("set recipient: %w", err)
	}
	msg.Subject(message.Subject)
	msg.SetBodyString(gomail.TypeTextPlain, message.Text)
	msg.AddAlternativeString(gomail.TypeTextHTML, message.HTML)

	client, err := gomail.NewClient(sender.config.Host, sender.clientOptions()...)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("send e-mail: %w", err)
	}
	return nil
}

func (sender *SMTPSender) clientOptions() []gomail.Option {
	options := []gomail.Option{gomail.WithPort(sender.config.Port)}
	switch sender.config.TLS {
	case "none":
		options = append(options, gomail.WithTLSPolicy(gomail.NoTLS))
	case "starttls":
		options = append(options, gomail.WithTLSPolicy(gomail.TLSMandatory))
	case "tls":
		options = append(options, gomail.WithSSL())
	}
	if sender.config.Username != "" {
		options = append(options,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(sender.config.Username),
			gomail.WithPassword(sender.config.Password))
	}
	return options
}
