package installer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/installer/caddy"
	"github.com/lubaskinc0de/beatstash/internal/installer/config"
)

const httpsGuide = "https://lubaskinc0de.github.io/beatstash/installation/https/"

// domain is the host of an https:// address with a domain name, which a
// proxy can get a certificate for; empty otherwise.
func domain(address string) string {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Port() != "" {
		return ""
	}
	host := u.Hostname()
	if !strings.Contains(host, ".") || strings.Trim(host, "0123456789.") == "" || strings.Contains(host, ":") {
		return ""
	}
	return host
}

func (s *setup) setUpHTTPS(ctx context.Context, in *installation) error {
	s.t.Stage("Set up HTTPS")
	name := domain(in.publicURL)
	switch {
	case in.mode == existingNavidrome:
		s.t.Note("Skipped: your existing Navidrome keeps its own proxy and HTTPS.")
		return nil
	case name == "":
		s.t.Note("Skipped: HTTPS setup needs a public listening URL like https://music.example.com.")
		return nil
	}
	upstream, err := s.project.Address(ctx, "navidrome", "4533")
	if err != nil {
		return err
	}
	s.t.Say("Navidrome listens only on this server. A reverse proxy makes it reachable at https://%s", name)
	s.t.Say("and gets its certificate from Let's Encrypt. Before you continue:")
	s.t.Warn("%s needs an A record pointing to this server. Add an AAAA record only if the server accepts IPv6.", name)
	s.t.Warn("Ports 80 and 443 must be open to the internet at your hosting provider and in the firewall.")
	s.t.Warn("Let's Encrypt limits certificates per domain; repeated failures can block it for a while.")

	if slices.Contains(profiles(s.project.Env()), "proxy") {
		return s.bundledProxy(ctx, name, upstream)
	}
	servers, err := caddy.Find(ctx, s.HTTP)
	if err != nil {
		return err
	}
	if len(servers) > 0 {
		return s.existingCaddy(ctx, s.chooseServer(servers), name, upstream)
	}
	if portsBusy(ctx) {
		s.t.Say("Another program already uses port 80 or 443, and it is not a Caddy the installer can configure.")
		s.manualProxy(name, upstream)
		return nil
	}
	return s.bundledProxy(ctx, name, upstream)
}

func (s *setup) chooseServer(servers []caddy.Server) caddy.Server {
	if len(servers) == 1 {
		return servers[0]
	}
	s.t.Say("Several Caddy servers run here:")
	for i, server := range servers {
		s.t.Say("%d. %s", i+1, server)
	}
	for {
		answer := s.t.Ask(fmt.Sprintf("Which one serves your sites [1-%d]:", len(servers)), "", "")
		for i := range servers {
			if answer == fmt.Sprint(i+1) {
				return servers[i]
			}
		}
	}
}

func (s *setup) existingCaddy(ctx context.Context, server caddy.Server, name, upstream string) error {
	s.t.Say("Found Caddy: %s", server)
	switch {
	case server.Kind == caddy.BridgeContainer:
		s.t.Warn("This Caddy runs on a Docker network, where Navidrome's port on the host loopback is out of reach.")
		s.t.Say("Attach its container to the beatstash network: docker network connect beatstash_default %s", server.Name)
		s.manualProxy(name, "navidrome:4533")
		return nil
	case server.File == "":
		s.t.Warn("This Caddy keeps its config inside its image, not in a file on this server.")
		s.manualProxy(name, upstream)
		return nil
	}
	current, err := server.Read()
	if err != nil {
		s.t.Warn("Cannot read %s: %v", server.File, err)
		s.manualProxy(name, upstream)
		return nil
	}
	next, err := caddy.WithSite(current, name, upstream)
	if errors.Is(err, caddy.ErrForeignSite) {
		s.t.Warn("%s already serves %s outside the beatstash block; the installer leaves it as it is.", server.File, name)
		s.manualProxy(name, upstream)
		return nil
	}
	if next == current {
		s.t.Done("Caddy already serves %s.", name)
		s.verifyHTTPS(ctx, name, server.Logs)
		return nil
	}
	s.t.Say("The installer adds a site for %s at the end of %s, between beatstash markers,", name, server.File)
	s.t.Say("and reloads Caddy. Your other sites stay as they are and keep serving during the reload.")
	if !s.t.Confirm(fmt.Sprintf("Configure HTTPS for %s in this Caddy", name), false) {
		s.manualProxy(name, upstream)
		return nil
	}
	switch s.applyCaddyfile(ctx, server, current, next) {
	case applied:
		s.verifyHTTPS(ctx, name, server.Logs)
	case untouched:
		s.manualProxy(name, upstream)
	case written:
		// applyCaddyfile said what is left to run.
	}
	return nil
}

