package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	navidromeAdmin    = "admin"
	navidromePassword = "admin"
)

var env struct {
	postgresHost string
	postgresPort string
	libraryRoot  string
	navidrome    *navidrome
}

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	libraryRoot, err := os.MkdirTemp("", "navidrome-tg-library-")
	if err != nil {
		log.Printf("create library root: %v", err)
		return 1
	}
	defer os.RemoveAll(libraryRoot)
	if err := os.Chmod(libraryRoot, 0o755); err != nil {
		log.Printf("chmod library root: %v", err)
		return 1
	}
	env.libraryRoot = libraryRoot

	pg, err := postgres.Run(ctx, "postgres:17",
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithDatabase("postgres"),
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

	nd, container, err := startNavidrome(ctx, libraryRoot)
	defer func() { _ = testcontainers.TerminateContainer(container) }()
	if err != nil {
		log.Printf("start navidrome: %v", err)
		return 1
	}
	env.navidrome = nd

	return m.Run()
}

func postgresDSN(database string) string {
	return fmt.Sprintf(
		"postgres://postgres:postgres@%s:%s/%s?sslmode=disable",
		env.postgresHost, env.postgresPort, database,
	)
}
