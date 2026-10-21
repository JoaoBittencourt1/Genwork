package tasks

import "github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"

// AssignedTask is a task with the board and column it sits in, as the caller's assigned-tasks list shows it.
type AssignedTask struct {
	TaskWithLabels
	Board  AssignedTaskBoard
	Column AssignedTaskColumn
}

type AssignedTaskBoard struct {
	Token      string
	Name       string
	TaskPrefix string
}

type AssignedTaskColumn struct {
	Token  string
	Preset *boards.ColumnPreset
	Name   *string
}
