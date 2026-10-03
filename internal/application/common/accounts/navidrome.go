package accounts

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
)

// Navidrome keeps Navidrome Account passwords sealed at rest.
type Navidrome struct {
	Repo repositories.NavidromeAccounts
	Box  common.SecretBox
}

func (a *Navidrome) Credentials(ctx context.Context, userID uint) (navidrome.Credentials, error) {
	account, err := a.Repo.Get(ctx, userID)
	if err != nil {
		return navidrome.Credentials{}, err
	}
	password, err := a.Box.Open(account.Password)
	if err != nil {
		return navidrome.Credentials{}, err
	}
	return navidrome.Credentials{Login: account.Login, Password: password}, nil
}

func (a *Navidrome) Save(ctx context.Context, userID uint, creds navidrome.Credentials) error {
	sealed, err := a.Box.Seal(creds.Password)
	if err != nil {
		return err
	}
	return a.Repo.Save(ctx, &access.NavidromeAccount{
		UserID:   userID,
		Login:    creds.Login,
		Password: sealed,
	})
}
