# Web

Static SPA (Vite + React + TypeScript) served by the gateway in production. It calls the API only through relative paths (`/api/...`, `/.well-known/...`), so one build works for every installation. UI components come from [shadcn/ui](https://ui.shadcn.com).

```sh
genwork web            # from anywhere in the repo; creates .env and installs dependencies on first run
npm test               # domain and datasource tests
npx shadcn@latest add <component>   # adds a component to src/shared/ui (the CLI is not a dependency)
```

Layout (constitution rules 72–74):

```
src/
  app/<module>/
    datasource/   HTTP calls (no React)
    domain/       types and pure rules (no React, no HTTP)
    components/   the module's React pieces
    <Name>Page.tsx
  shared/
    ui/           shadcn components
    components/   reusable compositions
    datasource/   HTTP client, discovery
    domain/       cross-module rules
    hooks/  lib/  i18n/
  test/           MSW server and fixtures
  App.tsx  router.tsx  main.tsx
```