// caddyChange is how far applyCaddyfile got.
type caddyChange int

const (
	// untouched leaves Caddy and its file as they were.
	untouched caddyChange = iota
	// written changed the file, but Caddy still runs the previous one.
	written
	applied
)

// applyCaddyfile changes another program's Caddy with every precaution: the
// running config must match the file, Caddy must accept the new file, the
// person sees and approves both the change and the reload, and a failed
// reload puts the old file back.
func (s *setup) applyCaddyfile(ctx context.Context, server caddy.Server, current, next string) caddyChange {
	if err := server.CheckDrift(ctx); err != nil {
		s.t.Warn("%v.", err)
		s.t.Say("A reload from the file could undo changes made another way, so the installer leaves Caddy alone.")
		return untouched
	}
	if err := server.Validate(ctx, next); err != nil {
		s.t.Warn("Caddy rejects the changed Caddyfile: %v", err)
		return untouched
	}
	s.t.Diff(server.File, current, next)
	if !s.t.Confirm("Write this change to "+server.File, false) {
		return untouched
	}
	backup := s.project.Path("Caddyfile.backup")
	if err := config.WriteFile(backup, current); err != nil {
		s.t.Warn("Cannot save a backup: %v", err)
		return untouched
	}
	if err := server.Write(ctx, next); err != nil {
		s.t.Warn("Cannot write %s: %v", server.File, err)
		return untouched
	}
	s.t.Done("Wrote %s; the previous version is in %s.", server.File, backup)
	s.t.Say("Caddy applies it with: %s", server.ReloadCommand())
	if !s.t.Confirm("Reload Caddy now", false) {
		s.t.Warn("Run that command yourself to apply the change.")
		return written
	}
	if err := server.Reload(ctx); err != nil {
		s.t.Warn("Caddy did not reload: %v", err)
		if err := server.Write(ctx, current); err != nil {
			s.t.Warn("Cannot restore %s: %v. Copy it back from %s.", server.File, err, backup)
			return written
		}
		if err := server.Reload(ctx); err != nil {
			s.t.Warn("Restored %s, but Caddy did not reload it: %v", server.File, err)
			return untouched
		}
		s.t.Say("Restored the previous Caddyfile; Caddy runs it again.")
		return untouched
	}
	s.t.Done("Caddy reloaded.")
	return applied
}

