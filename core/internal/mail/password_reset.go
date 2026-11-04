package mail

import (
	"fmt"
	"strings"
	"time"
)

type PasswordResetEmail struct {
	Locale       string
	To           string
	InstanceName string
	Link         string
	ExpiresAt    time.Time
}

type passwordResetTemplateData struct {
	InstanceName string
	Link         string
	ExpiresAt    string
}

func RenderPasswordReset(email PasswordResetEmail) (Message, error) {
	layout, supported := expiryLayouts[email.Locale]
	if !supported {
		return Message{}, fmt.Errorf("unsupported e-mail locale %q", email.Locale)
	}
	data := passwordResetTemplateData{
		InstanceName: email.InstanceName,
		Link:         email.Link,
		ExpiresAt:    email.ExpiresAt.UTC().Format(layout),
	}
	directory := "templates/" + email.Locale + "/"
	subject, err := renderText(directory+"password_reset_subject.txt", data)
	if err != nil {
		return Message{}, err
	}
	text, err := renderText(directory+"password_reset.txt", data)
	if err != nil {
		return Message{}, err
	}
	html, err := renderHTML(directory+"password_reset.html", data)
	if err != nil {
		return Message{}, err
	}
	return Message{To: email.To, Subject: strings.TrimSpace(subject), Text: text, HTML: html}, nil
}
