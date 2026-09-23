package database

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type txContextKey struct{}
type TxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db}
}

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
	return gorm.Open(
		postgres.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		},
	)
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.User{},
		&domain.Track{},
		&domain.TrackSource{},
		&domain.Upload{},
		&domain.IngestJob{},
		&domain.NavidromeSession{},
		&domain.NavidromeAccount{},
		&domain.Invite{},
	)
}

func Close(db *gorm.DB) error {
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
	db := m.db.WithContext(ctx)
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
