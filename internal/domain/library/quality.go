package library

import "strings"

// Quality is a value object: whether the audio is lossless, and its bitrate.
type Quality struct {
	Lossless bool
	Bitrate  int
}

// LosslessCodec reports whether audio in the codec, or in the container
// that implies one, keeps every sample.
func LosslessCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "flac", "alac", "wav", "pcm", "aiff", "ape", "wavpack", "wv", "dsd", "dsf":
		return true
	}
	return false
}

// Better reports whether q is better than other. Lossless beats lossy.
// Between lossy files the higher bitrate wins. On a tie it returns false.
func (q Quality) Better(other Quality) bool {
	if q.Lossless != other.Lossless {
		return q.Lossless
	}
	if q.Lossless {
		return false
	}
	return q.Bitrate > other.Bitrate
}
