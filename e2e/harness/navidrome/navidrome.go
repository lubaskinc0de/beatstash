package navidrome

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	AdminUser     = "admin"
	AdminPassword = "admin"
)

const dockerImage = "deluan/navidrome:0.64.1"

const LibraryMount = "/music"

// RootLibrary keeps Navidrome's own library, which can be neither
// moved nor deleted, away from the scenarios' libraries.
const RootLibrary = ".navidrome-root"

type Server struct {
	URL string
}

func Start(ctx context.Context, libraryRoot string) (*Server, testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Image:        dockerImage,
		ExposedPorts: []string{"4533/tcp"},
		Env: map[string]string{
			"ND_LOGLEVEL":                       "warn",
			"ND_ENABLEINSIGHTSCOLLECTOR":        "false",
			"ND_SUBSONIC_DEFAULTREPORTREALPATH": "true",
			"ND_AUTHREQUESTLIMIT":               "0",
			// Scenarios scan on their own: a watcher would index files at
			// its own pace and hide what waits for the scan.
			"ND_SCANNER_WATCHERWAIT": "0",
			"ND_MUSICFOLDER":         LibraryMount + "/" + RootLibrary,
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = append(hc.Binds, libraryRoot+":"+LibraryMount+":ro")
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

	nd := &Server{URL: endpoint}
	if err := nd.createAdmin(ctx); err != nil {
		return nil, c, err
	}
	return nd, c, nil
}

func (n *Server) createAdmin(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"username": AdminUser,
		"password": AdminPassword,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL+"/auth/createAdmin", bytes.NewReader(body))
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

type Track struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Path   string `json:"path"`
}

func (n *Server) subsonic(endpoint string, params url.Values, out any) error {
	return n.subsonicAs(adminAccount, endpoint, params, out)
}

func (n *Server) subsonicAs(user Account, endpoint string, params url.Values, out any) error {
	q := url.Values{}
	for key, values := range params {
		q[key] = values
	}
	q.Set("u", user.Login)
	q.Set("p", user.Password)
	q.Set("v", "1.16.1")
	q.Set("c", "e2e")
	q.Set("f", "json")

	resp, err := http.Get(n.URL + "/rest/" + endpoint + "?" + q.Encode())
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

// IndexedTrack rescans the Library until the account finds a song whose
// file lies in libraryDir, and returns it.
func (n *Server) IndexedTrack(t *testing.T, account Account, libraryDir string, title string) Track {
	t.Helper()

	var found Track
	require.Eventually(t, func() bool {
		_ = n.subsonic("startScan", nil, nil)
		songs, err := n.search(account, libraryDir, title)
		if err != nil || len(songs) == 0 {
			return false
		}
		found = songs[0]
		return true
	}, time.Minute, 200*time.Millisecond, "%s did not find %q in %s", account.Login, title, libraryDir)

	return found
}

// SearchFor lists songs the account finds in libraryDir; call it after
// IndexedTrack proved the scan has reached the song.
func (n *Server) SearchFor(t *testing.T, account Account, libraryDir string, title string) []Track {
	t.Helper()

	songs, err := n.search(account, libraryDir, title)
	require.NoError(t, err)
	return songs
}

func (n *Server) search(account Account, libraryDir string, title string) ([]Track, error) {
	var result struct {
		SearchResult3 struct {
			Song []Track `json:"song"`
		} `json:"searchResult3"`
	}
	err := n.subsonicAs(account, "search3", url.Values{
		"query":       {title},
		"artistCount": {"0"},
		"albumCount":  {"0"},
		"songCount":   {"50"},
	}, &result)
	if err != nil {
		return nil, err
	}

	var songs []Track
	for _, song := range result.SearchResult3.Song {
		if isUnder(song.Path, libraryDir) {
			songs = append(songs, song)
		}
	}
	return songs, nil
}

// Libraries lists the paths of the Navidrome Libraries the account may see.
func (n *Server) Libraries(t *testing.T, account Account) []string {
	t.Helper()

	var users []struct {
		ID        string `json:"id"`
		UserName  string `json:"userName"`
		Libraries []struct {
			Path string `json:"path"`
		} `json:"libraries"`
	}
	n.adminAPI(t, http.MethodGet, "/api/user", nil, &users)
	for _, user := range users {
		if user.UserName != account.Login {
			continue
		}
		var paths []string
		for _, library := range user.Libraries {
			paths = append(paths, library.Path)
		}
		slices.Sort(paths)
		return paths
	}
	t.Fatalf("navidrome has no user %s", account.Login)
	return nil
}

func (n *Server) adminAPI(t *testing.T, method, path string, body, out any) {
	t.Helper()
	require.NoError(t, n.callAdminAPI(method, path, body, out))
}

// adminAPIEventually retries while Navidrome is still busy with a library
// deleted a moment ago: meanwhile it answers 500 on the same path.
func (n *Server) adminAPIEventually(t *testing.T, method, path string, body, out any) {
	t.Helper()

	var err error
	require.Eventually(t, func() bool {
		err = n.callAdminAPI(method, path, body, out)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond, "%s %s: %v", method, path, err)
}

func (n *Server) callAdminAPI(method, path string, body, out any) error {
	token, err := n.login(adminAccount)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, n.URL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Nd-Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func isUnder(songPath, libraryDir string) bool {
	return strings.Contains(songPath, filepath.Base(libraryDir)+"/")
}

func (n *Server) StartPlaying(t *testing.T, user Account, songID string) {
	t.Helper()
	err := n.subsonicAs(user, "scrobble", url.Values{
		"id":         {songID},
		"submission": {"false"},
	}, nil)
	require.NoError(t, err)
}

func (n *Server) Play(t *testing.T, user Account, songID string) {
	t.Helper()
	err := n.subsonicAs(user, "scrobble", url.Values{
		"id":         {songID},
		"submission": {"true"},
	}, nil)
	require.NoError(t, err)
}

type Account struct {
	Login    string
	Password string
}

var adminAccount = Account{Login: AdminUser, Password: AdminPassword}

var loginSeq atomic.Int64

// UniqueLogin keeps logins apart: all scenarios share one Navidrome.
func UniqueLogin(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, loginSeq.Add(1))
}

func (n *Server) CreateAccount(t *testing.T, prefix string) Account {
	t.Helper()

	user := Account{Login: UniqueLogin(prefix), Password: "secret-" + prefix}
	token, err := n.login(adminAccount)
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]any{
		"userName": user.Login,
		"name":     user.Login,
		"password": user.Password,
		"isAdmin":  false,
	})
	req, err := http.NewRequest(http.MethodPost, n.URL+"/api/user", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Nd-Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "create navidrome user %s", user.Login)
	return user
}

