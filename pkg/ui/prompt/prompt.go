package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"golang.org/x/term"
)

// ErrCancelled is returned when the user cancels a prompt.
var ErrCancelled = errors.New("prompt cancelled")

type key int

const (
	keyUnknown key = iota
	keyEnter
	keyUp
	keyDown
	keyCancel
)

const (
	bracketedPasteStart   = "\x1b[200~"
	bracketedPasteEnd     = "\x1b[201~"
	bracketedPasteEnable  = "\x1b[?2004h"
	bracketedPasteDisable = "\x1b[?2004l"
)

// Prompter contains reusable prompt components backed by small terminal helpers.
type Prompter struct {
	input      io.Reader
	reader     *bufio.Reader
	output     io.Writer
	inputFD    int
	outputFD   int
	renderer   Renderer
	columns    int
	passwordFn func(fd int) ([]byte, error)
}

// Option customizes a Prompter.
type Option func(*Prompter)

// NewPrompter creates a prompter using stdin/stdout and the default theme.
func NewPrompter(options ...Option) *Prompter {
	p := &Prompter{
		input:      os.Stdin,
		output:     os.Stdout,
		inputFD:    int(os.Stdin.Fd()),
		outputFD:   int(os.Stdout.Fd()),
		renderer:   NewRenderer(DefaultTheme()),
		passwordFn: term.ReadPassword,
	}
	for _, option := range options {
		option(p)
	}
	p.reader = bufio.NewReader(p.input)
	return p
}

// WithTheme applies a prompt theme.
func WithTheme(theme Theme) Option {
	return func(p *Prompter) {
		p.renderer = NewRenderer(theme)
	}
}

// WithInput sets the prompt input. Terminal raw mode is enabled only when the
// reader is an *os.File backed by a TTY.
func WithInput(input io.Reader) Option {
	return func(p *Prompter) {
		p.input = input
		p.inputFD = fileDescriptor(input)
	}
}

// WithOutput sets the prompt output. Dynamic select rendering is cleared only
// when the writer is an *os.File backed by a TTY.
func WithOutput(output io.Writer) Option {
	return func(p *Prompter) {
		p.output = output
		p.outputFD = writerFileDescriptor(output)
	}
}

// WithIO sets both prompt input and output.
func WithIO(input io.Reader, output io.Writer) Option {
	return func(p *Prompter) {
		WithInput(input)(p)
		WithOutput(output)(p)
	}
}

// WithColumns overrides terminal width for wrapped-row calculations. It is
// primarily useful for tests and prompt harnesses that render to non-file
// writers.
func WithColumns(columns int) Option {
	return func(p *Prompter) {
		p.columns = columns
	}
}

// Select shows a single-select prompt with arrow-key navigation.
func (p *Prompter) Select(label string, options []SelectOption, defaultValue string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("select prompt %q has no options", label)
	}

	selected := selectedOptionIndex(options, defaultValue)
	restore, err := p.enterRawMode()
	if err != nil {
		return "", err
	}
	restored := false
	restoreOnce := func() {
		if !restored {
			restore()
			restored = true
		}
	}
	defer restoreOnce()

	lines := p.writeBlock(p.renderer.RenderSelect(label, options, selected), 0)
	for {
		pressed, err := p.readKey()
		if err != nil {
			if errors.Is(err, io.EOF) {
				restoreOnce()
				p.writeBlock(p.renderer.RenderCancelled(label), lines)
				return "", ErrCancelled
			}
			return "", err
		}

		switch pressed {
		case keyUp:
			selected--
			if selected < 0 {
				selected = len(options) - 1
			}
			lines = p.writeBlock(p.renderer.RenderSelect(label, options, selected), lines)
		case keyDown:
			selected = (selected + 1) % len(options)
			lines = p.writeBlock(p.renderer.RenderSelect(label, options, selected), lines)
		case keyEnter:
			restoreOnce()
			p.writeBlock(p.renderer.RenderSubmitted(label, options[selected].displayLabel()), lines)
			return options[selected].Result(), nil
		case keyCancel:
			restoreOnce()
			p.writeBlock(p.renderer.RenderCancelled(label), lines)
			return "", ErrCancelled
		}
	}
}

