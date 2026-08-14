package items

// SpecBaseline captures the numeric parts of an item's spec as they were BEFORE
// any enchantment was applied.
//
// Enchant tiers are re-applied from scratch every time (on first enchant, on
// tier-up, and on migration when a definition changes), so applying a tier has
// to start from a clean slate or the bonuses stack. That slate used to be the
// bare item TEMPLATE, which silently destroyed everything an instance had
// earned above it:
//
//   - affix scaling from instanced loot, whose budget is bought with the gold
//     paid to enter the instance (items.CalcLootBudget), and which spends that
//     budget on DamageMultiplier and mitigation ranks
//   - anything an admin set directly on the instance
//
// A player who enchanted good instance loot therefore lost the scaling they had
// paid for, with no message and no refund. Observed on prod: about a 16% damage
// drop on a set of affixed claws.
//
// Snapshotting the pre-enchant values instead keeps both properties: tiers still
// cannot stack, because every application resets to this baseline first, and the
// instance keeps what it earned.
//
// Nil on items that have never been enchanted.
type SpecBaseline struct {
	Damage               Damage         `yaml:"damage,omitempty"`
	DamageMultiplier     float64        `yaml:"damage_multiplier,omitempty"`
	PhysicalMitigation   int            `yaml:"physical_mitigation,omitempty"`
	MagicalMitigation    int            `yaml:"magical_mitigation,omitempty"`
	ConvictionMitigation int            `yaml:"conviction_mitigation,omitempty"`
	StatMods             map[string]int `yaml:"statmods,omitempty"`
}

// CaptureSpecBaseline snapshots the fields an enchant tier overwrites.
func CaptureSpecBaseline(spec ItemSpec) *SpecBaseline {
	b := &SpecBaseline{
		Damage:               spec.Damage,
		DamageMultiplier:     spec.DamageMultiplier,
		PhysicalMitigation:   spec.PhysicalMitigation,
		MagicalMitigation:    spec.MagicalMitigation,
		ConvictionMitigation: spec.ConvictionMitigation,
	}
	if len(spec.StatMods) > 0 {
		b.StatMods = make(map[string]int, len(spec.StatMods))
		for k, v := range spec.StatMods {
			b.StatMods[k] = v
		}
	}
	return b
}

// RestoreInto writes the baseline back over spec, giving a tier application a
// clean slate that still carries affix scaling.
func (b *SpecBaseline) RestoreInto(spec *ItemSpec) {
	if b == nil || spec == nil {
		return
	}
	spec.Damage = b.Damage
	spec.DamageMultiplier = b.DamageMultiplier
	spec.PhysicalMitigation = b.PhysicalMitigation
	spec.MagicalMitigation = b.MagicalMitigation
	spec.ConvictionMitigation = b.ConvictionMitigation

	mods := make(map[string]int, len(b.StatMods))
	for k, v := range b.StatMods {
		mods[k] = v
	}
	spec.StatMods = mods
}
