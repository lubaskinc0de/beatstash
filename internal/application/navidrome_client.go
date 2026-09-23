package application

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
)

const nativeAuthHeader = "X-Nd-Authorization"

type NavidromeClient struct {
	BaseURL  string
	Password string
	Username string
	Sessions NavidromeSessionRepository
}

func NewNavidromeClient(
	baseUrl, password, username string,
	sessions NavidromeSessionRepository,
) *NavidromeClient {
	return &NavidromeClient{
		BaseURL:  baseUrl,
		Password: password,
		Username: username,
		Sessions: sessions,
	}
}

type RecentSong struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Artist    string     `json:"artist"`
	Album     string     `json:"album"`
	Duration  float64    `json:"duration"`
	PlayDate  *time.Time `json:"playDate"`
	PlayCount int        `json:"playCount"`
}

type NowPlayingResponse struct {
	SubsonicResponse struct {
		Status string `json:"status"`

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

func (c *NavidromeClient) addAuth(q url.Values) {
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

func (c *NavidromeClient) GetNowPlaying(ctx context.Context) (*NowPlayingResponse, error) {
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

	var result NowPlayingResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.SubsonicResponse.Status != "ok" {
		return nil, fmt.Errorf("navidrome response status: %s", result.SubsonicResponse.Status)
	}

	return &result, nil
}

// GetRecentlyPlayed returns the songs the user listened to most recently.
// Subsonic API has no play history, so it goes through Navidrome's native API.
func (c *NavidromeClient) GetRecentlyPlayed(ctx context.Context, limit int) ([]RecentSong, error) {
	q := url.Values{}
	q.Set("_sort", "playDate")
	q.Set("_order", "DESC")
	q.Set("_start", "0")
	q.Set("_end", strconv.Itoa(limit))

	var songs []RecentSong
	if err := c.getNative(ctx, "/api/song", q, &songs); err != nil {
		return nil, err
	}

	played := songs[:0]
	for _, song := range songs {
		if song.PlayDate != nil {
			played = append(played, song)
		}
	}

	return played, nil
}

var errUnauthorized = errors.New("navidrome: unauthorized")

func (c *NavidromeClient) getNative(ctx context.Context, path string, q url.Values, out any) error {
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

func (c *NavidromeClient) doNative(ctx context.Context, path string, q url.Values, token string, out any) error {
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

func (c *NavidromeClient) login(ctx context.Context) (string, error) {
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
