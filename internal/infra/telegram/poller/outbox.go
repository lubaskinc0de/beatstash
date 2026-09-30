package poller

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// outbox is a table of rows waiting to be told to their chats. Each is
// told at least once: a row goes only after it is told, and a crash leaves
// it to another instance once its lease runs out.
type outbox struct {
	db       *gorm.DB
	table    string
	key      string
	leaseTTL time.Duration
}

const unclaimed = "(claimed_until IS NULL OR claimed_until < now())"

func (o outbox) waiting(ctx context.Context, rows any) error {
	return o.db.WithContext(ctx).Table(o.table).Where(unclaimed).Order(o.key).Find(rows).Error
}

func (o outbox) forget(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return o.db.WithContext(ctx).Exec("DELETE FROM "+o.table+" WHERE "+o.key+" IN ? AND "+unclaimed, ids).Error
}

// claim leases the unclaimed rows among ids into rows. One statement
// leases them all, so they share the lease's end: the token to drop them.
func (o outbox) claim(ctx context.Context, ids []uint, rows any) error {
	return o.db.WithContext(ctx).Raw(
		"UPDATE "+o.table+" SET claimed_until = now() + ? * interval '1 millisecond' WHERE "+o.key+" IN ? AND "+unclaimed+" RETURNING *",
		o.leaseTTL.Milliseconds(), ids,
	).Scan(rows).Error
}

func (o outbox) told(ctx context.Context, ids []uint, token time.Time) error {
	return o.db.WithContext(ctx).Exec("DELETE FROM "+o.table+" WHERE "+o.key+" IN ? AND claimed_until = ?", ids, token).Error
}
