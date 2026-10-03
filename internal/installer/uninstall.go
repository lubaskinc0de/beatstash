package installer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lubaskinc0de/beatstash/internal/installer/caddy"
	"github.com/lubaskinc0de/beatstash/internal/installer/compose"
	"github.com/lubaskinc0de/beatstash/internal/installer/shell"
	"github.com/lubaskinc0de/beatstash/internal/installer/telegram"
)

func (s *setup) uninstall(ctx context.Context) error {
	s.t.Banner("Uninstall beatstash",
		"Each part is removed only after you agree; Enter takes the default in brackets.",
		"Your music and databases stay unless you choose to delete them.")
	if err := s.locate(ctx); err != nil {
		return err
	}
	running, err := s.project.Running(ctx)
	if err != nil {
		return err
	}
	images, err := s.project.Output(ctx, "config", "--images")
	if err != nil {
		return err
	}
	if slices.Contains(running, "telegram-bot-api") {
		s.returnToCloud(ctx)
	}

	s.t.Say("The containers and network of %s stop and go away; the images stay downloaded.", s.project.Dir)
	stopped := s.t.Confirm("Remove the containers", true)
	if stopped {
		if err := s.project.Run(ctx, "down", "--remove-orphans"); err != nil {
			return err
		}
	}
	s.removeCaddySite(ctx)
	if !stopped {
		s.t.Note("Data stays while the containers run.")
		return nil
	}

	remove := func(path, question string) bool { return s.offerRemoval(ctx, path, question, strings.Fields(images)) }
	removedAll := true
	volumes, err := compose.Volumes(ctx)
	if err != nil {
		return err
	}
	if len(volumes) > 0 {
		s.t.Warn("Volumes %s hold the bot's database: accounts, libraries, links, imports.", strings.Join(volumes, ", "))
		if s.t.Confirm("Delete the volumes", false) {
			if _, err := shell.Run(ctx, "", "docker", append([]string{"volume", "rm"}, volumes...)...); err != nil {
				return err
			}
			s.t.Done("Deleted the volumes.")
		} else {
			removedAll = false
		}
	}
	if data := s.project.Path("data/navidrome"); exists(data) {
		s.t.Warn("%s holds Navidrome's own database: its users, playlists and play counts.", data)
		removedAll = remove(data, "Delete Navidrome's data") && removedAll
	}
	if music := s.project.Path("music"); exists(music) {
		s.t.Warn("%s holds %s of music uploaded and imported through the bot.", music, humanSize(size(music)))
		removedAll = remove(music, "Delete the music") && removedAll
	}
	if !removedAll {
		s.t.Note("Kept %s with its settings, so the kept data stays usable.", s.project.Dir)
		s.t.Done("Uninstalled; run the installer in the same directory to start again.")
		return nil
	}
	root := s.project.Dir
	if parent := filepath.Dir(root); onlyChild(parent, root) {
		root = parent
	}
	s.t.Warn("%s holds .env and config.toml: the bot token, passwords and the encryption key.", root)
	if remove(root, "Delete the installation directory") {
		s.t.Done("beatstash is fully removed.")
	}
	return nil
}

// returnToCloud logs the bot out of the local Bot API, so it can go back to
// Telegram's cloud servers.
func (s *setup) returnToCloud(ctx context.Context) {
	token := s.project.Env().Get("BOT_TOKEN")
	if token == "" {
		return
	}
	s.t.Say("The bot is registered with the local Telegram API. Logging it out there lets it")
	s.t.Say("return to Telegram's cloud API, which may take up to 10 minutes to accept it again.")
	if !s.t.Confirm("Return the bot to the cloud API", false) {
		return
	}
	// With a proxy, the Bot API lives in the proxy container's network.
	service := "telegram-bot-api"
	if exists(s.project.Path(proxyFile)) && strings.Contains(s.project.Env().Get("COMPOSE_FILE"), proxyFile) {
		service = "telegram-proxy"
	}
	id, err := s.project.Container(ctx, service)
	if err == nil {
		var ip string
		ip, err = shell.Run(ctx, "", "docker", "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}", id)
		fields := strings.Fields(ip)
		if err == nil && len(fields) > 0 {
			err = telegram.Client{URL: "http://" + fields[0] + ":8081", HTTP: s.HTTP}.LogOut(ctx, token)
		} else if err == nil {
			err = errors.New("the local Telegram API has no network address")
		}
	}
	if err != nil {
		s.t.Warn("Could not log the bot out of the local API: %v", err)
		return
	}
	if err := os.Remove(s.state().Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.t.Warn("Cannot reset %s: %v", s.state().Path, err)
	}
	s.t.Done("The bot left the local Telegram API.")
}

// removeCaddySite takes the beatstash site out of another program's Caddy.
func (s *setup) removeCaddySite(ctx context.Context) {
	servers, err := caddy.Find(ctx, s.HTTP)
	if err != nil {
		s.t.Warn("Cannot look for Caddy: %v", err)
		return
	}
	for _, server := range servers {
		if server.File == "" {
			continue
		}
		current, err := server.Read()
		if err != nil {
			continue
		}
		next := caddy.WithoutSite(current)
		if next == current {
			continue
		}
		s.t.Say("%s serves Navidrome through Caddy (%s).", server.File, server)
		if !s.t.Confirm("Remove the beatstash site from this Caddy", true) || s.applyCaddyfile(ctx, server, current, next) == untouched {
			s.t.Say("To remove it yourself, delete the lines from %q to %q in %s and reload Caddy.", "# beatstash:begin", "# beatstash:end", server.File)
		}
	}
}

// offerRemoval deletes path once the person agrees. Containers create files
// as root, so what the user cannot delete goes through a container of an
// image the project already has.
func (s *setup) offerRemoval(ctx context.Context, path, question string, images []string) bool {
	if !s.t.Confirm(question, false) {
		return false
	}
	err := os.RemoveAll(path)
	if err != nil && len(images) > 0 {
		parent := filepath.Dir(path)
		_, err = shell.Run(ctx, "", "docker", "run", "--rm", "--entrypoint", "rm", "-v", parent+":/parent", images[0], "-rf", "/parent/"+filepath.Base(path))
	}
	if err != nil {
		s.t.Warn("Cannot delete %s: %v", path, err)
		return false
	}
	s.t.Done("Deleted %s.", path)
	return true
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// onlyChild tells whether dir holds nothing but child, as the directory the
// installer created around deploy does.
func onlyChild(dir, child string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 1 && entries[0].Name() == filepath.Base(child)
}

func size(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func humanSize(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(n)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
