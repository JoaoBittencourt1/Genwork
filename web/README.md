# Web

Static SPA (Vite + React + TypeScript) served by the gateway in production. It calls the API only through relative paths (`/api/...`, `/.well-known/...`), so one build works for every installation.

```sh
cp .env.example .env   # or just run `genwork web` / `genwork up`, which create it
npm install
npm run dev            # proxies /api and /.well-known to DEV_API_PROXY_TARGET
npm test
```

Layout:

- `src/core/<feature>/` — core features (auth, invites, discovery)
- `src/modules/<name>/` — module UIs, one folder per module
- `src/shared/` — API client, design system, i18n
