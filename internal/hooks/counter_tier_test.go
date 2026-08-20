package hooks

// U6b Task 10 — the counter tier's hooks-side wiring:
//
//   - the spell exits (all four quadrants; both directions are pinned here so
//     mobs don't get a counter immunity nobody decided),
//   - melee riposte's damage fraction now reads CounterDamagePercent (same
//     shipped 0.5 — behaviour unchanged),
//   - the melee trio (riposte / auto-trip / auto-bash) still fires from the
//     melee swing path and ONLY there, and never chains into the tier.

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// pinCounterTierKnobs pins CounterDamagePercent. The dice package caches its
// spread factor at init, so damage assertions below use sampled means.
func pinCounterTierKnobs(t *testing.T, counterPct float64) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.CounterDamagePercent = configs.ConfigFloat(counterPct)
	configs.SetConfigForTest(t, cfg)
}

// sequencedContestRunner replays the given per-call results in order; extra
// calls reuse the last one. It counts calls through *calls.
func sequencedContestRunner(t *testing.T, calls *int,
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

func critEffectFighter(name string, str int) *characters.Character {
	c := characters.New()
	c.Name = name
	c.Stats.Strength.Base = str
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Base = 100
	c.Stats.Dexterity.Recalculate()
	c.Health = 100000
	c.HealthMax.Value = 100000
	c.Stamina = 500
	c.StaminaMax.Value = 500
	return c
}

// Riposte now reads the CounterDamagePercent knob where the 0.5 literal used
// to be: at the shipped 0.5 the damage is bit-identical to the old formula,
// at 1.0 it doubles, and at 0 (the documented off-switch) no riposte fires —
// the pipeline's itemMult<=0 fallback of 0.30 must never leak in.
func TestRiposte_ReadsCounterDamageKnob(t *testing.T) {
	// dice.RollStat variance cannot be zeroed from configs; use sampled
	// means (N=200: standard error ~1% of the mean).
	const samples = 200
	meanRiposteAt := func(t *testing.T, pct float64) float64 {
		t.Helper()
		pinCounterTierKnobs(t, pct)
		attacker := critEffectFighter("Aggressor", 100)
		defender := critEffectFighter("Fencer", 100)
		total := 0
		for i := 0; i < samples; i++ {
			attacker.Health = 100000
			res := applyCritEffects(attacker, defender,
				combat.AttackResult{ParryCritDetected: true}, nil)
			require.True(t, res.Riposte)
			require.Positive(t, res.RiposteDamage)
			total += res.RiposteDamage
		}
		return float64(total) / samples
	}

	// Behaviour unchanged at the shipped 0.5: the sampled mean must sit on
	// the old literal's damage mean (raw at itemMult 0.5, mitigated).
	meanHalf := meanRiposteAt(t, 0.5)
	defender := critEffectFighter("Fencer", 100)
	attacker := critEffectFighter("Aggressor", 100)
	wantRaw := combat.CalcRawDamage(defender.Stats.Strength.ValueAdj,
		defender.GetCombatSkillLevel(), 0.5, combat.ChannelPhysical)
	wantMean := combat.ApplyMitigation(wantRaw, attacker.GetPhysicalMitigation(),
		combat.MitigationCap(combat.ChannelPhysical))
	require.InDelta(t, wantMean, meanHalf, wantMean*0.10,
		"riposte at knob 0.5 must keep the old 0.5 literal's damage mean")

	// And the knob prices it: 2.0 is four times 0.5.
	meanDouble := meanRiposteAt(t, 2.0)
	ratio := meanDouble / meanHalf
	require.Greater(t, ratio, 3.0, "riposte damage must scale with CounterDamagePercent")
	require.Less(t, ratio, 5.0, "riposte damage must scale with CounterDamagePercent")

	// 0 is the documented off-switch: no riposte, and never a 0.30 fallback.
	pinCounterTierKnobs(t, 0)
	attackerOff := critEffectFighter("Aggressor", 100)
	defenderOff := critEffectFighter("Fencer", 100)
	resOff := applyCritEffects(attackerOff, defenderOff,
		combat.AttackResult{ParryCritDetected: true}, nil)
	require.False(t, resOff.Riposte,
		"CounterDamagePercent 0 must disable riposte, not fall back to 0.30")
	require.Zero(t, resOff.RiposteDamage)
}

// The melee trio is untouched: parry->riposte stays UNCONTESTED (no seam
// contest at all), dodge->auto-trip and block->auto-bash still fire from the
// melee swing path carrying IsCounter, and even when the countered attacker
// crit-defends those follow-ups the counter tier never chains a third swing.
func TestCounter_MeleeTrioUnchanged(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)

	// Parry crit alone: riposte fires with ZERO contests — the melee riposte
	// path did not become seam-routed.
	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(
		sequencedContestRunner(t, &calls, alwaysDefensiveCritContest(t)))
	t.Cleanup(restore)

	attacker := critEffectFighter("Aggressor", 100)
	defender := critEffectFighter("Fencer", 100)
	res := applyCritEffects(attacker, defender,
		combat.AttackResult{ParryCritDetected: true}, nil)
	require.True(t, res.Riposte)
	require.Zero(t, calls, "riposte must stay uncontested (no seam contest)")

	// Dodge + block crits: exactly one contest each (the counter-moves' own),
	// both marked IsCounter, and the tier never fires a further swing off
	// them even though every contest here resolves as a defensive crit.
	calls = 0
	attacker2 := critEffectFighter("Aggressor", 100)
	defender2 := critEffectFighter("Fencer", 100)
	res2 := applyCritEffects(attacker2, defender2,
		combat.AttackResult{DodgeCritDetected: true, BlockCritDetected: true}, nil)
	require.True(t, res2.AutoTrip)
	require.True(t, res2.AutoBash)
	require.Equal(t, 2, calls,
		"auto-trip + auto-bash must run exactly one contest each; a third is the tier recursing")
	require.True(t, res2.TripResult.IsCounter, "auto-trip must ride the seam as a counter")
	require.True(t, res2.BashResult.IsCounter, "auto-bash must ride the seam as a counter")
}

