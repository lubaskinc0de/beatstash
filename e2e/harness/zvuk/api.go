package zvuk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
)

const antiBotCookie = "__ddg1_"

// API is a double of Zvuk's unofficial API: tiny endpoints behind the
// anti-bot redirect, GraphQL for metadata and collections, and a CDN that
// serves the audio the stream links point at.
type API struct {
	server *httptest.Server

	mu        sync.Mutex
	accounts  map[string]*Account
	tracks    map[string]*Track
	releases  map[string]*Release
	playlists map[string]*Playlist
	down      bool
	// guarded makes the anti-bot guard turn every request away.
	guarded bool
	// refused counts the requests answered while down or guarded.
	refused int
	broken  map[string]bool
	// unavailable tracks are left out of playlist pages and come as null.
	unavailable map[string]bool
	downloads   map[string]int
	requests    map[string]int
	hold        chan struct{}
	held        chan struct{}
	holdAfter   int
	// holdOnly limits the hold to these tracks when set.
	holdOnly map[string]bool
	holding  map[string]bool
	events   []Event
}

type Event struct {
	Kind  string
	Track string
	At    time.Time
}

const (
	StreamAsked = "stream"
	AudioServed = "served"
)

type Account struct {
	subscription bool
	// Liked goes from the newest like to the oldest, as Zvuk lists them.
	Liked     []string
	Releases  []string
	Playlists []string
	Artists   []string
}

type Track struct {
	ID       string
	Title    string
	Artists  []string
	Release  string
	Position int
	Seconds  int
	HasFlac  bool
	Audio    Audio
}

type Release struct {
	ID      string
	Title   string
	Artists []string
	Date    string
}

type Playlist struct {
	ID     string
	Title  string
	Tracks []string
}

type Audio struct {
	flac string
	high string
	mid  string
}

func New(t *testing.T) *API {
	api := &API{
		accounts:    map[string]*Account{},
		tracks:      map[string]*Track{},
		releases:    map[string]*Release{},
		playlists:   map[string]*Playlist{},
		broken:      map[string]bool{},
		unavailable: map[string]bool{},
		holding:     map[string]bool{},
		downloads:   map[string]int{},
		requests:    map[string]int{},
	}
	api.server = httptest.NewServer(http.HandlerFunc(api.handle))
	t.Cleanup(api.server.Close)
	return api
}

func (z *API) URL() string {
	return z.server.URL
}

func (z *API) AddAccount(token string, subscription bool) *Account {
	z.mu.Lock()
	defer z.mu.Unlock()
	account := &Account{subscription: subscription}
	z.accounts[token] = account
	return account
}

func (z *API) AddRelease(release Release) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.releases[release.ID] = &release
}

func (z *API) AddTrack(track Track) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.tracks[track.ID] = &track
}

func (z *API) AddPlaylist(playlist Playlist) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.playlists[playlist.ID] = &playlist
}

func (z *API) Update(token string, change func(*Account)) {
	z.mu.Lock()
	defer z.mu.Unlock()
	change(z.accounts[token])
}

func (z *API) UpdatePlaylist(id string, change func(*Playlist)) {
	z.mu.Lock()
	defer z.mu.Unlock()
	change(z.playlists[id])
}

func (z *API) Revoke(token string) {
	z.mu.Lock()
	defer z.mu.Unlock()
	delete(z.accounts, token)
}

func (z *API) SetDown(down bool) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.down = down
}

func (z *API) RefusedRequests() int {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.refused
}

func (z *API) MakeUnavailable(id string) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.unavailable[id] = true
}

func (z *API) BreakStream(id string) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.broken[id] = true
}

func (z *API) SetGuarded(guarded bool) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.guarded = guarded
}

// HoldStreams lets n downloads through, then makes the next ones hang until
// ReleaseStreams; the returned channel closes when the first one hangs.
func (z *API) HoldStreams(n int) <-chan struct{} {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.hold = make(chan struct{})
	z.held = make(chan struct{})
	z.holdAfter = n
	z.holdOnly = nil
	return z.held
}