// Input shows a text prompt with optional default and required validation.
func (p *Prompter) Input(label string, defaultValue string, required bool) (string, error) {
	for {
		activeBlock := p.renderer.RenderInput(label, defaultValue)
		if _, err := fmt.Fprint(p.output, activeBlock); err != nil {
			return "", err
		}

		visibleAnswer, err := p.readLine()
		if err != nil && !(errors.Is(err, io.EOF) && visibleAnswer != "") {
			return "", err
		}
		answer := strings.TrimSpace(visibleAnswer)
		if answer == "" && defaultValue != "" {
			answer = defaultValue
		}
		if required && strings.TrimSpace(answer) == "" {
			if _, err := fmt.Fprint(p.output, p.renderer.RenderValidationError("This value is required.")); err != nil {
				return "", err
			}
			continue
		}

		p.replaceActiveInput(activeBlock, visibleAnswer, p.renderer.RenderSubmitted(label, answer))
		return answer, nil
	}
}

// Password shows a secret input prompt. Submitted output never includes the
// secret value.
func (p *Prompter) Password(label string) (string, error) {
	activeBlock := p.renderer.RenderInput(label, "")
	if _, err := fmt.Fprint(p.output, activeBlock); err != nil {
		return "", err
	}

	var answer string
	if p.inputIsTerminal() {
		data, err := p.passwordFn(p.inputFD)
		if err != nil {
			return "", err
		}
		answer = strings.TrimSpace(string(data))
		if _, err := fmt.Fprintln(p.output); err != nil {
			return "", err
		}
	} else {
		line, err := p.readLine()
		if err != nil && !(errors.Is(err, io.EOF) && line != "") {
			return "", err
		}
		answer = strings.TrimSpace(line)
	}

	displayValue := ""
	if answer != "" {
		displayValue = "configured"
	}
	p.replaceActiveInput(activeBlock, "", p.renderer.RenderSubmitted(label, displayValue))
	return answer, nil
}

// Multiline reads lines in canonical terminal mode until two consecutive empty
// lines or EOF. Avoiding raw-mode per-rune reads keeps large clipboard pastes
// from overflowing terminal/tmux input queues before the prompt can consume
// them.
func (p *Prompter) Multiline(label string, defaultValue string, required bool) (string, error) {
	restoreBracketedPaste := p.enableBracketedPaste()
	defer restoreBracketedPaste()
	return p.MultilineFallback(label, defaultValue, required)
}

// MultilineFallback reads lines until two consecutive empty lines or EOF. It is
// primarily intended for tests and non-TTY prompt harnesses.
func (p *Prompter) MultilineFallback(label string, defaultValue string, required bool) (string, error) {
	for {
		if _, err := fmt.Fprint(p.output, p.renderer.RenderMultilineInput(label, defaultValue)); err != nil {
			return "", err
		}

		answer, reachedEOF, err := p.readMultilineAnswer()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(answer) == "" && defaultValue != "" {
			answer = defaultValue
		}
		if required && strings.TrimSpace(answer) == "" {
			if reachedEOF {
				return "", io.EOF
			}
			if _, err := fmt.Fprint(p.output, p.renderer.RenderValidationError("This value is required.")); err != nil {
				return "", err
			}
			continue
		}
		if _, err := fmt.Fprint(p.output, p.renderer.RenderSubmitted(label, "entered")); err != nil {
			return "", err
		}
		return answer, nil
	}
}

