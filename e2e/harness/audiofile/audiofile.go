package audiofile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/taglib"
)

type Spec struct {
	// Seconds defaults to 1.5.
	Seconds float64
	// Bitrate applies to lossy formats, e.g. "320k".
	Bitrate string
	// Noise makes lossless files big: noise barely compresses.
	Noise bool
	// Tags are ffmpeg metadata keys: artist, album_artist, album, title, track, date.
	Tags map[string]string
}

var ffmpegCodecs = map[string][]string{
	".mp3":  {"-c:a", "libmp3lame", "-id3v2_version", "3"},
	".flac": {"-c:a", "flac"},
	".m4a":  {"-c:a", "aac"},
	".ogg":  {"-c:a", "libvorbis"},
	".opus": {"-c:a", "libopus"},
	".wav":  {"-c:a", "pcm_s16le"},
}

// Generate generates an audio file with ffmpeg; the extension of name
// picks the format. Files of the same Spec are generated once per run.
func Generate(t *testing.T, name string, spec Spec) string {
	t.Helper()

	seconds := spec.Seconds
	if seconds == 0 {
		seconds = 1.5
	}
	codec, ok := ffmpegCodecs[filepath.Ext(name)]
	require.True(t, ok, "no codec for %s", name)

	tags := make([]string, 0, len(spec.Tags))
	for key, value := range spec.Tags {
		tags = append(tags, key+"="+value)
	}
	slices.Sort(tags)
	key := fmt.Sprintf("%s|%g|%s|%t|%q", filepath.Ext(name), seconds, spec.Bitrate, spec.Noise, tags)

	return Cached(t, key, name, func(path string) error {
		source := fmt.Sprintf("sine=frequency=440:duration=%g", seconds)
		if spec.Noise {
			source = fmt.Sprintf("anoisesrc=duration=%g:sample_rate=44100", seconds)
		}
		args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", source, "-map_metadata", "-1"}
		if spec.Noise {
			args = append(args, "-ac", "2")
		}
		for _, tag := range tags {
			args = append(args, "-metadata", tag)
		}
		args = append(args, codec...)
		if spec.Bitrate != "" {
			args = append(args, "-b:a", spec.Bitrate)
		}
		args = append(args, path)

		out, err := exec.Command("ffmpeg", args...).CombinedOutput() //nolint:gosec,noctx // G204: fixture generation with test-controlled args; outlives the test that asked first
		if err != nil {
			return fmt.Errorf("ffmpeg: %w: %s", err, out)
		}
		return nil
	})
}

// cache keeps generated files for the whole run: ffmpeg would otherwise
// make the same fixtures again in every scenario.
var cache struct {
	mu      sync.Mutex
	dir     string
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	once sync.Once
	path string
	err  error
}

// Cached returns a copy, named name, of the file generate makes for key;
// the copy is the test's own to change.
func Cached(t *testing.T, key, name string, generate func(path string) error) string {
	t.Helper()

	entry, err := cacheEntryFor(key, filepath.Ext(name))
	require.NoError(t, err)
	entry.once.Do(func() { entry.err = generate(entry.path) })
	require.NoError(t, entry.err)

	data, err := os.ReadFile(entry.path)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o644)) //nolint:gosec // G306: Navidrome container reads fixtures
	return path
}

func cacheEntryFor(key, ext string) (*cacheEntry, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.dir == "" {
		dir, err := os.MkdirTemp("", "beatstash-fixtures-")
		if err != nil {
			return nil, err
		}
		cache.dir = dir
		cache.entries = map[string]*cacheEntry{}
	}
	entry, ok := cache.entries[key]
	if !ok {
		entry = &cacheEntry{path: filepath.Join(cache.dir, fmt.Sprintf("%d%s", len(cache.entries), ext))}
		cache.entries[key] = entry
	}
	return entry, nil
}

// RemoveCache deletes the generated files once the run is over.
func RemoveCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.dir != "" {
		_ = os.RemoveAll(cache.dir)
	}
}

func WriteFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, content, 0o644)) //nolint:gosec // G306: Navidrome container reads fixtures
	return path
}

func Tags(t *testing.T, path string) map[string]string {
	t.Helper()

	tags, err := taglib.ReadTags(path)
	require.NoError(t, err)

	flat := map[string]string{}
	for key, values := range tags {
		flat[key] = values[0]
	}
	return flat
}

// Retag sets taglib's keys, such as ARTIST or TITLE, in the file.
func Retag(t *testing.T, path string, tags map[string]string) {
	t.Helper()

	values := make(map[string][]string, len(tags))
	for key, value := range tags {
		values[key] = []string{value}
	}
	require.NoError(t, taglib.WriteTags(path, values, 0))
}

func Codec(t *testing.T, path string) (codec string, seconds float64) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), //nolint:gosec // G204: fixture generation with test-controlled args
		"ffprobe", "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=codec_name:format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path,
	).Output()
	require.NoError(t, err)

	_, err = fmt.Sscan(string(out), &codec, &seconds)
	require.NoError(t, err, string(out))
	return codec, seconds
}

func BitrateKbps(t *testing.T, path string) int {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), //nolint:gosec // G204: fixture generation with test-controlled args
		"ffprobe", "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=bit_rate", "-of", "default=noprint_wrappers=1:nokey=1", path,
	).Output()
	require.NoError(t, err)

	var bps int
	_, err = fmt.Sscan(string(out), &bps)
	require.NoError(t, err, string(out))
	return bps / 1000
}

var SongTags = map[string]string{"artist": "Artist", "album": "Album", "track": "1", "title": "Dup Song"}

const (
	FixtureTitle     = "Fixture Song"
	FixtureTrackPath = "Fixture Artist/Fixture Album/01 - Fixture Song.mp3"
)

func Fixture(name string) string {
	return filepath.Join("testdata", "audio", name)
}