// HoldStreamsOf holds only these tracks until ReleaseStreams.
func (z *API) HoldStreamsOf(ids ...string) <-chan struct{} {
	held := z.HoldStreams(0)
	z.mu.Lock()
	defer z.mu.Unlock()
	z.holdOnly = map[string]bool{}
	for _, id := range ids {
		z.holdOnly[id] = true
	}
	return held
}

func (z *API) HeldTracks() []string {
	z.mu.Lock()
	defer z.mu.Unlock()
	var ids []string
	for id := range z.holding {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (z *API) ReleaseStreams() {
	z.mu.Lock()
	defer z.mu.Unlock()
	close(z.hold)
	z.hold = nil
}

// RequestsOf counts authorized GraphQL requests of the operation.
func (z *API) RequestsOf(operation string) int {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.requests[operation]
}

func (z *API) EventsOf(kind string) []Event {
	z.mu.Lock()
	defer z.mu.Unlock()
	var events []Event
	for _, e := range z.events {
		if e.Kind == kind {
			events = append(events, e)
		}
	}
	return events
}

func (z *API) record(kind, track string) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.events = append(z.events, Event{Kind: kind, Track: track, At: time.Now()})
}

// DownloadsOf counts the times the CDN served the track's audio in full.
func (z *API) DownloadsOf(id string) int {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.downloads[id]
}

func (z *API) handle(w http.ResponseWriter, r *http.Request) {
	z.mu.Lock()
	down, guarded := z.down, z.guarded
	if down || guarded {
		z.refused++
	}
	z.mu.Unlock()
	if down {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if guarded {
		w.WriteHeader(http.StatusTeapot)
		return
	}
	// Zvuk's anti-bot guard turns away clients that do not look like a browser.
	if !strings.HasPrefix(r.Header.Get("User-Agent"), "Mozilla/") {
		w.WriteHeader(http.StatusTeapot)
		return
	}

	switch {
	case strings.HasPrefix(r.URL.Path, "/api/tiny/"), strings.HasPrefix(r.URL.Path, "/api/v2/tiny/"):
		z.tiny(w, r)
	case r.URL.Path == "/api/v1/graphql":
		z.graphql(w, r)
	case strings.HasPrefix(r.URL.Path, "/cdn/"):
		z.cdn(w, r)
	case strings.HasPrefix(r.URL.Path, "/image/"):
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(Cover)
	default:
		http.NotFound(w, r)
	}
}

// tiny answers only a client that went through the anti-bot redirect and
// came back with its cookie.
func (z *API) tiny(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(antiBotCookie); err != nil {
		http.SetCookie(w, &http.Cookie{Name: antiBotCookie, Value: "passed", Path: "/"}) //nolint:gosec // G124: imitates the anti-bot cookie as is
		http.Redirect(w, r, r.URL.RequestURI(), http.StatusTemporaryRedirect)            //nolint:gosec // G710: redirects to its own request URI
		return
	}
	account, ok := z.account(r)
	if !ok && r.URL.Path == "/api/v2/tiny/profile" {
		writeJSON(w, map[string]any{"result": map[string]any{
			"profile":      map[string]any{"id": 2, "is_anonymous": true},
			"subscription": nil,
		}})
		return
	}
	if !ok {
		http.Error(w, `{"message": "Unauthorized", "error": "401"}`, http.StatusUnauthorized)
		return
	}

	switch r.URL.Path {
	case "/api/v2/tiny/profile":
		var subscription any
		if account.subscription {
			subscription = map[string]any{
				"title": "СберПрайм", "status": "confirmed", "expiration": time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
			}
		}
		writeJSON(w, map[string]any{"result": map[string]any{
			"profile":      map[string]any{"id": 1, "is_registered": true},
			"subscription": subscription,
		}})
	case "/api/tiny/track/stream":
		id, quality := r.URL.Query().Get("id"), r.URL.Query().Get("quality")
		z.mu.Lock()
		track, ok := z.tracks[id]
		z.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		z.record(StreamAsked, id)
		// Zvuk answers a FLAC request for a track without FLAC with 128 kbps.
		if quality == "flac" && !track.HasFlac {
			quality = "mid"
		}
		writeJSON(w, map[string]any{"result": map[string]any{
			"stream": fmt.Sprintf("%s/cdn/%s/%s", z.URL(), id, quality),
		}})
	default:
		http.NotFound(w, r)
	}
}

func (z *API) account(r *http.Request) (*Account, bool) {
	z.mu.Lock()
	defer z.mu.Unlock()
	account, ok := z.accounts[r.Header.Get("X-Auth-Token")]
	return account, ok
}

func (z *API) cdn(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/cdn/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	id, quality := parts[0], parts[1]

	z.mu.Lock()
	track, ok := z.tracks[id]
	broken := z.broken[id]
	hold, held := z.hold, z.held
	if hold != nil {
		switch {
		case z.holdOnly != nil && !z.holdOnly[id]:
			hold = nil
		case z.holdAfter > 0:
			z.holdAfter--
			hold = nil
		default:
			select {
			case <-held:
			default:
				close(held)
			}
		}
	}
	z.mu.Unlock()

	if hold != nil {
		z.mu.Lock()
		z.holding[id] = true
		z.mu.Unlock()
		select {
		case <-hold:
		case <-r.Context().Done():
		}
		z.mu.Lock()
		delete(z.holding, id)
		z.mu.Unlock()
		// A download the bot gave up on while held must not count.
		if r.Context().Err() != nil {
			return
		}
	}
	if !ok || broken {
		http.Error(w, "gone", http.StatusInternalServerError)
		return
	}

	path := map[string]string{"flac": track.Audio.flac, "high": track.Audio.high, "mid": track.Audio.mid}[quality]
	data, err := os.ReadFile(path) //nolint:gosec // G304: paths come from the harness
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(data); err == nil {
		z.mu.Lock()
		z.downloads[id]++
		z.mu.Unlock()
		z.record(AudioServed, id)
	}
}

type graphqlRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func (z *API) graphql(w http.ResponseWriter, r *http.Request) {
	account, ok := z.account(r)
	if !ok {
		http.Error(w, `{"errors":[{"message":"unauthorized"}]}`, http.StatusUnauthorized)
		return
	}
	var req graphqlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	z.mu.Lock()
	defer z.mu.Unlock()
	z.requests[req.OperationName]++

	var data any
	switch req.OperationName {
	case "getTracks":
		tracks := []any{}
		for _, id := range stringsOf(req.Variables["ids"]) {
			if track, ok := z.tracks[id]; ok && !z.unavailable[id] {
				tracks = append(tracks, z.trackJSON(track))
			} else {
				tracks = append(tracks, nil)
			}
		}
		data = map[string]any{"getTracks": tracks}
	case "getReleases":
		releases := []any{}
		for _, id := range stringsOf(req.Variables["ids"]) {
			if release, ok := z.releases[id]; ok {
				releases = append(releases, map[string]any{
					"id": release.ID, "title": release.Title, "tracks": z.idsJSON(z.releaseTracks(id)),
				})
			}
		}
		data = map[string]any{"getReleases": releases}
	case "getPlaylists":
		playlists := []any{}
		for _, id := range stringsOf(req.Variables["ids"]) {
			if playlist, ok := z.playlists[id]; ok {
				playlists = append(playlists, map[string]any{
					"id": playlist.ID, "title": playlist.Title, "trackCount": len(playlist.Tracks),
				})
			}
		}
		data = map[string]any{"getPlaylists": playlists}
	case "getPlaylistTracks":
		playlist, ok := z.playlists[fmt.Sprint(req.Variables["id"])]
		if !ok {
			data = map[string]any{"getPlaylists": []any{}}
			break
		}
		limit, offset := intOf(req.Variables["limit"]), intOf(req.Variables["offset"])
		// A real page never exceeds 100 tracks, whatever the client asks,
		// and leaves out the tracks no longer available.
		limit = min(limit, 100)
		var page []string
		for _, id := range playlist.Tracks[min(offset, len(playlist.Tracks)):min(offset+limit, len(playlist.Tracks))] {
			if !z.unavailable[id] {
				page = append(page, id)
			}
		}
		data = map[string]any{"getPlaylists": []any{map[string]any{
			"id": playlist.ID, "trackCount": len(playlist.Tracks), "tracks": z.idsJSON(page),
		}}}
	case "userCollection":
		data = map[string]any{"collection": map[string]any{
			"tracks":    z.idsJSON(account.Liked),
			"releases":  z.idsJSON(account.Releases),
			"playlists": z.idsJSON(account.Playlists),
			"artists":   z.idsJSON(account.Artists),
		}}
	default:
		writeJSON(w, map[string]any{"errors": []any{map[string]any{"message": "unknown operation " + req.OperationName}}})
		return
	}
	writeJSON(w, map[string]any{"data": data})
}

func (z *API) releaseTracks(releaseID string) []string {
	var tracks []*Track
	for _, track := range z.tracks {
		if track.Release == releaseID {
			tracks = append(tracks, track)
		}
	}
	slices.SortFunc(tracks, func(a, b *Track) int { return a.Position - b.Position })
	ids := make([]string, 0, len(tracks))
	for _, track := range tracks {
		ids = append(ids, track.ID)
	}
	return ids
}

func (z *API) trackJSON(track *Track) map[string]any {
	release := z.releases[track.Release]
	return map[string]any{
		"id":       track.ID,
		"title":    track.Title,
		"duration": track.Seconds,
		"hasFlac":  track.HasFlac,
		"position": track.Position,
		"artists":  titlesJSON(track.Artists),
		"release": map[string]any{
			"id":      release.ID,
			"title":   release.Title,
			"date":    release.Date,
			"artists": titlesJSON(release.Artists),
			"image":   map[string]any{"src": z.URL() + "/image/" + release.ID + "?size={size}"},
		},
	}
}

func (z *API) idsJSON(ids []string) []any {
	result := make([]any, 0, len(ids))
	for _, id := range ids {
		result = append(result, map[string]any{"id": id})
	}
	return result
}

func titlesJSON(titles []string) []any {
	result := make([]any, 0, len(titles))
	for _, title := range titles {
		result = append(result, map[string]any{"title": title})
	}
	return result
}

func stringsOf(v any) []string {
	var result []string
	for _, item := range v.([]any) {
		result = append(result, fmt.Sprint(item))
	}
	return result
}

func intOf(v any) int {
	n, _ := strconv.Atoi(fmt.Sprint(v))
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Cover is a 1x1 picture: all a cover needs to be to get embedded.
var Cover = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
	0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99, 0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// NewAudio generates the files Zvuk serves for a track: FLAC packed
// into MP4 as Zvuk sends it, MP3 320 and MP3 128.
func NewAudio(t *testing.T, seconds float64) Audio {
	t.Helper()

	flac := audiofile.Cached(t, fmt.Sprintf("zvuk-flac|%g", seconds), "flac.mp4", func(path string) error {
		source := fmt.Sprintf("anoisesrc=duration=%g:sample_rate=44100", seconds)
		out, err := exec.Command( //nolint:gosec,noctx // G204: fixture generation with test-controlled args; outlives the test that asked first
			"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", source,
			"-ac", "2", "-c:a", "flac", "-f", "mp4", path,
		).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ffmpeg: %w: %s", err, out)
		}
		return nil
	})

	return Audio{
		flac: flac,
		high: audiofile.Generate(t, "high.mp3", audiofile.Spec{Seconds: seconds, Bitrate: "320k"}),
		mid:  audiofile.Generate(t, "mid.mp3", audiofile.Spec{Seconds: seconds, Bitrate: "128k"}),
	}
}