func (p *Prompter) readMultilineAnswer() (string, bool, error) {
	var lines []string
	pendingEmptyLines := 0
	inBracketedPaste := false
	reachedEOF := false

	flushPendingEmptyLines := func() {
		for pendingEmptyLines > 0 {
			lines = append(lines, "")
			pendingEmptyLines--
		}
	}

	for {
		line, err := p.readLine()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return "", reachedEOF, err
			}
			reachedEOF = true
			if line == "" {
				if len(lines) == 0 && pendingEmptyLines == 0 {
					return "", reachedEOF, io.EOF
				}
				break
			}
		}

		line, lineInBracketedPaste := stripBracketedPasteMarkers(line, &inBracketedPaste)
		if line == "" && !lineInBracketedPaste {
			pendingEmptyLines++
			if pendingEmptyLines >= 2 {
				break
			}
		} else {
			flushPendingEmptyLines()
			lines = append(lines, line)
		}

		if reachedEOF {
			break
		}
		if err := p.writeMultilineContinuation(lineInBracketedPaste); err != nil {
			return "", reachedEOF, err
		}
	}

	if reachedEOF {
		flushPendingEmptyLines()
	}

	return strings.Join(lines, "\n"), reachedEOF, nil
}

func stripBracketedPasteMarkers(line string, inPaste *bool) (string, bool) {
	lineInBracketedPaste := *inPaste
	for {
		start := strings.Index(line, bracketedPasteStart)
		end := strings.Index(line, bracketedPasteEnd)
		switch {
		case start == -1 && end == -1:
			return line, lineInBracketedPaste || *inPaste
		case start != -1 && (end == -1 || start < end):
			line = line[:start] + line[start+len(bracketedPasteStart):]
			*inPaste = true
			lineInBracketedPaste = true
		default:
			line = line[:end] + line[end+len(bracketedPasteEnd):]
			*inPaste = false
			lineInBracketedPaste = true
		}
	}
}

func (p *Prompter) writeMultilineContinuation(lineInBracketedPaste bool) error {
	if lineInBracketedPaste {
		return nil
	}
	_, err := fmt.Fprintf(p.output, "%s ", p.renderer.Theme.Accent(p.renderer.Theme.Symbols.Pointer))
	return err
}

func (p *Prompter) enableBracketedPaste() func() {
	if !p.inputIsTerminal() || !p.outputIsTerminal() {
		return func() {}
	}
	if _, err := fmt.Fprint(p.output, bracketedPasteEnable); err != nil {
		return func() {}
	}
	return func() {
		_, _ = fmt.Fprint(p.output, bracketedPasteDisable)
	}
}

func selectedOptionIndex(options []SelectOption, defaultValue string) int {
	if defaultValue == "" {
		return 0
	}
	for i, option := range options {
		if option.Result() == defaultValue || option.displayLabel() == defaultValue {
			return i
		}
	}
	return 0
}

func (p *Prompter) enterRawMode() (func(), error) {
	if !p.inputIsTerminal() {
		return func() {}, nil
	}
	state, err := term.MakeRaw(p.inputFD)
	if err != nil {
		return nil, err
	}
	return func() { _ = term.Restore(p.inputFD, state) }, nil
}

func (p *Prompter) inputIsTerminal() bool {
	return p.inputFD >= 0 && term.IsTerminal(p.inputFD)
}

func (p *Prompter) outputIsTerminal() bool {
	return p.outputFD >= 0 && term.IsTerminal(p.outputFD)
}

func (p *Prompter) replaceActiveInput(activeBlock string, visibleAnswer string, submittedBlock string) {
	if !p.outputIsTerminal() {
		_, _ = fmt.Fprint(p.output, submittedBlock)
		return
	}

	previousLines := visualLineCount(activeBlock+visibleAnswer+"\n", p.outputColumns())
	p.writeBlock(submittedBlock, previousLines)
}

func (p *Prompter) writeBlock(block string, previousLines int) int {
	if p.outputIsTerminal() && previousLines > 0 {
		_, _ = fmt.Fprintf(p.output, "\x1b[%dA\x1b[J", previousLines)
	}
	if p.outputIsTerminal() {
		_, _ = fmt.Fprint(p.output, terminalNewlines(block))
	} else {
		_, _ = fmt.Fprint(p.output, block)
	}
	return visualLineCount(block, p.outputColumns())
}

func (p *Prompter) readKey() (key, error) {
	b, err := p.reader.ReadByte()
	if err != nil {
		return keyUnknown, err
	}

	switch b {
	case '\r', '\n':
		return keyEnter, nil
	case 3:
		return keyCancel, nil
	case 27:
		return p.readEscapeKey()
	case 'j', 'J':
		return keyDown, nil
	case 'k', 'K':
		return keyUp, nil
	default:
		return keyUnknown, nil
	}
}

