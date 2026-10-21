# PR Description Generator

Generate a Pull Request description following `.github/pull_request_template.md`.

## Instructions

1. **Analyze the changes**: compare the current branch against `main` with `git diff main...HEAD` (ignore merge commits) to see what was modified, added, or removed, and which components (server, web, macos, windows, linux) are touched.

2. **Generate the PR title**: `tipo(N-<issue>): mensagem em pt-BR` (constitution rule 39). Ask for the issue number if it is not obvious from the branch name or commit messages.

3. **Write a clear description in pt-BR** (constitution rule 37):
   - What the PR does and why
   - The main changes and their purpose
   - If UI changes were made, note that screenshots should be attached

4. **Create test instructions**: step-by-step verification, including setup and which flows to test.

5. **Review the checklist**: go through each template item and flag anything needing attention (migrations, modules, user-facing strings, accessibility).

Provide the complete PR description in a markdown block, ready to paste into GitHub.
