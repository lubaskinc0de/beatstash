package library

// Song is a value object: a song Navidrome indexed in an Attached Library,
// with its tags taken as they are.
type Song struct {
	// ID is the song's id in Navidrome. A song moved or renamed may get a
	// new one.
	ID string
	// Path is relative to the Attached Library.
	Path       string
	Metadata   Metadata
	DurationMs int
	Format     Format
	Quality    Quality
	// Size is the file's, in bytes.
	Size int64
}
