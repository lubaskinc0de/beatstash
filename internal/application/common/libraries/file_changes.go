package libraries

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

// FileChanges holds what makes the Library follow a transaction: files
// change before the commit, so a rollback has to take them back, and old
// files go only after the commit.
type FileChanges struct {
	onRollback  []func()
	afterCommit []func()
}

func (c *FileChanges) OnRollback(undo func()) {
	c.onRollback = append(c.onRollback, undo)
}

func (c *FileChanges) AfterCommit(finish func()) {
	c.afterCommit = append(c.afterCommit, finish)
}

// Settle takes the changes back if the transaction failed with err, and
// finishes them otherwise.
func (c *FileChanges) Settle(err error) {
	steps := c.afterCommit
	if err != nil {
		steps = c.onRollback
	}
	for _, step := range steps {
		step()
	}
	c.onRollback, c.afterCommit = nil, nil
}

// Within runs fn in a transaction holding both user libraries.
func Within(
	ctx context.Context,
	tx repositories.TxManager,
	lock repositories.LibraryLock,
	libs UserLibraries,
	fn func(ctx context.Context, changes *FileChanges) error,
) error {
	var changes FileChanges
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := lock.Lock(ctx, libs.IDs()...); err != nil {
			return err
		}
		return fn(ctx, &changes)
	})
	changes.Settle(err)
	return err
}
