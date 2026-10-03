package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Client struct {
	// URL is the Bot API server, e.g. https://api.telegram.org.
	URL  string
	HTTP *http.Client
}

// LogOut releases the bot from this Bot API server so another one can serve
// it. A bot that already left counts as logged out.
func (c Client) LogOut(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/bot"+token+"/logOut", http.NoBody)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// The error would print the URL, and the URL holds the token.
		return fmt.Errorf("cannot reach %s", c.URL)
	}
	defer resp.Body.Close()
	var answer struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return fmt.Errorf("unexpected answer from %s: HTTP %d", c.URL, resp.StatusCode)
	}
	if answer.OK || strings.Contains(strings.ToLower(answer.Description), "logged out") {
		return nil
	}
	return fmt.Errorf("telegram refused to log the bot out: %s", answer.Description)
}
