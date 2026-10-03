package e2e

import (
	"os"
	"testing"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
)

func TestMain(m *testing.M) {
	os.Exit(harness.Run(m))
}

var (
	admin    = harness.Admin
	alice    = harness.Alice
	bob      = harness.Bob
	stranger = harness.Stranger
)
