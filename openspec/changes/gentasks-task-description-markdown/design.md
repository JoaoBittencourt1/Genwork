## Context

`TaskDialog.tsx` edits the description in a borderless `Textarea` that is always visible, and the API stores the text as typed (up to 10 000 characters). The web app has no Markdown renderer and no typography plugin. The developer chose a rendered view that switches to raw editing on click over a WYSIWYG editor.

## Goals / Non-Goals

**Goals:**
- Show the description as formatted Markdown in the task dialog.
- Keep editing raw text in the existing text area, with no change to how drafts are saved.
- Render safely: no raw HTML, no script-capable URLs.
- Keep the renderer reusable by other modules and themed only through `web/src/index.css` variables (rule 81).

**Non-Goals:**
- WYSIWYG editing, toolbars, interactive checkboxes, Markdown on cards (see proposal).

## Decisions

### 1. `react-markdown` + `remark-gfm`
`react-markdown` builds React elements from the Markdown syntax tree. It never uses `dangerouslySetInnerHTML`, ignores raw HTML unless `rehype-raw` is added, and its default `urlTransform` keeps only `http`, `https`, `mailto`, `irc`, `ircs`, `xmpp`, and relative URLs. `remark-gfm` adds tables, task lists, strikethrough, and autolinks.

Alternatives: `marked` or `markdown-it` produce HTML strings that need `dangerouslySetInnerHTML` plus a sanitizer such as DOMPurify (two dependencies and a larger attack surface); a hand-written parser would re-implement a spec. Both packages are runtime dependencies (they ship in the bundle) and are reviewed under rule 77; `npm audit --audit-level=high` must stay clean (rule 79).

### 2. Styling through a components map, not `@tailwindcss/typography`
`MarkdownContent` passes a `components` map to `react-markdown` that gives each element (`h1`–`h6`, `p`, `ul`, `ol`, `li`, `a`, `code`, `pre`, `blockquote`, `table`, `th`, `td`, `hr`, `input`) Tailwind classes built from theme variables (`text-foreground`, `text-muted-foreground`, `bg-muted`, `border`, `text-primary` for links). The typography plugin would be one more dependency and brings its own gray palette, which would have to be overridden to follow rule 81.

Paragraphs use `whitespace-pre-wrap`, so single line breaks in plain-text descriptions stay visible without adding `remark-breaks`. Links get `target="_blank"` and `rel="noopener noreferrer"`. Task-list checkboxes render `disabled`.

### 3. `MarkdownContent` lives in `web/src/shared/components/`
It is a reusable composition, not a shadcn primitive (rule 73), and nothing about it is specific to gentasks, so it is shared from the start (rule 59 forbids one module importing another's UI). It takes `markdown: string` and an optional `className`, and has no editing logic.

### 4. View/edit switch inside the task dialog
A small `TaskDescriptionField` component in `web/src/app/gentasks/components/` holds an `editing` flag:
- **Viewing:** a focusable region (`role="button"`, `tabIndex=0`, an accessible label "Edit description") wrapping `MarkdownContent`, or the placeholder when the description is blank. Click or Enter/Space sets `editing`. Clicks whose target is inside an `<a>` are ignored so links open without entering edit mode. When the person cannot write tasks, the region is a plain `div` with no role, tab stop, or handlers.
- **Editing:** the current `Textarea` with `autoFocus`; `onBlur` clears `editing`. The draft keeps living in `TaskForm`'s state, so blurring never loses text and Save works whether or not the text area is open.

The rendered region keeps the text area's minimum height so the dialog does not jump when switching.

### 5. Blank-description rule in `domain/`
`isDescriptionBlank(description: string): boolean` (true for empty or whitespace-only) lives in `web/src/app/gentasks/domain/` and is unit-tested (rule 74); the component only calls it. Rendering itself is markup and is not unit-tested (rule 74). The safety requirements are guaranteed by the library's defaults, which the component must not override (no `rehype-raw`, no custom `urlTransform`, no `skipHtml={false}` workarounds); the manual verification checklist covers them.

## Risks / Trade-offs

- [Bundle size: `react-markdown` + `remark-gfm` with their `unified`/`micromark` dependencies add roughly 40–50 kB gzipped] → Acceptable for the task dialog; if it matters later, `MarkdownContent` can be loaded lazily with `React.lazy` without changing callers.
- [A future change adds `rehype-raw` to support HTML and opens an XSS hole] → Design §5 records that the safety relies on the defaults; such a change needs its own spec and a sanitizer.
- [Clicking near a link starts editing unexpectedly] → Only clicks inside an `<a>` are excluded; everything else in the region means "edit", which matches the always-visible text area people use today.
- [Very long descriptions render on every keystroke-free open] → Rendering happens only in view mode and 10 000 characters render in a few milliseconds.

## Migration Plan

None: no API, schema, or stored-data change. Existing descriptions render as Markdown as soon as the web bundle is updated; plain-text descriptions keep their line breaks (Decision 2). Rollback is reverting the web change.

## Open Questions

None.
