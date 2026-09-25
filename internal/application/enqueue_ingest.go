package application

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type IngestRequest struct {
	Ref     domain.TrackRef
	Message MessageRef
}

type Waker interface {
	Wake()
}

type EnqueueIngest struct {
	Queue IngestQueue
	Waker Waker
}

func (i *EnqueueIngest) Execute(ctx context.Context, req IngestRequest) error {
	user, err := CurrentUser(ctx)
	if err != nil {
		return err
	}

	payload := req.Ref.Payload
	if payload == "" {
		payload = "{}"
	}
	job := &domain.IngestJob{
		UserID:    user.ID,
		Provider:  req.Ref.Provider,
		TrackRef:  req.Ref.ID,
		Payload:   payload,
		ChatID:    req.Message.ChatID,
		MessageID: req.Message.MessageID,
		Status:    domain.IngestJobPending,
		RunAt:     time.Now(),
	}
	if err := i.Queue.Enqueue(ctx, job); err != nil {
		return err
	}

	i.Waker.Wake()
	return nil
}
