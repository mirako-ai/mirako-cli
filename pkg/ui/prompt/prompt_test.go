package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrompterSelectReturnsOptionValue(t *testing.T) {
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader("\x1b[B\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.Select("Agent type", []SelectOption{
		{Label: "managed agent", Value: "managed_agent"},
		{Label: "custom agent", Value: "custom_agent"},
	}, "managed_agent")
	if err != nil {
		t.Fatalf("Select() returned error: %v", err)
	}
	if got != "custom_agent" {
		t.Fatalf("Select() = %q, want custom_agent", got)
	}
}

func TestTerminalNewlinesUseCarriageReturnsForRawMode(t *testing.T) {
	got := terminalNewlines("first\nsecond\r\nthird")
	want := "first\r\nsecond\r\nthird"
	if got != want {
		t.Fatalf("terminalNewlines() = %q, want %q", got, want)
	}
}

func TestVisualLineCountAccountsForSoftWrappedRows(t *testing.T) {
	block := "\x1b[32m◆\x1b[0m Agent type\n│     provide prompt, model/tools and host runtime on Mirako\n"

	got := visualLineCount(block, 20)
	if got <= 2 {
		t.Fatalf("visualLineCount() = %d, want more than logical line count for wrapped text", got)
	}
}

func TestPrompterMultilineHandlesLargePastedInput(t *testing.T) {
	var output bytes.Buffer
	wantLines := make([]string, 0, 800)
	for i := 0; i < 750; i++ {
		if i > 0 && i%125 == 0 {
			wantLines = append(wantLines, "")
		}
		wantLines = append(wantLines, fmt.Sprintf("line %03d %s", i, strings.Repeat("paste", 20)))
	}
	want := strings.Join(wantLines, "\n")
	prompter := NewPrompter(
		WithIO(strings.NewReader(want+"\n\n\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.Multiline("Instruction prompt", "", true)
	if err != nil {
		t.Fatalf("Multiline() returned error: %v", err)
	}
	if got != want {
		t.Fatalf("Multiline() returned %d bytes, want %d bytes", len(got), len(want))
	}
	if !strings.Contains(output.String(), "entered") {
		t.Fatalf("Multiline() output should confirm submitted text, got %q", output.String())
	}
}

func TestPrompterMultilinePreservesBlankLinesInsideBracketedPaste(t *testing.T) {
	var output bytes.Buffer
	want := "alpha\n\n\nomega"
	input := bracketedPasteStart + want + bracketedPasteEnd + "\n\n\n"
	prompter := NewPrompter(
		WithIO(strings.NewReader(input), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.Multiline("Instruction prompt", "", true)
	if err != nil {
		t.Fatalf("Multiline() returned error: %v", err)
	}
	if got != want {
		t.Fatalf("Multiline() = %q, want %q", got, want)
	}
	if strings.Contains(got, bracketedPasteStart) || strings.Contains(got, bracketedPasteEnd) {
		t.Fatalf("Multiline() should strip bracketed paste markers, got %q", got)
	}
}

func TestPrompterPathInputCompletesLongestCommonPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"instruction.md", "instructions.txt"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("prompt"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}
	prefix := filepath.Join(tmpDir, "inst")
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader(prefix+"\t\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.PathInput("Instruction file", "", true)
	if err != nil {
		t.Fatalf("PathInput() returned error: %v", err)
	}
	want := filepath.Join(tmpDir, "instruction")
	if got != want {
		t.Fatalf("PathInput() = %q, want longest common prefix %q", got, want)
	}
}

func TestPrompterPathInputCompletesRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	if err := os.WriteFile("relative.md", []byte("prompt"), 0644); err != nil {
		t.Fatalf("failed to write instruction file: %v", err)
	}
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader("rel\t\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.PathInput("Instruction file", "", true)
	if err != nil {
		t.Fatalf("PathInput() returned error: %v", err)
	}
	if got != "relative.md" {
		t.Fatalf("PathInput() = %q, want relative.md", got)
	}
}

func TestPrompterPathInputCompletesPathWithSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	want := filepath.Join(tmpDir, "with space.md")
	if err := os.WriteFile(want, []byte("prompt"), 0644); err != nil {
		t.Fatalf("failed to write instruction file: %v", err)
	}
	prefix := filepath.Join(tmpDir, "with")
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader(prefix+"\t\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.PathInput("Instruction file", "", true)
	if err != nil {
		t.Fatalf("PathInput() returned error: %v", err)
	}
	if got != want {
		t.Fatalf("PathInput() = %q, want %q", got, want)
	}
}

func TestPrompterPathInputCompletesTildePaths(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	if err := os.WriteFile(filepath.Join(homeDir, "instruction.md"), []byte("prompt"), 0644); err != nil {
		t.Fatalf("failed to write instruction file: %v", err)
	}
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader("~/inst\t\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.PathInput("Instruction file", "", true)
	if err != nil {
		t.Fatalf("PathInput() returned error: %v", err)
	}
	if got != "~/instruction.md" {
		t.Fatalf("PathInput() = %q, want ~/instruction.md", got)
	}
}

func TestPrompterPathInputDoesNotInsertTabCharacter(t *testing.T) {
	tmpDir := t.TempDir()
	prefix := filepath.Join(tmpDir, "missing")
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader(prefix+"\t\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.PathInput("Instruction file", "", true)
	if err != nil {
		t.Fatalf("PathInput() returned error: %v", err)
	}
	if got != prefix {
		t.Fatalf("PathInput() = %q, want unchanged prefix %q", got, prefix)
	}
	if strings.ContainsRune(got, '\t') {
		t.Fatalf("PathInput() inserted a tab character: %q", got)
	}
}

func TestPrompterPathInputBackspacesToBeginningAfterTabCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	completed := filepath.Join(tmpDir, "instruction.md")
	if err := os.WriteFile(completed, []byte("prompt"), 0644); err != nil {
		t.Fatalf("failed to write instruction file: %v", err)
	}
	prefix := filepath.Join(tmpDir, "inst")
	backspaces := strings.Repeat("\b", len([]rune(completed)))
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader(prefix+"\t"+backspaces+"manual path.md\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.PathInput("Instruction file", "", true)
	if err != nil {
		t.Fatalf("PathInput() returned error: %v", err)
	}
	if got != "manual path.md" {
		t.Fatalf("PathInput() = %q, want manual path after deleting completion", got)
	}
}

func TestPrompterPasswordDoesNotPrintSecret(t *testing.T) {
	var output bytes.Buffer
	prompter := NewPrompter(
		WithIO(strings.NewReader("super-secret-token\n"), &output),
		WithTheme(PlainTheme()),
	)

	got, err := prompter.Password("Custom agent bearer token (optional)")
	if err != nil {
		t.Fatalf("Password() returned error: %v", err)
	}
	if got != "super-secret-token" {
		t.Fatalf("Password() = %q, want secret", got)
	}
	if strings.Contains(output.String(), "super-secret-token") {
		t.Fatalf("Password() leaked secret in output: %q", output.String())
	}
	if !strings.Contains(output.String(), "configured") {
		t.Fatalf("Password() output should indicate configured secret, got %q", output.String())
	}
}
