package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/lubaskinc0de/beatstash"
	"github.com/lubaskinc0de/beatstash/internal/installer"
)

// Set by the release build; an empty repository is the upstream one.
var (
	version    = ""
	repository = ""
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
}

// Ctrl-C keeps its default effect: questions block on reading the terminal,
// so a cancelled context would go unnoticed until the next answer.
func run() error {
	return installer.Run(context.Background(), installer.Options{
		In:              os.Stdin,
		Out:             os.Stdout,
		Getenv:          os.Getenv,
		Version:         version,
		Repository:      repository,
		ComposeTemplate: beatstash.ComposeTemplate,
		ConfigTemplate:  beatstash.ConfigTemplate,
		ProxyTemplate:   beatstash.TelegramProxyTemplate,
		TelegramURL:     "https://api.telegram.org",
		TelegramDC:      "149.154.167.51:443",
		HTTP:            &http.Client{Timeout: 30 * time.Second},
		HTTPSWait:       time.Minute,
		Open:            openBrowser,
	}, os.Args[1:])
}

func openBrowser(url string) error {
	for _, opener := range []string{"wslview", "xdg-open", "open"} {
		if path, err := exec.LookPath(opener); err == nil {
			return exec.CommandContext(context.Background(), path, url).Start() //nolint:gosec // G204: a browser opener found on PATH
		}
	}
	return exec.ErrNotFound
}
