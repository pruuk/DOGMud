package actions

// U6b Task 5 — the taunt collapse. The old shape ran a hit gate
// (runTauntContest: Charisma + rhetoric x SkillWeight, conviction-depleted,
// against Willpower + rhetoric x SkillWeight) and only consulted the channel
// defence on non-crit hits, so the defender's Wil + rhetoric x 5 was contested
// TWICE. After the collapse combat.ResolveChannelAttack IS the contest:
// ExecuteTaunt builds an AttackSide (charisma + RAW rhetoric rank, with the
// conviction-depletion multiplier on Mult) and consumes the seam's
// AttackerCrit/AttackerFumble verdicts and DamageMultiplier.
//
// These tests drive the REAL seam (cost admission, defence progression, the
// bonus tier all live) against a deterministic contest core injected via
// combat.SetChannelAttackContestRunnerForTest, mirroring the Task 4 spell
// collapse tests in internal/hooks/spell_collapse_test.go.

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Instance ids for this file. Kept clear of the 200-221 drift-test block,
// contest_sign_taunt_test's 3300, and the rhetoric_progression counter ranges.
const (
	tauntCollapseWiringTargetId  = 3500
	tauntCollapseHitRateTargetId = 3501
	tauntCollapseDamageTargetId  = 3502
	tauntCollapseCritTargetId    = 9450 // a USER id, not a mob instance id
)

// pinTauntCollapseKnobs pins every knob that can move a taunt contest outcome
// or verdict, so the closed-form arithmetic below stays true in a test binary:
//
//   - ContestFloor 0: every iteration is a genuine contest; the +-1 sentinel
//     of a floored save can neither steal hit-rate iterations nor produce the
//     exactly-0.5 multiplier bucket by accident.
//   - MinAttackCritChance / MinDefenseCritChance 0: ApplyCritFloor promotes a
//     non-crit with that probability REGARDLESS of margin, which would hand
//     out crit-damage outliers in the sampled-mean tests and stray crits in
//     the hit-rate count.
//   - DefyEffectiveness 1.0: the seam scales the defy score by it before the
//     contest; the bit-identity claim against the old gate holds at 1.0.
//   - SkillWeight 5: the multiplier on both sides' rhetoric in every score
//     this file computes.
func pinTauntCollapseKnobs(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	c.Balance.MinAttackCritChance = 0
	c.Balance.MinDefenseCritChance = 0
	c.Balance.DefyEffectiveness = 1.0
	c.Balance.SkillWeight = 5.0
	configs.SetConfigForTest(t, c)
}

