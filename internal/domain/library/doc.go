// Package library: music files on disk.
//
// Aggregates:
//   - Library. It does not hold its Tracks.
//   - Track. Its TrackSources are part of it, one per Track Ref.
//   - Upload: a record that a User sent a Track.
//   - QuotaSettings: the Default Quota and the Shared Library's Quota the
//     Admin set.
//
// Two rules apply to a whole Library: no two Duplicates, and one Track per
// Track Ref. Library cannot check them without loading all its Tracks, so
// the application keeps them: it locks the Library, looks up the repository,
// and the database has a unique index.
//
// A Library is personal, shared, or attached: a Navidrome library the bot
// did not create. Tracks of an Attached Library follow its songs, and the
// bot never writes to its files.
//
// A Library weighs the sum of its Tracks' sizes and may take no more than
// the Quota in force for it. A Track in several Libraries counts in each.
// The Library does not keep its weight: like the Duplicate rule, the
// application checks the Quota with the Library locked and its Tracks
// summed. CapacityOf, a domain service, sums the Quotas of all Libraries.
//
// The files of Personal Libraries and the Shared Library are the truth: a
// file changed by hand changes its Track. SurveyFiles and Survey.Reconcile,
// a domain service, plan that and never touch a file.
//
// Value objects: Metadata, Quality, Format, Incoming, Probe, Outcome, Song,
// Placement, SystemFolders, NavidromeAccess, Quota, Usage, Capacity,
// ServerQuotas, LibraryFile, Survey, Reading, Reconciliation. Grant decides
// which libraries a Navidrome Account gets; KeptLibraries, where a user
// keeps music. LayoutPath chooses the file path.
//
// Fields are exported only because gorm needs them. Change an aggregate
// only through its methods.
package library
