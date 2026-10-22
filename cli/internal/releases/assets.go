package releases

import "fmt"

// Asset names of a release; the Makefile and install.sh build and fetch the same names (design §4).

func BundleAsset(version string) string {
	return "genwork-bundle_" + version + ".tar.gz"
}

func ExecutableAsset(version, goos, goarch string) string {
	return fmt.Sprintf("genwork_%s_%s_%s.tar.gz", version, goos, goarch)
}
