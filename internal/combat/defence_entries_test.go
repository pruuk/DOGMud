package combat

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// The fixtures below extend newDefenceTestCharacter
// (defence_progression_test.go) with the package's direct-Equipment idiom
// (see makeSword/makeShield in collect_attack_weapons_test.go) — no Wear()
// call, so no species fixture is required for equipping.

// newArmedDefenceTestCharacter wields a plain sword, no shield.
func newArmedDefenceTestCharacter(t *testing.T) *characters.Character {
	t.Helper()
	c := newDefenceTestCharacter(t)
	c.Equipment.Weapon = items.Item{ItemId: 10, Spec: &items.ItemSpec{Type: items.Weapon}}
	return c
}

// newShieldedDefenceTestCharacter wields a sword AND a mitigating offhand
// shield (HasAnyShield keys on Type==Offhand with PhysicalMitigation > 0).
func newShieldedDefenceTestCharacter(t *testing.T) *characters.Character {
	t.Helper()
	c := newArmedDefenceTestCharacter(t)
	c.Equipment.Offhand = items.Item{ItemId: 11, Spec: &items.ItemSpec{
		Type:               items.Offhand,
		PhysicalMitigation: 5,
	}}
	return c
}

// newDualWieldDefenceTestCharacter wields two one-handed weapons.
func newDualWieldDefenceTestCharacter(t *testing.T) *characters.Character {
	t.Helper()
	c := newArmedDefenceTestCharacter(t)
	c.Equipment.Offhand = items.Item{ItemId: 12, Spec: &items.ItemSpec{Type: items.Weapon}}
	return c
}

func assertSameSet(t *testing.T, got, want []combatvocab.Defence) {
	t.Helper()
	g := append([]combatvocab.Defence{}, got...)
	w := append([]combatvocab.Defence{}, want...)
	slices.Sort(g)
	slices.Sort(w)
	if len(g) != len(w) {
		t.Fatalf("defence set = %v, want %v", got, want)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("defence set = %v, want %v", got, want)
		}
	}
}

func countOf(set []combatvocab.Defence, name combatvocab.Defence) int {
	n := 0
	for _, s := range set {
		if s == name {
			n++
		}
	}
	return n
}

func TestDefenceEntriesFor_EquipmentGate(t *testing.T) {
	bare := newDefenceTestCharacter(t)       // no weapon, no shield
	armed := newArmedDefenceTestCharacter(t) // weapon, no shield
	shielded := newShieldedDefenceTestCharacter(t)

	cases := []struct {
		name    string
		channel combatvocab.Attack
		def     *characters.Character
		want    []combatvocab.Defence
	}{
		{"bare vs melee: dodge only", combatvocab.Melee(combatvocab.TargetSingle), bare, []combatvocab.Defence{combatvocab.DefenceDodge}},
		{"armed vs melee: dodge+parry", combatvocab.Melee(combatvocab.TargetSingle), armed, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceParry}},
		{"shielded vs melee: dodge+parry+block", combatvocab.Melee(combatvocab.TargetSingle), shielded, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceParry, combatvocab.DefenceBlock}},
		{"shielded vs ranged: dodge+block", combatvocab.Ranged(combatvocab.TargetSingle), shielded, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceBlock}},
		// THE new gate: no shield means no block, on ANY channel. Today the
		// channel path hands this defender a block roll against a bolt.
		{"bare vs ranged: dodge only", combatvocab.Ranged(combatvocab.TargetSingle), bare, []combatvocab.Defence{combatvocab.DefenceDodge}},
		{"armed vs ranged: dodge only (parry not in channel table)", combatvocab.Ranged(combatvocab.TargetSingle), armed, []combatvocab.Defence{combatvocab.DefenceDodge}},
		{"bare vs spell-physical: dodge only", combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetSingle), bare, []combatvocab.Defence{combatvocab.DefenceDodge}},
		{"shielded vs spell-physical: dodge+block", combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetSingle), shielded, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceBlock}},
		{"mental: quell regardless of equipment", combatvocab.Spell(combatvocab.DamageMental, combatvocab.TargetSingle), bare, []combatvocab.Defence{combatvocab.DefenceQuell}},
		{"social: defy regardless of equipment", combatvocab.Rhetoric(combatvocab.TargetSingle), bare, []combatvocab.Defence{combatvocab.DefenceDefy}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DefenceEntriesFor(tc.channel, tc.def, DefenceEntryOpts{})
			assertSameSet(t, got, tc.want)
		})
	}
}

// ⚠️ UPDATED 2026-08-30, and the old expectation was the BUG being fixed.
//
// This used to assert that a defender holding only a shield gets dodge only,
// because the gate's IsUnarmedStyle early return fired on "no weapon" before
// anything looked at the shield. That same early return is what made a tower
// shield on a third arm inert, so the gate is now per-slot: a shield blocks
// wherever it is held, armed or not.
//
// Parry is still correctly absent — there is nothing to deflect with.
func TestDefenceEntriesFor_ShieldWithoutWeaponBlocks(t *testing.T) {
	c := newDefenceTestCharacter(t)
	c.Equipment.Offhand = items.Item{ItemId: 11, Spec: &items.ItemSpec{
		Type:               items.Offhand,
		PhysicalMitigation: 5,
	}}
	got := DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), c, DefenceEntryOpts{})
	assertSameSet(t, got, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceBlock})

	// Ranged keeps its own channel table; a shield still blocks arrows.
	got = DefenceEntriesFor(combatvocab.Ranged(combatvocab.TargetSingle), c, DefenceEntryOpts{})
	assertSameSet(t, got, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceBlock})
}

