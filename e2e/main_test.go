package e2e

import (
	"os"
	"testing"

	tclog "github.com/testcontainers/testcontainers-go/log"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
)

func TestMain(m *testing.M) {
	tclog.SetDefault(tclog.NewNoopLogger())
	os.Exit(harness.Run(m))
}

var (
	admin    = harness.Admin
	alice    = harness.Alice
	bob      = harness.Bob
	stranger = harness.Stranger
)
