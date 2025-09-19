package ui

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
)

// Color definitions matching GitHub CLI aesthetics
var (
	// Header colors - light gray
	HeaderColor = color.New(color.FgHiBlack)

	// Timestamp colors - deeper gray
	TimestampColor = color.New(color.FgWhite)

	// ID colors - accent color (cyan/blue)
	IDColor = color.New(color.FgCyan)

	// Status colors - secondary accent with semantic colors
	StatusReady    = color.New(color.FgGreen)
	StatusError    = color.New(color.FgRed)
	StatusBuilding = color.New(color.FgYellow)
	StatusPending  = color.New(color.FgBlue)
	StatusDefault  = color.New(color.FgHiWhite)

	// Regular text
	TextColor = color.New(color.FgHiWhite)
)

// TableWriter provides a styled table writer similar to GitHub CLI
type TableWriter struct {
	writer *tabwriter.Writer
	header []string
}

// NewTableWriter creates a new styled table writer
func NewTableWriter(output io.Writer) *TableWriter {
	return &TableWriter{
		writer: tabwriter.NewWriter(output, 0, 0, 2, ' ', 0),
	}
}

// SetHeader sets the table header with styled formatting
func (t *TableWriter) SetHeader(headers []string) {
	t.header = headers
	styledHeaders := make([]string, len(headers))
	for i, h := range headers {
		styledHeaders[i] = HeaderColor.Sprint(strings.ToUpper(h))
	}
	fmt.Fprintln(t.writer, strings.Join(styledHeaders, "\t"))
}

// AddRow adds a row to the table with automatic styling based on content type
func (t *TableWriter) AddRow(values []interface{}) {
	styledValues := make([]string, len(values))
	for i, v := range values {
		styledValues[i] = t.styleValue(v, i)
	}
	fmt.Fprintln(t.writer, strings.Join(styledValues, "\t"))
}

// AddStyledRow adds a row with custom styling for specific columns
func (t *TableWriter) AddStyledRow(values []interface{}, columnStyles map[int]*color.Color) {
	styledValues := make([]string, len(values))
	for i, v := range values {
		if style, ok := columnStyles[i]; ok {
			styledValues[i] = style.Sprint(v)
		} else {
			styledValues[i] = t.styleValue(v, i)
		}
	}
	fmt.Fprintln(t.writer, strings.Join(styledValues, "\t"))
}

// styleValue applies appropriate styling based on the value type and column context
func (t *TableWriter) styleValue(value interface{}, columnIndex int) string {
	if value == nil {
		return ""
	}

	str := fmt.Sprintf("%v", value)

	// Apply styling based on column header
	if len(t.header) > columnIndex {
		header := strings.ToLower(t.header[columnIndex])

		switch {
		case strings.Contains(header, "id"):
			return IDColor.Sprint(str)
		case strings.Contains(header, "time") || strings.Contains(header, "created") || strings.Contains(header, "start"):
			return TimestampColor.Sprint(str)
		case strings.Contains(header, "status") || strings.Contains(header, "state"):
			return t.styleStatus(str)
		default:
			return TextColor.Sprint(str)
		}
	}

	// Default styling
	return TextColor.Sprint(str)
}

// styleStatus applies status-specific colors
func (t *TableWriter) styleStatus(status string) string {
	status = strings.ToLower(status)
	switch status {
	case "ready", "completed", "running", "active":
		return StatusReady.Sprint(strings.ToUpper(status))
	case "error", "failed", "cancelled", "timedout":
		return StatusError.Sprint(strings.ToUpper(status))
	case "building", "processing", "generating":
		return StatusBuilding.Sprint(strings.ToUpper(status))
	case "pending", "queued":
		return StatusPending.Sprint(strings.ToUpper(status))
	default:
		return StatusDefault.Sprint(strings.ToUpper(status))
	}
}

// Flush writes the table to output
func (t *TableWriter) Flush() error {
	return t.writer.Flush()
}

// FormatTimestamp formats a time.Time for consistent display in local timezone
func FormatTimestamp(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// Utility functions for common table operations

// NewAvatarTable creates a table for displaying avatar information
func NewAvatarTable(output io.Writer) *TableWriter {
	t := NewTableWriter(output)
	t.SetHeader([]string{"NAME", "ID", "STATUS", "CREATED"})
	return t
}

// NewSessionTable creates a table for displaying session information
func NewSessionTable(output io.Writer) *TableWriter {
	t := NewTableWriter(output)
	t.SetHeader([]string{"SESSION ID", "MODEL", "STATE", "START TIME"})
	return t
}

// NewVoiceProfileTable creates a table for displaying voice profile information
func NewVoiceProfileTable(output io.Writer) *TableWriter {
	t := NewTableWriter(output)
	t.SetHeader([]string{"ID", "NAME", "DESCRIPTION", "LANGUAGES"})
	return t
}
