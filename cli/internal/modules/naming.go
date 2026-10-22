package modules

import "strings"

// ModuleConventions derives every per-module name from the module id, so a new module needs no installer code (design §4).
type ModuleConventions struct {
	ID                    string
	Profile               string
	Service               string
	MigrateService        string
	DatabaseRole          string
	PasswordVariable      string
	DatabaseURLVariable   string
	UpstreamVariable      string
	EncryptionKeyVariable string
}

func ConventionsFor(id string) ModuleConventions {
	prefix := strings.ToUpper(strings.ReplaceAll(id, "-", "_"))
	return ModuleConventions{
		ID:                    id,
		Profile:               id,
		Service:               id,
		MigrateService:        id + "-migrate",
		DatabaseRole:          id,
		PasswordVariable:      prefix + "_DB_PASSWORD",
		DatabaseURLVariable:   prefix + "_DATABASE_URL",
		UpstreamVariable:      prefix + "_UPSTREAM",
		EncryptionKeyVariable: prefix + "_ENCRYPTION_KEY",
	}
}

func (conventions ModuleConventions) Services() []string {
	return []string{conventions.MigrateService, conventions.Service}
}