func (n *Server) CanLogin(user Account) bool {
	_, err := n.login(user)
	return err == nil
}

func (n *Server) login(user Account) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": user.Login, "password": user.Password})
	resp, err := http.Post(n.URL+"/auth/login", "application/json", bytes.NewReader(body))
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

func (n *Server) CreateLibrary(t *testing.T, name, path string) int {
	t.Helper()

	var created struct {
		ID string `json:"id"`
	}
	n.adminAPIEventually(t, "POST", "/api/library", map[string]string{"name": name, "path": path}, &created)
	var id int
	_, err := fmt.Sscan(created.ID, &id)
	require.NoError(t, err)
	return id
}

func (n *Server) DeleteLibrary(t *testing.T, id int) {
	t.Helper()
	n.adminAPIEventually(t, "DELETE", fmt.Sprintf("/api/library/%d", id), nil, nil)
}

func (n *Server) LibraryAt(t *testing.T, path string) int {
	t.Helper()

	var libraries []struct {
		ID   int    `json:"id"`
		Path string `json:"path"`
	}
	n.adminAPI(t, "GET", "/api/library", nil, &libraries)
	for _, library := range libraries {
		if library.Path == path {
			return library.ID
		}
	}
	t.Fatalf("navidrome has no library at %s", path)
	return 0
}

func (n *Server) GrantAllLibraries(t *testing.T, account Account) {
	t.Helper()

	var libraries []struct {
		ID int `json:"id"`
	}
	n.adminAPI(t, "GET", "/api/library", nil, &libraries)
	ids := make([]int, 0, len(libraries))
	for _, library := range libraries {
		ids = append(ids, library.ID)
	}

	var users []struct {
		ID       string `json:"id"`
		UserName string `json:"userName"`
	}
	n.adminAPI(t, "GET", "/api/user", nil, &users)
	for _, user := range users {
		if user.UserName == account.Login {
			n.adminAPI(t, "PUT", "/api/user/"+user.ID+"/library", map[string]any{"libraryIds": ids}, nil)
			return
		}
	}
	t.Fatalf("navidrome has no user %s", account.Login)
}

// UntilGone rescans until the account no longer finds the song in libraryDir.
func (n *Server) UntilGone(t *testing.T, account Account, libraryDir string, title string) {
	t.Helper()

	require.Eventually(t, func() bool {
		_ = n.subsonic("startScan", nil, nil)
		songs, err := n.search(account, libraryDir, title)
		return err == nil && len(songs) == 0
	}, time.Minute, 200*time.Millisecond, "%s still finds %q in %s", account.Login, title, libraryDir)
}

func (n *Server) Scan(t *testing.T) {
	t.Helper()
	require.NoError(t, n.subsonic("startScan", nil, nil))
}
