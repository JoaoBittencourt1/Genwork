package platform

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type Getenv func(string) string

type SMTPConfig struct {
	Host     string
	Port     int
	From     string
	TLS      string
	Username string
	Password string
}

type ServeConfig struct {
	DatabaseURL    string
	HTTPAddr       string
	PublicURL      string
	InstanceName   string
	DefaultLocale  string
	JWTSigningKey  string
	ModuleSecret   string
	SMTP           SMTPConfig
	TrustedProxies []netip.Prefix
	// UpdaterURL and UpdaterSecret reach the updater container, which core relays admins' update requests to (rule 84).
	UpdaterURL    string
	UpdaterSecret string
}

// MinSharedSecretLength matches 128 bits of entropy encoded as base64url; the module and updater secrets need it.
const MinSharedSecretLength = 22

var supportedLocales = []string{"pt-BR", "en"}
var supportedSMTPTLS = []string{"none", "starttls", "tls"}

type envReader struct {
	getenv  Getenv
	missing []string
	invalid []string
}

func (r *envReader) required(name string) string {
	value := strings.TrimSpace(r.getenv(name))
	if value == "" {
		r.missing = append(r.missing, name)
	}
	return value
}

func (r *envReader) optional(name string) string {
	return strings.TrimSpace(r.getenv(name))
}

func (r *envReader) markInvalid(name string) {
	r.invalid = append(r.invalid, name)
}

func (r *envReader) err() error {
	var problems []string
	if len(r.missing) > 0 {
		problems = append(problems, "missing required environment variables: "+strings.Join(r.missing, ", "))
	}
	if len(r.invalid) > 0 {
		problems = append(problems, "invalid environment variables: "+strings.Join(r.invalid, ", "))
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

func LoadDatabaseURL(getenv Getenv) (string, error) {
	reader := &envReader{getenv: getenv}
	databaseURL := reader.required("DATABASE_URL")
	return databaseURL, reader.err()
}

func LoadServeConfig(getenv Getenv) (ServeConfig, error) {
	reader := &envReader{getenv: getenv}
	cfg := ServeConfig{
		DatabaseURL:   reader.required("DATABASE_URL"),
		HTTPAddr:      reader.required("CORE_HTTP_ADDR"),
		PublicURL:     strings.TrimRight(reader.required("GENWORK_PUBLIC_URL"), "/"),
		InstanceName:  reader.required("GENWORK_INSTANCE_NAME"),
		DefaultLocale: reader.required("GENWORK_DEFAULT_LOCALE"),
		JWTSigningKey: reader.required("JWT_SIGNING_KEY"),
		ModuleSecret:  reader.required("GENWORK_MODULE_SECRET"),
		SMTP:          readSMTPConfig(reader),
		UpdaterURL:    strings.TrimRight(reader.required("GENWORK_UPDATER_URL"), "/"),
		UpdaterSecret: reader.required("GENWORK_UPDATER_SECRET"),
	}
	if cfg.ModuleSecret != "" && len(cfg.ModuleSecret) < MinSharedSecretLength {
		reader.markInvalid("GENWORK_MODULE_SECRET (at least 22 characters)")
	}
	if cfg.UpdaterSecret != "" && len(cfg.UpdaterSecret) < MinSharedSecretLength {
		reader.markInvalid("GENWORK_UPDATER_SECRET (at least 22 characters)")
	}
	if cfg.UpdaterURL != "" && !isAbsoluteHTTPURL(cfg.UpdaterURL) {
		reader.markInvalid("GENWORK_UPDATER_URL")
	}
	if cfg.PublicURL != "" && !isAbsoluteHTTPURL(cfg.PublicURL) {
		reader.markInvalid("GENWORK_PUBLIC_URL")
	}
	if cfg.DefaultLocale != "" && !contains(supportedLocales, cfg.DefaultLocale) {
		reader.markInvalid("GENWORK_DEFAULT_LOCALE")
	}
	cfg.TrustedProxies = readTrustedProxies(reader)
	return cfg, reader.err()
}

func readSMTPConfig(reader *envReader) SMTPConfig {
	smtp := SMTPConfig{
		Host:     reader.required("SMTP_HOST"),
		From:     reader.required("SMTP_FROM"),
		TLS:      reader.required("SMTP_TLS"),
		Username: reader.optional("SMTP_USERNAME"),
		Password: reader.optional("SMTP_PASSWORD"),
	}
	if rawPort := reader.required("SMTP_PORT"); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port <= 0 || port > 65535 {
			reader.markInvalid("SMTP_PORT")
		}
		smtp.Port = port
	}
	if smtp.TLS != "" && !contains(supportedSMTPTLS, smtp.TLS) {
		reader.markInvalid("SMTP_TLS")
	}
	if (smtp.Username == "") != (smtp.Password == "") {
		reader.markInvalid("SMTP_USERNAME/SMTP_PASSWORD (set both or neither)")
	}
	return smtp
}

func readTrustedProxies(reader *envReader) []netip.Prefix {
	raw := reader.optional("TRUSTED_PROXY_CIDRS")
	if raw == "" {
		return nil
	}
	var prefixes []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			reader.markInvalid(fmt.Sprintf("TRUSTED_PROXY_CIDRS (%q)", strings.TrimSpace(part)))
			continue
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes
}

func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
