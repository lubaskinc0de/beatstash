package libraries

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

// FileChanges keeps the Library's files in step with a transaction: files
// change before the commit, so a rollback takes them back, and old files go
// only after the commit.
type FileChanges struct {
	disk        common.Disk
	onRollback  []func()
	afterCommit []func()
}

func NewFileChanges(disk common.Disk) *FileChanges {
	return &FileChanges{disk: disk}
}

func (c *FileChanges) Place(staged, target string) error {
	if err := c.disk.Place(staged, target); err != nil {
		return err
	}
	c.undo(func() { c.disk.Remove(target) })
	return nil
}

func (c *FileChanges) Link(from, target string) error {
	if err := c.disk.Link(from, target); err != nil {
		return err
	}
	c.undo(func() { c.disk.Remove(target) })
	return nil
}

// Replace puts the staged file at target instead of old. The old file stays
// until the commit, so a failed commit loses nothing.
func (c *FileChanges) Replace(staged, old, target string) error {
	if target != old {
		if err := c.Place(staged, target); err != nil {
			return err
		}
		c.RemoveAfterCommit(old)
		return nil
	}

	// Overwriting old now would lose it on a failed commit, so the new file
	// waits in the scratch directory.
	pending, err := c.disk.Park(staged)
	if err != nil {
		return err
	}
	c.undo(func() { c.disk.Remove(pending) })
	c.afterCommit = append(c.afterCommit, func() { c.disk.Move(pending, target) })
	return nil
}

func (c *FileChanges) RemoveAfterCommit(path string) {
	c.afterCommit = append(c.afterCommit, func() { c.disk.Remove(path) })
}

func (c *FileChanges) undo(step func()) {
	c.onRollback = append(c.onRollback, step)
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
	disk common.Disk,
	libs ManagedLibraries,
	fn func(ctx context.Context, changes *FileChanges) error,
) error {
	changes := NewFileChanges(disk)
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := lock.Lock(ctx, libs.IDs()...); err != nil {
			return err
		}
		return fn(ctx, changes)
	})
	changes.Settle(err)
	return err
}
