package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// U10b-1 Task 9: AttackResult.SwingDefences is appended by calculateCombat's
// swing loop, guarded on best.defenseType != "". Every other test for this
// feature hand-builds the slice, so nothing else covers the append itself.
//
// These assert an INVARIANT rather than a pinned outcome, so no contest runner
// needs stubbing and no roll needs pinning: SwingDefences carries exactly one
// entry per CONTESTED swing, so it equals SwingsThrown when the defender has
// any defence at all and is empty when the defender has none. Both hold for
// every roll, which is what lets these cover the mixed round -- some swings
// defended, some not, some fumbled -- in one pass.
//
// Together they also pin the documented NON-parallelism with SwingEvents:
// SwingEvents is appended unconditionally, SwingDefences is not.

// swingDefencePopulationCombatants mirrors cost_seam_test.go's fixture: a real
// calculateCombat call needs a validated pair and an Aggro on the attacker,
// which calculateCombat reads unguarded and which is nil on a fresh character.
//
// DefaultAttack, not SurpriseAttack: a surprise round forces crits and would
// stop exercising the defence path.
func swingDefencePopulationCombatants() (*characters.Character, *characters.Character) {
	atk := characters.New()
	atk.Stats.Strength.Base = 200
	atk.Stats.Dexterity.Base = 200
	atk.Stats.Strength.Recalculate()
	atk.Stats.Dexterity.Recalculate()
	atk.Validate()
	atk.HealthMax.Value = 1000
	atk.Health = 1000
	atk.StaminaMax.Value = 1000
	atk.Stamina = 1000
	atk.SetUserId(1)
	atk.SetAggro(0, 0, characters.DefaultAttack)

	def := characters.New()
	def.Stats.Dexterity.Base = 100
	def.Stats.Vitality.Base = 100
	def.Stats.Dexterity.Recalculate()
	def.Stats.Vitality.Recalculate()
	def.Validate()
	def.HealthMax.Value = 5000
	def.Health = 5000
	def.StaminaMax.Value = 500
	def.Stamina = 500
	def.SetUserId(2)

	return atk, def
}

// A defender who has a defence available quotes exactly one per swing. Dodge is
// ungated in equipmentGatedMeleeDefences, so a bare-handed defender always has
// one, which makes this the ordinary case.
//
// Twenty rounds rather than one, so the assertion sees fumbles, deflections,
// defensive crits and clean attack wins mixed within and across rounds instead
// of whichever single outcome the first roll happened to produce. Stamina is
// deliberately allowed to drain across them: that exercises the mid-round
// includeSkill drift without changing the invariant.
func TestSwingDefences_OneEntryPerContestedSwing(t *testing.T) {
	atk, def := swingDefencePopulationCombatants()

	sawMultiSwingRound := false
	for i := 0; i < 20; i++ {
		plan := buildAttackPlan(atk, def)
		res := calculateCombat(atk, def, User, Mob, plan, combatContext{
			sourceCanSee: true, targetCanSee: true,
		})

		if res.SwingsThrown == 0 {
			continue
		}
		if res.SwingsThrown > 1 {
			sawMultiSwingRound = true
		}
		if len(res.SwingDefences) != res.SwingsThrown {
			t.Fatalf("round %d: %d SwingDefences for %d swings thrown; every swing against a defender with a defence is contested and must append exactly one",
				i, len(res.SwingDefences), res.SwingsThrown)
		}
		if len(res.SwingEvents) != res.SwingsThrown {
			t.Fatalf("round %d: %d SwingEvents for %d swings thrown; the fixture is not resolving swings normally",
				i, len(res.SwingEvents), res.SwingsThrown)
		}
		for j, sd := range res.SwingDefences {
			if sd.Defence == "" {
				t.Fatalf("round %d swing %d: an empty defence name was appended; the best.defenseType guard is gone", i, j)
			}
		}
	}

	if !sawMultiSwingRound {
		t.Log("note: no multi-swing round occurred, so the per-swing (rather than per-round) append is only weakly covered here")
	}
}

// The other half: a defender with NO defence available appends nothing, while
// still throwing and recording swings. That is the uncontested path
// processDefenderProgression must award nothing for.
//
// Reaching it takes a third-party attack on a grappled defender:
// equipmentGatedMeleeDefences gives every defender dodge unconditionally, so
// the ONLY production route to an empty melee defence set is
// thirdPartyGrappleDefences, which keeps block alone -- and a bare-handed
// defender has no block. Grapple state is set through the Position FSM because
// IsThirdPartyAttack reads GrappleData.Partner, not the legacy fields.
func TestSwingDefences_EmptyWhenTheDefenderHasNoDefence(t *testing.T) {
	atk, def := swingDefencePopulationCombatants()

	// Clinched with a THIRD character, so the attacker is not the partner.
	const otherUserId = 42
	if err := def.Position.TransitionToClinch(
		position.GrappleData{Partner: state.ActorRef{UserId: otherUserId}},
		state.TransitionReason{Trigger: position.TriggerGrappleEntry},
	); err != nil {
		t.Fatalf("fixture: TransitionToClinch failed: %v", err)
	}
	if !IsThirdPartyAttack(atk, def) {
		t.Fatal("precondition: the attack is not third-party, so the defence set is not being emptied")
	}
	if got := thirdPartyGrappleDefences(DefenceEntriesFor(ChannelMelee, def, DefenceEntryOpts{})); len(got) != 0 {
		t.Fatalf("precondition: the filtered defence set is %v, want empty; a bare-handed defender should have no block to keep", got)
	}

	plan := buildAttackPlan(atk, def)
	res := calculateCombat(atk, def, User, Mob, plan, combatContext{
		sourceCanSee: true, targetCanSee: true,
	})

	if res.SwingsThrown == 0 {
		t.Fatal("precondition: no swings were thrown, so the assertion below would pass vacuously")
	}
	if len(res.SwingEvents) != res.SwingsThrown {
		t.Fatalf("%d SwingEvents for %d swings thrown; SwingEvents is appended unconditionally and must still track every swing",
			len(res.SwingEvents), res.SwingsThrown)
	}
	if len(res.SwingDefences) != 0 {
		t.Errorf("%d SwingDefences on an UNCONTESTED round, want 0; an empty defence name is being appended and would displace a real candidate in the Best-of", len(res.SwingDefences))
	}
}
