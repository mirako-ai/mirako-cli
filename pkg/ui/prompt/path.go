package prompt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// PathInput shows a single-line file path prompt with simple tab completion.
// The prompt treats the whole line as the path prefix, so paths containing
// spaces do not need shell-style escaping while being edited.
func (p *Prompter) PathInput(label string, defaultValue string, required bool) (string, error) {
	for {
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

		activeBlock := p.renderer.RenderInput(label, defaultValue)
		if err := p.writePathPrompt(activeBlock); err != nil {
			restoreOnce()
			return "", err
		}

		visibleAnswer, err := p.readPathInputLine()
		restoreOnce()
		if err != nil && !(errors.Is(err, io.EOF) && visibleAnswer != "") {
			if errors.Is(err, ErrCancelled) {
				p.replaceActiveInput(activeBlock, visibleAnswer, p.renderer.RenderCancelled(label))
			}
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

func (p *Prompter) writePathPrompt(activeBlock string) error {
	if p.outputIsTerminal() {
		_, err := fmt.Fprint(p.output, terminalNewlines(activeBlock))
		return err
	}
	_, err := fmt.Fprint(p.output, activeBlock)
	return err
}

func (p *Prompter) readPathInputLine() (string, error) {
	var value []rune
	for {
		r, _, err := p.reader.ReadRune()
		if err != nil {
			return string(value), err
		}

		switch r {
		case '\r', '\n':
			p.writePathInputNewline()
			return string(value), nil
		case 3: // Ctrl-C
			return string(value), ErrCancelled
		case 4: // Ctrl-D
			if len(value) == 0 {
				return "", io.EOF
			}
			return string(value), io.EOF
		case '\t':
			current := string(value)
			completed := completePathPrefix(current)
			if completed != current {
				p.replacePathInputValue(current, completed)
				value = []rune(completed)
			}
		case '\b', 127:
			if len(value) > 0 {
				removed := value[len(value)-1]
				value = value[:len(value)-1]
				p.erasePathInputRune(removed)
			}
		case 27: // Escape or an escape sequence such as an arrow key.
			p.discardBufferedEscapeSequence()
		default:
			if unicode.IsControl(r) {
				continue
			}
			value = append(value, r)
			p.writePathInputText(string(r))
		}
	}
}

func (p *Prompter) writePathInputText(value string) {
	if p.outputIsTerminal() {
		_, _ = fmt.Fprint(p.output, value)
	}
}

func (p *Prompter) writePathInputNewline() {
	if p.outputIsTerminal() {
		_, _ = fmt.Fprint(p.output, "\r\n")
	}
}

func (p *Prompter) replacePathInputValue(oldValue string, newValue string) {
	if !p.outputIsTerminal() {
		return
	}
	if strings.HasPrefix(newValue, oldValue) {
		_, _ = fmt.Fprint(p.output, newValue[len(oldValue):])
		return
	}
	for _, r := range []rune(oldValue) {
		p.erasePathInputRune(r)
	}
	_, _ = fmt.Fprint(p.output, newValue)
}

func (p *Prompter) erasePathInputRune(r rune) {
	if !p.outputIsTerminal() {
		return
	}
	width := approximateDisplayWidth(string(r))
	if width < 1 {
		width = 1
	}
	_, _ = fmt.Fprint(p.output, strings.Repeat("\b \b", width))
}

func (p *Prompter) discardBufferedEscapeSequence() {
	if p.reader.Buffered() == 0 {
		return
	}
	prefix, err := p.reader.ReadByte()
	if err != nil || (prefix != '[' && prefix != 'O') {
		return
	}
	for p.reader.Buffered() > 0 {
		b, err := p.reader.ReadByte()
		if err != nil {
			return
		}
		if b >= 0x40 && b <= 0x7e {
			return
		}
	}
}

func completePathPrefix(prefix string) string {
	if prefix == "~" {
		if _, err := os.UserHomeDir(); err == nil {
			return "~/"
		}
		return prefix
	}

	displayDir, baseName := filepath.Split(prefix)
	lookupDir := displayDir
	if lookupDir == "" {
		lookupDir = "."
	}

	expandedLookupDir, err := expandHomePath(lookupDir)
	if err != nil {
		return prefix
	}
	entries, err := os.ReadDir(expandedLookupDir)
	if err != nil {
		return prefix
	}

	matches := make([]os.DirEntry, 0)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), baseName) {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return prefix
	}

	if len(matches) == 1 {
		completed := displayDir + matches[0].Name()
		if matches[0].IsDir() {
			completed += string(os.PathSeparator)
		}
		return completed
	}

	common := matches[0].Name()
	for _, match := range matches[1:] {
		common = commonPrefix(common, match.Name())
		if common == baseName {
			return prefix
		}
	}
	if common == "" || common == baseName {
		return prefix
	}
	return displayDir + common
}

func commonPrefix(first, second string) string {
	firstRunes := []rune(first)
	secondRunes := []rune(second)
	limit := len(firstRunes)
	if len(secondRunes) < limit {
		limit = len(secondRunes)
	}

	for i := 0; i < limit; i++ {
		if firstRunes[i] != secondRunes[i] {
			return string(firstRunes[:i])
		}
	}
	return string(firstRunes[:limit])
}

func expandHomePath(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	if os.PathSeparator != '/' && strings.HasPrefix(path, "~"+string(os.PathSeparator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
