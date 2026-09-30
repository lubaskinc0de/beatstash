package oversee_service

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

const usersPerPage = 10

type UserPage struct {
	Users []access.User
	// Page counts from zero.
	Page int
	More bool
}

type UserLister interface {
	// ByLastSeen lists a page of the Users seen last first, the never seen
	// at the end; more tells whether another page follows.
	ByLastSeen(ctx context.Context, page, perPage int) (users []access.User, more bool, err error)
}

type ListUsers struct {
	IDs   common.IDProvider
	Users UserLister
}

func (i *ListUsers) Execute(ctx context.Context, page int) (*UserPage, error) {
	if _, err := common.CurrentAdmin(ctx, i.IDs); err != nil {
		return nil, err
	}
	page = max(page, 0)
	users, more, err := i.Users.ByLastSeen(ctx, page, usersPerPage)
	if err != nil {
		return nil, err
	}
	return &UserPage{Users: users, Page: page, More: more}, nil
}
