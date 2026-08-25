package characters

import (
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// MigrateDetunedRangedWeapons rescales the persisted damage multiplier on
// ranged weapons a player already owns, so the U10d template detune actually
// reaches them. The arithmetic and its safety argument live in
// items.MigrateDetunedBow.
//
// ORDERING: this MUST run before MigrateEnchantments(). enchantments.ApplyTier
// does an unconditional item.EnchantBaseline.RestoreInto(&newSpec), so an
// enchantment pass running first would re-install the stale pre-detune value
// straight back over the fix.
//
// NO RUN-ONCE MARKER, deliberately -- this follows MigrateEnchantments, which
// is likewise unguarded because it is idempotent. items.MigrateDetunedBow only
// rescales values still at or above the pre-detune template, so re-running is a
// no-op, and that is the whole correctness argument.
//
// A marker would be actively WORSE than nothing here, in both directions:
//
//   - It cannot prevent the corruption it looks like it prevents.
//     characters.New() produces empty MiscData, so a freshly created alt -- or
//     a brand-new account, which plays its entire first session on an in-memory
//     record that never passes through LoadUser -- can enchant a post-detune
//     bow at 2.75 and then meet the migration for the "first" time.
//   - It would freeze the misses. Any pre-detune bow reaching a marked
//     character LATER (looted from a mob instance, bought from stale shop
//     stock, pulled from a corpse, or deposited by an un-migrated alt) would
//     never be rescaled at all. Running every load lets that exposure decay
//     instead.
//
// COVERAGE -- populations this reaches:
//   - backpack, component bag, potion bandolier, equipped items
//   - pet inventory (Character.Pet.Items): Pet.StoreItem accepts any item with
//     ItemId >= 1 with no type filter, and get.go/give.go route items into it,
//     so a pack pet is first-class player storage and can hold a bow.
//
// The account's bank is swept separately (it is not a Character collection);
// see users.Storage.MigrateDetunedRangedWeapons.
//
// COVERAGE -- populations this deliberately does NOT reach:
//   - Mob equipment persisted in _datafiles/world/dogmud/mobs.instances/**
//     (instance_save.go persists Equipment). A live damage source, so the
//     exposure is real, but it is prod-only: the smoke-test SOP wipes
//     mobs.instances/ and it is not deployed.
//   - Shop resale stock in _datafiles/world/dogmud/shops/**
//     (shopinventory.go persists full item instances).
//   - Room containers and corpses in _datafiles/world/dogmud/rooms.instances/**.
//
// Those three are world state, not user state, and reaching them needs a
// world-sweep migration rather than a per-load hook. This is a USER-SAVE
// migration and does not claim world coverage. Because the rescale is
// value-guarded and unmarked, a pre-detune bow that later reaches a player from
// any of them still migrates on the next load -- that exposure decays instead
// of being frozen in place.
func (c *Character) MigrateDetunedRangedWeapons() {
	ptrs := make([]*items.Item, 0,
		len(c.Items)+len(c.ComponentItems)+len(c.PotionItems)+len(c.Pet.Items))
	for i := range c.Items {
		ptrs = append(ptrs, &c.Items[i])
	}
	for i := range c.ComponentItems {
		ptrs = append(ptrs, &c.ComponentItems[i])
	}
	for i := range c.PotionItems {
		ptrs = append(ptrs, &c.PotionItems[i])
	}
	for i := range c.Pet.Items {
		ptrs = append(ptrs, &c.Pet.Items[i])
	}
	ptrs = append(ptrs, c.Equipment.GetAllItemPtrs()...)

	updated := items.MigrateDetunedRangedWeapons(ptrs)

	if updated > 0 {
		mudlog.Info("MigrateDetunedRangedWeapons", "character", c.Name, "items_updated", updated)
	}
}
