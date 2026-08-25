package items

// preDetuneBowMultipliers are the damage_multiplier values the eight
// Shooting-subtype templates carried BEFORE the U10d ranged detune.
//
// Hardcoded because the templates no longer hold them. The RATIO between the
// old and the new template value is what lets an affixed or admin-tuned
// instance keep the scaling it earned; the old value itself is also the
// threshold that makes the migration safe to re-run (see MigrateDetunedBow).
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

// MigrateDetunedRangedWeapons applies the U10d rescale to a set of items and
// returns how many were modified.
//
// Safe to call on any item set, any number of times -- see MigrateDetunedBow
// for why. Callers still carry run-once markers, but only to avoid pointless
// work, never for correctness.
func MigrateDetunedRangedWeapons(ptrs []*Item) int {
	updated := 0
	for _, ptr := range ptrs {
		if MigrateDetunedBow(ptr) {
			updated++
		}
	}
	return updated
}

// MigrateDetunedBow rescales one item's persisted damage multiplier from the
// pre-U10d template value onto the U10d one. Returns true if it changed the
// item.
//
// WHY THIS EXISTS: Item.GetSpec() returns the instance override (Item.Spec,
// persisted under the yaml key `overrides:`) whenever it is non-nil and never
// consults the template, so editing the template does not reach a bow that
// already exists. Anything that materialises a Spec -- an enchant, an affix
// roll, a rename, a worn buff -- pins that bow's damage forever.
//
// RESCALE PROPORTIONALLY. NEVER ASSIGN THE TEMPLATE VALUE. SpecBaseline exists
// precisely because the enchant path used to reset to the bare template, which
// silently destroyed everything an instance had earned above it: affix scaling
// whose budget is bought with the gold paid to enter an instance, and anything
// an admin set on the instance. Observed on prod as about a 16% damage drop on
// a set of affixed claws. applyBonus writes DamageMultiplier += 0.05 per affix
// rank, so an instanced Ironhorn Warbow can sit at 7.85; assigning 2.75 would
// delete the 0.35 the player paid gold for, with no message and no refund. Ids
// 10046 and 10049 exist only in loot pools, so that is exactly this population.
//
// Nor may EnchantBaseline.DamageMultiplier be cleared instead: RestoreInto does
// an unconditional spec.DamageMultiplier = b.DamageMultiplier, so a zero there
// writes 0 into the spec and ApplyTier then adds only the tier bonus -- an
// enchanted warbow would land near 0.10, a 96% nerf.
//
// IDEMPOTENT BY CONSTRUCTION, which is the important property and the reason
// this does not rely on a caller's run-once marker. The rescale is a
// MULTIPLICATION, so a second pass over the same item would detune it again
// (2.75 -> 1.01, silently and permanently). Markers alone cannot prevent that,
// because a character can hold an already-correct bow while carrying no marker:
// characters.New() and CreateUser both produce empty MiscData, and a brand-new
// account plays its entire first session on an in-memory record that has never
// been through LoadUser. Such a character can enchant a post-detune bow at
// 2.75, and the first migration afterwards would corrupt a correct item.
//
// The `>= old` threshold closes that: affix ranks and enchant tiers only ever
// ADD to the multiplier, so any genuine pre-detune instance is at or above its
// old template value, while every already-migrated or natively-post-detune item
// is below it and is skipped.
//
// Two edges worth naming:
//   - An admin who deliberately LOWERED an instance below its old template is
//     skipped. Fail-safe: it keeps the value the admin chose rather than being
//     silently rescaled.
//   - The Sling has the tightest margin (0.75 new against 2.00 old); it would
//     take 25 affix ranks of +0.05 to lift a post-detune sling back to 2.00 and
//     produce a false positive.
func MigrateDetunedBow(item *Item) bool {
	if item == nil {
		return false
	}

	old, ok := preDetuneBowMultipliers[item.ItemId]
	if !ok || old <= 0 {
		return false
	}

	tmpl := GetItemSpec(item.ItemId)
	if tmpl == nil || tmpl.DamageMultiplier <= 0 {
		return false
	}

	ratio := tmpl.DamageMultiplier / old

	changed := false
	if item.Spec != nil && item.Spec.DamageMultiplier >= old {
		item.Spec.DamageMultiplier *= ratio
		changed = true
	}
	if item.EnchantBaseline != nil && item.EnchantBaseline.DamageMultiplier >= old {
		item.EnchantBaseline.DamageMultiplier *= ratio
		changed = true
	}

	return changed
}
