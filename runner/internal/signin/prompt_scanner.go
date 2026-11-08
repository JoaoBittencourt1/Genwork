// Package signin drives the sign-ins of subscription accounts started from the web app: it runs each provider CLI's own
// sign-in in a sandbox with a terminal, relays the URL and code a person needs, types the code they paste, and reports
// how it ended. The provider's credentials stay in the account's profile on this machine.
package signin

import (
	"regexp"
	"strings"
	"sync"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
)

const scannerTailBytes = 2000

var (
	httpsURLPattern = regexp.MustCompile(`https://[^\s"'<>\x00-\x1f\x7f]+`)
)

// Prompt is what a person needs from the CLI: the page to open and, for a device flow, the code to enter there.
type Prompt struct {
	URL      string
	UserCode string
}

// PromptScanner reads a CLI's terminal output and keeps the latest prompt it printed and a readable tail of the output.
type PromptScanner struct {
	mutex           sync.Mutex
	pending         string
	tail            string
	prompt          Prompt
	userCodePattern *regexp.Regexp
}

func NewPromptScanner(userCodePattern *regexp.Regexp) *PromptScanner {
	return &PromptScanner{userCodePattern: userCodePattern}
}

// Write feeds terminal output; it returns whether the prompt changed. Text is examined line by line, so a URL split
// across two writes is still found whole.
func (scanner *PromptScanner) Write(data []byte) (int, error) {
	scanner.mutex.Lock()
	defer scanner.mutex.Unlock()
	scanner.pending += string(data)
	for {
		index := strings.IndexAny(scanner.pending, "\r\n")
		if index < 0 {
			break
		}
		scanner.examine(scanner.pending[:index])
		scanner.pending = scanner.pending[index+1:]
	}
	return len(data), nil
}

// Flush examines the last line, for output that ends without a newline (a CLI waiting at a "Paste code here" prompt).
func (scanner *PromptScanner) Flush() {
	scanner.mutex.Lock()
	defer scanner.mutex.Unlock()
	if scanner.pending != "" {
		scanner.examine(scanner.pending)
	}
}

func (scanner *PromptScanner) examine(line string) {
	readable := adapters.StripTerminalEscapes(line)
	if strings.TrimSpace(readable) == "" {
		return
	}
	scanner.tail += readable + "\n"
	if len(scanner.tail) > scannerTailBytes {
		scanner.tail = scanner.tail[len(scanner.tail)-scannerTailBytes:]
	}
	if found := httpsURLPattern.FindAllString(readable, -1); len(found) > 0 {
		scanner.prompt.URL = strings.TrimRight(found[len(found)-1], ".,;:)]}")
	}
	if scanner.userCodePattern != nil {
		if found := scanner.userCodePattern.FindAllString(readable, -1); len(found) > 0 {
			scanner.prompt.UserCode = found[len(found)-1]
		}
	}
}

func (scanner *PromptScanner) Prompt() Prompt {
	scanner.mutex.Lock()
	defer scanner.mutex.Unlock()
	return scanner.prompt
}

// Tail is the end of the output with credentials redacted, for a failure message a person can read.
func (scanner *PromptScanner) Tail(secrets ...string) string {
	scanner.mutex.Lock()
	defer scanner.mutex.Unlock()
	return strings.TrimSpace(adapters.Redact(scanner.tail+adapters.StripTerminalEscapes(scanner.pending), secrets...))
}
