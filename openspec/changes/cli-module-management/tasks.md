### Phased delivery

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | core: `core module enable|disable` subcommands and `SetEnabled` in the modules service and repository | — | 2 |
| 2 | cli: `internal/modules` — catalog from `module.yaml` (via `sdk/go/manifest`), `deploy/genwork.yaml`, selection plan, naming conventions, Caddyfile rendering | — | 1 |
| 3 | cli: `genwork modules` checklist and confirmation (`huh`), first-run checklist in `up`/`api`, install and uninstall steps, generic provisioning and deprovisioning, profiles of installed modules | 1, 2 merged | — |
| 4 | deploy: gentasks compose profile, gateway without module dependencies, generated files gitignored, committed Caddyfile removed; compose consistency test; gentasks Caddyfile test removed | 3 merged | — |

Phase 4 also touches a CLI test and removes a gentasks test, because both only guard the deploy files that phase changes. Every phase stops for developer sign-off before its PR (rule 41).

## Phase 1 (branch: feat/core-module-state)

- [x] 1.1 Write tests: `core module disable <id>` stores the module as disabled and capabilities stop listing it; `enable` lists it again; re-registration keeps a disabled module disabled; an unknown id exits 0 with a message; a missing id prints usage and fails
- [x] 1.2 Add `ModuleRepository.SetEnabled` and `ModuleService.SetEnabled` (reports whether the module was found)
- [x] 1.3 Add `core/internal/command/module.go` and dispatch `module enable|disable` from `command.Run`, with usage text
- [x] 1.4 Run `make lint`, `make test`, `make build`
- [ ] 1.5 Developer sign-off, then open PR

## Phase 2 (branch: feat/cli-module-catalog)

- [x] 2.1 Add `sdk/go` to `cli/go.mod` with `replace => ../sdk/go` and `gopkg.in/yaml.v3`; review `go.mod` (rule 77); add `sdk/**`, `modules/*/module.yaml`, and `deploy/docker-compose.yml` to the CLI CI path filters
- [x] 2.2 Write tests with fixture workspaces in `testdata/`: catalog loads valid manifests and reports invalid ones by path; installation config tells "never chosen" (no file) from "chose nothing" (empty list), round-trips, and fails on a malformed file
- [x] 2.3 Write tests for `PlanSelection`: nothing changed; install with required modules added recursively and noted; uninstall of unchecked installed modules; refusal when a staying module requires an unchecked one; unknown requirement; unsatisfied range; cycle; install order dependencies-first and uninstall order dependents-first; installed module missing from the catalog can be uninstalled
- [x] 2.4 Write tests for naming (`gentasks`, a dashed id) and for `RenderCaddyfile` (core routes and `/internal` block always present, one route per installed module sorted by id)
- [x] 2.5 Implement `catalog.go`, `installation.go`, `plan.go`, `naming.go`, `gateway.go`
- [x] 2.6 Run `make lint`, `make test`, `make build`
- [ ] 2.7 Developer sign-off, then open PR

## Phase 3 (branch: feat/cli-module-checklist)

- [x] 3.1 Write tests with the fake runner and a fake selector for `genwork modules`: installed modules arrive pre-checked; unchanged selection prints "nothing to change"; checking runs the install steps in order and writes `genwork.yaml`; unchecking asks for confirmation naming the data loss; a declined confirmation changes nothing; a confirmed uninstall runs the uninstall steps in order (psql script over stdin drops schema and role, `.env` values cleared, module removed from the file last); a refused plan changes nothing; no terminal fails without changes
- [x] 3.2 Write tests for stack commands: first run with a terminal shows the checklist with nothing checked and saves the choice; first run without a terminal saves an empty choice and prints the hint; later runs do not ask; `up` and `api` pass one `--profile` per installed module, render the Caddyfile, and provision every installed module; `down` passes `--profile "*"`
- [x] 3.3 Add `github.com/charmbracelet/huh`; review `go.mod` and justify it (rule 77)
- [x] 3.4 Implement `ModuleSelector` with `huh` (multi-select and confirm) and terminal detection with `golang.org/x/term`
- [x] 3.5 Make provisioning, deprovisioning, and secret generation generic over modules (remove the fixed list)
- [x] 3.6 Implement `genwork modules`, the install and uninstall steps, and the first-run step in `up`/`api`
- [x] 3.7 Run `make lint`, `make test`, `make build`
- [ ] 3.8 Developer sign-off, then open PR

## Phase 4 (branch: chore/deploy-module-profiles)

- [x] 4.1 Write the CLI compose consistency test (decision 11) against `deploy/docker-compose.yml`
- [x] 4.2 Put `gentasks` and `gentasks-migrate` under the `gentasks` profile and remove the gateway's `depends_on: gentasks`
- [x] 4.3 Stop committing `deploy/Caddyfile`, add it and `deploy/genwork.yaml` to `.gitignore`, and remove the gentasks test that read the Caddyfile
- [x] 4.4 Update the README (module checklist) and CLAUDE.md (commands list)
- [ ] 4.5 Verify manually: first `genwork up` shows the checklist; check gentasks and use it through the gateway; `genwork modules` shows it checked; uncheck it, confirm, and see it gone (404, hidden from capabilities, schema dropped); check it again and use it with an empty board list; `genwork down` and `genwork up` keep the choice
- [x] 4.6 Run `make lint`, `make test`, `make build`
- [ ] 4.7 Developer sign-off, then open PR
