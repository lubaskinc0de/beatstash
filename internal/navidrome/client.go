package navidrome

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
)

const nativeAuthHeader = "X-Nd-Authorization"

var ErrSessionNotFound = errors.New("navidrome session not found")

type SessionRepository interface {
	GetToken(ctx context.Context, username string) (string, error)
	SaveToken(ctx context.Context, username string, token string) error
}

type Client struct {
	BaseURL  string
	Sessions SessionRepository
}

func NewClient(baseUrl string, sessions SessionRepository) *Client {
	return &Client{
		BaseURL:  baseUrl,
		Sessions: sessions,
	}
}

type recentTrack struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Artist    string     `json:"artist"`
	Album     string     `json:"album"`
	Duration  float64    `json:"duration"`
	PlayDate  *time.Time `json:"playDate"`
	PlayCount int        `json:"playCount"`
}

type nowPlayingResponse struct {
	SubsonicResponse struct {
		Status string `json:"status"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`

		NowPlaying struct {
			Entry []struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Artist     string `json:"artist"`
				Album      string `json:"album"`
				Duration   int    `json:"duration"`
				Username   string `json:"username"`
				State      string `json:"state"`
				PositionMs int    `json:"positionMs"`
				PlayerName string `json:"playerName"`
				CoverArt   string `json:"coverArt"`
			} `json:"entry"`
		} `json:"nowPlaying"`
	} `json:"subsonic-response"`
}

func addAuth(q url.Values, creds application.NavidromeCredentials) {
	salt := randomSalt()

	hash := md5.Sum([]byte(creds.Password + salt))
	token := hex.EncodeToString(hash[:])

	q.Set("u", creds.Login)
	q.Set("s", salt)
	q.Set("t", token)
	q.Set("v", "1.16.1")
	q.Set("c", "navidrome-tg")
	q.Set("f", "json")
}

func (c *Client) NowPlaying(ctx context.Context, creds application.NavidromeCredentials) (*application.PlayingTrack, error) {
	resp, err := c.getNowPlaying(ctx, creds)
	if err != nil {
		return nil, err
	}

	for _, entry := range resp.SubsonicResponse.NowPlaying.Entry {
		if !strings.EqualFold(entry.Username, creds.Login) {
			continue
		}
		return &application.PlayingTrack{
			ID:         entry.ID,
			Artist:     entry.Artist,
			Title:      entry.Title,
			Album:      entry.Album,
			Duration:   entry.Duration,
			PositionMs: entry.PositionMs,
			State:      entry.State,
			CoverArt:   entry.CoverArt,
		}, nil
	}
	return nil, nil
}

func (c *Client) getNowPlaying(ctx context.Context, creds application.NavidromeCredentials) (*nowPlayingResponse, error) {
	u, err := url.Parse(c.BaseURL + "/rest/getNowPlaying")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	addAuth(q, creds)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("navidrome returned status %s", resp.Status)
	}

	var result nowPlayingResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.SubsonicResponse.Status != "ok" {
		return nil, fmt.Errorf("navidrome response status %s: %s", result.SubsonicResponse.Status, result.SubsonicResponse.Error.Message)
	}

	return &result, nil
}

// Subsonic API has no play history, so it goes through Navidrome's native API.
func (c *Client) RecentlyPlayed(ctx context.Context, creds application.NavidromeCredentials, limit int) ([]application.PlayedTrack, error) {
	q := url.Values{}
	q.Set("_sort", "playDate")
	q.Set("_order", "DESC")
	q.Set("_start", "0")
	q.Set("_end", strconv.Itoa(limit))

	var tracks []recentTrack
	if err := c.native(ctx, creds, nativeRequest{method: http.MethodGet, path: "/api/song", query: q}, &tracks); err != nil {
		return nil, err
	}

	played := make([]application.PlayedTrack, 0, len(tracks))
	for _, track := range tracks {
		if track.PlayDate == nil {
			continue
		}
		played = append(played, application.PlayedTrack{
			ID:       track.ID,
			Artist:   track.Artist,
			Title:    track.Title,
			Album:    track.Album,
			Duration: int(track.Duration),
			PlayedAt: *track.PlayDate,
		})
	}

	return played, nil
}

var errUnauthorized = errors.New("navidrome: unauthorized")

type nativeRequest struct {
	method string
	path   string
	query  url.Values
	body   any
}

// validationError is Navidrome's 400 answer: field name to message key.
type validationError struct {
	Errors map[string]string `json:"errors"`
}

func (e *validationError) Error() string {
	return fmt.Sprintf("navidrome validation failed: %v", e.Errors)
}

func (c *Client) native(ctx context.Context, creds application.NavidromeCredentials, r nativeRequest, out any) error {
	token, err := c.Sessions.GetToken(ctx, creds.Login)
	if errors.Is(err, ErrSessionNotFound) {
		token, err = c.login(ctx, creds)
	}
	if err != nil {
		return err
	}

	err = c.doNative(ctx, creds, r, token, out)
	if !errors.Is(err, errUnauthorized) {
		return err
	}

	token, err = c.login(ctx, creds)
	if err != nil {
		return err
	}
	return c.doNative(ctx, creds, r, token, out)
}

func (c *Client) doNative(ctx context.Context, creds application.NavidromeCredentials, r nativeRequest, token string, out any) error {
	u, err := url.Parse(c.BaseURL + r.path)
	if err != nil {
		return err
	}
	u.RawQuery = r.query.Encode()

	var body io.Reader
	if r.body != nil {
		data, err := json.Marshal(r.body)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, r.method, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set(nativeAuthHeader, "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode == http.StatusBadRequest {
		var invalid validationError
		if err := json.NewDecoder(resp.Body).Decode(&invalid); err != nil {
			return fmt.Errorf("navidrome returned status %s", resp.Status)
		}
		return &invalid
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("navidrome returned status %s", resp.Status)
	}

	// Navidrome sends a renewed JWT with every response, keep it so the session never expires.
	if refreshed := resp.Header.Get(nativeAuthHeader); refreshed != "" && refreshed != token {
		if err := c.Sessions.SaveToken(ctx, creds.Login, refreshed); err != nil {
			return err
		}
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) CreateAccount(ctx context.Context, admin, account application.NavidromeCredentials) error {
	var created struct {
		ID string `json:"id"`
	}
	err := c.native(ctx, admin, nativeRequest{
		method: http.MethodPost,
		path:   "/api/user",
		body: map[string]any{
			"userName": account.Login,
			"name":     account.Login,
			"password": account.Password,
			"isAdmin":  false,
		},
	}, &created)

	var invalid *validationError
	if errors.As(err, &invalid) && invalid.Errors["userName"] == "ra.validation.unique" {
		return application.ErrNavidromeLoginTaken
	}
	return err
}

func (c *Client) Authenticate(ctx context.Context, creds application.NavidromeCredentials) error {
	_, err := c.login(ctx, creds)
	return err
}

func (c *Client) login(ctx context.Context, creds application.NavidromeCredentials) (string, error) {
	body, err := json.Marshal(map[string]string{
		"username": creds.Login,
		"password": creds.Password,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", application.ErrNavidromeInvalidCredentials
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("navidrome login returned status %s", resp.Status)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Token == "" {
		return "", errors.New("navidrome login returned empty token")
	}

	if err := c.Sessions.SaveToken(ctx, creds.Login, result.Token); err != nil {
		return "", err
	}

	return result.Token, nil
}

func randomSalt() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 12)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}

	return string(b)
}
