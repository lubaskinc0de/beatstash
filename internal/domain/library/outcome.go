// Outcome: how storing incoming audio ended.

package library

// Outcome is a value object: the result of storing incoming audio.
type Outcome string

const (
	Stored        Outcome = "stored"
	StoredInInbox Outcome = "stored_in_inbox"
	Replaced      Outcome = "replaced"
	AlreadyExists Outcome = "already_exists"
)
