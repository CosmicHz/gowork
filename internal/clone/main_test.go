package clone

import (
	"os"
	"testing"

	"gowork/internal/testnet"
)

func TestMain(m *testing.M) {
	os.Exit(testnet.Block(m))
}
