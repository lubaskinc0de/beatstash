package navidrome

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appnd "github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
)

const nativeAuthHeader = "X-Nd-Authorization"

var ErrSessionNotFound = errors.New("navidrome session not found")

type SessionRepository interface {
	GetToken(ctx context.Context, username string) (string, error)
	SaveToken(ctx context.Context, username string, token string) error
}

// requestTimeout bounds a call to Navidrome: a hung server must not hang
// startup, whose context only ends with the process.
const requestTimeout = 30 * time.Second

type Client struct {
	baseURL  string
	sessions SessionRepository
	http     *http.Client
}

func NewClient(baseURL string, sessions SessionRepository) *Client {
	return &Client{
		baseURL:  baseURL,
		sessions: sessions,
		http:     &http.Client{Timeout: requestTimeout},
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

func addAuth(q url.Values, creds appnd.Credentials) {
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

func (c *Client) NowPlaying(ctx context.Context, creds appnd.Credentials) (*appnd.PlayingTrack, error) {
	resp, err := c.getNowPlaying(ctx, creds)
	if err != nil {
		return nil, err
	}

	for _, entry := range resp.SubsonicResponse.NowPlaying.Entry {
		if !strings.EqualFold(entry.Username, creds.Login) {
			continue
		}
		return &appnd.PlayingTrack{
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

func (c *Client) getNowPlaying(ctx context.Context, creds appnd.Credentials) (*nowPlayingResponse, error) {
	u, err := url.Parse(c.baseURL + "/rest/getNowPlaying")
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

	resp, err := c.http.Do(req)
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
func (c *Client) RecentlyPlayed(ctx context.Context, creds appnd.Credentials, limit int) ([]appnd.PlayedTrack, error) {
	q := url.Values{}
	q.Set("_sort", "playDate")
	q.Set("_order", "DESC")
	q.Set("_start", "0")
	q.Set("_end", strconv.Itoa(limit))

	var tracks []recentTrack
	if err := c.native(ctx, creds, nativeRequest{method: http.MethodGet, path: "/api/song", query: q}, &tracks); err != nil {
		return nil, err
	}

	played := make([]appnd.PlayedTrack, 0, len(tracks))
	for _, track := range tracks {
		if track.PlayDate == nil {
			continue
		}
		played = append(played, appnd.PlayedTrack{
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

func isUnique(err error, field string) bool {
	var invalid *validationError
	return errors.As(err, &invalid) && invalid.Errors[field] == "ra.validation.unique"
}

func (c *Client) native(ctx context.Context, creds appnd.Credentials, r nativeRequest, out any) error {
	token, err := c.sessions.GetToken(ctx, creds.Login)
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

func (c *Client) doNative(ctx context.Context, creds appnd.Credentials, r nativeRequest, token string, out any) error {
	u, err := url.Parse(c.baseURL + r.path)
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

	resp, err := c.http.Do(req)
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
		if err := c.sessions.SaveToken(ctx, creds.Login, refreshed); err != nil {
			return err
		}
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) CreateAccount(ctx context.Context, admin, account appnd.Credentials) error {
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

	if isUnique(err, "userName") {
		return appnd.ErrLoginTaken
	}
	return err
}

type library struct {
	ID              int    `json:"id,omitempty"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	DefaultNewUsers bool   `json:"defaultNewUsers"`
}

func toLibrary(l appnd.Library) library {
	return library{Name: l.Name, Path: l.Path, DefaultNewUsers: l.DefaultNewUsers}
}

func (c *Client) Libraries(ctx context.Context, admin appnd.Credentials) ([]appnd.Library, error) {
	var libraries []library
	if err := c.native(ctx, admin, nativeRequest{method: http.MethodGet, path: "/api/library"}, &libraries); err != nil {
		return nil, err
	}

	result := make([]appnd.Library, 0, len(libraries))
	for _, l := range libraries {
		result = append(result, appnd.Library{ID: l.ID, Name: l.Name, Path: l.Path, DefaultNewUsers: l.DefaultNewUsers})
	}
	return result, nil
}

func (c *Client) CreateLibrary(ctx context.Context, admin appnd.Credentials, l appnd.Library) (int, error) {
	var created struct {
		ID string `json:"id"`
	}
	err := c.native(ctx, admin, nativeRequest{
		method: http.MethodPost,
		path:   "/api/library",
		body:   toLibrary(l),
	}, &created)

	if isUnique(err, "name") {
		return 0, appnd.ErrNameTaken
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(created.ID)
}

func (c *Client) UpdateLibrary(ctx context.Context, admin appnd.Credentials, l appnd.Library) error {
	err := c.native(ctx, admin, nativeRequest{
		method: http.MethodPut,
		path:   "/api/library/" + strconv.Itoa(l.ID),
		body:   toLibrary(l),
	}, nil)

	if isUnique(err, "name") {
		return appnd.ErrNameTaken
	}
	return err
}

func (c *Client) SetLibraries(ctx context.Context, admin appnd.Credentials, login string, libraryIDs []int) error {
	var users []struct {
		ID       string `json:"id"`
		UserName string `json:"userName"`
		IsAdmin  bool   `json:"isAdmin"`
	}
	if err := c.native(ctx, admin, nativeRequest{method: http.MethodGet, path: "/api/user"}, &users); err != nil {
		return err
	}

	for _, user := range users {
		if !strings.EqualFold(user.UserName, login) {
			continue
		}
		if user.IsAdmin {
			return appnd.ErrAdminAccount
		}
		return c.native(ctx, admin, nativeRequest{
			method: http.MethodPut,
			path:   "/api/user/" + user.ID + "/library",
			body:   map[string]any{"libraryIds": libraryIDs},
		}, nil)
	}
	return fmt.Errorf("navidrome has no user %q", login)
}

func (c *Client) Authenticate(ctx context.Context, creds appnd.Credentials) error {
	_, err := c.login(ctx, creds)
	return err
}

func (c *Client) login(ctx context.Context, creds appnd.Credentials) (string, error) {
	body, err := json.Marshal(map[string]string{
		"username": creds.Login,
		"password": creds.Password,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", appnd.ErrInvalidCredentials
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

	if err := c.sessions.SaveToken(ctx, creds.Login, result.Token); err != nil {
		return "", err
	}

	return result.Token, nil
}

func randomSalt() string {
	return rand.Text()
}

const subsonicNotFound = 70

type subsonicError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *subsonicError) Error() string {
	return fmt.Sprintf("navidrome subsonic error %d: %s", e.Code, e.Message)
}

// subsonic posts the parameters as a form: a playlist of hundreds of songs
// would not fit into a URL.
func (c *Client) subsonic(ctx context.Context, creds appnd.Credentials, endpoint string, params url.Values, out any) error {
	form := maps.Clone(params)
	if form == nil {
		form = url.Values{}
	}
	addAuth(form, creds)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/"+endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("navidrome returned status %s", resp.Status)
	}

	var envelope struct {
		Response json.RawMessage `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	var status struct {
		Status string        `json:"status"`
		Error  subsonicError `json:"error"`
	}
	if err := json.Unmarshal(envelope.Response, &status); err != nil {
		return err
	}
	if status.Status != "ok" {
		return &status.Error
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Response, out)
}

const songsPage = 500

func (c *Client) Songs(ctx context.Context, creds appnd.Credentials, libraryID int) (map[string]string, error) {
	songs := map[string]string{}
	for start := 0; ; start += songsPage {
		q := url.Values{}
		q.Set("library_id", strconv.Itoa(libraryID))
		q.Set("_sort", "id")
		q.Set("_start", strconv.Itoa(start))
		q.Set("_end", strconv.Itoa(start+songsPage))

		var page []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		}
		if err := c.native(ctx, creds, nativeRequest{method: http.MethodGet, path: "/api/song", query: q}, &page); err != nil {
			return nil, err
		}
		for _, song := range page {
			songs[song.Path] = song.ID
		}
		if len(page) < songsPage {
			return songs, nil
		}
	}
}

func (c *Client) Star(ctx context.Context, creds appnd.Credentials, songIDs []string) error {
	if len(songIDs) == 0 {
		return nil
	}
	return c.subsonic(ctx, creds, "star", url.Values{"id": songIDs}, nil)
}

func (c *Client) Unstar(ctx context.Context, creds appnd.Credentials, songIDs []string) error {
	if len(songIDs) == 0 {
		return nil
	}
	return c.subsonic(ctx, creds, "unstar", url.Values{"id": songIDs}, nil)
}

func (c *Client) SavePlaylist(
	ctx context.Context,
	creds appnd.Credentials,
	id, name string,
	songIDs []string,
) (string, error) {
	params := url.Values{"songId": songIDs}
	if id != "" {
		params.Set("playlistId", id)
	} else {
		params.Set("name", name)
	}

	var result struct {
		Playlist struct {
			ID string `json:"id"`
		} `json:"playlist"`
	}
	err := c.subsonic(ctx, creds, "createPlaylist", params, &result)
	var missing *subsonicError
	if id != "" && errors.As(err, &missing) && missing.Code == subsonicNotFound {
		return c.SavePlaylist(ctx, creds, "", name, songIDs)
	}
	if err != nil {
		return "", err
	}
	return result.Playlist.ID, nil
}
