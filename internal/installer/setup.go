package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/installer/compose"
	"github.com/lubaskinc0de/beatstash/internal/installer/config"
	"github.com/lubaskinc0de/beatstash/internal/installer/prompt"
	"github.com/lubaskinc0de/beatstash/internal/installer/shell"
)

const usage = `Usage: beatstash-setup [install|upgrade|uninstall]

  install    set up beatstash, or continue an unfinished setup (default)
  upgrade    move an installation to this release
  uninstall  remove an installation, asking about each part
`

const defaultRepository = "lubaskinc0de/beatstash"

type Options struct {
	In     io.Reader
	Out    io.Writer
	Getenv func(string) string
	// Version is the release this tool installs, without the leading v.
	Version    string
	Repository string
	// ProjectName is the Compose project to manage; defaults to beatstash.
	ProjectName string
	// CaddyContainerFilter limits discovery to Docker containers matching this
	// filter. Empty also discovers the host's systemd Caddy service.
	CaddyContainerFilter string
	// ComposeTemplate and ConfigTemplate are the deployment files of the release.
	ComposeTemplate string
	ConfigTemplate  string
	ProxyTemplate   string
	// TelegramURL is the cloud Bot API server.
	TelegramURL string
	// TelegramDC is a Telegram data center the local Bot API connects to.
	TelegramDC string
	HTTP       *http.Client
	// HTTPSWait is how long a new site may take to answer over HTTPS.
	HTTPSWait time.Duration
	// Open shows a page in the person's browser.
	Open func(url string) error
}

type setup struct {
	Options
	t       *prompt.Terminal
	project compose.Project
	version version
}

// Run performs the command in args, install when there is none.
func Run(ctx context.Context, o Options, args []string) error {
	if o.ProjectName == "" {
		o.ProjectName = compose.Name
	}
	command := "install"
	if len(args) > 0 {
		command = args[0]
	}
	if len(args) > 1 || !slices.Contains([]string{"install", "upgrade", "uninstall", "--help", "-h"}, command) {
		return errors.New("unknown command; use --help for usage")
	}
	if command == "--help" || command == "-h" {
		_, err := io.WriteString(o.Out, usage)
		return err
	}
	v, err := parseVersion(o.Version)
	if err != nil {
		return fmt.Errorf("this build has no release version: %w", err)
	}
	if err := checkDocker(ctx); err != nil {
		return err
	}
	s := &setup{Options: o, t: prompt.New(o.In, o.Out), version: v}
	return prompt.Guard(func() error {
		switch command {
		case "upgrade":
			return s.upgrade(ctx)
		case "uninstall":
			return s.uninstall(ctx)
		default:
			return s.install(ctx)
		}
	})
}

func checkDocker(ctx context.Context) error {
	if _, err := shell.Run(ctx, "", "docker", "compose", "version"); err != nil {
		return errors.New("install Docker Engine with the Compose plugin first")
	}
	if _, err := shell.Run(ctx, "", "docker", "info"); err != nil {
		return errors.New("docker must be running and accessible to your user")
	}
	return nil
}

// state keeps what the installer itself must remember between runs.
func (s *setup) state() config.Env { return config.Env{Path: s.project.Path(".installer.env")} }

func (s *setup) configFile() string { return s.project.Path("config.toml") }

func (s *setup) readConfig() (string, error) {
	data, err := os.ReadFile(s.configFile())
	return string(data), err
}

// setConfig changes settings of config.toml, each as table, key, TOML value.
func (s *setup) setConfig(settings ...[3]string) error {
	doc, err := s.readConfig()
	if err != nil {
		return err
	}
	for _, setting := range settings {
		if doc, err = config.Set(doc, setting[0], setting[1], setting[2]); err != nil {
			return err
		}
	}
	return config.WriteFile(s.configFile(), doc)
}

// configString is a string setting, empty when it is missing.
func (s *setup) configString(table, key string) string {
	doc, err := s.readConfig()
	if err != nil {
		return ""
	}
	value, _ := config.Lookup(doc, table, key)
	text, _ := value.(string)
	return text
}

// composeFile is the release's compose.yml pointed at this repository's image.
func (s *setup) composeFile() string {
	if s.Repository == "" || s.Repository == defaultRepository {
		return s.ComposeTemplate
	}
	return strings.ReplaceAll(s.ComposeTemplate, "ghcr.io/"+defaultRepository+":", "ghcr.io/"+strings.ToLower(s.Repository)+":")
}

// expand resolves ~ the way a shell would, which a typed answer never gets.
func (s *setup) expand(path string) string {
	home := s.Getenv("HOME")
	switch {
	case home == "":
		return path
	case path == "~":
		return home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(home, path[2:])
	default:
		return path
	}
}

// link opens url on this machine, or asks to open it elsewhere when the
// setup runs on a remote server with no browser.
func (s *setup) link(url string) {
	if s.remote() || s.Open == nil {
		s.t.Step("Open on your computer or phone: %s", url)
		return
	}
	s.t.Note("↗ opening %s", url)
	if err := s.Open(url); err != nil {
		s.t.Warn("Couldn't open a browser; visit %s", url)
	}
}

func (s *setup) remote() bool {
	if s.Getenv("SSH_CONNECTION") != "" || s.Getenv("SSH_TTY") != "" {
		return true
	}
	return runtime.GOOS == "linux" && s.Getenv("DISPLAY") == "" && s.Getenv("WAYLAND_DISPLAY") == ""
}

// locate finds the installation an upgrade or uninstall works on.
func (s *setup) locate(ctx context.Context) error {
	owners, err := compose.Owners(ctx, s.ProjectName)
	if err != nil {
		return err
	}
	if len(owners) == 1 && s.t.Confirm(fmt.Sprintf("Use the installation in %s?", owners[0]), true) {
		s.project = compose.Project{Name: s.ProjectName, Dir: owners[0], Out: s.Out}
		return nil
	}
	for {
		dir := s.expand(s.t.Ask("Installation directory [./beatstash]:", "", "./beatstash"))
		for _, candidate := range []string{filepath.Join(dir, "deploy"), dir} {
			if _, err := os.Stat(filepath.Join(candidate, "compose.yml")); err == nil {
				abs, err := filepath.Abs(candidate)
				if err != nil {
					return err
				}
				s.project = compose.Project{Name: s.ProjectName, Dir: abs, Out: s.Out}
				return nil
			}
		}
		s.t.Warn("No beatstash installation in %s.", dir)
	}
}

func profiles(env config.Env) []string {
	var list []string
	for p := range strings.SplitSeq(env.Get("COMPOSE_PROFILES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			list = append(list, p)
		}
	}
	return list
}
