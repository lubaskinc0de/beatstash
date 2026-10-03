package zvuk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common/providers"
	"github.com/lubaskinc0de/beatstash/internal/infra/stall"
)

const authHeader = "X-Auth-Token"

// userAgent passes Zvuk's anti-bot guard, which answers 418 to clients that
// do not look like a browser.
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

// requestTimeout bounds an API call; downloads have no bound but ctx,
// a FLAC of a long track takes a while.
const requestTimeout = 30 * time.Second

// Client talks to the web client's endpoints: GraphQL for metadata and
// collections, tiny for the profile and stream links. tiny sits behind an
// anti-bot redirect that sets a cookie, hence the jar.
type Client struct {
	baseURL string
	http    *http.Client
	// stall cuts off a download that went silent.
	stall stall.Watch
}

func NewClient(baseURL string, stall stall.Watch) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Jar: jar, Transport: browser{http.DefaultTransport}},
		stall:   stall,
	}
}

type browser struct {
	next http.RoundTripper
}

func (b browser) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", userAgent)
	return b.next.RoundTrip(req)
}

type tinyResponse[T any] struct {
	Result T `json:"result"`
}

func (c *Client) tiny(ctx context.Context, token, path string, query url.Values, out any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u := c.baseURL + "/api/" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.do(req, token, out)
}

type graphqlResponse[T any] struct {
	Data   T `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) graphql(ctx context.Context, token, operation, query string, variables map[string]any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body, err := json.Marshal(map[string]any{"operationName": operation, "query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	var resp graphqlResponse[json.RawMessage]
	if err := c.do(req, token, &resp); err != nil {
		return fmt.Errorf("zvuk %s: %w", operation, err)
	}
	if len(resp.Errors) > 0 {
		return fmt.Errorf("zvuk %s: %s", operation, resp.Errors[0].Message)
	}
	return json.Unmarshal(resp.Data, out)
}

func (c *Client) do(req *http.Request, token string, out any) error {
	req.Header.Set(authHeader, token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return providers.ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("zvuk returned status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) download(ctx context.Context, link string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.stall.Do(c.http, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("zvuk cdn returned status %s", resp.Status)
	}
	return resp.Body, nil
}

type profile struct {
	Profile struct {
		Registered bool `json:"is_registered"`
	} `json:"profile"`
	Subscription *subscription `json:"subscription"`
}

type subscription struct {
	Status string `json:"status"`
	// Expiration is in Unix milliseconds.
	Expiration int64 `json:"expiration"`
}

// profile comes from v2: the first version says nothing of the subscription.
// A token Zvuk does not know gets an anonymous profile, not 401.
func (c *Client) profile(ctx context.Context, token string) (*profile, error) {
	var resp tinyResponse[profile]
	if err := c.tiny(ctx, token, "v2/tiny/profile", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

type stream struct {
	URL string `json:"stream"`
}

// streamURL asks for FLAC only when the track has it: otherwise Zvuk
// quietly answers with 128 kbps instead of the best MP3.
func (c *Client) streamURL(ctx context.Context, token string, t *track) (string, error) {
	quality := "high"
	if t.HasFlac {
		quality = "flac"
	}
	var resp tinyResponse[stream]
	err := c.tiny(ctx, token, "tiny/track/stream", url.Values{"id": {t.ID}, "quality": {quality}}, &resp)
	if err != nil {
		return "", err
	}
	if resp.Result.URL == "" {
		return "", errors.New("zvuk returned no stream link")
	}
	return resp.Result.URL, nil
}

type titled struct {
	Title string `json:"title"`
}

type track struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Duration int      `json:"duration"`
	HasFlac  bool     `json:"hasFlac"`
	Position int      `json:"position"`
	Artists  []titled `json:"artists"`
	Release  struct {
		ID      string   `json:"id"`
		Title   string   `json:"title"`
		Date    string   `json:"date"`
		Artists []titled `json:"artists"`
		Image   struct {
			Src string `json:"src"`
		} `json:"image"`
	} `json:"release"`
}

type identified struct {
	ID string `json:"id"`
}

const tracksQuery = `query getTracks($ids: [ID!]!) {
  getTracks(ids: $ids) {
    id title duration hasFlac position
    artists { title }
    release { id title date artists { title } image { src } }
  }
}`

const pageSize = 100

// tracks keeps the order of ids and skips the ones Zvuk does not know.
func (c *Client) tracks(ctx context.Context, token string, ids []string) ([]track, error) {
	return byIDs[track](ctx, c, token, "getTracks", tracksQuery, ids)
}

// byIDs asks for the items a page of ids at a time; Zvuk answers null for
// an id it does not know.
func byIDs[T any](ctx context.Context, c *Client, token, operation, query string, ids []string) ([]T, error) {
	var items []T
	for page := range slices.Chunk(ids, pageSize) {
		resp := map[string][]*T{}
		if err := c.graphql(ctx, token, operation, query, map[string]any{"ids": page}, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp[operation] {
			if item != nil {
				items = append(items, *item)
			}
		}
	}
	return items, nil
}

type release struct {
	ID     string       `json:"id"`
	Title  string       `json:"title"`
	Tracks []identified `json:"tracks"`
}

const releasesQuery = `query getReleases($ids: [ID!]!) {
  getReleases(ids: $ids) { id title tracks { id } }
}`

func (c *Client) releases(ctx context.Context, token string, ids []string) ([]release, error) {
	return byIDs[release](ctx, c, token, "getReleases", releasesQuery, ids)
}

type playlist struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

const playlistsQuery = `query getPlaylists($ids: [ID!]!) {
  getPlaylists(ids: $ids) { id title }
}`

func (c *Client) playlists(ctx context.Context, token string, ids []string) ([]playlist, error) {
	return byIDs[playlist](ctx, c, token, "getPlaylists", playlistsQuery, ids)
}

const playlistTracksQuery = `query getPlaylistTracks($id: ID!, $limit: Int!, $offset: Int!) {
  getPlaylists(ids: [$id]) { trackCount tracks(limit: $limit, offset: $offset) { id } }
}`

// playlistTracks pages up to trackCount: a page leaves out tracks no longer
// available, so a short one does not mean the end.
func (c *Client) playlistTracks(ctx context.Context, token, id string) ([]string, error) {
	var ids []string
	for offset := 0; ; offset += pageSize {
		var resp struct {
			Playlists []struct {
				TrackCount int          `json:"trackCount"`
				Tracks     []identified `json:"tracks"`
			} `json:"getPlaylists"`
		}
		vars := map[string]any{"id": id, "limit": pageSize, "offset": offset}
		if err := c.graphql(ctx, token, "getPlaylistTracks", playlistTracksQuery, vars, &resp); err != nil {
			return nil, err
		}
		if len(resp.Playlists) == 0 {
			return ids, nil
		}
		page := resp.Playlists[0]
		for _, t := range page.Tracks {
			ids = append(ids, t.ID)
		}
		if offset+pageSize >= page.TrackCount {
			return ids, nil
		}
	}
}

type collection struct {
	Tracks    []identified `json:"tracks"`
	Releases  []identified `json:"releases"`
	Playlists []identified `json:"playlists"`
}

// Artists come in the collection too, but are never expanded into their
// discographies.
const collectionQuery = `query userCollection {
  collection { tracks { id } releases { id } playlists { id } artists { id } }
}`

func (c *Client) collection(ctx context.Context, token string) (*collection, error) {
	var resp struct {
		Collection collection `json:"collection"`
	}
	if err := c.graphql(ctx, token, "userCollection", collectionQuery, map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return &resp.Collection, nil
}

func ids(items []identified) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ID)
	}
	return result
}
