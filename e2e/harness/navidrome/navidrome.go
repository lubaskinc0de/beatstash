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
	"strconv"
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

// RootLibraryPath is Navidrome's own library, empty in scenarios.
const RootLibraryPath = LibraryMount + "/" + RootLibrary

// RootLibrary keeps Navidrome's own library, which can be neither
// moved nor deleted, away from the scenarios' libraries.
const RootLibrary = ".navidrome-root"

type Server struct {
	URL string
}

func Start(ctx context.Context, libraryRoot string, sharing bool) (*Server, testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Image:        dockerImage,
		ExposedPorts: []string{"4533/tcp"},
		Env: map[string]string{
			"ND_ENABLESHARING":                  strconv.FormatBool(sharing),
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

// startScan scans only the libraries given: Navidrome holds the libraries
// of every scenario, and a scan of them all grows slower with each one.
func (n *Server) startScan(libraryIDs ...int) error {
	if len(libraryIDs) == 0 {
		return nil
	}
	targets := url.Values{}
	for _, id := range libraryIDs {
		targets.Add("target", fmt.Sprintf("%d:", id))
	}
	return n.subsonicAs(adminAccount, "startScan", targets, nil)
}

func (n *Server) scanFor(account Account) error {
	u, err := n.user(account)
	if err != nil {
		return err
	}
	return n.startScan(u.libraryIDs()...)
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, n.URL+"/rest/"+endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
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
		_ = n.scanFor(account)
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

	u, err := n.user(account)
	require.NoError(t, err)
	var paths []string
	for _, library := range u.Libraries {
		paths = append(paths, library.Path)
	}
	slices.Sort(paths)
	return paths
}

type user struct {
	ID        string `json:"id"`
	UserName  string `json:"userName"`
	Libraries []struct {
		ID   int    `json:"id"`
		Path string `json:"path"`
	} `json:"libraries"`
}

func (u *user) libraryIDs() []int {
	ids := make([]int, 0, len(u.Libraries))
	for _, library := range u.Libraries {
		ids = append(ids, library.ID)
	}
	return ids
}

func (n *Server) user(account Account) (*user, error) {
	var users []user
	if err := n.callAdminAPI(http.MethodGet, "/api/user", nil, &users); err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].UserName == account.Login {
			return &users[i], nil
		}
	}
	return nil, fmt.Errorf("navidrome has no user %s", account.Login)
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
	return n.callAPIAs(adminAccount, method, path, body, out)
}

func (n *Server) callAPIAs(account Account, method, path string, body, out any) error {
	token, err := n.login(account)
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
	req, err := http.NewRequestWithContext(context.Background(), method, n.URL+path, reader)
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

// CreateAccount limits the account to Navidrome's root library; otherwise it
// would see the shared libraries used by other scenarios.
func (n *Server) CreateAccount(t *testing.T, prefix string) Account {
	t.Helper()

	account := n.CreateAccountWithDefaults(t, prefix)
	n.setLibraries(t, account, []int{rootLibraryID})
	return account
}

// rootLibraryID is the library Navidrome makes from ND_MUSICFOLDER.
const rootLibraryID = 1

// CreateAccountWithDefaults makes an account that gets the libraries
// Navidrome gives new accounts.
func (n *Server) CreateAccountWithDefaults(t *testing.T, prefix string) Account {
	t.Helper()
	return n.createAccount(t, prefix, false)
}

// CreateAdminAccount makes a Navidrome admin, who sees every library.
func (n *Server) CreateAdminAccount(t *testing.T, prefix string) Account {
	t.Helper()
	return n.createAccount(t, prefix, true)
}

func (n *Server) createAccount(t *testing.T, prefix string, admin bool) Account {
	t.Helper()

	user := Account{Login: UniqueLogin(prefix), Password: "secret-" + prefix}
	token, err := n.login(adminAccount)
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]any{
		"userName": user.Login,
		"name":     user.Login,
		"password": user.Password,
		"isAdmin":  admin,
	})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, n.URL+"/api/user", bytes.NewReader(body))
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
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, n.URL+"/auth/login", bytes.NewReader(body))
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

