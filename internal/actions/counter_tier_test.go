package actions

// U6b Task 10 — the counter tier's actions-side wiring:
//
//   - the defy carve-out: a defy crit COUNTER-TAUNTS instead of
//     counter-swinging, through a dedicated cost-free entry point that
//     bypasses the special-move cooldown, U8 admission cost, and aggro
//     mutation, and can never earn a counter of its own;
//   - the skill-move exits (ExecuteSkillMove consumers) feed the tier via
//     result.Defence.DefensiveCrit;
//   - ExecuteFire's reach gate: the same-room shot is counterable, the
//     cross-room shot is the ONE uncounterable attack.

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/require"
)

const (
	counterTauntWiringTargetId = 3600
	counterTauntBypassTargetId = 3601
)

// pinActionsCounterKnob pins CounterDamagePercent on top of the taunt knobs.
func pinActionsCounterKnob(t *testing.T, pct float64) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.CounterDamagePercent = configs.ConfigFloat(pct)
	configs.SetConfigForTest(t, cfg)
}

// counterSequencedRunner replays the given runners in call order (extra calls
// reuse the last), counting through *calls.
func counterSequencedRunner(t *testing.T, calls *int,
	runners ...func(float64, []contest.Entry) contest.Result) func(float64, []contest.Entry) contest.Result {
	t.Helper()
	return func(atkScore float64, entries []contest.Entry) contest.Result {
		idx := *calls
		*calls++
		if idx >= len(runners) {
			idx = len(runners) - 1
		}
		return runners[idx](atkScore, entries)
	}
}

// The defy carve-out, wired at the taunt call site: a defy crit against a
// taunt fires a COUNTER-TAUNT (not a counter-swing), and both the original
// contest and the counter's contest run through the seam — never a third.
func TestExecuteTaunt_DefyCritCounterTaunts(t *testing.T) {
	pinTauntCollapseKnobs(t)
	pinActionsCounterKnob(t, 0.5)
	t.Cleanup(func() { mobs.SetInstanceForTest(counterTauntWiringTargetId, nil) })

	attacker, target := newTauntCollapsePair(t, counterTauntWiringTargetId,
		200, 10, 120, 7)
	// newTestMob leaves Health at zero; the counter tier refuses dead
	// participants, so give both a live pool.
	attacker.Character.Health = 1000
	attacker.Character.HealthMax.Value = 1000
	target.Character.Health = 1000
	target.Character.HealthMax.Value = 1000

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(counterSequencedRunner(t, &calls,
		tauntDeterministicRunner(t, -2.5, -0.5, 2.5), // the taunt is defy-critted
		tauntDeterministicRunner(t, 0.5, 0.5, -0.5),  // the counter-taunt lands
	))
	t.Cleanup(restore)

	convBefore := attacker.Character.Conviction

	res := ExecuteTaunt(&MobActor{Mob: attacker})

	require.True(t, res.Executed, "the taunt must execute (cost %+v)", res.Cost)
	require.True(t, res.Defence.DefensiveCrit, "fixture error: the taunt was supposed to be defy-critted")
	require.Equal(t, 2, calls,
		"a defy-critted taunt must run exactly TWO contests: the taunt and the counter-taunt")

	require.True(t, res.Counter.Fired, "the defy crit must fire the counter-taunt")
	require.Positive(t, res.Counter.Damage, "the landing counter-taunt must deal conviction damage")

	// The counter-taunt's damage landed on the ORIGINAL taunter, beyond the
	// admission cost they paid for their own taunt.
	require.Less(t, attacker.Character.Conviction, convBefore-res.Cost.Charged,
		"the counter-taunt must drain the original taunter's conviction beyond their own admission cost")

	// Countered-party economy: the original taunter DEFENDED the counter-taunt
	// through the seam, and that defence was charged.
	require.NotEqual(t, characters.CostNoCharge, res.Counter.Defence.Cost.Status,
		"the original taunter's defy of the counter-taunt must be charged (countered-party economy)")
}

