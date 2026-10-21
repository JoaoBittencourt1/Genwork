package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/discovery"
	"github.com/JoaoBittencourt1/Genwork/core/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

var errModuleStateUsage = errors.New("usage: core module enable|disable <id>")

// RunModuleState is how the installer CLI publishes or hides a module after installing or uninstalling it (rules 56, 64).
func RunModuleState(ctx context.Context, getenv platform.Getenv, schema string, args []string, stdout io.Writer) error {
	if len(args) != 2 || (args[0] != "enable" && args[0] != "disable") {
		return errModuleStateUsage
	}
	action, moduleName := args[0], args[1]
	databaseURL, err := platform.LoadDatabaseURL(getenv)
	if err != nil {
		return err
	}
	db, err := platform.OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		return err
	}
	defer db.Close()

	found, err := modules.NewModuleService(db, discovery.CoreAPIVersion, time.Now).SetEnabled(ctx, moduleName, action == "enable")
	if err != nil {
		return err
	}
	if !found {
		_, err = fmt.Fprintf(stdout, "module %s is not registered; nothing to change\n", moduleName)
		return err
	}
	_, err = fmt.Fprintf(stdout, "module %s %sd\n", moduleName, action)
	return err
}