// tauntDeterministicRunner mirrors internal/hooks' deterministicContestRunner:
// a contest.Result whose normalized attack margin is exactly normMargin sigma
// (attack-positive; negative is a defence win by that many sigma). Both roll
// ZScores are explicit so neither side trips a fumble or a defence crit by
// accident.
func tauntDeterministicRunner(t *testing.T, normMargin, atkZ, defZ float64) func(float64, []contest.Entry) contest.Result {
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

// newTauntCollapsePair builds a fresh attacker/target mob pair registered under
// targetId. Fresh per iteration: ExecuteTaunt consumes the special-move
// cooldown and fires progression, both of which would otherwise leak between
// iterations.
func newTauntCollapsePair(t *testing.T, targetId, atkCharisma, atkRhetoric, defWillpower, defRhetoric int) (*mobs.Mob, *mobs.Mob) {
	t.Helper()
	target := newTestMob(t, func(m *mobs.Mob) {
		m.InstanceId = targetId
		m.Character.Name = "Collapse Target"
		setTauntStatBase(t, &m.Character.Stats.Willpower, defWillpower)
		if defRhetoric > 0 {
			m.Character.Skills = map[string]int{string(skills.Rhetoric): defRhetoric}
		}
	})
	mobs.SetInstanceForTest(targetId, target)

	attacker := newTestMob(t, func(m *mobs.Mob) {
		m.Character.Name = "Collapse Taunter"
		setTauntStatBase(t, &m.Character.Stats.Charisma, atkCharisma)
		if atkRhetoric > 0 {
			m.Character.Skills = map[string]int{string(skills.Rhetoric): atkRhetoric}
		}
		m.Character.SetAggro(0, targetId, characters.DefaultAttack)
	})
	return attacker, target
}

// TestTauntOneContest_ScoreWiringAndSingleDefyContest proves ExecuteTaunt runs
// exactly ONE contest, through the channel seam, with the collapsed scores:
//
//   - attack score  = (Charisma + rhetoric x SkillWeight) x convMult — the
//     conviction-depletion multiplier the OLD GATE applied and the old defy
//     leg omitted (spec 4.1): the surviving score keeps it, via AttackSide.Mult.
//   - defence entry = defy alone, at Willpower + rhetoric x SkillWeight — the
//     defender's score enters ONE contest, not the gate-then-defy double.
func TestTauntOneContest_ScoreWiringAndSingleDefyContest(t *testing.T) {
	pinTauntCollapseKnobs(t)
	t.Cleanup(func() { mobs.SetInstanceForTest(tauntCollapseWiringTargetId, nil) })

	const (
		atkCharisma = 200
		atkRhetoric = 10
		defWil      = 120
		defRhetoric = 7
	)
	attacker, _ := newTauntCollapsePair(t, tauntCollapseWiringTargetId,
		atkCharisma, atkRhetoric, defWil, defRhetoric)
	// Deplete the attacker so convMult is measurably below 1.0 — a dropped
	// Mult would otherwise be invisible at a full pool.
	attacker.Character.Conviction = 300

	calls := 0
	var gotAtkScore float64
	var gotEntries []contest.Entry
	var convictionAtContest int
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		calls++
		gotAtkScore = atkScore
		gotEntries = append([]contest.Entry(nil), entries...)
		convictionAtContest = attacker.Character.Conviction
		return tauntDeterministicRunner(t, 0.5, 0.5, -0.5)(atkScore, entries)
	})
	t.Cleanup(restore)

	res := ExecuteTaunt(&MobActor{Mob: attacker})

	require.True(t, res.Executed, "the taunt must execute (cost %+v)", res.Cost)
	require.Equal(t, 1, calls,
		"ExecuteTaunt must run exactly ONE contest through the channel seam — "+
			"a second contest is the pre-collapse gate-then-defy double")

	cfg := configs.GetBalanceConfig()
	wantMult := combat.ResourceMultiplier(convictionAtContest,
		attacker.Character.EffectivePoolMax(characters.PoolConviction),
		float64(cfg.ConvictionPenaltyMax))
	require.Less(t, wantMult, 1.0,
		"fixture error: the depleted attacker must yield a convMult below 1.0, "+
			"or this test cannot see a dropped AttackSide.Mult")
	wantAtk := (float64(atkCharisma) + float64(atkRhetoric)*float64(cfg.SkillWeight)) * wantMult
	require.InDelta(t, wantAtk, gotAtkScore, 1e-9,
		"attack score must be (Charisma + rhetoric x SkillWeight) x convMult — "+
			"the old gate's exact score, conviction depletion included")

	require.Len(t, gotEntries, 1, "the social channel answers with defy alone")
	require.Equal(t, characters.DefenseDefy, gotEntries[0].Name)
	wantDef := float64(defWil) + float64(defRhetoric)*float64(cfg.SkillWeight)
	require.InDelta(t, wantDef, gotEntries[0].Score, 1e-9,
		"the defy entry must carry Willpower + rhetoric x SkillWeight — the old "+
			"gate's exact defence score, now contested ONCE")

	require.True(t, res.Hit)
	require.False(t, res.Defence.Defended)
	require.False(t, res.Crit, "a 0.5-sigma margin is below every crit bar")
}

