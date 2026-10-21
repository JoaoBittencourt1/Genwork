# PR Generator (prgen)

Triggered as `/prgen`. Turns the pending changes into a branch, commit, and PR.

## Instructions

Follow this sequence exactly, in order, without skipping steps:

1. **Review the changes.** Read `git status` and the whole `git diff` (constitution rule 80) and identify which components they touch.

2. **Pick the type** from the nature of the change: `feat`, `fix`, `chore`, `refactor`, `test`, `docs`, `ci`, etc. Check `git log --oneline` for precedent.

3. **Run the validation gates for every touched component, in this order, before any commit:**
   - lint — if it fails, run `make lint-fix` and re-run `make lint` until clean
   - tests (`make test`)
   - build (`make build`)

   Stop and fix the underlying issue if any gate fails. Never commit or push on a red gate, and never use `--no-verify`.

4. **Stop for developer sign-off** (constitution rule 41). Summarize what will be committed and wait for explicit approval.

5. **Create a branch** off the current HEAD: `tipo/short-slug-in-english` (e.g. `feat/task-crud`).

6. **Commit** with `tipo(component): mensagem em pt-BR descrevendo o que mudou` (constitution rule 39). Never use `git commit --amend` unless explicitly asked.

7. **Push:** `git push -u origin <branch>`.

8. **Open the PR** with `gh pr create`:
   - Title: same as the commit message.
   - Body in pt-BR following `.github/pull_request_template.md`, stating that lint/test/build passed locally.

## Rules

- Never skip or reorder steps 3 and 4.
- Never force-push or use `--no-verify`.
- If the working tree has nothing to commit, say so instead of creating an empty commit or PR.
