package githubsync

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	maxTaskPrefixLength = 10
	maxTaskNumberDigits = 9
)

// taskKeyCandidate finds "prefix-number" runs; extractTaskKeysFrom then checks the boundaries Go's RE2 cannot look around.
var taskKeyCandidate = regexp.MustCompile(`([A-Za-z][A-Za-z0-9]*)-([0-9]+)`)

// TaskKey is how people cite a task: its board's task prefix and its number, e.g. GEN-42.
type TaskKey struct {
	Prefix string
	Number int
}

// ExtractTaskKeys finds the task keys a pull request cites in its title or head branch, case-insensitively, without
// duplicates and in order of appearance. The body is never searched: it mentions tasks without working on them.
func ExtractTaskKeys(title, headRef string) []TaskKey {
	var keys []TaskKey
	for _, text := range []string{title, headRef} {
		for _, key := range extractTaskKeysFrom(text) {
			if !containsTaskKey(keys, key) {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

func extractTaskKeysFrom(text string) []TaskKey {
	var keys []TaskKey
	for _, match := range taskKeyCandidate.FindAllStringSubmatchIndex(text, -1) {
		start, end := match[0], match[1]
		prefix, digits := text[match[2]:match[3]], text[match[4]:match[5]]
		if !isKeyBoundary(text, start-1) || !isKeyBoundary(text, end) || len(prefix) > maxTaskPrefixLength || len(digits) > maxTaskNumberDigits {
			continue
		}
		number, err := strconv.Atoi(digits)
		if err != nil {
			continue
		}
		keys = append(keys, TaskKey{Prefix: strings.ToUpper(prefix), Number: number})
	}
	return keys
}

// isKeyBoundary treats the text edges and anything but an ASCII letter or digit as a separator, so "feat/gen-42_fix" cites GEN-42.
func isKeyBoundary(text string, index int) bool {
	if index < 0 || index >= len(text) {
		return true
	}
	character := text[index]
	isLetter := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
	isDigit := character >= '0' && character <= '9'
	return !isLetter && !isDigit
}

func containsTaskKey(keys []TaskKey, key TaskKey) bool {
	for _, existing := range keys {
		if existing == key {
			return true
		}
	}
	return false
}

// taskKeyColumns splits keys into parallel prefix and number arrays for unnest in SQL.
func taskKeyColumns(keys []TaskKey) (prefixes []string, numbers []int) {
	prefixes, numbers = make([]string, 0, len(keys)), make([]int, 0, len(keys))
	for _, key := range keys {
		prefixes, numbers = append(prefixes, key.Prefix), append(numbers, key.Number)
	}
	return prefixes, numbers
}
