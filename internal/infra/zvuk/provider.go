package zvuk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common/providers"
	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

const Name provider.ProviderName = "zvuk"

type Tokens interface {
	Token(ctx context.Context, userID uint, providerName provider.ProviderName) (string, error)
}

type Provider struct {
	Client *Client
	Tokens Tokens
	Pacer  *Pacer
}

func (p *Provider) Name() provider.ProviderName {
	return Name
}

func (p *Provider) CheckToken(ctx context.Context, token string) error {
	profile, err := p.Client.profile(ctx, token)
	if err != nil {
		return err
	}
	if !profile.Profile.Registered {
		return providers.ErrUnauthorized
	}
	sub := profile.Subscription
	if sub == nil || sub.Status != "confirmed" || time.UnixMilli(sub.Expiration).Before(time.Now()) {
		return providers.ErrNoSubscription
	}
	return nil
}

// Fetch waits out the pause before its first request, so the account's
// API calls are paced as well as its downloads.
func (p *Provider) Fetch(ctx context.Context, userID uint, ref provider.TrackRef) (*providers.FetchedAudio, error) {
	token, err := p.Tokens.Token(ctx, userID, Name)
	if err != nil {
		return nil, err
	}
	rest, err := p.Pacer.Take(ctx, userID)
	if err != nil {
		return nil, err
	}
	audio, err := p.fetch(ctx, token, ref)
	if err != nil {
		rest()
		return nil, err
	}
	audio.Body = &pacedBody{ReadCloser: audio.Body, rest: rest}
	return audio, nil
}

func (p *Provider) fetch(ctx context.Context, token string, ref provider.TrackRef) (*providers.FetchedAudio, error) {
	tracks, err := p.Client.tracks(ctx, token, []string{ref.ID})
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, providers.Permanent(ingest.ReasonFetchFailed, fmt.Errorf("zvuk has no track %s", ref.ID))
	}
	t := &tracks[0]

	link, err := p.Client.streamURL(ctx, token, t)
	if err != nil {
		return nil, err
	}
	body, err := p.Client.download(ctx, link)
	if err != nil {
		return nil, err
	}

	// FLAC comes packed into MP4; Ingest repacks it.
	format := library.FormatMP3
	if t.HasFlac {
		format = library.FormatM4A
	}
	return &providers.FetchedAudio{
		Body:     body,
		FileName: displayName(t) + format.Ext(),
		Format:   format,
		Hint:     hint(t),
		Cover:    p.cover(ctx, t),
	}, nil
}

// Describe asks for the track's metadata only, which needs no pause: the
// pause guards downloads.
func (p *Provider) Describe(
	ctx context.Context, userID uint, refs []provider.TrackRef,
) (map[string]*providers.Description, error) {
	token, err := p.Tokens.Token(ctx, userID, Name)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	tracks, err := p.Client.tracks(ctx, token, ids)
	if err != nil {
		return nil, err
	}
	described := make(map[string]*providers.Description, len(tracks))
	for n := range tracks {
		t := &tracks[n]
		described[t.ID] = &providers.Description{Metadata: hint(t), DurationMs: t.Duration * 1000}
	}
	return described, nil
}

// cover is a nicety: a track without one is still worth storing.
func (p *Provider) cover(ctx context.Context, t *track) []byte {
	src := t.Release.Image.Src
	if src == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	body, err := p.Client.download(ctx, strings.ReplaceAll(src, "{size}", "600x600"))
	if err != nil {
		slog.Warn("zvuk_cover", "track", t.ID, "error", err)
		return nil
	}
	defer body.Close()
	image, err := io.ReadAll(body)
	if err != nil {
		slog.Warn("zvuk_cover", "track", t.ID, "error", err)
		return nil
	}
	return image
}

func hint(t *track) library.Metadata {
	return library.Metadata{
		AlbumArtist: joinTitles(t.Release.Artists),
		Artist:      joinTitles(t.Artists),
		Album:       t.Release.Title,
		Title:       t.Title,
		Year:        year(t.Release.Date),
		TrackNumber: t.Position,
		Genres:      genres(t),
		Label:       label(t),
	}
}

// genres are the track's own, else its release's. The slice is never nil:
// Zvuk has been asked.
func genres(t *track) []string {
	names := t.Genres
	if len(names) == 0 {
		names = t.Release.Genres
	}
	found := make([]string, 0, len(names))
	for _, genre := range names {
		found = append(found, genre.Name)
	}
	return found
}

func label(t *track) string {
	if t.Release.Label == nil {
		return ""
	}
	return t.Release.Label.Title
}

func joinTitles(items []titled) string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return strings.Join(titles, ", ")
}

