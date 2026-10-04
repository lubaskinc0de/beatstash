package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/lubaskinc0de/beatstash/internal/installer/compose"
	"github.com/lubaskinc0de/beatstash/internal/installer/config"
	"github.com/lubaskinc0de/beatstash/internal/installer/shell"
)

type Kind int

const (
	// HostContainer is a Caddy container on the host network: it reaches
	// Navidrome at the port published on the host loopback.
	HostContainer Kind = iota
	// BridgeContainer is a Caddy container on a Docker network, where the
	// host loopback is out of reach.
	BridgeContainer
	// Service is Caddy installed on the host and run by systemd.
	Service
)

// ErrDrift means the running config differs from the Caddyfile, e.g. after
// changes through the admin API, which a reload from the file would erase.
var ErrDrift = errors.New("the running Caddy config differs from its Caddyfile")

// ErrNoAdmin means Caddy runs without its admin API, so neither a reload nor
// the drift check can reach it.
var ErrNoAdmin = errors.New("caddy runs without its admin API")

// Server is a Caddy that serves other sites already.
type Server struct {
	Kind Kind
	// Name is the container name or the systemd unit.
	Name      string
	container string
	// File is the Caddyfile on the host; empty when the container keeps
	// its config inside the image.
	File string
	// config is the Caddyfile path where Caddy reads it.
	config string
	client *http.Client
}

func (s Server) String() string {
	switch s.Kind {
	case HostContainer:
		return fmt.Sprintf("container %s on the host network, %s", s.Name, s.File)
	case BridgeContainer:
		return fmt.Sprintf("container %s on a Docker network", s.Name)
	default:
		return fmt.Sprintf("systemd service %s, %s", s.Name, s.File)
	}
}

type inspected struct {
	ID     string
	Name   string
	Config struct {
		Image      string
		Cmd        []string
		Entrypoint []string
		Labels     map[string]string
	}
	HostConfig struct{ NetworkMode string }
	Mounts     []struct {
		Type        string
		Source      string
		Destination string
	}
}

// Find lists the Caddy servers running on this host, except the one the
// beatstash project itself may run.
func Find(ctx context.Context, client *http.Client, projectName, containerFilter string) ([]Server, error) {
	var servers []Server
	args := []string{"ps", "-q"}
	if containerFilter != "" {
		args = append(args, "--filter", containerFilter)
	}
	ids, err := shell.Run(ctx, "", "docker", args...)
	if err != nil {
		return nil, err
	}
	if fields := strings.Fields(ids); len(fields) > 0 {
		out, err := shell.Run(ctx, "", "docker", append([]string{"inspect"}, fields...)...)
		if err != nil {
			return nil, err
		}
		var containers []inspected
		if err := json.Unmarshal([]byte(out), &containers); err != nil {
			return nil, err
		}
		for _, c := range containers {
			if isCaddy(c, projectName) {
				servers = append(servers, fromContainer(c, client))
			}
		}
	}
	if containerFilter == "" {
		if _, err := shell.Run(ctx, "", "systemctl", "is-active", "--quiet", "caddy"); err == nil {
			servers = append(servers, Server{Kind: Service, Name: "caddy", File: "/etc/caddy/Caddyfile", config: "/etc/caddy/Caddyfile", client: client})
		}
	}
	return servers, nil
}

func isCaddy(c inspected, projectName string) bool {
	if c.Config.Labels[compose.ProjectLabel] == projectName {
		return false
	}
	image := c.Config.Image
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	name := path.Base(image)
	if colon := strings.Index(name, ":"); colon >= 0 {
		name = name[:colon]
	}
	return strings.HasPrefix(name, "caddy")
}

func fromContainer(c inspected, client *http.Client) Server {
	s := Server{Kind: BridgeContainer, Name: strings.TrimPrefix(c.Name, "/"), container: c.ID, config: "/etc/caddy/Caddyfile", client: client}
	if c.HostConfig.NetworkMode == "host" {
		s.Kind = HostContainer
	}
	args := append(append([]string{}, c.Config.Entrypoint...), c.Config.Cmd...)
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			s.config = args[i+1]
		}
	}
	for _, m := range c.Mounts {
		if m.Type != "bind" {
			continue
		}
		if m.Destination == s.config {
			s.File = m.Source
		} else if rel, err := filepath.Rel(m.Destination, s.config); err == nil && !strings.HasPrefix(rel, "..") && s.File == "" {
			s.File = filepath.Join(m.Source, rel)
		}
	}
	return s
}

func (s Server) Read() (string, error) {
	data, err := os.ReadFile(s.File)
	return string(data), err
}

