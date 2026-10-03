package navidrome

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	// ErrHasUsers means Navidrome already has its first administrator.
	ErrHasUsers = errors.New("navidrome already has users")
	ErrLogin    = errors.New("navidrome rejected the username or password")
)

type Client struct {
	URL  string
	HTTP *http.Client
}

// WaitReady polls /ping until Navidrome answers or wait runs out.
func (c Client) WaitReady(ctx context.Context, wait time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/ping", http.NoBody)
		if err != nil {
			return err
		}
		if resp, err := c.HTTP.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("navidrome at %s did not start: %w", c.URL, ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

// CreateAdmin makes the first account, which Navidrome allows only while it
// has none.
func (c Client) CreateAdmin(ctx context.Context, user, password string) error {
	status, err := c.post(ctx, "/auth/createAdmin", user, password)
	switch {
	case err != nil:
		return err
	case status == http.StatusOK:
		return nil
	case status == http.StatusForbidden:
		return ErrHasUsers
	default:
		return fmt.Errorf("navidrome refused to create the administrator: HTTP %d", status)
	}
}

func (c Client) Login(ctx context.Context, user, password string) error {
	status, err := c.post(ctx, "/auth/login", user, password)
	switch {
	case err != nil:
		return err
	case status == http.StatusOK:
		return nil
	case status == http.StatusUnauthorized:
		return ErrLogin
	default:
		return fmt.Errorf("navidrome login failed: HTTP %d", status)
	}
}

func (c Client) post(ctx context.Context, path, user, password string) (int, error) {
	body, err := json.Marshal(map[string]string{"username": user, "password": password})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}
