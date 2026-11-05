## Purpose

Lets an admin author reusable skills in the Agent Skills format and attach them to bots, so every run of a bot has the same specialized instructions available.

## ADDED Requirements

### Requirement: Skill authoring
A caller holding `genagents.skill.manage` SHALL be able to create, edit, and delete skills. A skill SHALL have a name (lowercase letters, digits, and hyphens, 1–64 characters, unique among active skills), a description (1–1024 characters), a `SKILL.md` body in Markdown, and up to 20 extra text files with relative paths (each up to 256 KiB). Paths SHALL NOT be absolute or contain `..`.

#### Scenario: Create a skill
- **WHEN** an admin creates skill `go-conventions` with a description and body
- **THEN** it is listed and can be attached to bots

#### Scenario: Path traversal
- **WHEN** an extra file path is `../../etc/passwd`
- **THEN** the request is rejected with 400

### Requirement: Import a skill
An admin SHALL be able to import a skill by pasting a `SKILL.md` that starts with YAML front matter holding `name` and `description`; genagents SHALL fill the name and description from it and keep the body.

#### Scenario: Paste SKILL.md
- **WHEN** an admin pastes a `SKILL.md` with front matter `name: pr-review`
- **THEN** a skill named `pr-review` is created with that description and body

### Requirement: Attaching skills
A bot SHALL have zero or more skills. Every run SHALL receive the bot's skills as they are when the run is claimed; editing a skill SHALL affect later runs only. Deleting a skill attached to bots SHALL detach it from them.

#### Scenario: Skill edited during a run
- **WHEN** a skill is edited while a run that uses it is in progress
- **THEN** the run keeps the version it started with and the next run gets the new one

### Requirement: Skills reach the CLI
The runner SHALL install a run's skills where its provider CLI discovers skills; for a CLI without native skill support it SHALL append each skill's name and description to the run instructions and place the files in the workspace so the model can read them.

#### Scenario: Claude Code run
- **WHEN** a run on an Anthropic account has skill `go-conventions`
- **THEN** the skill is available to Claude Code as a project skill in the run workspace
