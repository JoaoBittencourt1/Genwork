## Purpose

Keeps bot conversations readable: no terminal noise, and results that show what they say.

## ADDED Requirements

### Requirement: No terminal control sequences
The runner SHALL strip terminal control sequences from CLI output and SHALL not report a line that is empty afterwards.

#### Scenario: Cursor sequence on exit
- **WHEN** the CLI prints `ESC[?25h` as it exits
- **THEN** no bot message appears for it

### Requirement: Result text
A result entry SHALL show its text or, without one, its summary. A review result SHALL link to the reviews it posted.

#### Scenario: Review result
- **WHEN** a review run posts its review on GitHub
- **THEN** the result reads "4 findings" with a link to the review
