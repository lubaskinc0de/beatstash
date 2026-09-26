package library

type Outcome string

const (
	Stored        Outcome = "stored"
	StoredInInbox Outcome = "stored_in_inbox"
	Replaced      Outcome = "replaced"
	AlreadyExists Outcome = "already_exists"
)
