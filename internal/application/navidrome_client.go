package application

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

type NavidromeClient struct {
	BaseURL  string
	Password string
	Username string
}

func NewNavidromeClient(baseUrl, password, username string) *NavidromeClient {
	return &NavidromeClient{BaseURL: baseUrl, Password: password, Username: username}
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

func randomSalt() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 12)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}

	return string(b)
}
