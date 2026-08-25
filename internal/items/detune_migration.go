package items

// preDetuneBowMultipliers are the damage_multiplier values the eight
// Shooting-subtype templates carried BEFORE the U10d ranged detune.
//
// Hardcoded because the templates no longer hold them. The RATIO between the
// old and the new template value is what lets an affixed or admin-tuned
// instance keep the scaling it earned. The old value also serves as the
// legacy-save threshold, and the key set decides which item ids New() stamps
// as already-migrated (see MigrateDetunedBow).
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
// for why. There are deliberately no run-once markers on the callers.
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
// IDEMPOTENT, VIA TWO GATES. The rescale is a MULTIPLICATION, so a second pass
// over the same item would detune it again (2.75 -> 1.01, silently and
// permanently). No run-once marker on the CHARACTER can prevent that, because a
// character can hold an already-correct bow while carrying no marker:
// characters.New() and CreateUser both produce empty MiscData, and a brand-new
// account plays its entire first session on an in-memory record that has never
// been through LoadUser.
//
// Gate 1, the authority: Item.DetuneMigrated. New() stamps it on every bow
// minted since the detune, and this function stamps it on every bow it
// inspects, so identity is recorded on the item rather than inferred from its
// value.
//
// Gate 2, the legacy fallback: `>= old`, for saves written before the stamp
// existed. Affix ranks and enchant tiers only ever ADD, so a genuine pre-detune
// instance is at or above its old template value.
//
// Gate 2 ALONE HAS A LIVE FALSE POSITIVE, which is why gate 1 exists. Item
// 10049 (Relic Sidearm) drops from the Core Guardian in Crash Site Interior, an
// `instanced: true` zone, so it can be affix-scaled. The affix budget is
// floor(LootBudgetScalar * sqrt(goldPaid)) and goldPaid has NO upper bound
// (actOpenInstancePortal enforces only a minimum; the 50000 cap applies to
// ScaleSpawnStatPools, not to the loot budget). A post-detune sidearm needs
// +3.80 over 2.20 -- 76 ranks of damage_mult_phys -- to reach the old 6.00, and
// against the real weighted-selection loop that is roughly a 10% outcome at
// 40k gold and 49% at 100k. Tripping it would multiply a legitimately earned,
// gold-bought item by 0.367: a 63% loss, the very bug SpecBaseline exists to
// prevent. The stamp closes that door; the fallback only ever sees saves that
// predate it, in which no post-detune item can exist.
//
// The remaining fallback-only edge is benign: an admin who deliberately LOWERED
// a legacy instance below its old template is skipped and keeps the value the
// admin chose.
//
// SPEC AND BASELINE ARE ONE QUANTITY and are judged together. Testing them
// independently lets them desync: ApplyTier sets spec = baseline + tierBonus,
// where damage_multiplier_bonus reaches +0.30 and DOUBLES on two-handers, so a
// baseline in [old-0.60, old) would leave the spec tripping the guard while the
// baseline did not -- cutting the spec to ~0.367x while the baseline kept the
// old value, until the next tier-up snapped it back. The threshold is therefore
// evaluated ONCE against the baseline where there is one (it is the item's
// pre-enchant multiplier, the quantity directly comparable to a template
// value), else against the spec, and that single decision is applied to both.
func MigrateDetunedBow(item *Item) bool {
	if item == nil {
		return false
	}

	old, ok := preDetuneBowMultipliers[item.ItemId]
	if !ok || old <= 0 {
		return false
	}

	// Gate 1: already known to be on the post-detune line.
	if item.DetuneMigrated {
		return false
	}
	// Whatever happens below, this item's line is settled after this pass.
	item.DetuneMigrated = true

	tmpl := GetItemSpec(item.ItemId)
	if tmpl == nil || tmpl.DamageMultiplier <= 0 {
		return false
	}

	// Gate 2: one threshold test, applied to both fields.
	reference := 0.0
	switch {
	case item.EnchantBaseline != nil && item.EnchantBaseline.DamageMultiplier > 0:
		reference = item.EnchantBaseline.DamageMultiplier
	case item.Spec != nil:
		reference = item.Spec.DamageMultiplier
	}
	if reference < old {
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
