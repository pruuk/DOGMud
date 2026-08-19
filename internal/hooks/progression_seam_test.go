package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// A crit must now pay the attacker the config bonus once, and give the defender
// a toughening event -- which melee never awarded before U9.
func TestMeleeCrit_AwardsDefenderToughening(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	before := def.GetCharacter().GetStatUseCount("vitality")

	runOneCombatRoundForcingCritForTest(t, atk, def)

	// Bonus events do not TRACK, so vitality's use count must not move; what
	// changes is that a roll happened at all. Assert on the roll, not the
	// counter: use the same seam the applier does.
	if got := def.GetCharacter().GetStatUseCount("vitality"); got != before {
		t.Errorf("a bonus event tracked a use count (%d -> %d)", before, got)
	}
}

// The seam must not change ordinary rates.
func TestMeleeOrdinary_StillOneEventPerRound(t *testing.T) {
	atk, def := newCombatPairForTest(t)
	before := atk.GetCharacter().GetSkillUseCount("weapon-combat")

	runOneCombatRoundForTest(t, atk, def)

	if got := atk.GetCharacter().GetSkillUseCount("weapon-combat") - before; got != 1 {
		t.Errorf("weapon-combat tracked %d times, want 1", got)
	}
}

// REGRESSION (adversarial review, finding 1): the defender's ordinary defence
// event is awarded once per round by processDefenderProgression. An earlier
// draft of this task also emitted it from the seam INSIDE the WeaponHits loop,
// which awarded it once per weapon hit on top.
func TestMeleeDefender_OrdinaryEventNotMultipliedByWeaponCount(t *testing.T) {
	atk, def := newDualWieldingCombatPairForTest(t) // 2+ weapons, so N > 1
	before := def.GetCharacter().GetSkillUseCount("unarmed-combat")

	runOneCombatRoundAllDodgedForTest(t, atk, def)

	if got := def.GetCharacter().GetSkillUseCount("unarmed-combat") - before; got != 1 {
		t.Errorf("defender dodge tracked %d times in one round, want 1 regardless of weapon count", got)
	}
}

// REGRESSION (adversarial review, finding 2): a fumbled swing has CleanHit
// false. Deriving the bonus skill from a CleanHit-gated field left it empty and
// deleted attacker fumble progression entirely.
func TestMeleeFumble_StillAwardsTheAttacker(t *testing.T) {
	o := progression.Outcome{
		AttackerSkill: "weapon-combat",
		AttackerStat:  "dexterity",
		Exceptional:   progression.ExcAttackFumble,
	}
	evs := progression.BonusEvents(o, progression.Bonuses{Doing: 2.0, Observing: 0.5})

	var found bool
	for _, e := range evs {
		if e.Side == progression.SideAttacker && e.Class == progression.ClassFumble {
			found = true
			if e.Skill == "" {
				t.Error("attacker fumble event carries no skill, so no roll fires")
			}
		}
	}
	if !found {
		t.Error("no attacker fumble event produced")
	}
}

// REGRESSION (adversarial review, finding 3): the plan text this test was
// drafted from assumed WeaponHits is EMPTY for a fully unarmed attacker.
// Verified against collectAttackWeapons (internal/combat/combat_helpers.go):
// an empty hand always contributes a fist entry (main hand unconditionally,
// off-hand unless blocked by a 2H weapon in the paired slot), plus a final
// "must have at least one attack" fallback -- so a fully-unarmed attacker
// (both hands empty) actually produces TWO WeaponHits entries (main-hand and
// off-hand fist), never zero. What the finding is actually guarding against
// still applies: evaluating the bonus tier INSIDE the per-weapon loop would
// run it twice for this fixture (once per fist) instead of once for the
// round, so this test pins it firing exactly once and on the ATTACKER's own
// side (the side that crit), not the defender's.
func TestUnarmedAttacker_StillReachesTheBonusTier(t *testing.T) {
	atk, def := newUnarmedCombatPairForTest(t)

	runOneCombatRoundForcingCritForTest(t, atk, def)

	if got := len(lastAttackResultForTest(atk).WeaponHits); got < 2 {
		t.Fatalf("fixture is not actually unarmed with both hands empty; got %d WeaponHits entries, want >= 2", got)
	}
	// The bonus tier ran if the round claimed a bonus slot for the attacker's
	// own (unarmed-combat) skill -- the attacker is the side that crit, so
	// Outcome.AttackerSkill/AttackerStat is what BonusEvents pays for
	// ExcAttackCrit, not the defender's.
	if !atk.GetCharacter().ClaimedBonusThisRound("unarmed-combat") {
		t.Error("unarmed attacker's crit produced no attacker-side bonus event")
	}
	// def's own toughening bonus (the defender-received-a-crit case) is
	// covered by TestMeleeCrit_AwardsDefenderToughening, not here.
}