func (p *Prompter) readEscapeKey() (key, error) {
	if p.reader.Buffered() < 2 {
		return keyCancel, nil
	}

	prefix, err := p.reader.ReadByte()
	if err != nil {
		return keyCancel, nil
	}
	direction, err := p.reader.ReadByte()
	if err != nil {
		return keyCancel, nil
	}
	if prefix != '[' {
		return keyCancel, nil
	}
	switch direction {
	case 'A':
		return keyUp, nil
	case 'B':
		return keyDown, nil
	default:
		return keyUnknown, nil
	}
}

func (p *Prompter) readLine() (string, error) {
	line, err := p.reader.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil && line != "" {
		return line, err
	}
	return line, err
}

func (p *Prompter) outputColumns() int {
	if p.columns > 0 {
		return p.columns
	}
	if p.outputFD >= 0 {
		columns, _, err := term.GetSize(p.outputFD)
		if err == nil && columns > 0 {
			return columns
		}
	}
	return 80
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func visualLineCount(value string, columns int) int {
	if value == "" {
		return 0
	}
	if columns <= 0 {
		columns = 80
	}

	logicalLines := strings.Split(strings.TrimSuffix(value, "\n"), "\n")
	rows := 0
	for _, line := range logicalLines {
		width := approximateDisplayWidth(stripANSI(line))
		lineRows := (width + columns - 1) / columns
		if lineRows < 1 {
			lineRows = 1
		}
		rows += lineRows
	}
	return rows
}

func stripANSI(value string) string {
	return ansiEscapePattern.ReplaceAllString(value, "")
}

func terminalNewlines(value string) string {
	// term.MakeRaw disables the terminal's automatic LF -> CRLF output
	// processing. Without explicit carriage returns, each rendered prompt line
	// starts wherever the previous line ended, producing a diagonal layout.
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}

func approximateDisplayWidth(value string) int {
	width := 0
	for _, r := range value {
		if r == 0 {
			continue
		}
		if isWideRune(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

func isWideRune(r rune) bool {
	return (r >= 0x1100 && r <= 0x115f) ||
		(r >= 0x231a && r <= 0x231b) ||
		(r >= 0x2329 && r <= 0x232a) ||
		(r >= 0x23e9 && r <= 0x23ec) ||
		r == 0x23f0 ||
		r == 0x23f3 ||
		(r >= 0x25fd && r <= 0x25fe) ||
		(r >= 0x2614 && r <= 0x2615) ||
		(r >= 0x2648 && r <= 0x2653) ||
		r == 0x267f ||
		r == 0x2693 ||
		r == 0x26a1 ||
		(r >= 0x26aa && r <= 0x26ab) ||
		(r >= 0x26bd && r <= 0x26be) ||
		(r >= 0x26c4 && r <= 0x26c5) ||
		r == 0x26ce ||
		r == 0x26d4 ||
		r == 0x26ea ||
		(r >= 0x26f2 && r <= 0x26f3) ||
		r == 0x26f5 ||
		r == 0x26fa ||
		r == 0x26fd ||
		r == 0x2705 ||
		(r >= 0x270a && r <= 0x270b) ||
		r == 0x2728 ||
		r == 0x274c ||
		r == 0x274e ||
		(r >= 0x2753 && r <= 0x2755) ||
		r == 0x2757 ||
		(r >= 0x2795 && r <= 0x2797) ||
		r == 0x27b0 ||
		r == 0x27bf ||
		(r >= 0x2b1b && r <= 0x2b1c) ||
		r == 0x2b50 ||
		r == 0x2b55 ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xa960 && r <= 0xa97c) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f000 && r <= 0x1f9ff)
}

func fileDescriptor(reader io.Reader) int {
	file, ok := reader.(*os.File)
	if !ok {
		return -1
	}
	return int(file.Fd())
}

func writerFileDescriptor(writer io.Writer) int {
	file, ok := writer.(*os.File)
	if !ok {
		return -1
	}
	return int(file.Fd())
}
