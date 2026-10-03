package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
)

type senderContextKey struct{}

type sender struct {
	from *models.User
	user *access.User
	// recipient: the Poller acts for the user; from has only the id.
	recipient bool
}

func withSender(ctx context.Context, from *models.User) context.Context {
	return context.WithValue(ctx, senderContextKey{}, &sender{from: from})
}

// asRecipient lets the Poller run interactors as the user it writes to.
func asRecipient(ctx context.Context, telegramID int64) context.Context {
	return context.WithValue(ctx, senderContextKey{}, &sender{from: &models.User{ID: telegramID}, recipient: true})
}

func senderFrom(ctx context.Context) (*sender, error) {
	s, ok := ctx.Value(senderContextKey{}).(*sender)
	if !ok {
		return nil, common.ErrNotAuthenticated
	}
	return s, nil
}

const Channel access.Channel = "telegram"

func Identity(telegramID int64) access.Identity {
	return access.Identity{Channel: Channel, ExternalID: strconv.FormatInt(telegramID, 10)}
}

type IDProvider struct {
	Users repositories.Users
	Clock func() time.Time
}

func (p *IDProvider) CurrentUser(ctx context.Context) (*access.User, error) {
	s, err := senderFrom(ctx)
	if err != nil {
		return nil, err
	}
	if s.user != nil {
		return s.user, nil
	}

	user, err := p.Users.GetByIdentity(ctx, Identity(s.from.ID))
	if errors.Is(err, repositories.ErrUserNotFound) {
		return nil, common.ErrNotAuthenticated
	}
	if err != nil {
		return nil, err
	}
	s.user = user
	return user, nil
}

// Profile is false for the Poller: acting for the user is no request of
// theirs.
func (p *IDProvider) Profile(ctx context.Context) (access.Profile, bool) {
	s, err := senderFrom(ctx)
	if err != nil || s.recipient {
		return access.Profile{}, false
	}
	return profileOf(s.from), true
}

func (p *IDProvider) NewUser(ctx context.Context) (*access.User, error) {
	s, err := senderFrom(ctx)
	if err != nil {
		return nil, err
	}
	return access.NewUser(profileOf(s.from), Identity(s.from.ID), p.Clock()), nil
}

func profileOf(from *models.User) access.Profile {
	return access.Profile{Username: from.Username, FirstName: from.FirstName, LastName: from.LastName}
}

// ChatsOf maps the Users to their private chats with the bot, leaving out
// those who never used Telegram.
func (p *IDProvider) ChatsOf(ctx context.Context, userIDs []uint) (map[uint]int64, error) {
	identities, err := p.Users.Identities(ctx, Channel, userIDs)
	if err != nil {
		return nil, err
	}
	chats := make(map[uint]int64, len(identities))
	for _, identity := range identities {
		chatID, err := strconv.ParseInt(identity.ExternalID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("telegram identity of user %d: %w", identity.UserID, err)
		}
		chats[identity.UserID] = chatID
	}
	return chats, nil
}
