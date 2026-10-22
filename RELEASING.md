# Releasing

Every component — core, the modules, the gateway with the web app, the MCP server, the updater, and the `genwork` and `genwork-mcp` binaries — ships in one platform release. Its version lives in [`VERSION`](VERSION) and nowhere else (rules 7 and 50): builds inject it, and a build without it reports `0.0.0-dev`.

## Cutting a release

1. Open a PR that only changes `VERSION` (e.g. `chore: prepara a versão 0.5.1`), following semantic versioning. A breaking change to a public contract also needs a new major API or event version (rule 61).
2. After it merges, tag the merge commit and push the tag:

   ```sh
   git switch main && git pull
   git tag v0.5.1
   git push origin v0.5.1
   ```

3. The [release workflow](.github/workflows/release.yml) runs. It stops before building anything unless the tag equals `v$(cat VERSION)`, then runs `make lint`, `make test`, and `make build`, and publishes:
   - the images `ghcr.io/<owner>/genwork-<name>:<version>` for `linux/amd64` and `linux/arm64` — core, every module, gateway, mcp, and updater (`make release-images`);
   - the release assets in `dist/` — the binaries, `genwork-bundle_<version>.tar.gz`, `install.sh`, and `checksums.txt` (`make release-artifacts`);
   - a GitHub release with notes generated from the merged PRs.

Installations see the release on their next check (`genwork update --check` or the home page card).

## The first release

GHCR packages start private. After the first release, open each `genwork-*` package under the repository owner's packages on GitHub and make it public, or installations fail to pull with `denied`.

## Checking a release locally

Build the release assets without publishing; `install.sh` takes the repository from the same variables GitHub Actions sets:

```sh
GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=JoaoBittencourt1/Genwork make release-artifacts
```

Build the images for one platform into the local Docker instead of pushing them (needs the buildx plugin):

```sh
make release-images RELEASE_PLATFORMS=linux/amd64 RELEASE_OUTPUT=--load GENWORK_IMAGE_REGISTRY=genwork-local
```

## What a release contains

| Asset | Contents |
| --- | --- |
| `genwork_<v>_<os>_<arch>.tar.gz` | the `genwork` CLI for Linux and macOS (amd64, arm64) |
| `genwork-mcp_<v>_<os>_<arch>.tar.gz` / `.zip` | the stdio MCP server for Linux, macOS, and Windows (amd64, arm64) |
| `genwork-bundle_<v>.tar.gz` | `VERSION`, `deploy/docker-compose.yml`, `deploy/.env.example`, and every `modules/<id>/module.yaml` — what a server needs, laid out like the repository |
| `install.sh` | installs that release on a server |
| `checksums.txt` | the SHA-256 of every other asset; the installer and updates verify against it |
