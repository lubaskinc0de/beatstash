package installer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/installer/compose"
	"github.com/lubaskinc0de/beatstash/internal/installer/config"
	"github.com/lubaskinc0de/beatstash/internal/installer/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/installer/telegram"
)

var (
	modePattern     = regexp.MustCompile(`^(new|existing)$`)
	tokenPattern    = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)
	idPattern       = regexp.MustCompile(`^[1-9][0-9]*$`)
	apiHashPattern  = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
	anything        = regexp.MustCompile(`.`)
	urlPattern      = regexp.MustCompile(`^https?://[^\s"\\]+$`)
	adminIDsPattern = regexp.MustCompile(`^telegram:([1-9][0-9]*)$`)
)

const (
	newNavidrome      = "new"
	existingNavidrome = "existing"
	// navidromeStart is how long a fresh Navidrome may take to answer.
	navidromeStart = 2 * time.Minute
)

// installation is what the install stages learn and pass on.
type installation struct {
	mode string
	// fresh means the config was just created from the example, so its
	// values are samples rather than answers to offer again.
	fresh     bool
	token     string
	publicURL string
}

func (s *setup) install(ctx context.Context) error {
	s.t.Banner("Install beatstash",
		"Set up beatstash with a new or existing Navidrome server.",
		"You will need your Telegram credentials and a Navidrome admin account.",
		"Each step says what it changes before it does so.",
		"Press Ctrl-C to stop. Run it again in the same directory to continue.")
	s.t.Stages(7)
	in := &installation{}
	stages := []func(context.Context, *installation) error{
		s.chooseInstallation, s.createBot, s.chooseAdmin, s.setUpLocalAPI, s.connectNavidrome, s.setUpHTTPS, s.start,
	}
	for _, stage := range stages {
		if err := stage(ctx, in); err != nil {
			return err
		}
	}
	return nil
}

