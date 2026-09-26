package library

// Quality is a value object: whether the audio is lossless, and its bitrate.
type Quality struct {
	Lossless bool
	Bitrate  int
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
