package application

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var (
	ErrNavidromeAccountNotFound = errors.New("navidrome account not found")
	ErrNavidromeAccountTaken    = errors.New("navidrome account is linked to another user")
)

type NavidromeAccountRepository interface {
	Get(ctx context.Context, userID uint) (*domain.NavidromeAccount, error)
	// ByLogin ignores case, as Navidrome logins do.
	ByLogin(ctx context.Context, login string) (*domain.NavidromeAccount, error)
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

func (a *NavidromeAccounts) Credentials(ctx context.Context, userID uint) (NavidromeCredentials, error) {
	account, err := a.Repo.Get(ctx, userID)
	if err != nil {
		return NavidromeCredentials{}, err
	}
	password, err := a.Box.Open(account.Password)
	if err != nil {
		return NavidromeCredentials{}, err
	}
	return NavidromeCredentials{Login: account.Login, Password: password}, nil
}

// CheckFree fails with ErrNavidromeAccountTaken if another user has linked the login.
func (a *NavidromeAccounts) CheckFree(ctx context.Context, userID uint, login string) error {
	linked, err := a.Repo.ByLogin(ctx, login)
	switch {
	case errors.Is(err, ErrNavidromeAccountNotFound):
		return nil
	case err != nil:
		return err
	case linked.UserID != userID:
		return ErrNavidromeAccountTaken
	}
	return nil
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