func (s *setup) chooseInstallation(ctx context.Context, in *installation) error {
	s.t.Stage("Choose your installation")
	s.t.Say("Run this installer on the server that will store your music.")
	dir := s.expand(s.t.Ask("Installation directory [./beatstash]:", "", "./beatstash"))
	deploy, err := filepath.Abs(filepath.Join(dir, "deploy"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(deploy, 0o755); err != nil { //nolint:gosec // G301: a native Navidrome reaches the music through it
		return err
	}
	s.project = compose.Project{Dir: deploy, Out: s.Out}
	s.t.Say("Installing v%s into %s", s.version, deploy)

	saved := ""
	if _, err := os.Stat(s.project.Env().Path); err == nil {
		saved = existingNavidrome
		if slices.Contains(profiles(s.project.Env()), "navidrome") {
			saved = newNavidrome
		}
	}
	in.mode = s.t.Require("Navidrome: new or existing [type new/existing]:", saved, modePattern, false)
	if saved != "" && in.mode != saved {
		return errors.New("use another directory to change the installation mode")
	}
	owners, err := compose.Owners(ctx)
	if err != nil {
		return err
	}
	for _, owner := range owners {
		if owner != deploy {
			return fmt.Errorf("another beatstash stack already runs from %s; run the installer there", owner)
		}
	}
	if installed := s.project.Env().Get("BEATSTASH_VERSION"); installed != "" && installed != s.version.String() {
		return fmt.Errorf("this directory has beatstash %s; use the upgrade command of the newer release to change versions", installed)
	}
	return s.writeTemplates(in)
}

func (s *setup) writeTemplates(in *installation) error {
	if _, err := os.Stat(s.configFile()); errors.Is(err, os.ErrNotExist) {
		in.fresh = true
		if err := config.WriteFile(s.configFile(), s.ConfigTemplate); err != nil {
			return err
		}
	}
	if _, err := os.Stat(s.project.Path("compose.yml")); errors.Is(err, os.ErrNotExist) {
		if err := config.WriteFile(s.project.Path("compose.yml"), s.composeFile()); err != nil {
			return err
		}
	}
	if _, err := os.Stat(s.project.Env().Path); errors.Is(err, os.ErrNotExist) {
		profile := ""
		if in.mode == newNavidrome {
			profile = "navidrome"
		}
		if err := s.project.Env().Set("COMPOSE_PROFILES", profile); err != nil {
			return err
		}
	}
	if err := s.project.Env().Set("BEATSTASH_VERSION", s.version.String()); err != nil {
		return err
	}
	dirs := []string{"music", "music/shared", "music/users"}
	if in.mode == newNavidrome {
		dirs = append(dirs, "data/navidrome")
	}
	for _, dir := range dirs {
		path := s.project.Path(dir)
		if err := os.MkdirAll(path, 0o755); err != nil { //nolint:gosec // G301: Navidrome reads the music as another user
			return err
		}
		if strings.HasPrefix(dir, "music") {
			if err := os.Chmod(path, 0o755); err != nil { //nolint:gosec // G302: as above, even under a strict umask
				return err
			}
		}
	}
	return nil
}

// current is a saved answer to offer again, or nothing on a fresh install.
func (s *setup) current(in *installation, table, key string) string {
	if in.fresh {
		return ""
	}
	return s.configString(table, key)
}

func (s *setup) createBot(_ context.Context, in *installation) error {
	s.t.Stage("Create your Telegram bot")
	s.link("https://t.me/BotFather")
	s.t.Step("Send /newbot to BotFather, choose a name and username, and copy its token.")
	in.token = s.t.Require("Bot token:", s.project.Env().Get("BOT_TOKEN"), tokenPattern, true)
	if err := s.project.Env().Set("BOT_TOKEN", in.token); err != nil {
		return err
	}
	s.t.Step("Send /setinline, select your bot, and enter a search prompt such as Search music.")
	s.t.Step("Send /setinlinefeedback, select your bot, and choose Enabled, not 1/10 or 1/100:")
	s.t.Note("  tracks picked from inline search reach a chat only when the bot hears of every pick.")
	s.t.Pause("Press Enter once inline mode and feedback are enabled.")
	return nil
}

func (s *setup) chooseAdmin(_ context.Context, in *installation) error {
	s.t.Stage("Choose the bot administrator")
	s.link("https://t.me/Get_myidrobot")
	s.t.Step("Start Get_myidrobot and copy your numeric Telegram user ID.")
	id := s.t.Require("Your Telegram user ID:", s.adminID(in), idPattern, false)
	contact := s.t.Ask("Contact shown to new visitors [optional, e.g. @your_name]:", s.current(in, "", "admin_contact"), "")
	return s.setConfig(
		[3]string{"", "admins", "[" + config.String("telegram:"+id) + "]"},
		[3]string{"", "admin_contact", config.String(contact)},
	)
}

func (s *setup) adminID(in *installation) string {
	doc, err := s.readConfig()
	if in.fresh || err != nil {
		return ""
	}
	admins, _ := config.Lookup(doc, "", "admins")
	if list, ok := admins.([]any); ok && len(list) == 1 {
		if text, ok := list[0].(string); ok {
			if m := adminIDsPattern.FindStringSubmatch(text); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

func (s *setup) setUpLocalAPI(ctx context.Context, _ *installation) error {
	s.t.Stage("Set up the local Telegram API")
	s.link("https://my.telegram.org/apps")
	s.t.Step("Sign in with your Telegram account. In API development tools, create an application if needed.")
	s.t.Step("Copy its api_id and api_hash. These belong to your Telegram application, separately from the bot token.")
	env := s.project.Env()
	if err := env.Set("TELEGRAM_API_ID", s.t.Require("API ID:", env.Get("TELEGRAM_API_ID"), idPattern, false)); err != nil {
		return err
	}
	if err := env.Set("TELEGRAM_API_HASH", s.t.Require("API hash:", env.Get("TELEGRAM_API_HASH"), apiHashPattern, true)); err != nil {
		return err
	}
	if err := s.ensureSecret(ctx, "SECRET_KEY", base64.StdEncoding.EncodeToString); err != nil {
		return err
	}
	if err := s.ensureSecret(ctx, "POSTGRES_PASSWORD", hex.EncodeToString); err != nil {
		return err
	}
	if err := s.chooseTelegramProxy(ctx); err != nil {
		return err
	}
	return s.setConfig(
		[3]string{"telegram", "bot_api_url", config.String("http://telegram-bot-api:8081")},
		[3]string{"library", "music_dir", config.String("/music")},
	)
}

// ensureSecret generates a missing secret, unless a database made with the
// lost one exists: a new value would lock the bot out of it.
func (s *setup) ensureSecret(ctx context.Context, name string, encode func([]byte) string) error {
	if s.project.Env().Get(name) != "" {
		return nil
	}
	volumes, err := compose.Volumes(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(volumes, compose.Name+"_postgres_data") {
		return fmt.Errorf("an existing database was found; restore its original .env before generating %s", name)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	return s.project.Env().Set(name, encode(secret))
}

func (s *setup) connectNavidrome(ctx context.Context, in *installation) error {
	s.t.Stage("Connect Navidrome")
	user := s.t.Ask("Navidrome administrator username [admin]:", s.current(in, "navidrome", "user"), "admin")
	address, musicDir := "http://navidrome:4533", "/music"
	if in.mode == existingNavidrome {
		address, musicDir = s.existingNavidrome(in)
	}
	in.publicURL = s.publicURL(in)
	password := s.t.Require("Navidrome administrator password:", s.project.Env().Get("NAVIDROME_PASSWORD"), anything, true)
	if err := s.project.Env().Set("NAVIDROME_PASSWORD", password); err != nil {
		return err
	}
	if err := s.setConfig(
		[3]string{"navidrome", "user", config.String(user)},
		[3]string{"navidrome", "url", config.String(address)},
		[3]string{"navidrome", "public_url", config.String(in.publicURL)},
		[3]string{"library", "navidrome_music_dir", config.String(musicDir)},
	); err != nil {
		return err
	}
	if in.mode == newNavidrome {
		return s.startNavidrome(ctx, user, password)
	}
	return nil
}

func (s *setup) existingNavidrome(in *installation) (address, musicDir string) {
	for {
		address = s.t.Require("Navidrome URL reachable from the bot (e.g. https://music.example.com):", s.current(in, "navidrome", "url"), urlPattern, false)
		if !strings.Contains(address, "://localhost") && !strings.Contains(address, "://127.0.0.1") {
			break
		}
		s.t.Warn("localhost inside the bot points to the bot itself. Use your Navidrome server address.")
	}
	for {
		musicDir = s.t.Ask("New music folder as Navidrome sees it [/beatstash-music]:", s.current(in, "library", "navidrome_music_dir"), "/beatstash-music")
		if strings.HasPrefix(musicDir, "/") && musicDir != "/" {
			break
		}
		s.t.Warn("Use an absolute directory path for new music.")
	}
	s.t.Say("Your existing music stays in its current folders. Add this read-only mount to your Navidrome Compose service:")
	s.t.Block("- " + s.project.Path("music") + ":" + musicDir + ":ro")
	s.t.Say("Keep the existing music mounts. Apply the change to your Navidrome stack before continuing.")
	s.t.Note("For a native Navidrome install, use the host music path instead and allow Navidrome to read it.")
	s.t.Pause("Press Enter once Navidrome can read the new music folder.")
	return address, musicDir
}

func (s *setup) publicURL(in *installation) string {
	for {
		answer := s.t.Ask("Public listening URL [optional, e.g. https://music.example.com]:", s.current(in, "navidrome", "public_url"), "")
		if answer == "" || urlPattern.MatchString(answer) {
			return strings.TrimRight(answer, "/")
		}
		s.t.Warn("Enter an address starting with https:// or http://, or leave it empty.")
	}
}

// startNavidrome starts Navidrome and makes the bot's administrator account
// in it, so nobody has to open its page first.
func (s *setup) startNavidrome(ctx context.Context, user, password string) error {
	s.t.Say("This starts Navidrome and creates its administrator %s with the password you entered.", user)
	if !s.t.Confirm("Start Navidrome now", true) {
		return errors.New("settings saved; rerun the installer when ready")
	}
	if err := s.project.Run(ctx, "up", "-d", "navidrome"); err != nil {
		return err
	}
	address, err := s.project.Address(ctx, "navidrome", "4533")
	if err != nil {
		return err
	}
	client := navidrome.Client{URL: "http://" + address, HTTP: s.HTTP}
	if err := client.WaitReady(ctx, navidromeStart); err != nil {
		return err
	}
	for {
		err := client.CreateAdmin(ctx, user, password)
		if err == nil {
			s.t.Done("Created the Navidrome administrator %s.", user)
			return nil
		}
		if !errors.Is(err, navidrome.ErrHasUsers) {
			return err
		}
		err = client.Login(ctx, user, password)
		if err == nil {
			s.t.Done("Navidrome already has the administrator %s; the bot will use it.", user)
			return nil
		}
		if !errors.Is(err, navidrome.ErrLogin) {
			return err
		}
		s.t.Warn("Navidrome already has accounts and rejected %s with this password.", user)
		s.t.Say("Enter an existing administrator account.")
		user = s.t.Ask("Navidrome administrator username:", user, "")
		password = s.t.Require("Navidrome administrator password:", "", anything, true)
		if err := s.project.Env().Set("NAVIDROME_PASSWORD", password); err != nil {
			return err
		}
		if err := s.setConfig([3]string{"navidrome", "user", config.String(user)}); err != nil {
			return err
		}
	}
}

func (s *setup) start(ctx context.Context, in *installation) error {
	s.t.Stage("Start beatstash")
	if err := s.project.Run(ctx, "config", "--quiet"); err != nil {
		return err
	}
	s.t.Say("This downloads the images and starts PostgreSQL, the local Telegram API, and your bot.")
	if !s.t.Confirm("Download the images and start this stack", false) {
		return errors.New("settings saved; to start later, rerun this installer")
	}
	if err := s.leaveCloudAPI(ctx, in.token); err != nil {
		return err
	}
	running, err := s.project.Running(ctx)
	if err != nil {
		return err
	}
	if err := s.project.Run(ctx, "pull"); err != nil {
		return err
	}
	if err := s.project.Run(ctx, "up", "-d"); err != nil {
		return err
	}
	// Compose recreates a container for a changed .env, not for a changed
	// config.toml, which the bot reads only at start.
	if slices.Contains(running, "bot") {
		if err := s.project.Run(ctx, "restart", "bot"); err != nil {
			return err
		}
	}
	if err := s.project.Run(ctx, "ps"); err != nil {
		return err
	}
	s.t.Done("beatstash v%s is running.", s.version)
	s.t.Say("Configuration: %s", s.configFile())
	s.t.Say("To manage this stack: cd '%s' && docker compose ps", s.project.Dir)
	s.t.Say("After editing config.toml or .env there: docker compose up -d --force-recreate bot")
	s.t.Say("Open your bot in Telegram and send /start. Link your Navidrome account in Settings.")
	if in.publicURL == "" {
		s.t.Say("For HTTPS and listening links: https://lubaskinc0de.github.io/beatstash/installation/https/")
	}
	return nil
}

// leaveCloudAPI logs the bot out of Telegram's cloud servers once per token:
// a bot served by the local Bot API must leave the cloud first.
func (s *setup) leaveCloudAPI(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	if s.state().Get("LOGOUT_TOKEN_HASH") == hash {
		return nil
	}
	s.t.Say("Before using the local Telegram API, this bot must leave the cloud API.")
	s.t.Warn("Telegram then keeps it off the cloud API for 10 minutes.")
	if !s.t.Confirm("All other instances of this Telegram bot are stopped; switch it to the local API", false) {
		return errors.New("settings saved; stop the other instance before continuing")
	}
	httpClient, err := s.telegramHTTP(ctx)
	if err != nil {
		return err
	}
	if err := (telegram.Client{URL: s.TelegramURL, HTTP: httpClient}).LogOut(ctx, token); err != nil {
		return err
	}
	return s.state().Set("LOGOUT_TOKEN_HASH", hash)
}
