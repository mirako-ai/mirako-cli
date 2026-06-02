package prompt

import (
	"bytes"
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
