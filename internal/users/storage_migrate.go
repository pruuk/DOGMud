package users

import "github.com/GoMudEngine/GoMud/internal/characters"

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
// The bank is swept separately from the character's own inventory because it is
// ACCOUNT-scoped while a character migration marker is CHARACTER-scoped. Alt
// characters (<userId>.alts.yaml) each carry their own MiscData but share this
// one Storage, and SwapToAlt promotes an alt to u.Character -- so a
// character-scoped guard would let the next LoadUser rescale every banked bow a
// second time. The rescale is a multiplication, not an assignment, so a second
// pass is silently destructive.
//
// Returns true if any item was modified (caller should mark the user dirty).
func (s *Storage) MigrateDetunedRangedWeapons() bool {
	const migrationKey = "u10d-bow-detune"

	if s == nil || s.MigrationApplied(migrationKey) {
		return false
	}

	updated := characters.MigrateDetunedRangedWeaponItems(s.AllItemPtrs())
	s.MarkMigrationApplied(migrationKey)

	return updated > 0
}