// alwaysDefensiveCritContest resolves every contest as a decisive defensive
// win (defensive crit), for recursion-proofing.
func alwaysDefensiveCritContest(t *testing.T) func(float64, []contest.Entry) contest.Result {
	t.Helper()
	return deterministicContestRunner(t, -2.5, -0.5, 2.5)
}

// attackWinContest resolves every contest as a clean attack win.
func attackWinContest(t *testing.T) func(float64, []contest.Entry) contest.Result {
	t.Helper()
	return deterministicContestRunner(t, 0.5, 0.5, -0.5)
}

// Spell quadrant: player caster, MOB defender. The mob's quell/dodge crit
// counters the PLAYER — the direction a player-attacker-only wiring list
// would have silently dropped, handing mobs a counter immunity nobody
// decided.
func TestSpellCounter_MobDefenderCountersPlayerCaster(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	caster := users.GetByUserId(1)
	mob := mobInstanceForCollapseTest(t)
	room := roomForCollapseTest(t)
	caster.Character.Health = 100000
	caster.Character.HealthMax.Value = 100000
	caster.Character.Stamina = 500
	caster.Character.StaminaMax.Value = 500

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(sequencedContestRunner(t, &calls,
		alwaysDefensiveCritContest(t), // the cast is crit-defended
		attackWinContest(t),           // the counter-swing lands
	))
	t.Cleanup(restore)

	spell := physicalHarmSpellForCollapseTest()
	side := spellAttackSideFor(spell, caster.Character)
	fumbled := resolveAgainstMob(caster, mob, room, spell, side, spell.EffectMagnitude)
	require.False(t, fumbled)

	require.Equal(t, 2, calls,
		"a crit-defended cast must run exactly two contests: the cast and the counter-swing")
	require.Less(t, caster.Character.Health, 100000,
		"the MOB defender's counter must damage the PLAYER caster")
}

// Spell quadrant: MOB caster, player defender. The player's crit defence
// counters the mob.
func TestSpellCounter_PlayerDefenderCountersMobCaster(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	caster := mobInstanceForCollapseTest(t)
	target := users.GetByUserId(1)
	room := roomForCollapseTest(t)
	caster.Character.Health = 100000
	caster.Character.HealthMax.Value = 100000
	target.Character.Health = 100000
	target.Character.HealthMax.Value = 100000
	target.Character.Stamina = 500
	target.Character.StaminaMax.Value = 500
	target.Character.Stats.Strength.Base = 100
	target.Character.Stats.Strength.Recalculate()

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(sequencedContestRunner(t, &calls,
		alwaysDefensiveCritContest(t),
		attackWinContest(t),
	))
	t.Cleanup(restore)

	spell := physicalHarmSpellForCollapseTest()
	side := spellAttackSideFor(spell, &caster.Character)
	resolveMobSpellAgainstPlayer(caster, target, room, spell, side, spell.EffectMagnitude)

	require.Equal(t, 2, calls,
		"a crit-defended mob cast must run exactly two contests: the cast and the counter-swing")
	require.Less(t, caster.Character.Health, 100000,
		"the PLAYER defender's counter must damage the MOB caster")
}

