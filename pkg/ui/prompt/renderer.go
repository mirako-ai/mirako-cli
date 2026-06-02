package prompt

import (
	"fmt"
	"strings"
)

// SelectOption describes a reusable single-select option.
type SelectOption struct {
	Label       string
	Value       string
	Description string
	Hint        string
}

// Result returns the value submitted by the prompt.
func (o SelectOption) Result() string {
	if o.Value != "" {
		return o.Value
	}
	return o.Label
}

func (o SelectOption) displayLabel() string {
	if o.Label != "" {
		return o.Label
	}
	return o.Value
}

func (o SelectOption) supportingText() string {
	if o.Description != "" {
		return o.Description
	}
	return o.Hint
}

// Renderer renders prompt components without reading from a terminal.
type Renderer struct {
	Theme Theme
}

// NewRenderer returns a renderer using the provided theme.
func NewRenderer(theme Theme) Renderer {
	return Renderer{Theme: theme.withDefaults()}
}

// RenderSelect renders an active single-select prompt.
func (r Renderer) RenderSelect(label string, options []SelectOption, selectedIndex int) string {
	theme := r.Theme.withDefaults()
	if selectedIndex < 0 || selectedIndex >= len(options) {
		selectedIndex = 0
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", theme.Accent(theme.Symbols.ActiveStep), theme.Bold(label))
	fmt.Fprintf(&b, "%s\n", theme.Muted(theme.Symbols.Bar))
	for i, option := range options {
		pointer := " "
		radio := theme.Muted(theme.Symbols.InactiveRadio)
		labelStyle := theme.Primary
		if i == selectedIndex {
			pointer = theme.Symbols.Pointer
			radio = theme.Accent(theme.Symbols.ActiveRadio)
			labelStyle = theme.Bold
		}

		fmt.Fprintf(&b, "%s %s %s %s\n", theme.Muted(theme.Symbols.Bar), theme.Accent(pointer), radio, labelStyle(option.displayLabel()))
		if text := option.supportingText(); text != "" {
			fmt.Fprintf(&b, "%s     %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted(text))
		}
	}
	fmt.Fprintf(&b, "%s\n", theme.Muted(theme.Symbols.Bar))
	fmt.Fprintf(&b, "%s  %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted("Use ↑/↓ to choose, Enter to submit"))
	fmt.Fprintf(&b, "%s\n", theme.Muted(theme.Symbols.Corner))
	return b.String()
}

// RenderInput renders an active text input prompt.
func (r Renderer) RenderInput(label string, defaultValue string) string {
	theme := r.Theme.withDefaults()
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", theme.Accent(theme.Symbols.ActiveStep), theme.Bold(label))
	if defaultValue != "" {
		fmt.Fprintf(&b, "%s  %s %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted("default:"), theme.Muted(defaultValue))
	}
	fmt.Fprintf(&b, "%s ", theme.Accent(theme.Symbols.Pointer))
	return b.String()
}

// RenderSubmitted renders a submitted prompt result.
func (r Renderer) RenderSubmitted(label string, value string) string {
	theme := r.Theme.withDefaults()
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", theme.Success(theme.Symbols.SubmittedStep), theme.Bold(label))
	if value != "" {
		fmt.Fprintf(&b, "%s  %s\n", theme.Muted(theme.Symbols.Bar), theme.Primary(value))
	}
	return b.String()
}

// RenderCancelled renders a cancelled prompt.
func (r Renderer) RenderCancelled(label string) string {
	theme := r.Theme.withDefaults()
	return fmt.Sprintf("%s %s\n", theme.Danger(theme.Symbols.CancelledStep), theme.Bold(label))
}

// RenderValidationError renders inline validation feedback.
func (r Renderer) RenderValidationError(message string) string {
	theme := r.Theme.withDefaults()
	return fmt.Sprintf("%s  %s\n", theme.Muted(theme.Symbols.Bar), theme.Danger(message))
}

// RenderSection renders a small reusable section divider.
func (r Renderer) RenderSection(label string) string {
	theme := r.Theme.withDefaults()
	return fmt.Sprintf("%s %s %s\n", theme.Muted(strings.Repeat(theme.Symbols.Horizontal, 2)), theme.Bold(label), theme.Muted(strings.Repeat(theme.Symbols.Horizontal, 2)))
}
