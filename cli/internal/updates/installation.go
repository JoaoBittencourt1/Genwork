package updates

import (
	"context"
	"io"
)

// Installation is what an update changes. The engine owns the order, the files, and the rollback; the CLI implements
// the Docker side on its workspace (rule 9), so the engine's tests need no Docker.
type Installation interface {
	Root() string
	IsRepositoryCheckout() bool
	// PullImages pulls the images of version that the compose file in stagedRoot names, leaving the running stack untouched.
	PullImages(ctx context.Context, version, stagedRoot string) error
	DumpDatabase(ctx context.Context, destination io.Writer) error
	// RefreshConfiguration adds the .env keys the new example declares, fills their secrets, and renders the gateway routes.
	RefreshConfiguration(ctx context.Context) error
	// StartStack runs migrations and starts every service but the updater with version's images, waiting until they run.
	StartStack(ctx context.Context, version string) error
	StopServicesExceptDatabase(ctx context.Context, version string) error
	RestoreDatabase(ctx context.Context, dump io.Reader) error
	// ReportedVersion is the version the gateway reports at /.well-known/genwork.
	ReportedVersion(ctx context.Context) (string, error)
}
