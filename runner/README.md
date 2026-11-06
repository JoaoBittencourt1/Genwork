# genwork-runner

The runner executes bot runs for the `genagents` module. It runs on a machine you control, claims runs from your Genwork instance, and executes each one in a **disposable sandbox container**: non-root, no capabilities, resource limits, and only that run's workspace and its own account profile mounted. Credentials never enter a sandbox.

## Install

Download `genwork-runner_<version>_<os>_<arch>.tar.gz` from the release and put the binary on your `PATH`. The machine needs `git` and a container engine (Docker, or Podman, ideally rootless).

## Register

In Genwork, open **AI → Runners**, create a runner, and run the command it shows on the runner's machine within one hour:

```sh
genwork-runner register --instance https://genwork.example.com --token <registration token>
```

The runner's secret is stored in `~/.genwork-runner/runner.json` (override the directory with `GENWORK_RUNNER_HOME`), readable only by you.

## GitHub credentials

Task runs and reviews clone, push, open pull requests, and post reviews from the host. Give the runner a fine-grained GitHub token with contents and pull request access to the repositories bots work in:

```sh
export GH_TOKEN=github_pat_...
```

The token stays on this machine. It reaches Git through a one-off credential helper and is never written into a repository or a sandbox. A runner without it still serves admin chat runs and says so on the Runners page.

## Run

```sh
export GENWORK_IMAGE_REGISTRY=ghcr.io/<owner>   # where the release images live
genwork-runner run
```

The runner pulls `genwork-sandbox:<version>` from that registry. Options, all environment variables:

| Variable | Meaning |
| --- | --- |
| `GENWORK_SANDBOX_IMAGE` | A custom sandbox image, for projects that need other toolchains. Reported on the Runners page. |
| `GENWORK_RUNNER_CONCURRENCY` | Runs at once (default 2). |
| `GENWORK_CONTAINER_SOCKET` | The engine socket, when it is not in a well-known place. |
| `GENWORK_GIT_BASE_URL` / `GITHUB_API_URL` | A GitHub Enterprise server. |

It refuses to start without a container engine. For development only, `genwork-runner run --directory-mode` runs the CLIs on the host **without isolation**; the Runners page shows a warning, and Genwork sends such a runner work only when no sandboxed runner can take it.

## Sign in a subscription account

For an account linked in **subscription** mode, sign in on the runner that holds it:

```sh
genwork-runner account login <account token>
```

The provider CLI's own sign-in runs in a sandbox that mounts only that account's profile. Two accounts of the same provider never share a profile, and credentials never leave this machine.

## Custom sandbox images

Build on `genwork-sandbox` and label the CLIs you provide (`genwork.tool.<cli>=<version>`). The runner reads the tools and versions from the image labels and reports them.

## Safety notes

- Bots run with tool approval disabled, because nobody can approve prompts on a headless runner. The sandbox is the boundary.
- A sandbox has outbound network access. Use API keys dedicated to bots, with spend limits at the provider.
- Keep the container engine updated and prefer a dedicated machine. Rootless Podman or Docker reduces the blast radius of an engine bug.
- Provider terms may limit automated or shared use of consumer subscriptions. API-key accounts are always available.
