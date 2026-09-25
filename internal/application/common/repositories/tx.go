package repositories

import "context"

type TxManager interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}
