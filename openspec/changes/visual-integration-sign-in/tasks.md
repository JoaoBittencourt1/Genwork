## Phase 1 (branch: feat/visual-integration-sign-in) — single PR by developer request

- [x] 1.1 Write tests: genagents sign-in API and runner sign-in API, MCP OAuth flow against a fake authorization server, GitHub token relay; gentasks repository tokens against a fake GitHub; runner sign-in session, URL/code extraction, per-run GitHub token; web domain and datasource
- [x] 1.2 genagents migration `00005`: `account_sign_ins`, `mcp_server_oauth_connections`, `mcp_server_oauth_authorizations`
- [x] 1.3 genagents sign-in model, repository, service, handlers (web and runner); account response `signIn`
- [x] 1.4 genagents MCP OAuth client, service, handlers; claim resolution adds the bearer header
- [x] 1.5 gentasks repository tokens: App JWT, installation lookup, token request, handler, permission in `module.yaml`, manifest permissions
- [x] 1.6 genagents GitHub token relay and gentasks client; relaxed eligibility
- [x] 1.7 runner: interactive sandbox, adapter sign-in specs, sign-in loop, per-run GitHub token with fallback
- [x] 1.8 web: sign-in button and panel, MCP connect, i18n en and pt-BR
- [x] 1.9 README updates, VERSION 2.2.0
- [x] 1.10 Run `make lint`, `make test`, `make build`
- [x] 1.11 Open PR