// CheckDrift compares the config Caddy runs with the one its Caddyfile gives.
func (s Server) CheckDrift(ctx context.Context) error {
	adapted, err := s.adapt(ctx)
	if err != nil {
		return err
	}
	admin, err := adminAddress(adapted)
	if err != nil {
		return err
	}
	live, err := s.live(ctx, admin)
	if err != nil {
		return fmt.Errorf("cannot read the running Caddy config: %w", err)
	}
	var want, have any
	if err := json.Unmarshal([]byte(adapted), &want); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(live), &have); err != nil {
		return err
	}
	if !reflect.DeepEqual(want, have) {
		return ErrDrift
	}
	return nil
}

func adminAddress(adapted string) (string, error) {
	var cfg struct {
		Admin struct {
			Disabled bool   `json:"disabled"`
			Listen   string `json:"listen"`
		} `json:"admin"`
	}
	if err := json.Unmarshal([]byte(adapted), &cfg); err != nil {
		return "", err
	}
	switch {
	case cfg.Admin.Disabled || strings.HasPrefix(cfg.Admin.Listen, "unix/"):
		return "", ErrNoAdmin
	case cfg.Admin.Listen == "":
		return "127.0.0.1:2019", nil
	default:
		// Caddy binds localhost to IPv4, while busybox wget tries ::1 first.
		listen := strings.TrimPrefix(cfg.Admin.Listen, "tcp/")
		return strings.Replace(listen, "localhost:", "127.0.0.1:", 1), nil
	}
}

func (s Server) adapt(ctx context.Context) (string, error) {
	return s.caddy(ctx, "adapt", "--config", s.config, "--adapter", "caddyfile")
}

func (s Server) live(ctx context.Context, admin string) (string, error) {
	if s.Kind == Service {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+admin+"/config/", http.NoBody)
		if err != nil {
			return "", err
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}
	return shell.Run(ctx, "", "docker", "exec", s.container, "wget", "-qO-", "http://"+admin+"/config/")
}

// Validate checks content as Caddy would load it. The candidate lies next to
// the real Caddyfile, so its relative imports resolve the same way.
func (s Server) Validate(ctx context.Context, content string) error {
	candidate := path.Join(path.Dir(s.config), ".Caddyfile.beatstash")
	if err := s.put(ctx, candidate, content); err != nil {
		candidate = "/tmp/Caddyfile.beatstash"
		if err := s.put(ctx, candidate, content); err != nil {
			return err
		}
	}
	defer s.remove(context.WithoutCancel(ctx), candidate)
	_, err := s.caddy(ctx, "validate", "--config", candidate, "--adapter", "caddyfile")
	return err
}

// Write replaces the Caddyfile in place, keeping the inode a container's
// single-file mount is bound to.
func (s Server) Write(ctx context.Context, content string) error {
	if s.Kind == Service {
		_, err := shell.Run(ctx, content, "sudo", "tee", s.File)
		return err
	}
	return config.WriteFile(s.File, content)
}

func (s Server) Reload(ctx context.Context) error {
	if s.Kind == Service {
		_, err := shell.Run(ctx, "", "sudo", "systemctl", "reload", s.Name)
		return err
	}
	_, err := s.caddy(ctx, "reload", "--config", s.config, "--adapter", "caddyfile")
	return err
}

// ReloadCommand is what Reload runs, to show before running it.
func (s Server) ReloadCommand() string {
	if s.Kind == Service {
		return "sudo systemctl reload " + s.Name
	}
	return fmt.Sprintf("docker exec %s caddy reload --config %s --adapter caddyfile", s.Name, s.config)
}

// Logs are Caddy's latest log lines.
func (s Server) Logs(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", "20", s.container) //nolint:gosec // G204: the id comes from docker ps
	if s.Kind == Service {
		cmd = exec.CommandContext(ctx, "journalctl", "-u", s.Name, "-n", "20", "--no-pager") //nolint:gosec // G204: a fixed unit name
	}
	// Caddy logs to stderr.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err.Error()
	}
	return string(out)
}

func (s Server) caddy(ctx context.Context, args ...string) (string, error) {
	if s.Kind == Service {
		return shell.Run(ctx, "", "caddy", args...)
	}
	return shell.Run(ctx, "", "docker", append([]string{"exec", s.container, "caddy"}, args...)...)
}

func (s Server) put(ctx context.Context, file, content string) error {
	var err error
	if s.Kind == Service {
		_, err = shell.Run(ctx, content, "sudo", "tee", file)
	} else {
		_, err = shell.Run(ctx, content, "docker", "exec", "-i", s.container, "sh", "-c", `cat > "$1"`, "sh", file)
	}
	return err
}

func (s Server) remove(ctx context.Context, file string) {
	if s.Kind == Service {
		_, _ = shell.Run(ctx, "", "sudo", "rm", "-f", file)
		return
	}
	_, _ = shell.Run(ctx, "", "docker", "exec", s.container, "rm", "-f", file)
}
