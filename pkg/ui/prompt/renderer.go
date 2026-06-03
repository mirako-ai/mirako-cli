package prompt

import (
	"fmt"
	"strings"
)

const defaultSearchSelectVisibleOptions = 8

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

// RenderSearchSelect renders an active searchable single-select prompt.
func (r Renderer) RenderSearchSelect(label string, options []SelectOption, selectedIndex int, query string, maxVisible int) string {
	theme := r.Theme.withDefaults()
	if maxVisible <= 0 {
		maxVisible = defaultSearchSelectVisibleOptions
	}
	if selectedIndex < 0 || selectedIndex >= len(options) {
		selectedIndex = 0
	}

	start, end := searchSelectWindow(len(options), selectedIndex, maxVisible)
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s\n", theme.Muted(strings.Repeat(theme.Symbols.Horizontal, 2)), theme.Bold(label), theme.Muted(strings.Repeat(theme.Symbols.Horizontal, 24)))
	fmt.Fprintf(&b, "%s %s %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted("Search:"), theme.Primary(query))
	fmt.Fprintf(&b, "%s %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted("↑/↓ move, type to filter, enter confirm"))
	fmt.Fprintf(&b, "%s\n", theme.Muted(theme.Symbols.Bar))

	if len(options) == 0 {
		fmt.Fprintf(&b, "%s  %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted("No matches"))
	} else {
		if start > 0 {
			fmt.Fprintf(&b, "%s %s %d more\n", theme.Muted(theme.Symbols.Bar), theme.Muted("↑"), start)
		}
		for i := start; i < end; i++ {
			option := options[i]
			pointer := " "
			radio := theme.Muted(theme.Symbols.InactiveRadio)
			labelStyle := theme.Primary
			if i == selectedIndex {
				pointer = theme.Symbols.Pointer
				radio = theme.Accent(theme.Symbols.ActiveRadio)
				labelStyle = theme.Bold
			}

			fmt.Fprintf(&b, "%s %s %s %s", theme.Muted(theme.Symbols.Bar), theme.Accent(pointer), radio, labelStyle(option.displayLabel()))
			if option.Hint != "" {
				fmt.Fprintf(&b, " %s", theme.Muted(fmt.Sprintf("(%s)", option.Hint)))
			}
			fmt.Fprintln(&b)
			if option.Description != "" {
				fmt.Fprintf(&b, "%s     %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted(option.Description))
			}
		}
		if end < len(options) {
			fmt.Fprintf(&b, "%s %s %d more\n", theme.Muted(theme.Symbols.Bar), theme.Muted("↓"), len(options)-end)
		}
	}

	fmt.Fprintf(&b, "%s\n", theme.Muted(theme.Symbols.Corner))
	return b.String()
}

func searchSelectWindow(total int, selectedIndex int, maxVisible int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if maxVisible <= 0 || total <= maxVisible {
		return 0, total
	}
	if selectedIndex < 0 {
		selectedIndex = 0
	}
	if selectedIndex >= total {
		selectedIndex = total - 1
	}

	start := selectedIndex - maxVisible + 1
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}
	return start, end
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

// RenderMultilineInput renders an active multiline text input prompt.
func (r Renderer) RenderMultilineInput(label string, defaultValue string) string {
	theme := r.Theme.withDefaults()
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", theme.Accent(theme.Symbols.ActiveStep), theme.Bold(label))
	if defaultValue != "" {
		fmt.Fprintf(&b, "%s  %s %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted("default:"), theme.Muted(defaultValue))
	}
	fmt.Fprintf(&b, "%s  %s\n", theme.Muted(theme.Symbols.Bar), theme.Muted("Enter two empty lines to finish; large pastes are supported."))
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
	fmt.Fprintf(&b, "%s \n", theme.Muted(theme.Symbols.Bar))
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