// bundledProxy runs Caddy from the beatstash stack itself.
func (s *setup) bundledProxy(ctx context.Context, name, upstream string) error {
	file := s.project.Path("Caddyfile")
	current, err := os.ReadFile(file) //nolint:gosec // G304: the installation's own Caddyfile
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	running := slices.Contains(profiles(s.project.Env()), "proxy")
	if same, err := caddy.WithSite(string(current), name, "navidrome:4533"); running && err == nil && same == string(current) {
		s.t.Done("Caddy already serves %s.", name)
		s.verifyHTTPS(ctx, name, s.proxyLogs)
		return nil
	}
	if !running {
		s.t.Say("No other proxy runs here. The installer can start Caddy with beatstash: it takes ports 80")
		s.t.Say("and 443, keeps its settings in %s, and renews the certificate by itself.", file)
		if !s.t.Confirm(fmt.Sprintf("Start Caddy for %s", name), false) {
			s.manualProxy(name, upstream)
			return nil
		}
	}
	email := s.t.Ask("Email for certificate expiry notices [optional]:", caddyEmail(string(current)), "")
	next, err := caddy.WithSite(globalEmail(email), name, "navidrome:4533")
	if err != nil {
		return err
	}
	if running {
		s.t.Diff(file, string(current), next)
		if !s.t.Confirm("Write this change and reload Caddy", false) {
			s.t.Warn("Kept %s; Caddy does not serve %s yet.", file, name)
			return nil
		}
	}
	if err := config.WriteFile(file, next); err != nil {
		return err
	}
	if !running {
		if err := s.project.Env().Set("COMPOSE_PROFILES", strings.Join(append(profiles(s.project.Env()), "proxy"), ",")); err != nil {
			return err
		}
	}
	if err := s.project.Run(ctx, "up", "-d", "caddy"); err != nil {
		return err
	}
	if running {
		if err := s.project.Run(ctx, "exec", "-T", "caddy", "caddy", "reload", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"); err != nil {
			return err
		}
	}
	s.verifyHTTPS(ctx, name, s.proxyLogs)
	return nil
}

func (s *setup) proxyLogs(ctx context.Context) string {
	logs, err := s.project.Output(ctx, "logs", "--tail", "20", "--no-log-prefix", "caddy")
	if err != nil {
		return err.Error()
	}
	return logs
}

func globalEmail(email string) string {
	if email == "" {
		return ""
	}
	return fmt.Sprintf("{\n\temail %s\n}\n", email)
}

func caddyEmail(caddyfile string) string {
	for line := range strings.SplitSeq(caddyfile, "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "email" {
			return fields[1]
		}
	}
	return ""
}

func (s *setup) manualProxy(name, upstream string) {
	s.t.Say("To serve Navidrome yourself, point your reverse proxy at %s for https://%s.", upstream, name)
	s.t.Say("For Caddy, the site is:")
	s.t.Block(caddy.Site(name, upstream))
	s.t.Say("Check the file with caddy validate --config <Caddyfile>, then apply it with caddy reload --config <Caddyfile>.")
	s.t.Say("Guide for this and other proxies: %s", httpsGuide)
	s.t.Note("Listening links work once https://%s opens from the internet.", name)
}

// verifyHTTPS waits for the site to answer through the proxy, and explains
// what to check when it does not.
func (s *setup) verifyHTTPS(ctx context.Context, name string, logs func(context.Context) string) {
	address := "https://" + name + "/ping"
	for {
		s.t.Say("Checking %s ...", address)
		if s.answers(ctx, address) {
			s.t.Done("Navidrome answers at https://%s", name)
			return
		}
		s.t.Warn("https://%s does not answer yet.", name)
		if ips, err := net.DefaultResolver.LookupHost(ctx, name); err != nil {
			s.t.Warn("%s does not resolve. Add an A record pointing to this server.", name)
		} else {
			s.t.Say("%s resolves to %s; it must be this server's public address.", name, strings.Join(ips, ", "))
		}
		s.t.Say("Latest Caddy log lines:")
		s.t.Block(logs(ctx))
		if !s.t.Confirm("Check again", false) {
			s.t.Warn("Skipped the check. Listening links work once https://%s opens.", name)
			return
		}
	}
}

func (s *setup) answers(ctx context.Context, address string) bool {
	ctx, cancel := context.WithTimeout(ctx, s.HTTPSWait)
	defer cancel()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, http.NoBody)
		if err != nil {
			return false
		}
		if resp, err := s.HTTP.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
		}
	}
}

// portsBusy tells whether something accepts connections on port 80 or 443.
func portsBusy(ctx context.Context) bool {
	var dialer net.Dialer
	for _, port := range []string{"80", "443"} {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
		cancel()
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}
