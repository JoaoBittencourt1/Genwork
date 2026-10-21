# PR Generator (prgen)

Triggered as `/prgen #<numero>`, where `<numero>` is a GitHub issue number (e.g. `/prgen #34`). If no `#<numero>` is given, ask for it before doing anything else.

## Instructions

Follow this sequence exactly, in order, without skipping steps:

1. **Read the issue.** Run `gh issue view <numero>`. Compare it against the pending changes (`git status` / `git diff`) to confirm the work matches the issue before proceeding.

2. **Pick the type** from the nature of the change: `feat`, `fix`, `chore`, `refactor`, `test`, `docs`, `ci`, etc. Check `git log --oneline` for precedent.

3. **Run the validation gates for every touched component, in this order, before any commit:**
   - lint — if it fails, run the matching lint-fix and re-run lint until clean (`make lint` / `make lint-fix` for the server)
   - tests (`make test` for the server)
   - build (`make build` for the server)

   Stop and fix the underlying issue if any gate fails. Never commit or push on a red gate, and never use `--no-verify`.

4. **Stop for developer sign-off** (constitution rule 41). Summarize what will be committed and wait for explicit approval.

5. **Create a branch** off the current HEAD: `tipo(N-<numero>)/short-slug-in-english` (e.g. `feat(N-34)/task-crud`). Git refs cannot contain `:` or spaces, so the branch uses `/` where the commit message uses `: `.

6. **Commit** with `tipo(N-<numero>): mensagem em pt-BR descrevendo o que mudou`. Never use `git commit --amend` unless explicitly asked.

7. **Push:** `git push -u origin <branch>`.

8. **Open the PR** with `gh pr create`:
   - Title: same as the commit message.
   - Body in pt-BR following `.github/pull_request_template.md`, stating that lint/test/build passed locally, and ending with the literal line `This PR closes #<numero>` — GitHub's auto-close keywords only work in English.
   - Verify the link took: `gh pr view <pr-number> --json closingIssuesReferences` must list issue `<numero>`.

## Rules

- Never skip or reorder steps 3 and 4.
- Never force-push or use `--no-verify`.
- If the working tree has nothing to commit, say so instead of creating an empty commit or PR.
