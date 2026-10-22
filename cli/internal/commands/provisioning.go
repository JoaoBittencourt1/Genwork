package commands

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
	"github.com/spf13/cobra"
)

var (
	//go:embed module_database.sql
	moduleDatabaseScript string
	//go:embed module_database_drop.sql
	moduleDatabaseDropScript string
)

// provisionModuleDatabase sends the password on psql's stdin, so it never shows up in process arguments (rule 67).
func provisionModuleDatabase(command *cobra.Command, dependencies Dependencies, workspace Workspace, moduleID string) error {
	conventions := modules.ConventionsFor(moduleID)
	script := fmt.Sprintf("\\set module_name '%s'\n\\set module_password '%s'\n%s",
		conventions.DatabaseRole, workspace.StackEnvValue(conventions.PasswordVariable), moduleDatabaseScript)
	return runPsqlScript(command, dependencies, workspace, script)
}

func deprovisionModuleDatabase(command *cobra.Command, dependencies Dependencies, workspace Workspace, moduleID string) error {
	script := fmt.Sprintf("\\set module_name '%s'\n%s", modules.ConventionsFor(moduleID).DatabaseRole, moduleDatabaseDropScript)
	return runPsqlScript(command, dependencies, workspace, script)
}

func runPsqlScript(command *cobra.Command, dependencies Dependencies, workspace Workspace, script string) error {
	return runComposeCall(command, dependencies, workspace, composeCall{
		args:  []string{"exec", "-T", "postgres", "psql", "-X", "-q", "-U", workspace.StackEnvValue("POSTGRES_USER"), "-d", workspace.StackEnvValue("POSTGRES_DB")},
		stdin: strings.NewReader(script),
	})
}
