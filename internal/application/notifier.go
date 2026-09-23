package application

import "context"

type MessageRef struct {
	ChatID    int64
	MessageID int
}

type IngestOutcome int

const (
	IngestStored IngestOutcome = iota
	IngestStoredInInbox
	IngestAlreadyExists
	IngestReplaced
)

type IngestNotifier interface {
	Ingested(ctx context.Context, msg MessageRef, outcome IngestOutcome)
	IngestFailed(ctx context.Context, msg MessageRef, reason FailureReason)
}
