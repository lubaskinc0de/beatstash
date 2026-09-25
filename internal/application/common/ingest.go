package common

import (
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Waker interface {
	Wake()
}

func NewJob(userID uint, track providers.ListedTrack, msg MessageRef) *domain.IngestJob {
	payload := track.Ref.Payload
	if payload == "" {
		payload = "{}"
	}
	return &domain.IngestJob{
		UserID:      userID,
		Provider:    track.Ref.Provider,
		TrackRef:    track.Ref.ID,
		Payload:     payload,
		DisplayName: track.DisplayName,
		ChatID:      msg.ChatID,
		MessageID:   msg.MessageID,
		Status:      domain.IngestJobPending,
		RunAt:       time.Now(),
	}
}
