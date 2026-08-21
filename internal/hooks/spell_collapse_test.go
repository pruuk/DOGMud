package hooks

// U6b Task 4 — the spell-channel collapse. The old shape ran a hit gate
// (attacker Willpower + spellcasting x SkillWeight x a config skill factor —
// the x15-per-rank score — against the defender's RAW stat, skill x0) and only
// consulted the channel defence on non-crit hits, inside the effect appliers.
// After the collapse the channel defence IS the contest: the resolver runs
// combat.ResolveChannelAttack ONCE and threads the ChannelDefenceResult
// through the appliers.
//
// These tests drive the REAL seam (cost admission, progression, the bonus
// tier all live) against a deterministic contest core injected via
// combat.SetChannelAttackContestRunnerForTest — stubbing the package seam
// variable would bypass exactly the seam side effects Task 4 must prove.

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// spellContestAttackWin is the threaded-result fixture for "the one contest
// ran and the attack won": full damage, no crit, nothing defended.
func spellContestAttackWin() combat.ChannelDefenceResult {
	return combat.ChannelDefenceResult{DamageMultiplier: 1}
}

// spellContestAttackCrit is the attack-win fixture with the seam's
// margin-derived crit verdict set.
func spellContestAttackCrit() combat.ChannelDefenceResult {
	return combat.ChannelDefenceResult{DamageMultiplier: 1, AttackerCrit: true}
}

// deterministicContestRunner builds a contest.Result whose normalized attack
// margin is exactly normMargin sigma (attack-positive; negative values are a
// defence win by that many sigma). Both roll ZScores are set explicitly so
// neither side trips a fumble or a defence crit by accident.
func deterministicContestRunner(t *testing.T, normMargin, atkZ, defZ float64) func(float64, []contest.Entry) contest.Result {
	t.Helper()
	return func(atkScore float64, entries []contest.Entry) contest.Result {
		require.NotEmpty(t, entries, "the seam must send at least one defence entry to the contest")
		stdDev := dice.StdDevFor(atkScore)
		if stdDev <= 0 {
			stdDev = 15
		}
		margin := normMargin * stdDev * math.Sqrt2
		return contest.Result{
			Contested: true,
			Success:   margin > 0,
			Winner:    entries[0].Name,
			Margin:    margin,
			AttackRoll: dice.RollResult{
				Value: atkScore + atkZ*stdDev, Mean: atkScore,
				StdDev: stdDev, ZScore: atkZ,
			},
			DefenseRoll: dice.RollResult{
				Value: atkScore + atkZ*stdDev - margin, Mean: entries[0].Score,
				StdDev: stdDev, ZScore: defZ,
			},
		}
	}
}

func mentalHarmSpellForCollapseTest() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "mind-lance", Name: "Mind Lance", Type: spells.HarmSingle,
		EffectType: "damage", TargetDefenseType: "mental",
		DamageMultiplier: 1.0, EffectMagnitude: 30,
		Schools: []string{spells.SchoolMental},
	}
}

// ONE contest, and quell IS it. The old hit gate read the defender's RAW
// Willpower (skill x0), so two defenders with equal Willpower were equally
// hard to hit regardless of spellcasting skill — impossible to distinguish.
// Now the defender enters through GetDefenseScoreFor, so the skilled
// defender's quell score is strictly higher. The attacker's score is the
// AttackSide's stat + rank x SkillWeight — the deleted hit-gate helper's
// extra skill-factor term (x15 per rank at shipped knobs) is gone.
func TestSpellResolution_OneContest_QuellAlwaysConsulted(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	room := roomForCollapseTest(t)
	caster.Character.Skills = map[string]int{"spellcasting": 40}
	target.Character.Conviction = 500
	target.Character.ConvictionMax.Value = 500

	spell := mentalHarmSpellForCollapseTest()

	var capturedAtk float64
	var capturedEntries []contest.Entry
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		capturedAtk = atkScore
		capturedEntries = append([]contest.Entry{}, entries...)
		return deterministicContestRunner(t, 0.5, 0.5, -0.5)(atkScore, entries)
	})
	t.Cleanup(restore)

	scoreWithDefenderSkill := func(rank int) float64 {
		capturedEntries = nil
		target.Character.Skills = map[string]int{"spellcasting": rank}
		side := spellAttackSideFor(spell, caster.Character)
		resolveAgainstPlayer(caster, target, room, spell, side, spell.EffectMagnitude)
		require.Len(t, capturedEntries, 1, "a mental spell must face exactly one defence: quell")
		require.Equal(t, characters.DefenseQuell, capturedEntries[0].Name)
		return capturedEntries[0].Score
	}

	unskilled := scoreWithDefenderSkill(0)
	skilled := scoreWithDefenderSkill(100)
	require.Greater(t, skilled, unskilled,
		"the defender's spellcasting skill must raise the quell score in the ONE contest -- "+
			"the old gate read only raw Willpower and could not tell these defenders apart")

	skillWeight := float64(configs.GetBalanceConfig().SkillWeight)
	wantAtk := float64(caster.Character.Stats.Willpower.ValueAdj) + 40*skillWeight
	require.Equal(t, wantAtk, capturedAtk,
		"attack score must be stat + rank x SkillWeight; the deleted x15 skill-factor term must not survive")
}