// TestTauntHitRate_SingleContestClosedForm pins the collapse's hit-rate
// tripwire: at parity the full-effect rate must match the SINGLE-contest
// closed form (~0.5), not the squared double-contest form (~0.26).
//
// Arithmetic: both sides score 100 (Cha 100 vs Wil 100, no skills, full
// pools, floor pinned 0), so one contest is a coin flip. The OLD shape needed
// the gate AND the defy leg to both go the attacker's way for full effect:
// P(gate win) x [P(crit|win) + P(no-crit) x P(defy loses)] ~ 0.5 x 0.51 ~ 0.26.
// Over 400 iterations the sample SE is ~0.025, so the (0.375, 0.625) window
// rejects the squared form at ~5.8 sigma and false-fails the true 0.5 at
// ~5 sigma (~3e-7, inside the 1e-6 budget). A HIT-rate move outside the
// window means the wiring diverged from the modelled collapse — stop and
// investigate, per the plan.
func TestTauntHitRate_SingleContestClosedForm(t *testing.T) {
	pinTauntCollapseKnobs(t)
	t.Cleanup(func() { mobs.SetInstanceForTest(tauntCollapseHitRateTargetId, nil) })

	const iterations = 400
	fullEffect, fumbles, other := 0, 0, 0
	for i := 0; i < iterations; i++ {
		attacker, _ := newTauntCollapsePair(t, tauntCollapseHitRateTargetId, 100, 0, 100, 0)
		res := ExecuteTaunt(&MobActor{Mob: attacker})
		require.True(t, res.Executed, "iteration %d did not execute (cost %+v)", i, res.Cost)
		switch {
		case res.Fumble:
			fumbles++
		case res.Hit && !res.Defence.Defended:
			fullEffect++
		default:
			other++
		}
	}

	require.Equal(t, iterations, fullEffect+fumbles+other,
		"the loop did not run to completion")
	rate := float64(fullEffect) / float64(iterations)
	require.Greater(t, rate, 0.375,
		"full-effect rate %.3f at parity (fumbles=%d defended/miss=%d): the "+
			"single-contest closed form is ~0.5; ~0.26 is the squared "+
			"double-contest form — the defender is being contested twice", rate, fumbles, other)
	require.Less(t, rate, 0.625,
		"full-effect rate %.3f at parity is above the single-contest closed "+
			"form — the defence stopped answering the contest", rate)
}

// tauntDamageSample runs one full ExecuteTaunt against a fresh pair (the
// contest outcome is forced through the seam runner set by the caller) and
// returns the damage dealt.
func tauntDamageSample(t *testing.T, wantCrit bool) int {
	t.Helper()
	attacker, _ := newTauntCollapsePair(t, tauntCollapseDamageTargetId, 200, 10, 20, 0)
	res := ExecuteTaunt(&MobActor{Mob: attacker})
	require.True(t, res.Executed, "sample did not execute (cost %+v)", res.Cost)
	require.False(t, res.Fumble)
	require.Equal(t, wantCrit, res.Crit,
		"the crit verdict must be the seam's margin-derived AttackerCrit")
	return res.Damage
}

