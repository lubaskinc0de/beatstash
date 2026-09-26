// Package sharing: the Shared Library and the Top.
//
// Aggregates:
//   - SharedTrack: a Track in the Shared Library and its Shares. It exists
//     while at least one Share is left. Only the Share that copied the file
//     counts in the Top.
//   - Take: a copy of a SharedTrack in a User's Personal Library. It is
//     credited to the author at the moment of the Take.
//
// The Top is not a domain object: repository queries count Shares and Takes.
//
// Fields are exported only because gorm needs them. Change an aggregate
// only through its methods.
package sharing
