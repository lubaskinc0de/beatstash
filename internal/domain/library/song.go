// Song: a song Navidrome indexed in an Attached Library, which an Attached
// Track follows. Its ID is Navidrome's and may change when the song is
// moved or renamed. Path is relative to the Attached Library; Size is in
// bytes.

package library

// Song is a value object: a song Navidrome indexed in an Attached Library,
// with its tags taken as they are.
type Song struct {
	ID         string
	Path       string
	Metadata   Metadata
	DurationMs int
	Format     Format
	Quality    Quality
	Size       int64
}
