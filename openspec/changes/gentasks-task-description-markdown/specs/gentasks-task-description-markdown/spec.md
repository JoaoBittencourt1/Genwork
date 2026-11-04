## ADDED Requirements

### Requirement: The task dialog renders the description as Markdown
The task dialog SHALL render a task's description as GitHub Flavored Markdown, supporting headings, emphasis, strikethrough, ordered and unordered lists, task lists, links, inline code, code blocks, block quotes, tables, and horizontal rules, styled only with the theme variables (rule 81).

#### Scenario: Heading
- **WHEN** a task's description is `## Context`
- **THEN** the dialog shows "Context" as a second-level heading, without the `##` characters

#### Scenario: Task list
- **WHEN** a task's description contains `- [x] done` and `- [ ] pending`
- **THEN** the dialog shows a checked and an unchecked checkbox, both read-only

#### Scenario: Plain text stays readable
- **WHEN** a task's description has no Markdown syntax
- **THEN** the dialog shows the text as paragraphs, keeping its line breaks

### Requirement: The rendered description is safe
The rendered description SHALL NOT render raw HTML, SHALL drop link and image URLs whose protocol is not `http`, `https`, `mailto`, or relative, and SHALL open every link in a new tab with `rel="noopener noreferrer"`.

#### Scenario: Raw HTML
- **WHEN** a description contains `<script>alert(1)</script>` or `<img src=x onerror=alert(1)>`
- **THEN** no element is created from it and no script runs

#### Scenario: Unsafe link
- **WHEN** a description contains `[click](javascript:alert(1))`
- **THEN** the link text is shown without a `javascript:` target

#### Scenario: External link
- **WHEN** someone clicks a link in the rendered description
- **THEN** it opens in a new tab and the task dialog stays open

### Requirement: People with write access edit the raw Markdown
For people who can write tasks, the dialog SHALL show the rendered description and switch to a text area with the raw Markdown when the rendered view is clicked or activated with the keyboard; leaving the text area SHALL show the rendered result of the current draft. Clicking a link inside the rendered view SHALL open the link and SHALL NOT switch to editing. The description is saved only with the dialog's Save action, as today.

#### Scenario: Start editing
- **WHEN** a person who can write tasks clicks the rendered description
- **THEN** the text area appears with the raw Markdown and receives focus

#### Scenario: Keyboard activation
- **WHEN** a person who can write tasks focuses the rendered description with Tab and presses Enter
- **THEN** the text area appears with the raw Markdown and receives focus

#### Scenario: Finish editing
- **WHEN** the person types `## Plan` in the text area and moves focus elsewhere
- **THEN** the dialog shows "Plan" as a heading and the draft is not yet saved

#### Scenario: New task
- **WHEN** the dialog is creating a task
- **THEN** the description starts as an empty rendered view showing the placeholder, and clicking it opens the text area

### Requirement: Read-only people see only the rendered view
For people who cannot write tasks, the dialog SHALL show the rendered description and SHALL NOT offer editing.

#### Scenario: Read-only click
- **WHEN** a person without write access clicks the rendered description
- **THEN** the rendered view stays and no text area appears

### Requirement: An empty description shows the placeholder
A description that is empty or contains only whitespace SHALL be shown as the existing description placeholder instead of an empty rendered view.

#### Scenario: Whitespace-only description
- **WHEN** a task's description is `"   \n  "`
- **THEN** the dialog shows the "Add a description…" placeholder
