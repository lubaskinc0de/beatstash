package application

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var ErrNavidromeAccountNotFound = errors.New("navidrome account not found")

type NavidromeAccountRepository interface {
	Get(ctx context.Context, userID uint) (*domain.NavidromeAccount, error)
	Save(ctx context.Context, account *domain.NavidromeAccount) error
}

type SecretBox interface {
	Seal(plain string) ([]byte, error)
	Open(sealed []byte) (string, error)
}

// NavidromeAccounts keeps Navidrome Account passwords sealed at rest.
type NavidromeAccounts struct {
	Repo NavidromeAccountRepository
	Box  SecretBox
}

func NewNavidromeAccounts(repo NavidromeAccountRepository, box SecretBox) *NavidromeAccounts {
	return &NavidromeAccounts{Repo: repo, Box: box}
}

func (a *NavidromeAccounts) Credentials(ctx context.Context) (NavidromeCredentials, error) {
	user, ok := UserFromContext(ctx)
	if !ok {
		return NavidromeCredentials{}, ErrNotAuthenticated
	}

	account, err := a.Repo.Get(ctx, user.ID)
	if err != nil {
		return NavidromeCredentials{}, err
	}
	password, err := a.Box.Open(account.Password)
	if err != nil {
		return NavidromeCredentials{}, err
	}
	return NavidromeCredentials{Login: account.Login, Password: password}, nil
}

func (a *NavidromeAccounts) Save(ctx context.Context, userID uint, creds NavidromeCredentials) error {
	sealed, err := a.Box.Seal(creds.Password)
	if err != nil {
		return err
	}
	return a.Repo.Save(ctx, &domain.NavidromeAccount{
		UserID:   userID,
		Login:    creds.Login,
		Password: sealed,
	})
}
