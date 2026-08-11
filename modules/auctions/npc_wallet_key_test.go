package auctions

import "testing"

// Wallet balances are persisted under a STABLE id, not the display name.
//
// Keyed by name, renaming a buyer silently reset its balance to the
// compile-time default on the next load: the map lookup simply missed, with no
// error and no warning. Gold quietly reappearing is the kind of economy bug
// nobody reports because it looks like nothing happened.
func TestNpcBuyerIdsAreStableAndUnique(t *testing.T) {
	seen := map[string]string{}
	for _, b := range npcBuyers {
		id := b.Id()
		if id == "" {
			t.Errorf("buyer %q has no Id; its wallet balance cannot be persisted", b.Name())
			continue
		}
		if prev, dup := seen[id]; dup {
			t.Errorf("id %q used by both %q and %q; one would overwrite the other's balance", id, prev, b.Name())
		}
		seen[id] = b.Name()
	}
}

// An id must not merely equal the display name, or the fragility is unchanged.
func TestNpcBuyerIdsAreNotDisplayNames(t *testing.T) {
	for _, b := range npcBuyers {
		if b.Id() == b.Name() {
			t.Errorf("buyer %q uses its display name as its persistence id; renaming it would still orphan its balance", b.Name())
		}
	}
}

// Every buyer holding a wallet must be persistable; a wallet with no id would
// silently never be saved.
func TestEveryWalletBearingBuyerHasAnId(t *testing.T) {
	walletCt := 0
	for _, b := range npcBuyers {
		if b.Wallet() == nil {
			continue
		}
		walletCt++
		if b.Id() == "" {
			t.Errorf("buyer %q holds a wallet but has no id", b.Name())
		}
	}
	if walletCt == 0 {
		t.Fatal("no wallet-bearing buyers found; the fixture or the registry is wrong")
	}
}
