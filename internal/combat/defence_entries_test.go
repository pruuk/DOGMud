package combat

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
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

func assertSameSet(t *testing.T, got, want []string) {
	t.Helper()
	g := append([]string{}, got...)
	w := append([]string{}, want...)
	sort.Strings(g)
	sort.Strings(w)
	if len(g) != len(w) {
		t.Fatalf("defence set = %v, want %v", got, want)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("defence set = %v, want %v", got, want)
		}
	}
}

func countOf(set []string, name string) int {
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
		channel AttackChannel
		def     *characters.Character
		want    []string
	}{
		{"bare vs melee: dodge only", ChannelMelee, bare, []string{characters.DefenseDodge}},
		{"armed vs melee: dodge+parry", ChannelMelee, armed, []string{characters.DefenseDodge, characters.DefenseParry}},
		{"shielded vs melee: dodge+parry+block", ChannelMelee, shielded, []string{characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock}},
		{"shielded vs ranged: dodge+block", ChannelRanged, shielded, []string{characters.DefenseDodge, characters.DefenseBlock}},
		// THE new gate: no shield means no block, on ANY channel. Today the
		// channel path hands this defender a block roll against a bolt.
		{"bare vs ranged: dodge only", ChannelRanged, bare, []string{characters.DefenseDodge}},
		{"armed vs ranged: dodge only (parry not in channel table)", ChannelRanged, armed, []string{characters.DefenseDodge}},
		{"bare vs spell-physical: dodge only", ChannelSpellPhysical, bare, []string{characters.DefenseDodge}},
		{"shielded vs spell-physical: dodge+block", ChannelSpellPhysical, shielded, []string{characters.DefenseDodge, characters.DefenseBlock}},
		{"mental: quell regardless of equipment", ChannelSpellMental, bare, []string{characters.DefenseQuell}},
		{"social: defy regardless of equipment", ChannelSocial, bare, []string{characters.DefenseDefy}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DefenceEntriesFor(tc.channel, tc.def, DefenceEntryOpts{})
			assertSameSet(t, got, tc.want)
		})
	}
}

// Review subtlety 1: block requires a weapon AND HasShield(). A defender
// holding only a shield hits the IsUnarmedStyle early return (no weapon) and
// gets dodge only — exactly what characters.GetDefenseSequence did.
func TestDefenceEntriesFor_ShieldWithoutWeaponIsDodgeOnly(t *testing.T) {
	c := newDefenceTestCharacter(t)
	c.Equipment.Offhand = items.Item{ItemId: 11, Spec: &items.ItemSpec{
		Type:               items.Offhand,
		PhysicalMitigation: 5,
	}}
	got := DefenceEntriesFor(ChannelMelee, c, DefenceEntryOpts{})
	assertSameSet(t, got, []string{characters.DefenseDodge})

	got = DefenceEntriesFor(ChannelRanged, c, DefenceEntryOpts{})
	assertSameSet(t, got, []string{characters.DefenseDodge})
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
	got := DefenceEntriesFor(ChannelMelee, c, DefenceEntryOpts{})
	assertSameSet(t, got, []string{characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock})

	got = DefenceEntriesFor(ChannelRanged, c, DefenceEntryOpts{})
	assertSameSet(t, got, []string{characters.DefenseDodge, characters.DefenseBlock})
}

// Review subtlety 3: IsUnarmedStyle() never parries — a wielded Fist/Claws
// weapon is "armed" but fights unarmed-style, and the early return also
// strips block even with a shield equipped (verbatim GetDefenseSequence
// behaviour).
func TestDefenceEntriesFor_UnarmedStyleWeaponNeverParries(t *testing.T) {
	c := newDefenceTestCharacter(t)
	c.Equipment.Weapon = items.Item{ItemId: 13, Spec: &items.ItemSpec{
		Type:    items.Weapon,
		Subtype: items.Fist,
	}}
	c.Equipment.Offhand = items.Item{ItemId: 11, Spec: &items.ItemSpec{
		Type:               items.Offhand,
		PhysicalMitigation: 5,
	}}
	got := DefenceEntriesFor(ChannelMelee, c, DefenceEntryOpts{})
	assertSameSet(t, got, []string{characters.DefenseDodge})
}

// Dual-wield double parry survives the migration: two parry entries.
func TestDefenceEntriesFor_DualWieldDoubleParry(t *testing.T) {
	dw := newDualWieldDefenceTestCharacter(t)
	got := DefenceEntriesFor(ChannelMelee, dw, DefenceEntryOpts{})
	if countOf(got, characters.DefenseParry) != 2 {
		t.Errorf("dual-wield parry entries = %d, want 2 (set: %v)", countOf(got, characters.DefenseParry), got)
	}
	// And no block: the dual-wield branch returns before the shield check,
	// exactly as GetDefenseSequence did.
	if countOf(got, characters.DefenseBlock) != 0 {
		t.Errorf("dual-wield block entries = %d, want 0 (set: %v)", countOf(got, characters.DefenseBlock), got)
	}
}

// Third-party grapple filtering survives via opts. filterDefensesForThirdParty's
// contract: only block remains for a grappled defender attacked by a bystander.
func TestDefenceEntriesFor_ThirdPartyGrappleFilter(t *testing.T) {
	shielded := newShieldedDefenceTestCharacter(t)
	got := DefenceEntriesFor(ChannelMelee, shielded, DefenceEntryOpts{ThirdPartyVsGrappler: true})
	assertSameSet(t, got, []string{characters.DefenseBlock})

	bare := newDefenceTestCharacter(t)
	got = DefenceEntriesFor(ChannelMelee, bare, DefenceEntryOpts{ThirdPartyVsGrappler: true})
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
			return deterministicDefenceResult(t, atkScore, entries, entries[0].Name, atkScore, atkScore+10)
		}
	}

	var standingScores []float64
	resolveChannelDefenceWithRunner(ChannelSpellPhysical, attacker, defender, capture(&standingScores))

	setCombatPositionParallel(defender, position.Prone)
	defender.Stamina = 100
	var proneScores []float64
	resolveChannelDefenceWithRunner(ChannelSpellPhysical, attacker, defender, capture(&proneScores))

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
