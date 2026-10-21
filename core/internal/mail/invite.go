package mail

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
	"time"
)

//go:embed templates
var templateFiles embed.FS

var expiryLayouts = map[string]string{
	"pt-BR": "02/01/2006 15:04 MST",
	"en":    "Jan 2, 2006 15:04 MST",
}

type InviteEmail struct {
	Locale       string
	To           string
	InstanceName string
	InviterName  string
	Link         string
	ExpiresAt    time.Time
}

type inviteTemplateData struct {
	InstanceName string
	InviterName  string
	Link         string
	ExpiresAt    string
}

func RenderInvite(email InviteEmail) (Message, error) {
	layout, supported := expiryLayouts[email.Locale]
	if !supported {
		return Message{}, fmt.Errorf("unsupported e-mail locale %q", email.Locale)
	}
	data := inviteTemplateData{
		InstanceName: email.InstanceName,
		InviterName:  email.InviterName,
		Link:         email.Link,
		ExpiresAt:    email.ExpiresAt.UTC().Format(layout),
	}
	directory := "templates/" + email.Locale + "/"
	subject, err := renderText(directory+"invite_subject.txt", data)
	if err != nil {
		return Message{}, err
	}
	text, err := renderText(directory+"invite.txt", data)
	if err != nil {
		return Message{}, err
	}
	html, err := renderHTML(directory+"invite.html", data)
	if err != nil {
		return Message{}, err
	}
	return Message{To: email.To, Subject: strings.TrimSpace(subject), Text: text, HTML: html}, nil
}

func renderText(path string, data any) (string, error) {
	parsed, err := texttemplate.ParseFS(templateFiles, path)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render %s: %w", path, err)
	}
	return output.String(), nil
}

func renderHTML(path string, data any) (string, error) {
	parsed, err := htmltemplate.ParseFS(templateFiles, path)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render %s: %w", path, err)
	}
	return output.String(), nil
}