// Review subtlety 2: HasShield() includes species NaturalBash — an armed
// earth elemental blocks with no shield ITEM. Gating on BestBlockRating()
// (zero here) would strip it.
func TestDefenceEntriesFor_NaturalBashSpeciesBlocksWithoutShieldItem(t *testing.T) {
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "earth elemental", NaturalBash: true},
	})
	defer cleanup()

	c := newArmedDefenceTestCharacter(t) // weapon, empty offhand, no shield item
	got := DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), c, DefenceEntryOpts{})
	assertSameSet(t, got, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceParry, combatvocab.DefenceBlock})

	got = DefenceEntriesFor(combatvocab.Ranged(combatvocab.TargetSingle), c, DefenceEntryOpts{})
	assertSameSet(t, got, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceBlock})
}

// A wielded Fist/Claws weapon is "armed" by ItemType but fights unarmed-style
// and still NEVER parries — that half is unchanged and is why
// ParryCapableArmCount excludes those two subtypes.
//
// ⚠️ UPDATED 2026-08-30: it used to assert dodge ALONE, because the
// IsUnarmedStyle early return stripped block too. That early return is what
// made a shield on an extra arm inert, so block is now per-slot. A claw fighter
// carrying a shield blocks with it; they still cannot parry with claws.
func TestDefenceEntriesFor_UnarmedStyleWeaponNeverParriesButStillBlocks(t *testing.T) {
	c := newDefenceTestCharacter(t)
	c.Equipment.Weapon = items.Item{ItemId: 13, Spec: &items.ItemSpec{
		Type:    items.Weapon,
		Subtype: items.Fist,
	}}
	c.Equipment.Offhand = items.Item{ItemId: 11, Spec: &items.ItemSpec{
		Type:               items.Offhand,
		PhysicalMitigation: 5,
	}}
	got := DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), c, DefenceEntryOpts{})
	assertSameSet(t, got, []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceBlock})
	if countOf(got, combatvocab.DefenceParry) != 0 {
		t.Errorf("unarmed-style must never parry, got %v", got)
	}
}

// Dual-wield double parry survives the migration: two parry entries.
func TestDefenceEntriesFor_DualWieldDoubleParry(t *testing.T) {
	dw := newDualWieldDefenceTestCharacter(t)
	got := DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), dw, DefenceEntryOpts{})
	if countOf(got, combatvocab.DefenceParry) != 2 {
		t.Errorf("dual-wield parry entries = %d, want 2 (set: %v)", countOf(got, combatvocab.DefenceParry), got)
	}
	// And no block — but note WHY, because the reason changed on 2026-08-30.
	// It is no longer "the dual-wield branch returns before the shield check";
	// there is no such branch now. This fixture simply carries NO SHIELD, so the
	// per-slot gate finds nothing to block with. Give the same character a
	// shield on a third arm and it WOULD block, which is the whole point of the
	// change.
	if countOf(got, combatvocab.DefenceBlock) != 0 {
		t.Errorf("dual-wield block entries = %d, want 0 (set: %v)", countOf(got, combatvocab.DefenceBlock), got)
	}
}

// Third-party grapple filtering survives via opts. filterDefensesForThirdParty's
// contract: only block remains for a grappled defender attacked by a bystander.
func TestDefenceEntriesFor_ThirdPartyGrappleFilter(t *testing.T) {
	shielded := newShieldedDefenceTestCharacter(t)
	got := DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), shielded, DefenceEntryOpts{ThirdPartyVsGrappler: true})
	assertSameSet(t, got, []combatvocab.Defence{combatvocab.DefenceBlock})

	bare := newDefenceTestCharacter(t)
	got = DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), bare, DefenceEntryOpts{ThirdPartyVsGrappler: true})
	if len(got) != 0 {
		t.Errorf("bare third-party set = %v, want empty (auto-hit)", got)
	}
}

// Named behaviour change (2) of U6b Task 2: prone defence penalties now apply
// on every channel. Before this, a prone defender dodged a bolt at full score
// while dodging a sword at penalty.
func TestChannelDefence_ProneAppliesDefencePenalties(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	cfg := configs.GetConfig()
	cfg.Balance.ProneDodgePenalty = 0.5
	configs.SetConfigForTest(t, cfg)

	attacker, defender := defenceAdmissionCharacters()
	defender.Stamina = 100

	capture := func(scores *[]float64) defenceContestRunner {
		return func(atkScore float64, entries []contest.Entry) contest.Result {
			for _, e := range entries {
				*scores = append(*scores, e.Score)
			}
			return deterministicDefenceResult(t, atkScore, entries, combatvocab.Defence(entries[0].Name), atkScore, atkScore+10)
		}
	}

	var standingScores []float64
	resolveChannelAttackWithRunner(nil, combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetSingle), channelSideForSignTest(combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetSingle), attacker), attacker, defender, capture(&standingScores))

	setCombatPositionParallel(defender, position.Prone)
	defender.Stamina = 100
	var proneScores []float64
	resolveChannelAttackWithRunner(nil, combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetSingle), channelSideForSignTest(combatvocab.Spell(combatvocab.DamagePhysical, combatvocab.TargetSingle), attacker), attacker, defender, capture(&proneScores))

	if len(standingScores) != 1 || len(proneScores) != 1 {
		t.Fatalf("entry counts standing=%d prone=%d, want 1 each (bare defender: dodge only)",
			len(standingScores), len(proneScores))
	}
	want := standingScores[0] * 0.5
	if proneScores[0] != want {
		t.Errorf("prone channel dodge score = %.2f, want %.2f (standing %.2f x ProneDodgePenalty 0.5)",
			proneScores[0], want, standingScores[0])
	}
}