// TestTauntDamage_RankInputIsRawRhetoricRank pins Assumption 8's BOTH
// consumers: the rank fed to CalcRawDamage's SkillMultiplier AND to
// CritOrMitigatedDamage's CritDamageMultiplier is the RAW rhetoric rank
// (side.SkillRank), not the x SkillWeight-weighted score term the old code
// passed. Two named nerfs, each with its own subtest:
//
//   - base damage (below the skill soft cap): SkillMultiplier(10) vs the old
//     SkillMultiplier(50) — 1.894x vs 3.0x at defaults.
//   - crit damage: CritDamageMultiplier(10) vs CritDamageMultiplier(50) —
//     2.5x vs 4.5x at defaults.
//
// Damage rolls through dice.RollStat (spread 0.15), so each subtest samples
// the mean over 300 taunts: SE ~0.87% of the mean, the 5% InEpsilon window
// sits ~5.7 sigma out, and the raw/weighted expectations differ by ~1.6-1.8x,
// far outside the window. The expected values are computed from the SAME
// combat primitives with the raw rank, so knob drift cannot go stale here.
func TestTauntDamage_RankInputIsRawRhetoricRank(t *testing.T) {
	const (
		samples   = 300
		rawRank   = 10
		atkCha    = 200
		tauntBase = 0.5 // taunt's fixed conviction item multiplier
	)

	expectedMults := func(t *testing.T) (convMult float64, weightedRank int) {
		t.Helper()
		cfg := configs.GetBalanceConfig()
		// Every sample pays the admission cost before the contest, so the
		// conviction-depletion multiplier is computed at (full - cost).
		probe := newTestMob(t, nil)
		cost := int(cfg.RhetoricActionBaseConvictionCost)
		return combat.ResourceMultiplier(probe.Character.Conviction-cost,
				probe.Character.EffectivePoolMax(characters.PoolConviction),
				float64(cfg.ConvictionPenaltyMax)),
			rawRank * int(cfg.SkillWeight)
	}

	t.Run("base damage uses the raw rank", func(t *testing.T) {
		pinTauntCollapseKnobs(t)
		t.Cleanup(func() { mobs.SetInstanceForTest(tauntCollapseDamageTargetId, nil) })
		restore := combat.SetChannelAttackContestRunnerForTest(tauntDeterministicRunner(t, 0.5, 0.5, -0.5))
		t.Cleanup(restore)

		sum := 0
		for i := 0; i < samples; i++ {
			sum += tauntDamageSample(t, false)
		}
		mean := float64(sum) / float64(samples)

		convMult, weightedRank := expectedMults(t)
		wantRaw := combat.CalcRawDamage(atkCha, rawRank, tauntBase, combat.ChannelConviction) * convMult
		wantWeighted := combat.CalcRawDamage(atkCha, weightedRank, tauntBase, combat.ChannelConviction) * convMult
		require.Greater(t, wantWeighted, wantRaw*1.3,
			"fixture error: the raw and weighted expectations are not separated "+
				"enough for the sampled mean to distinguish them")
		require.InEpsilon(t, wantRaw, mean, 0.05,
			"mean base damage %.1f: want ~%.1f (SkillMultiplier at the RAW rank %d), "+
				"not ~%.1f (the x SkillWeight-weighted %d the old code passed)",
			mean, wantRaw, rawRank, wantWeighted, weightedRank)
	})

	t.Run("crit damage uses the raw rank", func(t *testing.T) {
		pinTauntCollapseKnobs(t)
		t.Cleanup(func() { mobs.SetInstanceForTest(tauntCollapseDamageTargetId, nil) })
		// 4 sigma clears CritBarCeiling; a +2z attack roll cannot fumble.
		restore := combat.SetChannelAttackContestRunnerForTest(tauntDeterministicRunner(t, 4, 2, -1))
		t.Cleanup(restore)

		sum := 0
		for i := 0; i < samples; i++ {
			sum += tauntDamageSample(t, true)
		}
		mean := float64(sum) / float64(samples)

		convMult, weightedRank := expectedMults(t)
		wantRaw := combat.CalcRawDamage(atkCha, rawRank, tauntBase, combat.ChannelConviction) *
			convMult * combat.CritDamageMultiplier(rawRank)
		wantWeighted := combat.CalcRawDamage(atkCha, weightedRank, tauntBase, combat.ChannelConviction) *
			convMult * combat.CritDamageMultiplier(weightedRank)
		require.Greater(t, wantWeighted, wantRaw*1.3,
			"fixture error: the raw and weighted expectations are not separated "+
				"enough for the sampled mean to distinguish them")
		require.InEpsilon(t, wantRaw, mean, 0.05,
			"mean crit damage %.1f: want ~%.1f (both consumers at the RAW rank %d), "+
				"not ~%.1f (the x SkillWeight-weighted %d the old code passed)",
			mean, wantRaw, rawRank, wantWeighted, weightedRank)
	})
}

