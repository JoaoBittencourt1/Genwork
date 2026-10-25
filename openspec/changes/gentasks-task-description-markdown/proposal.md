## Why

Task descriptions are plain text today: people already write Markdown in them (headings, checklists, code), but the task dialog shows the raw `##` and `- [ ]` characters, so long descriptions are hard to read.

## What Changes

- The task dialog renders the description as GitHub Flavored Markdown: headings, emphasis, lists, task lists, links, inline code and code blocks, block quotes, tables, and horizontal rules.
- Editing stays raw Markdown: the dialog shows the rendered description; clicking it (or focusing it with the keyboard and pressing Enter) swaps in the existing text area with the raw text, and leaving the text area shows the rendered result again. Saving is unchanged (the dialog's Save button).
- An empty or whitespace-only description shows the existing placeholder in place of the rendered view.
- Raw HTML in a description is never rendered, and links only keep safe protocols; every link opens in a new tab with `rel="noopener noreferrer"`.
- People who cannot write tasks see only the rendered view.
- A shared `MarkdownContent` component renders Markdown with the theme's typography and colors, so other modules can reuse it.

Components touched: **web** only (`web/src/app/gentasks/`, `web/src/shared/`). The gentasks module and its API are unchanged: the description is still stored and returned as plain text (at most 10 000 characters).

## Capabilities

### New Capabilities

- `gentasks-task-description-markdown`: rendering a task's description as Markdown in the task dialog and switching between the rendered view and raw editing.

### Modified Capabilities

None — no capability has been archived into `openspec/specs/` yet.

## Non-goals

- A rich-text (WYSIWYG) editor where Markdown shortcuts format as you type; the developer chose rendered view plus raw editing.
- A formatting toolbar, image upload, or `@mentions`.
- Ticking task-list checkboxes in the rendered view (they render read-only; edit the text to change them).
- Rendering Markdown on the task card or in the assigned-tasks list (cards do not show the description).
- Server-side rendering or sanitizing: the API keeps storing the text as typed.

## Impact

- `web/package.json` and lockfile: new runtime dependencies `react-markdown` and `remark-gfm` (reviewed per rule 77; `npm audit` must stay clean, rule 79).
- `web/src/shared/components/MarkdownContent.tsx`: new shared component.
- `web/src/app/gentasks/components/TaskDialog.tsx`: description field becomes a rendered view / text area pair.
- `web/src/app/gentasks/domain/`: description display rule with tests.
- `web/src/shared/i18n/locales/`: new copy for the "edit description" affordance in English and pt-BR.
