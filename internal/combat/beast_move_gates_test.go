package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/stretchr/testify/require"
)

// beastMoveGates is every CanUse* gate that species data decides.
var beastMoveGates = map[string]func(*characters.Character) bool{
	"rake": CanUseRake, "maul": CanUseMaul, "pounce": CanUsePounce, "gore": CanUseGore,
	"throttle": CanUseThrottle, "hamstring": CanUseHamstring, "drain": CanUseDrain,
	"smother": CanUseSmother,
}

// Spec section 6: a species whose list omits a move cannot use it even with
// the matching attack family, and a listed move opens the gate. Both halves,
// so the test fails whichever way a gate stops reading the list.
func TestBeastMoveGatesReadTheSpeciesList(t *testing.T) {
	all := []string{"rake", "maul", "pounce", "gore", "throttle", "hamstring", "drain", "smother"}
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9401: {SpeciesId: 9401, Name: "unlisted", BodyParts: []string{"legs", "mouth", "horns"}, NaturalAttack: items.Bite, LifeDrain: true},
		9402: {SpeciesId: 9402, Name: "listed", BodyParts: []string{"legs", "mouth", "horns"}, NaturalAttack: items.Bite, LifeDrain: true, BeastMoves: all},
	}))
	unlisted := &characters.Character{SpeciesId: 9401}
	listed := &characters.Character{SpeciesId: 9402}
	for move, gate := range beastMoveGates {
		if gate(unlisted) {
			t.Errorf("%s: a species that does not list it can use it", move)
		}
		if !gate(listed) {
			t.Errorf("%s: a species that lists it, with the body for it, cannot use it", move)
		}
	}
}

// The table in practice: the gate lets a cat throttle and refuses a rat, a
// bear no longer pounces, a serpent does nothing but bite. (D4: whether the
// cat's AI ever picks throttle is profile tuning, #574; this tests the gate.)
func TestBeastMoveGates_TableExamples(t *testing.T) {
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		11: {SpeciesId: 11, Name: "feline", BodyParts: []string{"legs", "eyes", "mouth", "skin", "tail"}, NaturalAttack: items.Claws,
			BeastMoves: []string{"rake", "pounce", "hamstring", "throttle"}},
		10: {SpeciesId: 10, Name: "rodent", BodyParts: []string{"legs", "eyes", "mouth", "skin", "tail"}, NaturalAttack: items.Bite,
			BeastMoves: []string{"hamstring"}},
		3: {SpeciesId: 3, Name: "bear", BodyParts: []string{"arms", "legs", "eyes", "mouth", "skin"}, NaturalAttack: items.Claws,
			BeastMoves: []string{"rake"}},
		8: {SpeciesId: 8, Name: "serpent", BodyParts: []string{"eyes", "mouth", "skin"}, NaturalAttack: items.Bite},
	}))
	cat, rat, bear, snake := &characters.Character{SpeciesId: 11}, &characters.Character{SpeciesId: 10},
		&characters.Character{SpeciesId: 3}, &characters.Character{SpeciesId: 8}
	if !CanUseThrottle(cat) || CanUseThrottle(rat) {
		t.Error("the gate must allow a cat's throttle and refuse a rat's")
	}
	if CanUsePounce(bear) || CanUseHamstring(bear) || !CanUseRake(bear) {
		t.Error("a bear rakes; it does not pounce or hamstring")
	}
	for move, gate := range beastMoveGates {
		if gate(snake) {
			t.Errorf("a serpent must not %s", move)
		}
	}
}

func TestScoreSmother_FavoursCastersAndSightedTargets(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		perception.ConditionIdBlinded: {ConditionId: perception.ConditionIdBlinded, Name: "Blinded", TriggerCount: 3, RoundInterval: 1},
	}))
	mob := &mobs.Mob{}
	sighted := characters.New()
	base := ScoreSmother(mob, sighted)
	if base <= 0 {
		t.Fatal("smother must score against a sighted target")
	}

	caster := characters.New()
	if caster.Activity == nil {
		caster.Activity = activity.NewMachine()
	}
	require.NoError(t, caster.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-spell", FoldsNeeded: 3},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	require.True(t, caster.IsCasting(), "fixture: the caster must be casting")
	if got := ScoreSmother(mob, caster); got <= base {
		t.Errorf("a casting target scores %d, a non-casting one %d: casting must score higher", got, base)
	}

	blind := characters.New()
	blind.Conditions = conditions.New()
	require.NoError(t, blind.AddCondition(perception.ConditionIdBlinded, false))
	require.True(t, blind.HasAnyBlindSource(), "fixture: the target must be blind")
	if got := ScoreSmother(mob, blind); got >= base {
		t.Errorf("a blind target scores %d, a sighted one %d: blind must score lower", got, base)
	}
}

// The Elemental Prince (322) has no aiprofile, so it runs "default", which
// carries no smother weight: smother falls back to weight 10 and, being the
// only move a bodiless smoke species can make, is still the one selected.
func TestSelectSpecialMove_SmokeSpeciesPicksSmother(t *testing.T) {
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9444: {SpeciesId: 9444, Name: "smoke", BodyParts: []string{}, NaturalAttack: items.Claws, BeastMoves: []string{"smother"}},
	}))
	mob := &mobs.Mob{}
	mob.Character.SpeciesId = 9444
	if got := SelectSpecialMove(mob, characters.New()); got != "smother" {
		t.Fatalf("SelectSpecialMove = %q, want smother", got)
	}
}
