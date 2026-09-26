// Package library: music files on disk.
//
// Aggregates:
//   - Library. It does not hold its Tracks.
//   - Track. Its TrackSources are part of it, one per Track Ref.
//   - Upload: a record that a User sent a Track.
//
// Two rules apply to a whole Library: no two Duplicates, and one Track per
// Track Ref. Library cannot check them without loading all its Tracks, so
// the application keeps them: it locks the Library, looks up the repository,
// and the database has a unique index.
//
// Value objects: Metadata, Quality, Format, Incoming, Probe, Outcome.
// LayoutPath chooses the file path.
//
// Fields are exported only because gorm needs them. Change an aggregate
// only through its methods.
package library
