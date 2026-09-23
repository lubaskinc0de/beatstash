package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// wavToFlac packs PCM into FLAC: lossless, smaller and properly taggable.
func wavToFlac(ctx context.Context, wav string) (string, error) {
	flac := strings.TrimSuffix(wav, ".wav") + ".flac"

	out, err := exec.CommandContext(ctx,
		"ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-i", wav, "-map", "0:a", "-map_metadata", "0", "-c:a", "flac", flac,
	).CombinedOutput()
	if err != nil {
		os.Remove(flac)
		return "", fmt.Errorf("ffmpeg: %w: %s", err, out)
	}
	return flac, nil
}
