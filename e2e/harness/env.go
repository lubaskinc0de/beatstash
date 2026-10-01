package harness

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/database"
)

// templateDatabase has the schema: migrating anew for each scenario would
// take a good part of its time.
const templateDatabase = "scenario_template"

var env struct {
	postgresHost string
	postgresPort string
	libraryRoot  string
	navidrome    *navidrome.Server
}

func Run(m *testing.M) int {
	ctx := context.Background()
	defer audiofile.RemoveCache()

	libraryRoot, err := os.MkdirTemp("", "navidrome-tg-library-")
	if err != nil {
		log.Printf("create library root: %v", err)
		return 1
	}
	defer os.RemoveAll(libraryRoot)
	if err := os.Chmod(libraryRoot, 0o755); err != nil { //nolint:gosec // G302: Navidrome container reads the library
		log.Printf("chmod library root: %v", err)
		return 1
	}
	env.libraryRoot = libraryRoot
	if err := os.Mkdir(filepath.Join(libraryRoot, navidrome.RootLibrary), 0o755); err != nil { //nolint:gosec // G301: Navidrome container reads the library
		log.Printf("create navidrome root library: %v", err)
		return 1
	}

	pg, err := postgres.Run(ctx, "postgres:17",
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithDatabase("postgres"),
		// SCRAM costs both sides a key derivation on every connection, and
		// each scenario opens many.
		testcontainers.WithEnv(map[string]string{"POSTGRES_HOST_AUTH_METHOD": "trust"}),
		postgres.BasicWaitStrategies(),
	)
	defer func() { _ = testcontainers.TerminateContainer(pg) }()
	if err != nil {
		log.Printf("start postgres: %v", err)
		return 1
	}
	if env.postgresHost, err = pg.Host(ctx); err != nil {
		log.Printf("postgres host: %v", err)
		return 1
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		log.Printf("postgres port: %v", err)
		return 1
	}
	env.postgresPort = port.Port()
	if err := migrateTemplate(ctx); err != nil {
		log.Printf("migrate template database: %v", err)
		return 1
	}

	nd, container, err := navidrome.Start(ctx, libraryRoot)
	defer func() { _ = testcontainers.TerminateContainer(container) }()
	if err != nil {
		log.Printf("start navidrome: %v", err)
		return 1
	}
	env.navidrome = nd

	return m.Run()
}

func Navidrome() *navidrome.Server {
	return env.navidrome
}

func postgresDSN(database string) string {
	return fmt.Sprintf(
		"postgres://postgres:postgres@%s:%s/%s?sslmode=disable",
		env.postgresHost, env.postgresPort, database,
	)
}

func migrateTemplate(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, postgresDSN("postgres"))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+templateDatabase); err != nil {
		return err
	}

	db, err := database.New(postgresDSN(templateDatabase))
	if err != nil {
		return err
	}
	// A template must have no connections while databases are made from it.
	defer func() { _ = database.Close(db) }()
	return database.Migrate(db)
}

var databaseSeq, newcomerSeq atomic.Int64

// NewDatabase creates a database with the schema and returns its DSN.
func NewDatabase(t *testing.T) string {
	t.Helper()

	name := fmt.Sprintf("scenario_%d", databaseSeq.Add(1))

	conn, err := pgx.Connect(t.Context(), postgresDSN("postgres"))
	require.NoError(t, err)
	defer conn.Close(context.Background())

	_, err = conn.Exec(t.Context(), "CREATE DATABASE "+name+" TEMPLATE "+templateDatabase)
	require.NoError(t, err)
	return postgresDSN(name)
}
