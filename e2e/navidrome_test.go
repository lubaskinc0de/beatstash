package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const navidromeImage = "deluan/navidrome:0.64.1"

type navidrome struct {
	url string
}

func startNavidrome(ctx context.Context, libraryRoot string) (*navidrome, testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Image:        navidromeImage,
		ExposedPorts: []string{"4533/tcp"},
		Env: map[string]string{
			"ND_LOGLEVEL":                       "warn",
			"ND_ENABLEINSIGHTSCOLLECTOR":        "false",
			"ND_SUBSONIC_DEFAULTREPORTREALPATH": "true",
			"ND_AUTHREQUESTLIMIT":               "0",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = append(hc.Binds, libraryRoot+":/music:ro")
		},
		WaitingFor: wait.ForHTTP("/ping").WithPort("4533/tcp"),
		Started:    true,
	})
	if err != nil {
		return nil, c, err
	}

	endpoint, err := c.PortEndpoint(ctx, "4533/tcp", "http")
	if err != nil {
		return nil, c, err
	}

	nd := &navidrome{url: endpoint}
	if err := nd.createAdmin(ctx); err != nil {
		return nil, c, err
	}
	return nd, c, nil
}

func (n *navidrome) createAdmin(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"username": navidromeAdmin,
		"password": navidromePassword,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url+"/auth/createAdmin", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create navidrome admin: %s", resp.Status)
	}
	return nil
}

type subsonicTrack struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Path   string `json:"path"`
}

func (n *navidrome) subsonic(endpoint string, params url.Values, out any) error {
	return n.subsonicAs(navidromeAdminAccount, endpoint, params, out)
}

func (n *navidrome) subsonicAs(user navidromeAccount, endpoint string, params url.Values, out any) error {
	q := url.Values{}
	for key, values := range params {
		q[key] = values
	}
	q.Set("u", user.Login)
	q.Set("p", user.Password)
	q.Set("v", "1.16.1")
	q.Set("c", "e2e")
	q.Set("f", "json")

	resp, err := http.Get(n.url + "/rest/" + endpoint + "?" + q.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", endpoint, resp.Status)
	}

	var envelope struct {
		Response json.RawMessage `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}

	var status struct {
		Status string `json:"status"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(envelope.Response, &status); err != nil {
		return err
	}
	if status.Status != "ok" {
		return fmt.Errorf("%s: %s", endpoint, status.Error.Message)
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Response, out)
}

// indexedTrack rescans the Library until Navidrome indexes a song
// whose file lies in libraryDir, and returns it.
func (n *navidrome) indexedTrack(t *testing.T, libraryDir string, title string) subsonicTrack {
	t.Helper()

	var found subsonicTrack
	require.Eventually(t, func() bool {
		_ = n.subsonic("startScan", nil, nil)

		var result struct {
			SearchResult3 struct {
				Song []subsonicTrack `json:"song"`
			} `json:"searchResult3"`
		}
		err := n.subsonic("search3", url.Values{
			"query":       {title},
			"artistCount": {"0"},
			"albumCount":  {"0"},
			"songCount":   {"50"},
		}, &result)
		if err != nil {
			return false
		}

		for _, song := range result.SearchResult3.Song {
			if isUnder(song.Path, libraryDir) {
				found = song
				return true
			}
		}
		return false
	}, time.Minute, 200*time.Millisecond, "navidrome did not index %q from %s", title, libraryDir)

	return found
}

func isUnder(songPath, libraryDir string) bool {
	return strings.Contains(songPath, filepath.Base(libraryDir)+"/")
}

func (n *navidrome) startPlaying(t *testing.T, user navidromeAccount, songID string) {
	t.Helper()
	err := n.subsonicAs(user, "scrobble", url.Values{
		"id":         {songID},
		"submission": {"false"},
	}, nil)
	require.NoError(t, err)
}

func (n *navidrome) play(t *testing.T, user navidromeAccount, songID string) {
	t.Helper()
	err := n.subsonicAs(user, "scrobble", url.Values{
		"id":         {songID},
		"submission": {"true"},
	}, nil)
	require.NoError(t, err)
}

type navidromeAccount struct {
	Login    string
	Password string
}

var navidromeAdminAccount = navidromeAccount{Login: navidromeAdmin, Password: navidromePassword}

var navidromeLoginSeq atomic.Int64

// uniqueLogin keeps logins apart: all scenarios share one Navidrome.
func uniqueLogin(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, navidromeLoginSeq.Add(1))
}

func (n *navidrome) createAccount(t *testing.T, prefix string) navidromeAccount {
	t.Helper()

	user := navidromeAccount{Login: uniqueLogin(prefix), Password: "secret-" + prefix}
	token, err := n.login(navidromeAdminAccount)
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]any{
		"userName": user.Login,
		"name":     user.Login,
		"password": user.Password,
		"isAdmin":  false,
	})
	req, err := http.NewRequest(http.MethodPost, n.url+"/api/user", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Nd-Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "create navidrome user %s", user.Login)
	return user
}

func (n *navidrome) canLogin(user navidromeAccount) bool {
	_, err := n.login(user)
	return err == nil
}

func (n *navidrome) login(user navidromeAccount) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": user.Login, "password": user.Password})
	resp, err := http.Post(n.url+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login %s: %s", user.Login, resp.Status)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}