// Both player-vs-player and mob-vs-mob spell exits also feed the tier.
func TestSpellCounter_PlayerVsPlayerAndMobVsMob(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	room := roomForCollapseTest(t)
	spell := physicalHarmSpellForCollapseTest()

	// Player vs player.
	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	caster.Character.Health = 100000
	caster.Character.HealthMax.Value = 100000
	target.Character.Health = 100000
	target.Character.HealthMax.Value = 100000
	target.Character.Stamina = 500
	target.Character.StaminaMax.Value = 500

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(sequencedContestRunner(t, &calls,
		alwaysDefensiveCritContest(t),
		attackWinContest(t),
	))
	t.Cleanup(restore)

	side := spellAttackSideFor(spell, caster.Character)
	resolveAgainstPlayer(caster, target, room, spell, side, spell.EffectMagnitude)
	require.Equal(t, 2, calls)
	require.Less(t, caster.Character.Health, 100000,
		"the defending player's counter must damage the player caster")

	// Mob vs mob.
	mobCaster := mobInstanceForCollapseTest(t)
	mobCaster.Character.Health = 100000
	mobCaster.Character.HealthMax.Value = 100000
	mobTarget := critEffectFighter("Counter Golem", 100)
	mobDefender := &mobs.Mob{MobId: 2, InstanceId: 999, Character: *mobTarget}
	mobDefender.Character.Buffs = buffs.New()

	calls = 0
	side2 := spellAttackSideFor(spell, &mobCaster.Character)
	resolveMobSpellAgainstMob(mobCaster, mobDefender, room, spell, side2, spell.EffectMagnitude)
	require.Equal(t, 2, calls,
		"the mob-vs-mob exit must feed the tier too")
	require.Less(t, mobCaster.Character.Health, 100000,
		"the defending mob's counter must damage the mob caster")
}

// U6b playtest closeout (2026-08-19): the quell lane's counter narration was
// never observed live (the caster mob never cast). The dispatch is pinned
// here: on a decisive defensive crit against a cast, the countered CASTER
// must receive the counter line, rendered from the counter-quell pool (both
// spell channels share it), never silently dropped.
func TestSpellCounter_NarrationReachesCasterFromCounterQuellPool(t *testing.T) {
	pinCounterTierKnobs(t, 0.5)
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{
		items.DefenseCounterQuell: counterQuellNarrationFixture(),
	})
	defer restoreMessages()

	caster := users.GetByUserId(1)
	mob := mobInstanceForCollapseTest(t)
	room := roomForCollapseTest(t)
	caster.Character.Health = 100000
	caster.Character.HealthMax.Value = 100000
	caster.Character.Stamina = 500
	caster.Character.StaminaMax.Value = 500

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(sequencedContestRunner(t, &calls,
		alwaysDefensiveCritContest(t), // the cast is crit-defended
		attackWinContest(t),           // the counter-swing lands
	))
	t.Cleanup(restore)

	spell := physicalHarmSpellForCollapseTest()
	side := spellAttackSideFor(spell, caster.Character)
	events.DrainQueuedMessagesForTest(caster.UserId)
	fumbled := resolveAgainstMob(caster, mob, room, spell, side, spell.EffectMagnitude)
	require.False(t, fumbled)
	require.Equal(t, 2, calls, "the cast and the counter-swing")

	lines := events.DrainQueuedMessagesForTest(caster.UserId)
	counterLines := []string{}
	for _, line := range lines {
		if strings.Contains(line, "COUNTER!") {
			counterLines = append(counterLines, line)
		}
	}
	require.Len(t, counterLines, 1,
		"the countered caster must receive exactly one counter line; got %v", lines)
	require.Contains(t, strings.ToLower(counterLines[0]), "counterquell-attacker",
		"the caster's line must be the counter-quell pool's attacker-audience render")
	require.Contains(t, counterLines[0], mob.Character.Name,
		"the counter line must name the countering defender")
}

// counterQuellNarrationFixture seeds a marked counter-quell pool for the
// dispatch pin above.
func counterQuellNarrationFixture() *items.DefenseMessageGroup {
	mk := func(band string) items.DefenseOptions {
		messages := func(audience string) items.MessageOptions {
			result := make(items.MessageOptions, 5)
			for i := range result {
				result[i] = items.ItemMessage("counterquell-" + audience + "-" + band +
					" {defender} steps through the gap {attacker} left")
			}
			return result
		}
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: messages("defender"),
			ToAttacker: messages("attacker"),
			ToRoom:     messages("room"),
		}}
	}
	return &items.DefenseMessageGroup{OptionId: items.DefenseCounterQuell, Options: items.DefenseIntensity{
		items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
	}}
}