// The three bypasses of the carve-out, asserted directly on the entry point:
// no cooldown gate or consumption, no U8 admission cost, no aggro mutation.
func TestCounterTaunt_BypassesCooldownCostAndAggro(t *testing.T) {
	pinTauntCollapseKnobs(t)
	pinActionsCounterKnob(t, 0.5)
	t.Cleanup(func() { mobs.SetInstanceForTest(counterTauntBypassTargetId, nil) })

	// Roles: `target` taunted; `counterer` defy-critted and now counter-taunts.
	taunter, counterer := newTauntCollapsePair(t, counterTauntBypassTargetId,
		200, 10, 120, 7)
	taunter.Character.Health = 1000
	taunter.Character.HealthMax.Value = 1000
	counterer.Character.Health = 1000
	counterer.Character.HealthMax.Value = 1000

	// BYPASS 1 (cooldown): the counterer's special-move cooldown is HOT. A
	// counter-taunt is free — it neither checks nor consumes the cooldown.
	require.True(t, counterer.Character.TryCooldown("special-move", "5 rounds"))
	cooldownBefore := counterer.Character.GetCooldown("special-move")
	require.Positive(t, cooldownBefore, "fixture error: cooldown must be hot")

	// BYPASS 3 (aggro): snapshot both sides' aggro. newTestMob seeds a default
	// aggro on the counterer; clear it so any mutation is visible.
	counterer.Character.Aggro = nil
	taunterAggroBefore := *taunter.Character.Aggro // newTauntCollapsePair sets it

	convBefore := counterer.Character.Conviction
	targetConvBefore := taunter.Character.Conviction

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(counterSequencedRunner(t, &calls,
		tauntDeterministicRunner(t, 0.5, 0.5, -0.5), // the counter-taunt lands
	))
	t.Cleanup(restore)

	res := executeCounterTaunt(&counterer.Character, &taunter.Character)

	require.True(t, res.Fired, "the counter-taunt must fire despite a hot special-move cooldown")
	require.Positive(t, res.Damage)
	require.Less(t, taunter.Character.Conviction, targetConvBefore,
		"the counter-taunt must deal conviction damage to the original taunter")

	// BYPASS 1 asserted: fired while hot, and the cooldown was not touched.
	require.Equal(t, cooldownBefore, counterer.Character.GetCooldown("special-move"),
		"the counter-taunt must not consume or reset the special-move cooldown")

	// BYPASS 2 asserted: the counterer paid NO admission cost — their
	// conviction is untouched (the counter is free for the counterer; the
	// TAUNTER pays for defending it, which is the countered-party economy).
	require.Equal(t, convBefore, counterer.Character.Conviction,
		"the counter-taunt must not charge the counterer any admission cost")

	// BYPASS 3 asserted: no aggro mutation on either side.
	require.Nil(t, counterer.Character.Aggro,
		"the counter-taunt must not seed aggro on the counterer")
	require.Equal(t, taunterAggroBefore.UserId, taunter.Character.Aggro.UserId)
	require.Equal(t, taunterAggroBefore.MobInstanceId, taunter.Character.Aggro.MobInstanceId,
		"the counter-taunt must not re-point the original taunter's aggro")
}

// A counter-taunt can never earn a counter: even when the original taunter
// defy-crits the counter-taunt, exactly ONE contest runs from the entry point.
func TestCounterTaunt_NeverEarnsCounter(t *testing.T) {
	pinTauntCollapseKnobs(t)
	pinActionsCounterKnob(t, 0.5)
	t.Cleanup(func() { mobs.SetInstanceForTest(counterTauntBypassTargetId, nil) })

	taunter, counterer := newTauntCollapsePair(t, counterTauntBypassTargetId,
		200, 10, 120, 7)
	taunter.Character.Health = 1000
	taunter.Character.HealthMax.Value = 1000
	counterer.Character.Health = 1000
	counterer.Character.HealthMax.Value = 1000

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(counterSequencedRunner(t, &calls,
		tauntDeterministicRunner(t, -2.5, -0.5, 2.5), // EVERY contest defy-crits
	))
	t.Cleanup(restore)

	res := executeCounterTaunt(&counterer.Character, &taunter.Character)
	require.True(t, res.Fired)
	require.Equal(t, 1, calls,
		"a crit-defended counter-taunt must not chain another counter — counters never recurse")
	require.Zero(t, res.Damage, "a defy-critted counter-taunt deals nothing")
}

