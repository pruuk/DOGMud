package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func pinRestockNarration(t *testing.T, rounds int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ShopRestockNarrationRounds = configs.ConfigInt(rounds)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(resetRestockNarrationForTest())
}

// #725: a shop announces a delivery at most once per window; restocks
// inside it are silent.
func TestRestockNarrationDue_OncePerWindow(t *testing.T) {
	pinRestockNarration(t, 225)
	if !restockNarrationDue(7, 1000) {
		t.Fatal("first restock must be narrated")
	}
	if restockNarrationDue(7, 1224) {
		t.Fatal("a restock inside the window must be silent")
	}
	if !restockNarrationDue(8, 1100) {
		t.Fatal("another shop keeps its own window")
	}
	if !restockNarrationDue(7, 1225) {
		t.Fatal("the window has passed, the restock must be narrated")
	}
}

func TestRestockNarrationDue_ZeroNarratesEveryRestock(t *testing.T) {
	pinRestockNarration(t, 0)
	for _, round := range []uint64{1000, 1001, 1002} {
		if !restockNarrationDue(7, round) {
			t.Fatalf("round %d: window 0 must narrate every restock", round)
		}
	}
}
