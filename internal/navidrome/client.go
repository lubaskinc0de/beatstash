package navidrome

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
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
	Password string
	Username string
	Sessions SessionRepository
}

func NewClient(
	baseUrl, password, username string,
	sessions SessionRepository,
) *Client {
	return &Client{
		BaseURL:  baseUrl,
		Password: password,
		Username: username,
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

func (c *Client) addAuth(q url.Values) {
	salt := randomSalt()

	hash := md5.Sum([]byte(c.Password + salt))
	token := hex.EncodeToString(hash[:])

	q.Set("u", c.Username)
	q.Set("s", salt)
	q.Set("t", token)
	q.Set("v", "1.16.1")
	q.Set("c", "navidrome-tg")
	q.Set("f", "json")
}

func (c *Client) NowPlaying(ctx context.Context) (*application.PlayingTrack, error) {
	resp, err := c.getNowPlaying(ctx)
	if err != nil {
		return nil, err
	}

	for _, entry := range resp.SubsonicResponse.NowPlaying.Entry {
		if entry.Username != c.Username {
			continue
		}
		return &application.PlayingTrack{
				ID:       entry.ID,
				Artist:   entry.Artist,
				Title:    entry.Title,
				Album:    entry.Album,
				Duration: entry.Duration,
			PositionMs: entry.PositionMs,
			State:      entry.State,
			CoverArt:   entry.CoverArt,
		}, nil
	}
	return nil, nil
}

func (c *Client) getNowPlaying(ctx context.Context) (*nowPlayingResponse, error) {
	u, err := url.Parse(c.BaseURL + "/rest/getNowPlaying")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.addAuth(q)
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
func (c *Client) RecentlyPlayed(ctx context.Context, limit int) ([]application.PlayedTrack, error) {
	q := url.Values{}
	q.Set("_sort", "playDate")
	q.Set("_order", "DESC")
	q.Set("_start", "0")
	q.Set("_end", strconv.Itoa(limit))

	var tracks []recentTrack
	if err := c.getNative(ctx, "/api/song", q, &tracks); err != nil {
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

func (c *Client) getNative(ctx context.Context, path string, q url.Values, out any) error {
	token, err := c.Sessions.GetToken(ctx, c.Username)
	if errors.Is(err, ErrSessionNotFound) {
		token, err = c.login(ctx)
	}
	if err != nil {
		return err
	}

	err = c.doNative(ctx, path, q, token, out)
	if !errors.Is(err, errUnauthorized) {
		return err
	}

	token, err = c.login(ctx)
	if err != nil {
		return err
	}
	return c.doNative(ctx, path, q, token, out)
}

func (c *Client) doNative(ctx context.Context, path string, q url.Values, token string, out any) error {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return err
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set(nativeAuthHeader, "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("navidrome returned status %s", resp.Status)
	}

	// Navidrome sends a renewed JWT with every response, keep it so the session never expires.
	if refreshed := resp.Header.Get(nativeAuthHeader); refreshed != "" && refreshed != token {
		if err := c.Sessions.SaveToken(ctx, c.Username, refreshed); err != nil {
			return err
		}
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) login(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]string{
		"username": c.Username,
		"password": c.Password,
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

	if err := c.Sessions.SaveToken(ctx, c.Username, result.Token); err != nil {
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
