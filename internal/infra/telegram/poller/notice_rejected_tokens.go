package poller

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// AccountNotice is the last invalidation of the user's Provider Account
// the user was told of.
type AccountNotice struct {
	UserID    uint                  `gorm:"primaryKey;autoIncrement:false"`
	Provider  provider.ProviderName `gorm:"primaryKey"`
	NoticedAt time.Time             `gorm:"not null"`
}

func (AccountNotice) TableName() string {
	return "telegram_account_notices"
}

// noticeRejectedTokens tells a user of each invalidation at most once: the
// notice is written down before it is sent.
type noticeRejectedTokens struct {
	db     *gorm.DB
	chats  Chats
	tokens *connect_provider.ListRejectedTokens
}

func (s *noticeRejectedTokens) Run(ctx context.Context) error {
	accounts, err := s.tokens.Execute(ctx)
	if err != nil || len(accounts) == 0 {
		return err
	}
	values := make([]string, 0, len(accounts))
	args := make([]any, 0, 3*len(accounts))
	for _, account := range accounts {
		values = append(values, "(?::bigint, ?::text, ?::timestamptz)")
		args = append(args, account.UserID, account.Provider, *account.InvalidatedAt)
	}
	var noticed []AccountNotice
	err = s.db.WithContext(ctx).Raw(`
		INSERT INTO telegram_account_notices (user_id, provider, noticed_at)
		SELECT * FROM (VALUES `+strings.Join(values, ", ")+`) AS v(user_id, provider, noticed_at)
		ON CONFLICT (user_id, provider) DO UPDATE SET noticed_at = EXCLUDED.noticed_at
		WHERE telegram_account_notices.noticed_at < EXCLUDED.noticed_at
		RETURNING *`,
		args...,
	).Scan(&noticed).Error
	if err != nil || len(noticed) == 0 {
		return err
	}

	userIDs := make([]uint, 0, len(noticed))
	for _, n := range noticed {
		userIDs = append(userIDs, n.UserID)
	}
	chatOf, err := s.chats.ChatsOf(ctx, userIDs)
	if err != nil || len(chatOf) == 0 {
		return err
	}
	recipients, err := s.chats.Recipients(ctx, slices.Collect(maps.Values(chatOf)))
	if err != nil {
		return err
	}
	for _, n := range noticed {
		if chatID, ok := chatOf[n.UserID]; ok {
			s.chats.NoticeRejectedToken(recipients.As(ctx, chatID), chatID, n.Provider)
		}
	}
	return nil
}
