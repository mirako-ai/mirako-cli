package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
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

// Prompter contains reusable prompt components backed by small terminal helpers.
type Prompter struct {
	input      io.Reader
	reader     *bufio.Reader
	output     io.Writer
	inputFD    int
	outputFD   int
	renderer   Renderer
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
		if _, err := fmt.Fprint(p.output, p.renderer.RenderInput(label, defaultValue)); err != nil {
			return "", err
		}

		answer, err := p.readLine()
		if err != nil && !(errors.Is(err, io.EOF) && answer != "") {
			return "", err
		}
		answer = strings.TrimSpace(answer)
		if answer == "" && defaultValue != "" {
			answer = defaultValue
		}
		if required && strings.TrimSpace(answer) == "" {
			if _, err := fmt.Fprint(p.output, p.renderer.RenderValidationError("This value is required.")); err != nil {
				return "", err
			}
			continue
		}

		if _, err := fmt.Fprint(p.output, p.renderer.RenderSubmitted(label, answer)); err != nil {
			return "", err
		}
		return answer, nil
	}
}

// Password shows a secret input prompt. Submitted output never includes the
// secret value.
func (p *Prompter) Password(label string) (string, error) {
	if _, err := fmt.Fprint(p.output, p.renderer.RenderInput(label, "")); err != nil {
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
	if _, err := fmt.Fprint(p.output, p.renderer.RenderSubmitted(label, displayValue)); err != nil {
		return "", err
	}
	return answer, nil
}

// Multiline wraps the existing survey multiline behavior for TTY usage and
// falls back to a simple line reader for non-terminal tests.
func (p *Prompter) Multiline(label string, defaultValue string, required bool) (string, error) {
	if p.inputIsTerminal() {
		var answer string
		prompt := &survey.Multiline{Message: label, Default: defaultValue}
		if err := survey.AskOne(prompt, &answer, surveyAskOptions(required)...); err != nil {
			return "", err
		}
		return answer, nil
	}
	return p.MultilineFallback(label, defaultValue, required)
}

// MultilineFallback reads lines until an empty line or EOF. It is primarily
// intended for tests and non-TTY prompt harnesses.
func (p *Prompter) MultilineFallback(label string, defaultValue string, required bool) (string, error) {
	for {
		if _, err := fmt.Fprint(p.output, p.renderer.RenderInput(label, defaultValue)); err != nil {
			return "", err
		}

		var lines []string
		for {
			line, err := p.readLine()
			if err != nil && !(errors.Is(err, io.EOF) && line != "") {
				if errors.Is(err, io.EOF) && len(lines) > 0 {
					break
				}
				return "", err
			}
			if line == "" {
				break
			}
			lines = append(lines, line)
			if errors.Is(err, io.EOF) {
				break
			}
			if _, err := fmt.Fprintf(p.output, "%s ", p.renderer.Theme.Accent(p.renderer.Theme.Symbols.Pointer)); err != nil {
				return "", err
			}
		}

		answer := strings.Join(lines, "\n")
		if strings.TrimSpace(answer) == "" && defaultValue != "" {
			answer = defaultValue
		}
		if required && strings.TrimSpace(answer) == "" {
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

func (p *Prompter) writeBlock(block string, previousLines int) int {
	if p.outputIsTerminal() && previousLines > 0 {
		_, _ = fmt.Fprintf(p.output, "\x1b[%dA\x1b[J", previousLines)
	}
	_, _ = fmt.Fprint(p.output, block)
	return lineCount(block)
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

func lineCount(value string) int {
	if value == "" {
		return 0
	}
	lines := strings.Count(value, "\n")
	if !strings.HasSuffix(value, "\n") {
		lines++
	}
	return lines
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

func surveyAskOptions(required bool) []survey.AskOpt {
	if !required {
		return nil
	}
	return []survey.AskOpt{survey.WithValidator(survey.Required)}
}
