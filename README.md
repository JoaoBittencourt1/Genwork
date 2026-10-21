# Vanep Workspace

A modular, self-hosted workspace platform designed to centralize task management, team workflows, infrastructure insights, and automation in a single ecosystem. Built with Go on the server and native desktop applications for macOS, Windows, and Linux, with independently versioned releases, optional modules, and extensible integrations.

## Repository layout

| Path | Description | Stack |
| --- | --- | --- |
| `server/` | API and background worker | Go |
| `web/` | Self-hosted web frontend | Next.js |
| `clients/macos/` | macOS desktop app | Swift / SwiftUI |
| `clients/windows/` | Windows desktop app | C# / .NET (UI framework TBD) |
| `clients/linux/` | Linux desktop app | Rust / GTK |
| `deploy/` | Self-hosting setup | Docker Compose |
| `openspec/` | Specs and change proposals | OpenSpec |
| `.github/workflows/` | CI per component | GitHub Actions |

Each component is versioned and released independently.

## Running the server locally

```sh
make up            # creates deploy/.env from .env.example on first run
curl localhost:8484/healthz
```

## Contributing

Read [constitution.md](constitution.md) first — its rules are non-negotiable. Features start as OpenSpec changes (`/opsx:propose`) under `openspec/`.
