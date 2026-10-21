// Package commandtest answers command prompts from a script instead of a terminal.
package commandtest

import "errors"

var ErrScriptExhausted = errors.New("the scripted prompter has no more answers")

// ScriptedPrompter answers each prompt with the next scripted answer and records the labels it was asked.
type ScriptedPrompter struct {
	answers []string
	Asked   []string
}

func NewScriptedPrompter(answers ...string) *ScriptedPrompter {
	return &ScriptedPrompter{answers: answers}
}

func (prompter *ScriptedPrompter) PromptLine(label string) (string, error) {
	return prompter.next(label)
}

func (prompter *ScriptedPrompter) PromptPassword(label string) (string, error) {
	return prompter.next(label)
}

func (prompter *ScriptedPrompter) next(label string) (string, error) {
	prompter.Asked = append(prompter.Asked, label)
	if len(prompter.answers) == 0 {
		return "", ErrScriptExhausted
	}
	answer := prompter.answers[0]
	prompter.answers = prompter.answers[1:]
	return answer, nil
}
