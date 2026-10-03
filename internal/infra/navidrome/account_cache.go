package navidrome

import (
	"context"
	"sync"
	"time"

	appnd "github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
)

// AccountCache keeps what Navidrome answered of an account for TTL, keyed by
// login: the admin asking is always the same. A zero TTL asks every time.
type AccountCache struct {
	Client appnd.Client
	TTL    time.Duration
	Clock  func() time.Time

	mu       sync.Mutex
	accounts map[string]cachedAccount
}

type cachedAccount struct {
	account *appnd.Account
	until   time.Time
}

func (c *AccountCache) Account(ctx context.Context, admin appnd.Credentials, login string) (*appnd.Account, error) {
	c.mu.Lock()
	cached, ok := c.accounts[login]
	c.mu.Unlock()
	if ok && c.Clock().Before(cached.until) {
		return cached.account, nil
	}
	account, err := c.Client.Account(ctx, admin, login)
	if err != nil || c.TTL <= 0 {
		return account, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accounts == nil {
		c.accounts = map[string]cachedAccount{}
	}
	c.accounts[login] = cachedAccount{account: account, until: c.Clock().Add(c.TTL)}
	return account, nil
}
