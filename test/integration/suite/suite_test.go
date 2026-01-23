package suite

import (
	"flag"
	"os"
	"testing"

	"github.com/zrs-products/hetero-compute-router/test/integration/kind"
)

var (
	testCluster *kind.TestCluster
	skipKind    = flag.Bool("skip-kind", false, "Skip kind cluster tests")
	keepCluster = flag.Bool("keep-cluster", false, "Keep kind cluster after tests")
)

func TestMain(m *testing.M) {
	flag.Parse()

	if !*skipKind {
		if !kind.IsInstalled() {
			println("kind is not installed, skipping integration tests")
			os.Exit(0)
		}

		testCluster = kind.New(nil)

		if err := testCluster.WaitForReady(); err != nil {
			testCluster.Cleanup()
			panic(err)
		}
	}

	code := m.Run()

	if testCluster != nil && !*keepCluster {
		testCluster.Cleanup()
	}

	os.Exit(code)
}
