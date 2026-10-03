// Package listening tells who can get a Listen Link.
package listening

import (
	"context"
	"errors"
	"fmt"

	"github.com/lubaskinc0de/beatstash/internal/application/common/accounts"
	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
)

var (
	ErrListenLinksOff     = errors.New("listen links are off")
	ErrNoPublicAddress    = fmt.Errorf("%w: navidrome has no public address", ErrListenLinksOff)
	ErrNoNavidromeAccount = fmt.Errorf("%w: no navidrome account", ErrListenLinksOff)
)

// ListenLinks are made on behalf of the user's Navidrome Account.
type ListenLinks struct {
	Accounts    *accounts.Navidrome
	AccountRepo repositories.NavidromeAccounts
	// On: Navidrome has an address a listener outside can open.
	On bool
}

func (l *ListenLinks) Available(ctx context.Context, userID uint) (bool, error) {
	err := l.Check(ctx, userID)
	if errors.Is(err, ErrListenLinksOff) {
		return false, nil
	}
	return err == nil, err
}

// Check returns why the user gets no Listen Links.
func (l *ListenLinks) Check(ctx context.Context, userID uint) error {
	if !l.On {
		return ErrNoPublicAddress
	}
	_, err := l.AccountRepo.Get(ctx, userID)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		return ErrNoNavidromeAccount
	}
	return err
}

// Credentials returns ErrListenLinksOff without Listen Links.
func (l *ListenLinks) Credentials(ctx context.Context, userID uint) (navidrome.Credentials, error) {
	if !l.On {
		return navidrome.Credentials{}, ErrNoPublicAddress
	}
	creds, err := l.Accounts.Credentials(ctx, userID)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		return navidrome.Credentials{}, ErrNoNavidromeAccount
	}
	return creds, err
}
