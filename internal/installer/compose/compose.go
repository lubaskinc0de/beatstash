package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lubaskinc0de/beatstash/internal/installer/config"
	"github.com/lubaskinc0de/beatstash/internal/installer/shell"
)

// Name is the Compose project name the deployment template declares.
const Name = "beatstash"

// ProjectLabel marks the containers and volumes of a Compose project.
const ProjectLabel = "com.docker.compose.project"

// Project is a deployment directory: compose.yml, .env and config.toml.
type Project struct {
	Name string
	Dir  string
	Out  io.Writer
}

func (p Project) Path(name string) string { return filepath.Join(p.Dir, name) }

func (p Project) Env() config.Env { return config.Env{Path: p.Path(".env")} }

// Run streams the output of docker compose to the terminal.
func (p Project) Run(ctx context.Context, args ...string) error {
	cmd := p.command(ctx, args)
	cmd.Stdout, cmd.Stderr = p.Out, p.Out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (p Project) Output(ctx context.Context, args ...string) (string, error) {
	return shell.Output(p.command(ctx, args), "")
}

// Pipe runs docker compose with stdout going to w.
func (p Project) Pipe(ctx context.Context, w io.Writer, args ...string) error {
	cmd := p.command(ctx, args)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = w, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (p Project) command(ctx context.Context, args []string) *exec.Cmd {
	// No -f: Compose then takes the files from COMPOSE_FILE in .env, as it
	// does when a person runs it in the directory.
	base := []string{"compose", "--project-directory", p.Dir, "--env-file", p.Path(".env")}
	if p.Name != "" {
		base = append(base, "--project-name", p.Name)
	}
	cmd := exec.CommandContext(ctx, "docker", append(base, args...)...) //nolint:gosec // G204: the installer drives the docker CLI
	cmd.Dir = p.Dir
	// A variable exported in the shell would override the one in .env.
	values, _ := p.Env().Read()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, ok := values[name]; !ok {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	return cmd
}

// Address is where the host reaches a service's published port.
func (p Project) Address(ctx context.Context, service, port string) (string, error) {
	address, err := p.Output(ctx, "port", service, port)
	if err != nil {
		return "", err
	}
	address = strings.TrimSpace(strings.Split(address, "\n")[0])
	address = strings.Replace(address, "0.0.0.0:", "127.0.0.1:", 1)
	if address == "" || strings.HasSuffix(address, ":0") {
		return "", fmt.Errorf("%s does not publish port %s", service, port)
	}
	return address, nil
}

// Container is the id of the service's container, empty when there is none.
func (p Project) Container(ctx context.Context, service string) (string, error) {
	id, err := p.Output(ctx, "ps", "-q", service)
	return strings.TrimSpace(id), err
}

// Running is the services whose containers run now.
func (p Project) Running(ctx context.Context) ([]string, error) {
	out, err := p.Output(ctx, "ps", "--status", "running", "--services")
	return strings.Fields(out), err
}

// Volumes are the named volumes of the project, created or not.
func Volumes(ctx context.Context, name string) ([]string, error) {
	out, err := shell.Run(ctx, "", "docker", "volume", "ls", "-q", "--filter", "label="+ProjectLabel+"="+name)
	return strings.Fields(out), err
}

// Owners are the directories that run containers of the project.
func Owners(ctx context.Context, name string) ([]string, error) {
	ids, err := shell.Run(ctx, "", "docker", "ps", "-aq", "--filter", "label="+ProjectLabel+"="+name)
	if err != nil || strings.TrimSpace(ids) == "" {
		return nil, err
	}
	out, err := shell.Run(ctx, "", "docker", append([]string{"inspect", "--format", `{{index .Config.Labels "com.docker.compose.project.working_dir"}}`}, strings.Fields(ids)...)...)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, dir := range strings.Fields(out) {
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}
