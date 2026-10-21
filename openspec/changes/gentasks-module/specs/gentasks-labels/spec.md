## Purpose

Lets people define their own labels per board, each with a name and a palette color, to categorize tasks.

## ADDED Requirements

### Requirement: Label colors come from a fixed palette
A label color SHALL be one of `gray`, `red`, `orange`, `yellow`, `green`, `teal`, `blue`, `purple`, `pink`. Clients map each value to a theme variable (rule 81). Any other value is rejected.

#### Scenario: Unknown color
- **WHEN** a label is created with color `#ff0000`
- **THEN** gentasks responds `400` with a validation error on `color`

### Requirement: Create and list labels
`POST /boards/{token}/labels` (permission `gentasks.board.manage`) SHALL create a label with a name of 1 to 40 characters and a palette color. A board cannot have two labels with the same name, ignoring case, among labels that are not deleted. `GET /boards/{token}/labels` (`gentasks.board.read`) SHALL list the board's labels ordered by name.

#### Scenario: Create a label
- **WHEN** a user creates label "bug" with color `red`
- **THEN** gentasks responds `201` with the label's token, name, and color

#### Scenario: Duplicate name
- **WHEN** the board already has a label "Bug"
- **THEN** creating "bug" responds `409` with code `label_name_taken`

#### Scenario: Name of a deleted label is free again
- **WHEN** label "bug" was deleted
- **THEN** a new "bug" label can be created

### Requirement: Edit and delete labels
`PATCH /labels/{token}` (manage) SHALL change the name and/or color, keeping omitted fields unchanged (rule 17). `DELETE /labels/{token}` (manage) SHALL soft-delete the label and detach it from every task.

#### Scenario: Recolor only
- **WHEN** the body is `{ "color": "blue" }`
- **THEN** the color changes and the name is unchanged

#### Scenario: Delete a label in use
- **WHEN** a label attached to five tasks is deleted
- **THEN** none of those tasks lists it anymore

#### Scenario: Null name
- **WHEN** the body is `{ "name": null }`
- **THEN** gentasks responds `400`