// A spell crit must be beatable by quell. The old code decided isCrit from the
// hit gate BEFORE any defence was contested, then skipped quell entirely on a
// crit (the "crit-skips-quell" defect). With one contest, a defensive-crit
// outcome means the attack LOST: no crit, no damage, and the heavy-band
// defence triad narrates the stop.
func TestSpellCrit_FacesQuell(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	room := roomForCollapseTest(t)
	target.Character.Health = 100
	target.Character.HealthMax.Value = 100

	// Defence wins by 2.5 sigma: past DefenseCritBar, a full negation.
	restore := combat.SetChannelAttackContestRunnerForTest(deterministicContestRunner(t, -2.5, 0.5, 2.5))
	t.Cleanup(restore)

	spell := mentalHarmSpellForCollapseTest()
	drainChannelRoutingQueues(1, 2)
	side := spellAttackSideFor(spell, caster.Character)
	resolveAgainstPlayer(caster, target, room, spell, side, spell.EffectMagnitude)

	require.Equal(t, 100, target.Character.Health, "a defensive crit must fully negate the spell's damage")
	for userID, lines := range drainChannelRoutingQueues(1, 2) {
		joined := strings.ToLower(strings.Join(lines, " | "))
		require.NotContains(t, joined, "crit", "user %d: a defended cast must never narrate an attacker crit", userID)
		require.NotContains(t, joined, "fizzle", "user %d: the fizzle strings are deleted", userID)
		require.Contains(t, joined, "heavy-", "user %d: the defensive crit must speak the heavy-band defence triad", userID)
	}
}

// Fizzle is now a partial-damage defence outcome (Assumption 1): a defended,
// non-crit cast deals reduced-but-nonzero damage per the shared
// defenceDamageMultiplier curve, and the defence triad narrates it — the
// "Your X fizzles against Y." strings are deleted.
func TestSpellDefendedCast_DealsPartialDamage(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	room := roomForCollapseTest(t)
	target.Character.Health = 1000
	target.Character.HealthMax.Value = 1000

	// Defence wins by 0.3 sigma: a rolled defensive win well short of the
	// crit bar, so the multiplier is on the open (0, 0.5) stretch.
	restore := combat.SetChannelAttackContestRunnerForTest(deterministicContestRunner(t, -0.3, 0.2, 0.3))
	t.Cleanup(restore)

	spell := mentalHarmSpellForCollapseTest()
	drainChannelRoutingQueues(1, 2)
	side := spellAttackSideFor(spell, caster.Character)
	resolveAgainstPlayer(caster, target, room, spell, side, spell.EffectMagnitude)

	require.Less(t, target.Character.Health, 1000,
		"a defended, non-crit cast must deal reduced-but-nonzero damage, not fizzle to nothing")
	for userID, lines := range drainChannelRoutingQueues(1, 2) {
		joined := strings.ToLower(strings.Join(lines, " | "))
		require.NotContains(t, joined, "fizzle", "user %d: the fizzle strings are deleted", userID)
	}
}