// GiveToNewAccounts makes Navidrome give the library to every account it
// creates from now on.
func (n *Server) GiveToNewAccounts(t *testing.T, libraryID int) {
	t.Helper()

	var library struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	n.adminAPI(t, http.MethodGet, fmt.Sprintf("/api/library/%d", libraryID), nil, &library)
	n.adminAPI(t, http.MethodPut, fmt.Sprintf("/api/library/%d", libraryID), map[string]any{
		"name": library.Name, "path": library.Path, "defaultNewUsers": true,
	}, nil)
}

// MoveLibrary changes the path of an existing Navidrome library while keeping
// its ID, as happens when an administrator relocates a library on disk.
func (n *Server) MoveLibrary(t *testing.T, libraryID int, path string) {
	t.Helper()

	var library struct {
		Name            string `json:"name"`
		Path            string `json:"path"`
		DefaultNewUsers bool   `json:"defaultNewUsers"`
	}
	n.adminAPI(t, http.MethodGet, fmt.Sprintf("/api/library/%d", libraryID), nil, &library)
	library.Path = path
	n.adminAPI(t, http.MethodPut, fmt.Sprintf("/api/library/%d", libraryID), map[string]any{
		"name": library.Name, "path": library.Path, "defaultNewUsers": library.DefaultNewUsers,
	}, nil)
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

// DeleteLibrariesUnder deletes the libraries whose path lies in dir.
func (n *Server) DeleteLibrariesUnder(t *testing.T, dir string) {
	t.Helper()

	var libraries []struct {
		ID   int    `json:"id"`
		Path string `json:"path"`
	}
	n.adminAPI(t, http.MethodGet, "/api/library", nil, &libraries)
	for _, library := range libraries {
		if library.Path == dir || strings.HasPrefix(library.Path, dir+"/") {
			n.DeleteLibrary(t, library.ID)
		}
	}
}

func (n *Server) setLibraries(t *testing.T, account Account, ids []int) {
	t.Helper()

	u, err := n.user(account)
	require.NoError(t, err)
	n.adminAPI(t, http.MethodPut, "/api/user/"+u.ID+"/library", map[string]any{"libraryIds": ids}, nil)
}

// OpenLibrary adds the library to those the account may see.
func (n *Server) OpenLibrary(t *testing.T, account Account, libraryID int) {
	t.Helper()

	u, err := n.user(account)
	require.NoError(t, err)
	n.setLibraries(t, account, append(u.libraryIDs(), libraryID))
}

// UntilGone rescans until the account no longer finds the song in libraryDir.
func (n *Server) UntilGone(t *testing.T, account Account, libraryDir string, title string) {
	t.Helper()

	require.Eventually(t, func() bool {
		_ = n.scanFor(account)
		songs, err := n.search(account, libraryDir, title)
		return err == nil && len(songs) == 0
	}, time.Minute, 200*time.Millisecond, "%s still finds %q in %s", account.Login, title, libraryDir)
}

// UntilSongs rescans until the library holds exactly count songs.
func (n *Server) UntilSongs(t *testing.T, libraryID int, count int) {
	t.Helper()

	var got int
	require.Eventually(t, func() bool {
		_ = n.startScan(libraryID)
		var songs []struct {
			ID string `json:"id"`
		}
		query := fmt.Sprintf("/api/song?library_id=%d&missing=false&_start=0&_end=1000", libraryID)
		if err := n.callAdminAPI(http.MethodGet, query, nil, &songs); err != nil {
			return false
		}
		got = len(songs)
		return got == count
	}, time.Minute, 200*time.Millisecond, "library %d has %d songs", libraryID, &got)
}

func (n *Server) Scan(t *testing.T, account Account) {
	t.Helper()
	require.NoError(t, n.scanFor(account))
}
