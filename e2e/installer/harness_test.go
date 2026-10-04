package installer_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash"
	"github.com/lubaskinc0de/beatstash/internal/installer"
)

//go:embed testdata/compose.yml
var composeTemplate string

const (
	token    = "123456:test-token"
	apiHash  = "00000000000000000000000000000000"
	password = "navidrome-secret"
	domain   = "music.example.test"
)

// CaddyGlobals keeps the test Caddy off the usual ports and away from Let's
// Encrypt: it signs certificates with its own authority.
func (s *Setup) CaddyGlobals() string {
	_, httpsPort, _ := net.SplitHostPort(s.CaddyHTTPS)
	return fmt.Sprintf(`{
	admin %s
	http_port %s
	https_port %s
	local_certs
}

other.example.test {
	respond "other site"
}
`, s.CaddyAdmin, s.CaddyHTTPPort, httpsPort)
}

// Setup is one server the installer runs on: its home directory, a fake
// Telegram cloud, and the release the installer brings.
type Setup struct {
	t             *testing.T
	Home          string
	Telegram      *Telegram
	Version       string
	Compose       string
	Config        string
	HTTP          *http.Client
	ProjectName   string
	CaddyName     string
	CaddyHTTPS    string
	CaddyAdmin    string
	CaddyHTTPPort string
	// TelegramDC is where the installer checks that Telegram is reachable.
	TelegramDC string
}

func newSetup(t *testing.T) *Setup {
	t.Helper()
	t.Parallel()
	home := t.TempDir()
	sum := sha256.Sum256([]byte(home))
	project := fmt.Sprintf("beatstash-test-%x", sum[:6])
	t.Cleanup(func() {
		removeStack(t, project)
		// Usually the test user owns all files; keep a fallback for images
		// that leave root-owned files behind.
		if err := os.RemoveAll(home); err != nil {
			docker(t, "run", "--rm", "-v", home+":/home", "alpine:3.24", "sh", "-c", "rm -rf /home/* /home/.[!.]*")
		}
	})
	s := &Setup{
		t:           t,
		Home:        home,
		Telegram:    newTelegram(t),
		Version:     "1.0.0",
		Compose:     strings.Replace(composeTemplate, "name: beatstash\n", "name: "+project+"\n", 1),
		Config:      beatstash.ConfigTemplate,
		ProjectName: project,
		CaddyName:   project + "-caddy",
		CaddyHTTPS:  freeAddress(t),
		CaddyAdmin:  freeAddress(t),
	}
	// Navidrome writes its database as the test user, so cleanup does not
	// need to start another container just to delete its files.
	s.Compose = strings.Replace(s.Compose, "image: deluan/navidrome:0.64.2\n",
		fmt.Sprintf("image: deluan/navidrome:0.64.2\n    user: '%d:%d'\n", os.Getuid(), os.Getgid()), 1)
	require.NoError(t, os.MkdirAll(s.Deploy("data/navidrome"), 0o750))
	_, s.CaddyHTTPPort, _ = net.SplitHostPort(freeAddress(t))
	s.HTTP = client(s.CaddyHTTPS)
	s.TelegramDC = strings.TrimPrefix(s.Telegram.URL, "http://")
	return s
}

// savedInstallation provides installed files for checks that need no running
// services, such as refusing a downgrade or detecting the current release.
func savedInstallation(t *testing.T) *Setup {
	t.Helper()
	s := newSetup(t)
	for name, content := range map[string]string{
		"compose.yml": s.Compose,
		"config.toml": s.Config,
		".env":        `BEATSTASH_VERSION="1.0.0"` + "\n",
	} {
		require.NoError(t, os.WriteFile(s.Deploy(name), []byte(content), 0o600))
	}
	return s
}

// Run runs the installer with answers typed one per line.
func (s *Setup) Run(args []string, answers ...string) (string, error) {
	var out bytes.Buffer
	env := map[string]string{"HOME": s.Home, "SSH_CONNECTION": "10.0.0.1 50000 10.0.0.2 22"}
	err := installer.Run(context.Background(), installer.Options{
		In:                   strings.NewReader(strings.Join(answers, "\n") + "\n"),
		Out:                  &out,
		Getenv:               func(key string) string { return env[key] },
		Version:              s.Version,
		ProjectName:          s.ProjectName,
		CaddyContainerFilter: "label=beatstash.installer.test=" + s.ProjectName,
		ComposeTemplate:      s.Compose,
		ConfigTemplate:       s.Config,
		ProxyTemplate:        beatstash.TelegramProxyTemplate,
		TelegramURL:          s.Telegram.URL,
		TelegramDC:           s.TelegramDC,
		HTTP:                 s.HTTP,
		HTTPSWait:            5 * time.Second,
	}, args)
	return out.String(), err
}

func (s *Setup) Deploy(name string) string {
	return filepath.Join(s.Home, "beatstash", "deploy", name)
}

func (s *Setup) Read(name string) string {
	s.t.Helper()
	return fileText(s.t, s.Deploy(name))
}

func fileText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: files the test made
	require.NoError(t, err)
	return string(data)
}

func (s *Setup) Settings() map[string]any {
	s.t.Helper()
	var settings map[string]any
	_, err := toml.Decode(s.Read("config.toml"), &settings)
	require.NoError(s.t, err)
	return settings
}

