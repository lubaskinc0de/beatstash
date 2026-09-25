package audio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FFmpeg struct{}

func (FFmpeg) WavToFlac(ctx context.Context, wav string) (string, error) {
	return toFlac(ctx, wav, "flac")
}

func (FFmpeg) UnpackFlac(ctx context.Context, mp4 string) (string, error) {
	return toFlac(ctx, mp4, "copy")
}

func toFlac(ctx context.Context, path, codec string) (string, error) {
	flac := strings.TrimSuffix(path, filepath.Ext(path)) + ".flac"

	out, err := exec.CommandContext(ctx,
		"ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-i", path, "-map", "0:a", "-map_metadata", "0", "-c:a", codec, flac,
	).CombinedOutput()
	if err != nil {
		_ = os.Remove(flac)
		return "", fmt.Errorf("ffmpeg: %w: %s", err, out)
	}
	return flac, nil
}

func (FFmpeg) Codec(ctx context.Context, path string) string {
	out, err := exec.CommandContext(ctx,
		"ffprobe", "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", path,
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
