## ADDED Requirements

### Requirement: Neutral dark surfaces
In the dark theme, every surface and text token in `web/src/index.css` (`--background`, `--foreground`, `--card`, `--card-foreground`, `--popover`, `--popover-foreground`, `--secondary`, `--muted`, `--accent`, `--accent-foreground`, `--primary-foreground`, `--sidebar`, `--sidebar-primary-foreground`) SHALL have zero chroma. Green SHALL remain only on `--primary`, `--ring`, `--sidebar-primary`, `--sidebar-ring`, and the tokens derived from `--primary` (rule 81).

#### Scenario: Dark theme
- **WHEN** the dark theme is active
- **THEN** the page, cards, sidebar, menus, and hover highlights are black and gray with no green tint, and buttons, focus rings, and the active navigation item stay green

#### Scenario: Light theme untouched
- **WHEN** the light theme is active
- **THEN** its colors are unchanged
