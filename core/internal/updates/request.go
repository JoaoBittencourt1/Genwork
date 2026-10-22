package updates

import "github.com/Masterminds/semver/v3"

// StartUpdateRequest names the release to install, which must be the latest one the updater knows.
type StartUpdateRequest struct {
	Version string `json:"version"`
}

func (request StartUpdateRequest) Validate() map[string]string {
	if _, err := semver.StrictNewVersion(request.Version); err != nil {
		return map[string]string{"version": "invalid"}
	}
	return nil
}