// TestTauntCritReceived_TracksDefenderCharismaOnce proves crit-received
// toughening flows ONLY through the seam's bonus tier — taunt's direct
// crit-received block (the Task 4-class duplicate) is gone.
//
// The observable is the defender's charisma use count, which moves through two
// distinct paths per taunt:
//
//  1. AwardDefenceProgression(defy) fires whenever the contest ran and calls
//     OnSkillUse(rhetoric), whose primary governing stat is charisma: +1 on
//     EVERY resolved taunt, win or lose.
//  2. The bonus tier's crit-received toughening (ToughenStatFor("conviction")
//     = charisma) tracks once more on an attacker crit only.
//
// So a plain win must move it by exactly 1 and a forced crit by exactly 2. A
// surviving direct block would make the crit delta 3; a crit-received path
// that stopped firing entirely would make it 1.
func TestTauntCritReceived_TracksDefenderCharismaOnce(t *testing.T) {
	pinTauntCollapseKnobs(t)

	// The bonus tier's cells are gated behind progression being on and a
	// non-zero observing multiplier (applyBonusProgression skips otherwise).
	prevBal := configs.GetBalanceConfig()
	prevGp := configs.GetGamePlayConfig()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{
		"GamePlay.UseSkillProgression":         true,
		"Balance.CritProgressionBonus":         1.0,
		"Balance.ObservedCritProgressionBonus": 1.0,
	}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"GamePlay.UseSkillProgression":         bool(prevGp.UseSkillProgression),
			"Balance.CritProgressionBonus":         float64(prevBal.CritProgressionBonus),
			"Balance.ObservedCritProgressionBonus": float64(prevBal.ObservedCritProgressionBonus),
		})
	})

	runCase := func(t *testing.T, normMargin float64, wantCrit bool, wantDelta int) {
		t.Helper()
		// A PLAYER defender: the deleted direct block was gated on
		// target.UserId > 0, so only a player target can prove it stayed dead.
		target := users.NewTestUser(tauntCollapseCritTargetId, "tauntcollapsedef", "Collapse Defender", 0)
		target.Character.ConvictionMax.Value = 100000
		target.Character.Conviction = 100000
		restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{target.UserId: target})
		t.Cleanup(restoreUsers)

		attacker := newTestMob(t, func(m *mobs.Mob) {
			m.Character.Name = "Collapse Taunter"
			m.Character.RoomId = target.Character.RoomId
			setTauntStatBase(t, &m.Character.Stats.Charisma, 200)
			m.Character.Skills = map[string]int{string(skills.Rhetoric): 10}
			m.Character.SetAggro(target.UserId, 0, characters.DefaultAttack)
		})

		restore := combat.SetChannelAttackContestRunnerForTest(tauntDeterministicRunner(t, normMargin, 0.5, -0.5))
		t.Cleanup(restore)

		before := target.Character.GetStatUseCount("charisma")
		res := ExecuteTaunt(&MobActor{Mob: attacker})
		require.True(t, res.Executed, "taunt did not execute (cost %+v)", res.Cost)
		require.Equal(t, wantCrit, res.Crit)

		if got := target.Character.GetStatUseCount("charisma") - before; got != wantDelta {
			t.Errorf("defender charisma use count changed by %d, want %d -- "+
				"crit-received must fire exactly once, inside the seam's bonus tier "+
				"(1 = defy's ordinary award via rhetoric's primary stat; the crit adds "+
				"exactly 1 more; 3 means the deleted direct block came back)", got, wantDelta)
		}
	}

	t.Run("plain win tracks the ordinary defy award only", func(t *testing.T) {
		runCase(t, 0.5, false, 1)
	})
	t.Run("forced crit adds exactly one toughening track", func(t *testing.T) {
		runCase(t, 4, true, 2)
	})
}
