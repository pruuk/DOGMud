package characters

import (
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// preDetuneBowMultipliers are the damage_multiplier values the eight
// Shooting-subtype templates carried BEFORE the U10d ranged detune.
//
// Hardcoded because the templates no longer hold them, and the RATIO between
// the old and new template values is what lets an affixed or admin-tuned
// instance keep the scaling it earned. See migrateDetunedBow.
var preDetuneBowMultipliers = map[int]float64{
	10046: 7.50, // Ironhorn Warbow
	10042: 7.00, // Arbalest
	10049: 6.00, // Relic Sidearm
	10041: 5.50, // Hunting Bow
	10004: 4.00, // Training Bow
	10040: 3.50, // Primitive Pistol
	10039: 3.00, // Hand Crossbow
	10038: 2.00, // Sling
}

// MigrateDetunedRangedWeapons rescales the persisted damage multiplier on
// ranged weapons that already exist in a player's save, so the U10d template
// detune actually reaches them.
//
// WHY THIS IS NEEDED AT ALL: Item.GetSpec() returns the instance override
// (Item.Spec, persisted under the yaml key `overrides:`) whenever it is
// non-nil, and never consults the template. An existing bow carrying an
// override therefore keeps its pre-U10d multiplier forever, and the template
// table test in internal/items cannot see it.
//
// ORDERING: this MUST run before MigrateEnchantments(). enchantments.ApplyTier
// does an unconditional item.EnchantBaseline.RestoreInto(&newSpec), so an
// enchantment pass running first would re-install the stale pre-detune value
// straight back over the fix.
//
// COVERAGE -- populations this reaches:
//   - character backpack, component bag, potion bandolier, equipped items
//   - the account's bank storage, passed in via extra (see users.LoadUser)
//
// COVERAGE -- populations this deliberately does NOT reach:
//   - Mob equipment persisted in _datafiles/world/dogmud/mobs.instances/**
//     (instance_save.go persists Equipment). This is a live damage source, so
//     the exposure is real, but it is prod-only: the project's smoke-test SOP
//     wipes mobs.instances/ and it is not deployed, and the exposure decays as
//     mobs respawn from templates.
//   - Shop resale stock in _datafiles/world/dogmud/shops/** (shopinventory.go
//     persists full item instances). Prod-only and self-clearing as stock sells
//     through and restocks from templates.
//   - Room containers and corpses in _datafiles/world/dogmud/rooms.instances/**
//     (also wiped by the smoke-test SOP and not deployed).
//
// Those three are world state, not user state, and reaching them needs a
// world-sweep migration rather than a per-load hook. This is a USER-SAVE
// migration and does not claim world coverage.
//
// Run-once via MiscData. It is NOT idempotent -- it multiplies by a ratio, so
// a second pass would detune an already-detuned bow again.
//
// THE MARKER IS PER-CHARACTER, WHICH IS WHY THE BANK IS NOT SWEPT HERE.
// Alt characters live in <userId>.alts.yaml and carry their OWN MiscData, but
// the whole account shares ONE ItemStorage. users.SwapToAlt promotes an alt to
// u.Character, so the next LoadUser would find an unmarked character, run this
// again, and detune every banked bow a second time. The bank therefore carries
// its own account-scoped marker -- see users.Storage.MigrateDetunedRangedWeapons.
func (c *Character) MigrateDetunedRangedWeapons() {
	const migrationKey = "migration-u10d-bow-detune-done"
	if c.GetMiscData(migrationKey) != nil {
		return
	}

	ptrs := make([]*items.Item, 0, len(c.Items)+len(c.ComponentItems)+len(c.PotionItems))
	for i := range c.Items {
		ptrs = append(ptrs, &c.Items[i])
	}
	for i := range c.ComponentItems {
		ptrs = append(ptrs, &c.ComponentItems[i])
	}
	for i := range c.PotionItems {
		ptrs = append(ptrs, &c.PotionItems[i])
	}
	ptrs = append(ptrs, c.Equipment.GetAllItemPtrs()...)

	updated := MigrateDetunedRangedWeaponItems(ptrs)

	c.SetMiscData(migrationKey, "1")

	if updated > 0 {
		mudlog.Info("MigrateDetunedRangedWeapons", "character", c.Name, "items_updated", updated)
	}
}

// MigrateDetunedRangedWeaponItems applies the U10d rescale to an arbitrary set
// of items and returns how many were modified.
//
// UNGUARDED ON PURPOSE. The rescale is not idempotent, so every caller MUST
// supply its own run-once marker scoped to whatever owns the items. Exported so
// internal/users can sweep bank storage under an account-scoped marker rather
// than a per-character one.
func MigrateDetunedRangedWeaponItems(ptrs []*items.Item) int {
	updated := 0
	for _, ptr := range ptrs {
		if migrateDetunedBow(ptr) {
			updated++
		}
	}
	return updated
}

// migrateDetunedBow rescales one item's persisted damage multiplier by the
// ratio between the U10d template value and the pre-U10d template value.
// Returns true if the item was modified.
//
// RESCALE PROPORTIONALLY. NEVER ASSIGN THE TEMPLATE VALUE.
//
// items.SpecBaseline exists precisely because the enchant path used to reset
// to the bare template, which silently destroyed everything an instance had
// earned above it -- affix scaling from instanced loot, whose budget is bought
// with the gold paid to enter the instance, and anything an admin set on the
// instance. Observed on prod: about a 16% damage drop on a set of affixed
// claws. affixgen.applyBonus writes spec.DamageMultiplier += 0.05 per rank, so
// an instanced Ironhorn Warbow can sit at 7.85; assigning 2.75 would delete
// the 0.35 the player paid gold for, with no message and no refund. Ids 10046
// and 10049 exist only in loot pools, so that is exactly the population here.
//
// Nor may EnchantBaseline.DamageMultiplier simply be cleared: RestoreInto does
// an unconditional spec.DamageMultiplier = b.DamageMultiplier, so a zero there
// writes 0 into the spec and ApplyTier then adds only the tier bonus -- an
// enchanted warbow would land near 0.10, a 96% nerf.
func migrateDetunedBow(item *items.Item) bool {
	if item == nil {
		return false
	}

	old, ok := preDetuneBowMultipliers[item.ItemId]
	if !ok || old <= 0 {
		return false
	}

	tmpl := items.GetItemSpec(item.ItemId)
	if tmpl == nil || tmpl.DamageMultiplier <= 0 {
		return false
	}

	ratio := tmpl.DamageMultiplier / old

	changed := false
	if item.Spec != nil && item.Spec.DamageMultiplier > 0 {
		item.Spec.DamageMultiplier *= ratio
		changed = true
	}
	if item.EnchantBaseline != nil && item.EnchantBaseline.DamageMultiplier > 0 {
		item.EnchantBaseline.DamageMultiplier *= ratio
		changed = true
	}

	return changed
}
