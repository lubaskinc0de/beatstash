package repositories

import "context"

type TxManager interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
	// WithinSnapshot runs fn, which only reads, on one consistent view of
	// the data: a transaction that commits meanwhile is seen whole or not
	// at all, across all of fn's reads.
	WithinSnapshot(ctx context.Context, fn func(context.Context) error) error
}
