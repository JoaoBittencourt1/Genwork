### Phased delivery

**Exception to rules 42, 44, and 46, approved by the developer:** this change ships together with `gentasks-task-description-markdown`, `gentasks-task-pull-requests`, and `web-live-refresh` on one branch, `feat/task-pull-requests-markdown-live-refresh`, in **one PR**. The phases still set the implementation and review order, each one starts with its tests and ends with lint and tests green, and the PR is opened only after every phase of the three changes and the developer's manual verification (rule 41).

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | Dependencies, blank-description rule in `domain/`, shared `MarkdownContent` | — | — |
| 2 | Task dialog view/edit switch, i18n, manual verification, PR | 1 | — |

## Phase 1 — rendering foundation

- [x] 1.1 Write tests in `web/src/app/gentasks/domain/`: `isDescriptionBlank` is true for `""`, `" "`, and `"\n\t "`, and false for `"## Title"` and `" a "`
- [x] 1.2 Implement `isDescriptionBlank` in `web/src/app/gentasks/domain/`
- [x] 1.3 Add `react-markdown` and `remark-gfm` as dependencies in `web/`; read the `package.json` and lockfile diff and justify every added package (rule 77); run `npm audit --audit-level=high` (rule 79)
- [x] 1.4 Implement `web/src/shared/components/MarkdownContent.tsx`: `react-markdown` with `remark-gfm` and a `components` map styled only with theme variables (design §2); links open in a new tab with `rel="noopener noreferrer"`; task-list checkboxes are disabled; paragraphs keep line breaks; no `rehype-raw` and no custom `urlTransform`
- [x] 1.5 Run `make lint` and the web tests

## Phase 2 — task dialog

- [x] 2.1 Add the "edit description" accessible label to `web/src/shared/i18n/locales/en.json` and `pt-BR.json`
- [x] 2.2 Implement `TaskDescriptionField` in `web/src/app/gentasks/components/` (design §4): rendered view or placeholder when viewing; click, Enter, or Space switches to the text area with focus, except clicks on links; blur switches back; read-only people get a plain rendered view
- [x] 2.3 Replace the description `Textarea` in `TaskDialog.tsx` with `TaskDescriptionField`, keeping the draft in `TaskForm`
- [x] 2.4 Run `make lint`, `make test`, and `make build`
- [ ] 2.5 Manual verification with the developer (rule 41): headings, lists, task lists, code block, table, and link render; `<script>`, `<img onerror>`, and `javascript:` links do nothing; clicking a link opens a new tab without entering edit mode; click/keyboard enter edit mode and blur returns to the rendered draft; Save persists; read-only role cannot edit; blank description shows the placeholder; dark and light themes
- [x] 2.6 Read the whole diff (rule 80) and open the PR after sign-off
