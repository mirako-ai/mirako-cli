package prompt

import (
	"os"

	"github.com/fatih/color"
)

// Symbols groups the small set of glyphs used by prompt components.
type Symbols struct {
	ActiveStep    string
	SubmittedStep string
	CancelledStep string
	Bar           string
	Horizontal    string
	Pointer       string
	ActiveRadio   string
	InactiveRadio string
	Bullet        string
}

// DefaultSymbols returns Clack-inspired symbols for prompt rendering.
func DefaultSymbols() Symbols {
	return Symbols{
		ActiveStep:    "◆",
		SubmittedStep: "◇",
		CancelledStep: "■",
		Bar:           "│",
		Horizontal:    "─",
		Pointer:       "❯",
		ActiveRadio:   "●",
		InactiveRadio: "○",
		Bullet:        "•",
	}
}

// Theme centralizes prompt text roles and symbols.
type Theme struct {
	Symbols Symbols
	NoColor bool
}

// DefaultTheme returns the default prompt theme. Color follows fatih/color's
// TTY detection and the NO_COLOR convention.
func DefaultTheme() Theme {
	return Theme{
		Symbols: DefaultSymbols(),
		NoColor: color.NoColor || os.Getenv("NO_COLOR") != "",
	}
}

// PlainTheme returns the default symbols with color disabled, useful for tests.
func PlainTheme() Theme {
	theme := DefaultTheme()
	theme.NoColor = true
	return theme
}

func (t Theme) withDefaults() Theme {
	if t.Symbols.ActiveStep == "" {
		t.Symbols = DefaultSymbols()
	}
	return t
}

// Primary styles normal foreground text.
func (t Theme) Primary(value string) string {
	return t.style(value, color.FgHiWhite)
}

// Accent styles focused prompt symbols and selected controls.
func (t Theme) Accent(value string) string {
	return t.style(value, color.FgCyan)
}

// Muted styles secondary descriptions and guide bars.
func (t Theme) Muted(value string) string {
	return t.style(value, color.FgHiBlack)
}

// Success styles successful outcomes.
func (t Theme) Success(value string) string {
	return t.style(value, color.FgGreen)
}

// Danger styles validation and cancellation text.
func (t Theme) Danger(value string) string {
	return t.style(value, color.FgRed)
}

// Bold styles important labels without changing their semantic role.
func (t Theme) Bold(value string) string {
	return t.style(value, color.Bold)
}

func (t Theme) style(value string, attrs ...color.Attribute) string {
	c := color.New(attrs...)
	if t.NoColor {
		c.DisableColor()
	} else {
		c.EnableColor()
	}
	return c.Sprint(value)
}
