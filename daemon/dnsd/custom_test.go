package dnsd

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddressRecord_Shuffled(t *testing.T) {
	rand.Seed(1)
	addr := AddressRecord{
		Addresses: []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"},
	}
	if err := addr.Lint("ip4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	shuffled1 := addr.Shuffled()
	shuffled2 := addr.Shuffled()
	require.NotEqualValues(t, shuffled1, shuffled2)
	require.ElementsMatch(t, shuffled1, shuffled2)
}
