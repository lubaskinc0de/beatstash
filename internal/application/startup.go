package application

import "context"

type AdminRegistry interface {
	EnsureExist(ctx context.Context, telegramIDs []uint64) error
}

type Startup struct {
	Admins    AdminRegistry
	AdminIDs  []uint64
	Libraries *Libraries
}

// Execute makes every configured Admin a User before the libraries are
// brought in step, so the Admins get their Personal Libraries too.
func (i *Startup) Execute(ctx context.Context) error {
	if err := i.Admins.EnsureExist(ctx, i.AdminIDs); err != nil {
		return err
	}
	return i.Libraries.Prepare(ctx)
}