// physicalHarmSpellForCollapseTest declares target_defense_type physical, so
// the contest is answered by dodge — whose ordinary defence award trains
// dexterity/unarmed-combat, keeping willpower's use count clean for the
// crit-received assertions below (both spell channels toughen willpower:
// channelDamageChannel maps them to "magical").
func physicalHarmSpellForCollapseTest() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "stone-lash", Name: "Stone Lash", Type: spells.HarmSingle,
		EffectType: "damage", TargetDefenseType: "physical",
		DamageMultiplier: 1.0, EffectMagnitude: 30,
		Schools: []string{spells.SchoolElemental},
	}
}

// enableObservedCritProgression turns on the config the bonus tier's
// ClassObserved cell is gated behind (applyBonusProgression skips on
// UseSkillProgression false or a zero Observing multiplier). Mirrors
// enableMobProgression, minus the mob knobs — the defenders here are players.
func enableObservedCritProgression(t *testing.T) {
	t.Helper()
	prevBal := configs.GetBalanceConfig()
	prevGp := configs.GetGamePlayConfig()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{
		"GamePlay.UseSkillProgression":         true,
		"Balance.ObservedCritProgressionBonus": 1.0,
	}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"GamePlay.UseSkillProgression":         bool(prevGp.UseSkillProgression),
			"Balance.ObservedCritProgressionBonus": float64(prevBal.ObservedCritProgressionBonus),
		})
	})
}

// roomForCollapseTest returns seeded room 1 (see seedAllRegistries).
func roomForCollapseTest(t *testing.T) *rooms.Room {
	t.Helper()
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	return room
}

// mobInstanceForCollapseTest returns seeded mob instance 100.
func mobInstanceForCollapseTest(t *testing.T) *mobs.Mob {
	t.Helper()
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	return mob
}

// The seam's bonus tier is now the ONLY crit-received source for spell casts:
// the U9-era direct toughening blocks in resolveAgainstPlayer and
// resolveMobSpellAgainstPlayer were deleted with the collapse. Mirroring
// TestMeleeCritReceived_TracksDefenderVitality, a forced spell crit must move
// the defender's willpower use count by EXACTLY one — the ClassObserved event
// tracks before rolling, and a surviving duplicate block (or a second contest)
// would show up here as a second track once the once-per-round dedupe is gone.
func TestSpellCritReceived_TracksDefenderWillpowerOnce(t *testing.T) {
	enableObservedCritProgression(t)
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	room := roomForCollapseTest(t)
	target.Character.Health = 100000
	target.Character.HealthMax.Value = 100000

	// Attack wins by 4 sigma: past every clamped pair bar (CritBarCeiling
	// included), with a clean +2z attack roll so no fumble fires.
	restore := combat.SetChannelAttackContestRunnerForTest(deterministicContestRunner(t, 4, 2, -1))
	t.Cleanup(restore)

	spell := physicalHarmSpellForCollapseTest()
	before := target.Character.GetStatUseCount("willpower")
	side := spellAttackSideFor(spell, caster.Character)
	resolveAgainstPlayer(caster, target, room, spell, side, spell.EffectMagnitude)

	if got := target.Character.GetStatUseCount("willpower") - before; got != 1 {
		t.Errorf("defender willpower use count changed by %d for one spell crit, want 1 -- "+
			"crit-received must fire exactly once, inside the seam's bonus tier", got)
	}
}

// Same exactly-once proof for the mob-caster path, whose own U9-era direct
// toughening block was the second deletion site.
func TestMobSpellCritReceived_TracksDefenderWillpowerOnce(t *testing.T) {
	enableObservedCritProgression(t)
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	caster := mobInstanceForCollapseTest(t)
	target := users.GetByUserId(1)
	room := roomForCollapseTest(t)
	target.Character.Health = 100000
	target.Character.HealthMax.Value = 100000

	restore := combat.SetChannelAttackContestRunnerForTest(deterministicContestRunner(t, 4, 2, -1))
	t.Cleanup(restore)

	spell := physicalHarmSpellForCollapseTest()
	before := target.Character.GetStatUseCount("willpower")
	side := spellAttackSideFor(spell, &caster.Character)
	resolveMobSpellAgainstPlayer(caster, target, room, spell, side, spell.EffectMagnitude)

	if got := target.Character.GetStatUseCount("willpower") - before; got != 1 {
		t.Errorf("defender willpower use count changed by %d for one mob spell crit, want 1 -- "+
			"crit-received must fire exactly once, inside the seam's bonus tier", got)
	}
}