// The ranged reach gate, through the REAL ExecuteFire path: a same-room shot
// that is crit-defended is countered; the cross-room shot is not — the one
// uncounterable attack, as a property of the weapon.
func TestFireCounter_ReachGate(t *testing.T) {
	pinFireSeamKnobs(t)
	pinActionsCounterKnob(t, 0.5)

	fireOnce := func(t *testing.T, defenderRoomId int, rest string) (FireResult, *characters.Character, int) {
		t.Helper()
		instanceId, cleanup := seedFireMobInRoom(t, defenderRoomId, 100)
		t.Cleanup(cleanup)
		mob := mobs.GetInstance(instanceId)
		require.NotNil(t, mob)
		mob.Character.Stats.Strength.Base = 100
		mob.Character.Stats.Strength.Recalculate()
		mob.Character.Stamina = 500
		mob.Character.StaminaMax.Value = 500

		calls := 0
		restore := combat.SetChannelAttackContestRunnerForTest(counterSequencedRunner(t, &calls,
			tauntDeterministicRunner(t, -2.5, -0.5, 2.5), // the shot is crit-defended
			tauntDeterministicRunner(t, 0.5, 0.5, -0.5),  // the counter-swing lands
		))
		t.Cleanup(restore)

		char := fireAttacker()
		char.Health = 100000
		char.HealthMax.Value = 100000
		char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
		actor := newStubActor(char, rooms.LoadRoom(1))

		res := ExecuteFire(actor, rest)
		require.True(t, res.Executed, "the shot must execute (cost %+v)", res.Cost)
		require.True(t, res.MoveResult.Defence.DefensiveCrit,
			"fixture error: the shot was supposed to be crit-defended")
		return res, char, calls
	}

	// Same room: the crit defence counters the shooter.
	res, shooter, calls := fireOnce(t, 1, "skeleton")
	require.False(t, res.CrossRoom)
	require.Equal(t, 2, calls,
		"a crit-defended same-room shot must run two contests: the shot and the counter-swing")
	require.Less(t, shooter.Health, 100000,
		"the same-room counter-swing must damage the shooter")

	// Cross room: the ONE uncounterable attack. One contest, no damage back.
	res2, shooter2, calls2 := fireOnce(t, 2, "skeleton north")
	require.True(t, res2.CrossRoom)
	require.Equal(t, 1, calls2,
		"a cross-room shot must never be countered — it is the one uncounterable attack")
	require.Equal(t, 100000, shooter2.Health,
		"no counter damage may reach a cross-room shooter")
}

// Every ExecuteSkillMove consumer feeds the tier: a crit-defended special
// move earns the defender a counter-swing at the mover, in BOTH directions
// (the mover here is a mob, so this is also a mob-gets-countered case; the
// hooks spell tests pin mob-defender-counters-player).
func TestSkillMoveExit_DefensiveCritCounters(t *testing.T) {
	pinTauntCollapseKnobs(t)
	pinActionsCounterKnob(t, 0.5)
	t.Cleanup(func() { mobs.SetInstanceForTest(counterTauntWiringTargetId, nil) })

	// ExecuteTrip's anatomy gate needs a species with legs.
	cleanupSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		8102: {SpeciesId: 8102, Name: "humanoid", BodyParts: []string{"arms", "hands", "legs"}},
	})
	t.Cleanup(cleanupSpecies)

	mover, target := newTauntCollapsePair(t, counterTauntWiringTargetId,
		200, 10, 120, 7)
	mover.Character.SpeciesId = 8102
	mover.Character.Health = 100000
	mover.Character.HealthMax.Value = 100000
	mover.Character.Stats.Strength.Base = 100
	mover.Character.Stats.Strength.Recalculate()
	target.Character.Health = 100000
	target.Character.HealthMax.Value = 100000
	target.Character.Stats.Strength.Base = 100
	target.Character.Stats.Strength.Recalculate()

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(counterSequencedRunner(t, &calls,
		tauntDeterministicRunner(t, -2.5, -0.5, 2.5), // the trip is crit-defended
		tauntDeterministicRunner(t, 0.5, 0.5, -0.5),  // the counter-swing lands
	))
	t.Cleanup(restore)

	res := ExecuteTrip(&MobActor{Mob: mover})
	require.True(t, res.Executed, "the trip must execute (cost %+v)", res.Cost)
	require.True(t, res.MoveResult.Defence.DefensiveCrit,
		"fixture error: the trip was supposed to be crit-defended")

	require.Equal(t, 2, calls,
		"a crit-defended skill move must run two contests: the move and the counter-swing")
	require.Less(t, mover.Character.Health, 100000,
		"the counter-swing must damage the one who attempted the move")
}