// Running lists the services of the stack that run.
func (s *Setup) Running() []string {
	s.t.Helper()
	return strings.Fields(docker(s.t, "ps", "--filter", "label=com.docker.compose.project="+s.ProjectName,
		"--format", `{{.Label "com.docker.compose.service"}}`))
}

// Navidrome is where the host reaches the installed Navidrome.
func (s *Setup) Navidrome() string {
	s.t.Helper()
	id := strings.TrimSpace(docker(s.t, "ps", "-q", "--filter", "label=com.docker.compose.project="+s.ProjectName, "--filter", "label=com.docker.compose.service=navidrome"))
	port := strings.TrimSpace(docker(s.t, "port", id, "4533"))
	return "http://" + port
}

func (s *Setup) Login(user, password string) int {
	s.t.Helper()
	body, err := json.Marshal(map[string]string{"username": user, "password": password})
	require.NoError(s.t, err)
	req, err := http.NewRequestWithContext(s.t.Context(), http.MethodPost, s.Navidrome()+"/auth/login", bytes.NewReader(body))
	require.NoError(s.t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	require.NoError(s.t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func (s *Setup) Get(url string) (int, string) {
	s.t.Helper()
	req, err := http.NewRequestWithContext(s.t.Context(), http.MethodGet, url, http.NoBody)
	require.NoError(s.t, err)
	resp, err := s.HTTP.Do(req)
	require.NoError(s.t, err)
	defer resp.Body.Close()
	var body bytes.Buffer
	_, err = body.ReadFrom(resp.Body)
	require.NoError(s.t, err)
	return resp.StatusCode, body.String()
}

// install answers the installer for a new server; publicURL may be empty.
func (s *Setup) install(publicURL string, https ...string) string {
	s.t.Helper()
	answers := append([]string{
		"~/beatstash", "new", token, "", "42", "@me", "1", apiHash,
		"", publicURL, password, "",
	}, https...)
	out, err := s.Run(nil, append(answers, "y", "y")...)
	require.NoError(s.t, err, out)
	return out
}

// client reaches every *.example.test site through the test Caddy and
// trusts its own certificate authority.
func client(caddyHTTPS string) *http.Client {
	var dialer net.Dialer
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				if host, _, _ := net.SplitHostPort(address); strings.HasSuffix(host, ".example.test") {
					address = caddyHTTPS
				}
				return dialer.DialContext(ctx, network, address)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // the test Caddy signs with its own authority
		},
	}
}

// Proxy forwards plain HTTP requests, as a VPN client's HTTP inbound does,
// and records what went through it.
type Proxy struct {
	// Port is where it listens on every host address, the Docker gateway too.
	Port  string
	mu    sync.Mutex
	paths []string
}

func newProxy(t *testing.T) *Proxy {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "0.0.0.0:0") //nolint:gosec // G102: the installer reaches it through the Docker gateway
	require.NoError(t, err)
	p := &Proxy{}
	_, p.Port, err = net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	server := &httptest.Server{Listener: listener, Config: &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.paths = append(p.paths, r.URL.Path)
		p.mu.Unlock()
		r.RequestURI = ""
		resp, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})}}
	server.Start()
	t.Cleanup(server.Close)
	return p
}

func (p *Proxy) Paths() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.paths...)
}

// Telegram is the cloud Bot API.
type Telegram struct {
	URL string
	// Answer is what every request gets.
	Answer string
	mu     sync.Mutex
	calls  []string
}

func newTelegram(t *testing.T) *Telegram {
	tg := &Telegram{Answer: `{"ok":true,"result":true}`}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tg.mu.Lock()
		tg.calls = append(tg.calls, r.Method+" "+r.URL.Path)
		answer := tg.Answer
		tg.mu.Unlock()
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	tg.URL = server.URL
	return tg
}

func (tg *Telegram) Calls() []string {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	return append([]string(nil), tg.calls...)
}

// startCaddy runs Caddy on the host network, as a shared proxy on a server
// does, and returns its Caddyfile on the host.
func (s *Setup) startCaddy(caddyfile string) string {
	t := s.t
	t.Helper()
	file := filepath.Join(t.TempDir(), "Caddyfile")
	require.NoError(t, os.WriteFile(file, []byte(caddyfile), 0o600))
	docker(t, "run", "-d", "--name", s.CaddyName, "--label", "beatstash.installer.test="+s.ProjectName, "--network", "host", "-v", file+":/etc/caddy/Caddyfile:ro", "caddy:2")
	t.Cleanup(func() { docker(t, "rm", "-f", s.CaddyName) })
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+s.CaddyAdmin+"/config/", http.NoBody)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 30*time.Second, 200*time.Millisecond)
	return file
}

func removeStack(t *testing.T, project string) {
	t.Helper()
	docker(t, "compose", "-p", project, "down", "-v", "--remove-orphans")
}

// Each scenario has its own host-network Caddy, with ports assigned by the OS.
func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func (s *Setup) Container(service string) string {
	return s.ProjectName + "-" + service + "-1"
}

func docker(t *testing.T, args ...string) string {
	t.Helper()
	// Cleanups call it too, after the test's own context is cancelled.
	out, err := exec.CommandContext(context.Background(), "docker", args...).CombinedOutput() //nolint:gosec // G204: the test drives the docker CLI
	require.NoError(t, err, string(out))
	return string(out)
}
