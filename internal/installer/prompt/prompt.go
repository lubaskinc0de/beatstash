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

// ErrInputClosed ends the setup when its input runs out mid-question.
var ErrInputClosed = errors.New("input closed before setup finished")

// closed carries ErrInputClosed up to Guard, so questions return plain values.
type closed struct{}

type Terminal struct {
	in                                         *bufio.Reader
	secret                                     func() (string, error)
	out                                        io.Writer
	bold, dim, reset, blue, green, yellow, red string
	stage, stages                              int
}

func New(in io.Reader, out io.Writer) *Terminal {
	t := &Terminal{in: bufio.NewReader(in), out: out}
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		t.secret = func() (string, error) {
			b, err := term.ReadPassword(int(file.Fd()))
			_, _ = fmt.Fprintln(out)
			return string(b), err
		}
	}
	if file, ok := out.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		t.bold, t.dim, t.reset = "\033[1m", "\033[2m", "\033[0m"
		t.blue, t.green, t.yellow, t.red = "\033[34m", "\033[32m", "\033[33m", "\033[31m"
	}
	return t
}

// Guard turns running out of input inside fn into ErrInputClosed.
func Guard(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(closed); !ok {
				panic(r)
			}
			err = ErrInputClosed
		}
	}()
	return fn()
}

func (t *Terminal) Banner(title string, lines ...string) {
	t.printf("\n%s%s  %s%s\n\n", t.bold, t.blue, title, t.reset)
	for _, line := range lines {
		t.printf("%s  %s%s\n", t.dim, line, t.reset)
	}
	t.printf("\n")
}

// Stages sets how many stages the progress counts.
func (t *Terminal) Stages(n int) { t.stages = n }

func (t *Terminal) Stage(title string) {
	t.stage++
	t.printf("\n%s%s▸ Stage %d/%d · %s%s\n", t.bold, t.blue, t.stage, t.stages, title, t.reset)
}

func (t *Terminal) Say(format string, args ...any) {
	t.printf("  %s\n", fmt.Sprintf(format, args...))
}

// Step is an action the person takes outside the terminal.
func (t *Terminal) Step(format string, args ...any) {
	t.printf("  %s•%s %s\n", t.blue, t.reset, fmt.Sprintf(format, args...))
}

func (t *Terminal) Note(format string, args ...any) {
	t.printf("  %s%s%s\n", t.dim, fmt.Sprintf(format, args...), t.reset)
}

func (t *Terminal) Warn(format string, args ...any) {
	t.printf("  %s⚠ %s%s\n", t.yellow, fmt.Sprintf(format, args...), t.reset)
}

func (t *Terminal) Done(format string, args ...any) {
	t.printf("  %s✓%s %s\n", t.green, t.reset, fmt.Sprintf(format, args...))
}

// Block prints text indented as is, for snippets to copy.
func (t *Terminal) Block(text string) {
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		t.printf("    %s\n", line)
	}
}

func (t *Terminal) line() string {
	s, err := t.in.ReadString('\n')
	if err != nil && s == "" {
		panic(closed{})
	}
	return strings.TrimRight(s, "\r\n")
}

func (t *Terminal) ask(question, current string) {
	if current != "" {
		t.printf("  %s%s%s %s[Enter keeps current]%s ", t.bold, question, t.reset, t.dim, t.reset)
	} else {
		t.printf("  %s%s%s ", t.bold, question, t.reset)
	}
}

// Ask returns the answer, else current, else fallback.
func (t *Terminal) Ask(question, current, fallback string) string {
	t.ask(question, current)
	answer := strings.TrimSpace(t.line())
	return first(answer, current, fallback)
}

// Secret is Ask with hidden input when the input is a terminal.
func (t *Terminal) Secret(question, current string) string {
	t.ask(question, current)
	var answer string
	if t.secret != nil {
		var err error
		if answer, err = t.secret(); err != nil {
			panic(closed{})
		}
	} else {
		answer = t.line()
	}
	return first(answer, current)
}

// Require asks until the answer matches pattern.
func (t *Terminal) Require(question, current string, pattern *regexp.Regexp, secret bool) string {
	for {
		var answer string
		if secret {
			answer = t.Secret(question, current)
		} else {
			answer = t.Ask(question, current, "")
		}
		if pattern.MatchString(answer) {
			return answer
		}
		t.Warn("Please enter a value in the requested format.")
	}
}

// Confirm asks a yes/no question; Enter takes the default.
func (t *Terminal) Confirm(question string, yes bool) bool {
	choice := "[y/N]"
	if yes {
		choice = "[Y/n]"
	}
	t.printf("  %s? %s %s%s ", t.yellow, question, choice, t.reset)
	answer := strings.ToLower(strings.TrimSpace(t.line()))
	if answer == "" {
		return yes
	}
	return strings.HasPrefix(answer, "y")
}

func (t *Terminal) Pause(message string) {
	t.printf("  %s%s%s ", t.dim, message, t.reset)
	t.line()
}

// Diff shows the lines that change from before to after.
func (t *Terminal) Diff(name, before, after string) {
	t.printf("  %s--- %s%s\n", t.bold, name, t.reset)
	for _, l := range lineDiff(before, after) {
		switch l.op {
		case '-':
			t.printf("  %s- %s%s\n", t.red, l.text, t.reset)
		case '+':
			t.printf("  %s+ %s%s\n", t.green, l.text, t.reset)
		default:
			t.printf("  %s  %s%s\n", t.dim, l.text, t.reset)
		}
	}
}

// printf writes to the terminal; a person who closed it reads nothing anyway.
func (t *Terminal) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(t.out, format, args...)
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
