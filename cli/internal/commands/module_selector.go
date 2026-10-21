package commands

import (
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

type ModuleOption struct {
	ID      string
	Label   string
	Checked bool
}

// ModuleSelector is the interactive part of choosing modules; tests replace it with a fake.
type ModuleSelector interface {
	ChooseModules(title string, options []ModuleOption) ([]string, error)
	ConfirmPlan(summary string, deletesData bool) (bool, error)
}

// HuhModuleSelector shows a checklist (space toggles, Enter confirms) and a yes/no confirmation that defaults to "no".
type HuhModuleSelector struct{}

func (HuhModuleSelector) ChooseModules(title string, options []ModuleOption) ([]string, error) {
	huhOptions := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		huhOptions = append(huhOptions, huh.NewOption(option.Label, option.ID).Selected(option.Checked))
	}
	var selected []string
	err := huh.NewMultiSelect[string]().
		Title(title).
		Description("Space checks or unchecks a module, Enter confirms. Checked modules are installed.").
		Options(huhOptions...).
		Value(&selected).
		Run()
	return selected, err
}

func (HuhModuleSelector) ConfirmPlan(summary string, deletesData bool) (bool, error) {
	title := "Apply these changes?"
	if deletesData {
		title = "Apply these changes and permanently delete the data of the modules to uninstall?"
	}
	confirmed := false
	err := huh.NewConfirm().Title(title).Description(summary).Affirmative("Yes").Negative("No").Value(&confirmed).Run()
	return confirmed, err
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