// year reads "2021-05-14T00:00:00" as 2021.
func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func displayName(t *track) string {
	return joinTitles(t.Artists) + " — " + t.Title
}

// Bitrates estimate the download: FLAC 16/44.1 averages around 900 kbps.
const (
	flacBitrate = 900_000
	mp3Bitrate  = 320_000
)

func listed(t *track) providers.ListedTrack {
	bitrate := mp3Bitrate
	if t.HasFlac {
		bitrate = flacBitrate
	}
	return providers.ListedTrack{
		Ref:         provider.TrackRef{Provider: Name, ID: t.ID},
		DisplayName: displayName(t),
		Bytes:       int64(t.Duration) * int64(bitrate) / 8,
	}
}

func (p *Provider) Collection(ctx context.Context, userID uint) (*providers.Collection, error) {
	token, err := p.Tokens.Token(ctx, userID, Name)
	if err != nil {
		return nil, err
	}
	saved, err := p.Client.collection(ctx, token)
	if err != nil {
		return nil, err
	}

	result := providers.Collection{LikedRefs: ids(saved.Tracks)}
	if result.Liked, err = p.listedTracks(ctx, token, result.LikedRefs); err != nil {
		return nil, err
	}

	releases, err := p.Client.releases(ctx, token, ids(saved.Releases))
	if err != nil {
		return nil, err
	}
	var albumTracks []string
	for _, r := range releases {
		albumTracks = append(albumTracks, ids(r.Tracks)...)
	}
	if result.AlbumTracks, err = p.listedTracks(ctx, token, albumTracks); err != nil {
		return nil, err
	}

	playlists, err := p.Client.playlists(ctx, token, ids(saved.Playlists))
	if err != nil {
		return nil, err
	}
	for _, pl := range playlists {
		trackIDs, err := p.Client.playlistTracks(ctx, token, pl.ID)
		if err != nil {
			return nil, err
		}
		tracks, err := p.listedTracks(ctx, token, trackIDs)
		if err != nil {
			return nil, err
		}
		result.Playlists = append(result.Playlists, providers.ListedPlaylist{ID: pl.ID, Title: pl.Title, Tracks: tracks})
	}
	return &result, nil
}

func (p *Provider) listedTracks(ctx context.Context, token string, trackIDs []string) ([]providers.ListedTrack, error) {
	tracks, err := p.Client.tracks(ctx, token, trackIDs)
	if err != nil {
		return nil, err
	}
	result := make([]providers.ListedTrack, 0, len(tracks))
	for i := range tracks {
		result = append(result, listed(&tracks[i]))
	}
	return result, nil
}

// Pacer gives an account PerUser download slots. A slot rests for a random
// pause from [Min, Max] after each download, so the account does not look
// like a bot; accounts do not wait for each other.
type Pacer struct {
	Min, Max time.Duration
	PerUser  int

	mu    sync.Mutex
	slots map[uint][]paceSlot
}

type paceSlot struct {
	busy bool
	next time.Time
}

// Take waits for the account's slot that rests the shortest and returns
// the func that ends the download. The queue never runs more downloads of
// an account than it has slots.
func (p *Pacer) Take(ctx context.Context, userID uint) (rest func(), err error) {
	p.mu.Lock()
	if p.slots == nil {
		p.slots = map[uint][]paceSlot{}
	}
	slots, ok := p.slots[userID]
	if !ok {
		slots = make([]paceSlot, max(p.PerUser, 1))
		p.slots[userID] = slots
	}
	i := -1
	for j := range slots {
		if !slots[j].busy && (i < 0 || slots[j].next.Before(slots[i].next)) {
			i = j
		}
	}
	if i < 0 {
		p.mu.Unlock()
		return nil, errors.New("zvuk: no free download slot")
	}
	slots[i].busy = true
	wait := time.Until(slots[i].next)
	p.mu.Unlock()

	free := func(pause time.Duration) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.slots[userID][i] = paceSlot{next: time.Now().Add(pause)}
	}
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			p.mu.Lock()
			p.slots[userID][i].busy = false
			p.mu.Unlock()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return func() { free(p.pause()) }, nil
}

func (p *Pacer) pause() time.Duration {
	pause := p.Min
	if p.Max > p.Min {
		pause += rand.N(p.Max - p.Min) //nolint:gosec // G404: jitter needs no crypto randomness
	}
	return pause
}

// pacedBody ends the download when Ingest has read and closed it.
type pacedBody struct {
	io.ReadCloser
	rest func()
	once sync.Once
}

func (b *pacedBody) Close() error {
	b.once.Do(b.rest)
	return b.ReadCloser.Close()
}