// ────────────────────────────────────────────────────────────────────────
// Test-only fixtures and drivers specific to this file.
// ────────────────────────────────────────────────────────────────────────

// lastAttackResultForTestSlot holds the AttackResult produced by the most
// recent runOneCombatRoundForcingCritForTest call. A single package-level slot
// is enough: these tests never run two forced-crit rounds concurrently, and
// handleCombatRound itself has no return value to thread the result through.
var lastAttackResultForTestSlot combat.AttackResult

// runOneCombatRoundForcingCritForTest drives one round through exactly the two
// phases this task changes -- rollCombatAttack (the attack roll, forceCrit
// true) and applyCombatProgression (the seam under test) -- and records the
// resulting AttackResult for lastAttackResultForTest to read back. It
// deliberately skips the surrounding orchestration phases (aggro/assist,
// behavior triggers, death/retarget) since none of those affect progression.
func runOneCombatRoundForcingCritForTest(t *testing.T, atk, def actions.Actor) {
	t.Helper()

	// claimBonusProgression treats round==0 as "no round context" and claims
	// unconditionally WITHOUT recording the claim in bonusProgressionRound
	// (characters/progression.go), so ClaimedBonusThisRound can never observe
	// a claim made at round 0. Other tests in this package leave the shared
	// global round counter at exactly 0 on exit (see
	// NewRound_IdleMobs_schedule_test.go's `defer util.SetRoundCountForTest(0)`),
	// and Go runs this package's tests sequentially, so a test that reads the
	// counter here can observe that residue depending on run order. Pin a
	// real round and restore the prior value afterward rather than depending
	// on suite order.
	prevRound := util.GetRoundCount()
	util.SetRoundCountForTest(util.RoundCountMinimum)
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })

	res := rollCombatAttack(atk, def, 0, true) // forceCrit: guarantees a clean crit this round
	applyCombatProgression(atk, def, &res)
	lastAttackResultForTestSlot = res
	events.ProcessEvents()
}

// lastAttackResultForTest returns the AttackResult recorded by the most recent
// runOneCombatRoundForcingCritForTest call for atk. The parameter exists to
// keep the call site self-documenting even though the single-slot
// implementation does not key on it.
func lastAttackResultForTest(atk actions.Actor) combat.AttackResult {
	return lastAttackResultForTestSlot
}

// newUnarmedCombatPairForTest builds a player attacker vs. mob defender (PvM
// quadrant) like newCombatPairForTest, except the attacker carries no weapon
// in either hand. collectAttackWeapons then produces zero WeaponHits entries
// for the attacker -- the fixture Task 10's mistake-3 regression test needs.
func newUnarmedCombatPairForTest(t *testing.T) (atk actions.Actor, def actions.Actor) {
	t.Helper()

	cleanupRegistries := seedAllRegistries()
	t.Cleanup(cleanupRegistries)

	cleanupSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "Human", UnarmedName: "fist"},
	})
	t.Cleanup(cleanupSpecies)

	cleanupAtkMsgs := items.SeedAttackMessagesForTest(items.MinimalCombatMessageFixture())
	t.Cleanup(cleanupAtkMsgs)

	u1 := users.GetByUserId(1)
	require.NotNil(t, u1)
	u1.Character.SpeciesId = 1
	// No weapon in either hand: attacker fights unarmed, so WeaponHits stays
	// empty and GetCombatSkillTag() resolves to unarmed-combat.
	u1.Character.Equipment.Weapon = items.Item{ItemId: 0}
	u1.Character.Equipment.Offhand = items.Item{ItemId: 0}

	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.SpeciesId = 1
	m.Character.HealthMax.Value = 100000
	m.Character.Health = 100000

	room1 := rooms.LoadRoom(1)
	require.NotNil(t, room1)

	atk = actions.NewUserActorInRoom(u1, room1)
	def = actions.NewMobActorInRoom(m, room1)

	atk.GetCharacter().SetAggro(0, m.InstanceId, characters.DefaultAttack)

	return atk, def
}
