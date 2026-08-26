package users

import "github.com/GoMudEngine/GoMud/internal/items"

// MigrateStorageSlots converts the legacy Storage.Items flat list into the
// new Storage.Slots shape. It is idempotent: if Items is already empty (the
// migration has run before), it returns false without touching Slots.
//
// Called from LoadUser after YAML deserialisation, alongside the character
// migration calls.
//
// Returns true if any data was migrated (caller should mark user dirty).
func (s *Storage) MigrateStorageSlots() bool {
	if len(s.Items) == 0 {
		return false
	}

	for _, itm := range s.Items {
		s.AddItem(itm) // AddItem handles stack folding
	}

	// Clear the legacy field so the next save writes only Slots.
	s.Items = nil

	return true
}

// MigrateDetunedRangedWeapons applies the U10d ranged detune to banked items.
//
// The bank is swept separately from a character's own inventory because it is
// ACCOUNT-scoped: alt characters (<userId>.alts.yaml) each have their own
// inventory but the whole account shares this one Storage.
//
// Unmarked and run every load, for the same reason as the character sweep --
// items.MigrateDetunedBow is idempotent by construction. A run-once marker here
// would permanently strand any pre-detune bow deposited AFTER it was set, which
// is exactly what an un-migrated alt promoted by SwapToAlt would do.
//
// Returns true if any item was modified.
func (s *Storage) MigrateDetunedRangedWeapons() bool {
	if s == nil {
		return false
	}
	return items.MigrateDetunedRangedWeapons(s.AllItemPtrs()) > 0
}
