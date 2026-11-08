package signin_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/signin"
)

func TestScannerFindsTheURLThroughColorsAndSplitWrites(t *testing.T) {
	scanner := signin.NewPromptScanner(nil)

	_, _ = scanner.Write([]byte("\x1b[1mBrowser didn't open? Use the url below to sign in:\x1b[0m\r\n\x1b[36mhttps://claude.ai/oauth/auth"))
	_, _ = scanner.Write([]byte("orize?code=true&state=abc\x1b[39m\r\n"))

	if got := scanner.Prompt().URL; got != "https://claude.ai/oauth/authorize?code=true&state=abc" {
		t.Errorf("url = %q", got)
	}
}

func TestScannerKeepsTheLastURLAndTrimsPunctuation(t *testing.T) {
	scanner := signin.NewPromptScanner(nil)

	_, _ = scanner.Write([]byte("Opening https://localhost/first.\nIf that fails, open (https://example.com/second).\n"))

	if got := scanner.Prompt().URL; got != "https://example.com/second" {
		t.Errorf("url = %q", got)
	}
}

func TestScannerFindsADeviceCodeWithTheAdapterPattern(t *testing.T) {
	scanner := signin.NewPromptScanner(regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4,5}\b`))

	_, _ = scanner.Write([]byte("Open https://auth.openai.com/codex/device\nEnter this one-time code: \x1b[94mWXYZ-12345\x1b[0m\n"))

	prompt := scanner.Prompt()
	if prompt.URL != "https://auth.openai.com/codex/device" || prompt.UserCode != "WXYZ-12345" {
		t.Errorf("prompt = %+v", prompt)
	}
}

func TestScannerReadsALastLineWithoutNewlineOnFlush(t *testing.T) {
	scanner := signin.NewPromptScanner(nil)
	_, _ = scanner.Write([]byte("Visit https://example.com/login"))

	if scanner.Prompt().URL != "" {
		t.Fatal("a partial line was examined before flush")
	}
	scanner.Flush()

	if scanner.Prompt().URL != "https://example.com/login" {
		t.Errorf("url = %q", scanner.Prompt().URL)
	}
}

func TestScannerTailIsReadableAndRedacted(t *testing.T) {
	scanner := signin.NewPromptScanner(nil)
	_, _ = scanner.Write([]byte("\x1b]8;;https://x\x07link\x1b]8;;\x07\r\nOAuth error: invalid code pasted-code-123456\r\nkey sk-ant-abcdefghijklmnop\r\n"))

	tail := scanner.Tail("pasted-code-123456")

	if strings.Contains(tail, "\x1b") || strings.Contains(tail, "pasted-code-123456") || strings.Contains(tail, "sk-ant-abcdefghijklmnop") {
		t.Errorf("tail leaks escapes or secrets: %q", tail)
	}
	if !strings.Contains(tail, "OAuth error: invalid code") {
		t.Errorf("tail lost the message: %q", tail)
	}
}
