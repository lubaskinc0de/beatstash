package database

import (
	"context"
	"errors"
	"log"
	"os"
	"sync/atomic"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/navidrome"
)

type (
	txContextKey struct{}
	TxManager    struct {
		DB *gorm.DB
	}
)

func New(dsn string) (*gorm.DB, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(
		postgres.Open(dsn), &gorm.Config{
			Logger: &quietOnShutdown{Interface: gormLogger},
		},
	)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)
	return db, nil
}

// maxIdleConns keeps a connection for each worker polling the queue: with
// database/sql's default of two, the others would open a new one, and
// authenticate it, on every poll.
const maxIdleConns = 32

var models = []any{
	&access.User{},
	&access.Identity{},
	&library.Library{},
	&library.Track{},
	&library.TrackSource{},
	&library.Upload{},
	&library.QuotaSettings{},
	&sharing.Share{},
	&sharing.Take{},
	&ingest.IngestBatch{},
	&ingest.IngestJob{},
	&navidrome.Session{},
	&access.NavidromeAccount{},
	&provider.ProviderAccount{},
	&access.Invite{},
}

func Close(db *gorm.DB) error {
	if quiet, ok := db.Logger.(*quietOnShutdown); ok {
		quiet.closed.Store(true)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (m *TxManager) WithinTx(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	db := m.DB.WithContext(ctx)
	if tx, ok := txFromContext(ctx); ok {
		db = tx
	}

	// gorm turns a transaction inside a transaction into a savepoint.
	return db.Transaction(func(tx *gorm.DB) error {
		txCtx := withTx(ctx, tx)

		return fn(txCtx)
	})
}

func dbForContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}

	return db.WithContext(ctx)
}

func withTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

func txFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	return tx, ok
}

func first[T any](q *gorm.DB, notFound error) (*T, error) {
	var row T
	err := q.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, notFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// orderBy is an ORDER BY with arguments. gorm ignores an Order given a bare
// gorm.Expr, and a later Order drops the expression, so the whole ORDER BY
// goes in one.
func orderBy(sql string, args ...any) clause.OrderBy {
	return clause.OrderBy{Expression: gorm.Expr(sql, args...)}
}

// quietOnShutdown leaves out the queries a shutdown fails: those cut short
// by a cancelled context, and those that reach the database after Close.
type quietOnShutdown struct {
	logger.Interface
	closed atomic.Bool
}

func (l *quietOnShutdown) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if errors.Is(err, context.Canceled) || l.closed.Load() {
		return
	}
	l.Interface.Trace(ctx, begin, fc, err)
}
