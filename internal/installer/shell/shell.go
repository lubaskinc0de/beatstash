package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Run runs a command with stdin and returns its standard output; a failure
// carries the command's error output.
func Run(ctx context.Context, stdin, name string, args ...string) (string, error) {
	return Output(exec.CommandContext(ctx, name, args...), stdin) //nolint:gosec // G204: callers run fixed programs
}

// Output runs a prepared command the same way as Run.
func Output(cmd *exec.Cmd, stdin string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s %s: %s", cmd.Args[0], strings.Join(cmd.Args[1:], " "), message)
	}
	return stdout.String(), nil
}
