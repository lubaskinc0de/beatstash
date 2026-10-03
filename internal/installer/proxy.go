package installer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/installer/config"
	"github.com/lubaskinc0de/beatstash/internal/installer/shell"
)

// proxyFile adds the container that routes the local Bot API through a proxy.
const proxyFile = "compose.telegram-proxy.yml"

// dockerHost is how containers address this server.
const dockerHost = "host.docker.internal"

// chooseTelegramProxy asks for a proxy when this server cannot reach
// Telegram, and only then: the local Bot API cannot sign in without it.
func (s *setup) chooseTelegramProxy(ctx context.Context) error {
	saved := s.project.Env().Get("TELEGRAM_PROXY")
	if saved == "" && reachable(ctx, s.TelegramDC) {
		return nil
	}
	if saved == "" {
		s.t.Warn("This server cannot reach Telegram: it is blocked in this country or by the hosting provider.")
	}
	s.t.Say("Without Telegram the local Bot API cannot sign in, and the bot does not start. It can")
	s.t.Say("reach Telegram through a proxy, given as http://host:port or socks5://host:port.")
	s.t.Say("Usually that is a VPN client on this server, such as xray, v2ray or sing-box: take the")
	s.t.Say("port of its HTTP or SOCKS inbound from the inbounds section of its config.")
	s.t.Say("Containers reach this server as %s, so the proxy must listen on the Docker", dockerHost)
	s.t.Say("gateway 172.17.0.1, not only on 127.0.0.1. Check it yourself with:")
	s.t.Block("curl -x http://172.17.0.1:10809 https://api.telegram.org")
	for {
		answer := s.t.Ask("Proxy for Telegram, e.g. http://"+dockerHost+":10809 [optional]:", saved, "")
		if answer == "" {
			s.t.Warn("Skipped: the bot starts once this server reaches Telegram.")
			return s.useProxy("")
		}
		if err := s.checkProxy(ctx, answer); err != nil {
			s.t.Warn("%v", err)
			continue
		}
		s.t.Done("Telegram answers through %s.", answer)
		return s.useProxy(answer)
	}
}

func (s *setup) checkProxy(ctx context.Context, address string) error {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "socks5") || u.Port() == "" {
		return errors.New("enter the proxy as http://host:port or socks5://host:port")
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return fmt.Errorf("containers cannot reach 127.0.0.1 of this server; use %s and let the proxy listen on 172.17.0.1", dockerHost)
	}
	client, err := s.proxied(ctx, address)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.TelegramURL, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram does not answer through %s: %w", address, err)
	}
	_ = resp.Body.Close()
	return nil
}

// useProxy turns the proxy container on for address, or off when it is empty.
func (s *setup) useProxy(address string) error {
	env := s.project.Env()
	if address == "" {
		if env.Get("COMPOSE_FILE") == "" {
			return nil
		}
		return env.Set("COMPOSE_FILE", "compose.yml")
	}
	if _, err := os.Stat(s.project.Path(proxyFile)); errors.Is(err, os.ErrNotExist) {
		if err := config.WriteFile(s.project.Path(proxyFile), s.ProxyTemplate); err != nil {
			return err
		}
	}
	if err := env.Set("TELEGRAM_PROXY", address); err != nil {
		return err
	}
	return env.Set("COMPOSE_FILE", "compose.yml:"+proxyFile)
}

// telegramHTTP reaches Telegram the way the local Bot API does.
func (s *setup) telegramHTTP(ctx context.Context) (*http.Client, error) {
	address := s.project.Env().Get("TELEGRAM_PROXY")
	if address == "" {
		return s.HTTP, nil
	}
	return s.proxied(ctx, address)
}

// proxied sends requests through the proxy; the address containers use for
// this server becomes the Docker gateway, which is the same host seen from it.
func (s *setup) proxied(ctx context.Context, address string) (*http.Client, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if u.Hostname() == dockerHost {
		gateway, err := shell.Run(ctx, "", "docker", "network", "inspect", "bridge", "--format", "{{(index .IPAM.Config 0).Gateway}}")
		if err != nil {
			return nil, err
		}
		u.Host = net.JoinHostPort(strings.TrimSpace(gateway), u.Port())
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(u)
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}, nil
}

func reachable(ctx context.Context, address string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