// U6b playtest closeout (2026-08-19): the defy counter-taunt exchange was
// never decisively observed live, so the dispatch is pinned here: when a
// PLAYER's taunt is defy-critted, counterTauntExit must put the retort's
// taunter-audience line (rendered from the counter-defy pool) in front of
// that player — the exchange can never resolve silently for the one player
// who can see it.
func TestCounterTaunt_RetortNarratesToThePlayerTaunter(t *testing.T) {
	pinTauntCollapseKnobs(t)
	pinActionsCounterKnob(t, 0.5)

	restoreMessages := items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{
		items.DefenseCounterDefy: counterRetortMessageFixture(),
	})
	defer restoreMessages()

	// Player taunter vs mob counterer, through the REAL ExecuteTaunt.
	actor, char, target := newRhetoricActor(t, true, 100, 0)
	char.Health = 1000
	char.HealthMax.Value = 1000
	target.Health = 1000
	target.HealthMax.Value = 1000

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(counterSequencedRunner(t, &calls,
		tauntDeterministicRunner(t, -2.5, -0.5, 2.5), // the taunt is defy-critted
		tauntDeterministicRunner(t, 0.5, 0.5, -0.5),  // the counter-taunt lands
	))
	t.Cleanup(restore)

	events.DrainQueuedMessagesForTest(actor.GetUserId())

	res := ExecuteTaunt(actor)

	require.True(t, res.Executed, "the taunt must execute (cost %+v)", res.Cost)
	require.True(t, res.Defence.DefensiveCrit, "fixture error: the taunt was supposed to be defy-critted")
	require.True(t, res.Counter.Fired, "the defy crit must fire the counter-taunt")

	lines := events.DrainQueuedMessagesForTest(actor.GetUserId())
	retortLines := []string{}
	for _, line := range lines {
		if strings.Contains(line, "RETORT!") {
			retortLines = append(retortLines, line)
		}
	}
	require.Len(t, retortLines, 1,
		"the countered player taunter must receive exactly one retort line; got %v", lines)
	require.Contains(t, strings.ToLower(retortLines[0]), "counterretort-attacker",
		"the taunter's line must be the counter-defy pool's attacker-audience render")
	require.Contains(t, retortLines[0], char.Name,
		"the retort must name the original taunter")
}

// counterRetortMessageFixture seeds a marked counter-defy pool for the
// dispatch pin above.
func counterRetortMessageFixture() *items.DefenseMessageGroup {
	mk := func(band string) items.DefenseOptions {
		messages := func(audience string) items.MessageOptions {
			result := make(items.MessageOptions, 5)
			for i := range result {
				result[i] = items.ItemMessage("counterretort-" + audience + "-" + band +
					" {defender} turns the jeer back on {attacker}")
			}
			return result
		}
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: messages("defender"),
			ToAttacker: messages("attacker"),
			ToRoom:     messages("room"),
		}}
	}
	return &items.DefenseMessageGroup{OptionId: items.DefenseCounterDefy, Options: items.DefenseIntensity{
		items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
	}}
}
