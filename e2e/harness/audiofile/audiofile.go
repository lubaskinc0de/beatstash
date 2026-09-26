package audiofile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
// picks the format.
func Generate(t *testing.T, name string, spec Spec) string {
	t.Helper()

	seconds := spec.Seconds
	if seconds == 0 {
		seconds = 1.5
	}
	source := fmt.Sprintf("sine=frequency=440:duration=%g", seconds)
	if spec.Noise {
		source = fmt.Sprintf("anoisesrc=duration=%g:sample_rate=44100", seconds)
	}

	path := filepath.Join(t.TempDir(), name)
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", source, "-map_metadata", "-1"}
	if spec.Noise {
		args = append(args, "-ac", "2")
	}
	for key, value := range spec.Tags {
		args = append(args, "-metadata", key+"="+value)
	}
	codec, ok := ffmpegCodecs[filepath.Ext(name)]
	require.True(t, ok, "no codec for %s", name)
	args = append(args, codec...)
	if spec.Bitrate != "" {
		args = append(args, "-b:a", spec.Bitrate)
	}
	args = append(args, path)

	out, err := exec.CommandContext(t.Context(), "ffmpeg", args...).CombinedOutput() //nolint:gosec // G204: fixture generation with test-controlled args
	require.NoError(t, err, string(out))
	return path
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
