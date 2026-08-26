package actions

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// U10d: the same-room ranged surprise shot.
//
// Four behaviours are pinned here, and each has a control that fails if the
// feature fires where it must not:
//
//   1. a same-room shot from stealth crits AND stacks the skullduggery-scaled
//      opening-strike multiplier (control: the identical unhidden shot);
//   2. it reveals the shooter, EXPLICITLY — the fixture is already engaged
//      (Aggro != nil), so ExecuteFire's SetAggro/Awareness cascade cannot be
//      what reveals them;
//   3. it burns the shared special-move cooldown;
//   4. an ordinary unhidden shot does NOT (control for 3, and today's shipped
//      behaviour);
//   5. a CROSS-ROOM shot from stealth gets none of it;
//   6. a hidden shooter whose special-move cooldown is already claimed fires an
//      ordinary shot and is told so.
// ---------------------------------------------------------------------------

const (
	// Pinned explicitly: characters.New() seeds every skill at rank 1, so a
	// fixture that left these unset would make the expected multipliers depend
	// on that seed rather than on the values this test reasons about.
	surpriseRangedRank      = 10
	surpriseSkullduggery    = 40
	surpriseMobHealth       = 1000000
	surpriseDamageSamples   = 80
	surpriseRatioTolerance  = 0.12
	surpriseRangedKnob      = 0.5
	surpriseMeleeKnobUnused = 1.0
)

// pinRangedSurpriseBalance pins every knob the surprise-shot arithmetic reads,
// so a config default drifting cannot quietly move the separation these tests
// rely on. SetConfigForTest restores on cleanup.
//
// The two ambush knobs are pinned to DIFFERENT values on purpose: the ranged
// shot must read SurpriseRangedStrikeMultiplier, and if it read the melee knob
// instead the sampled stack would double. That is mutant #3.
func pinRangedSurpriseBalance(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.RollSpread = 0.15
	b.ContestFloor = 0
	b.SkillWeight = 2.0
	// Bars well above the deterministic runner's 0.5-sigma margin, so an
	// ordinary win is an ordinary win and only CritOnWin can upgrade it.
	b.CritBarSkillSlope = 0.05
	b.CritBarFloor = 1.5
	b.CritBarCeiling = 3.0
	// Zero crit floors: a floor promotes a non-crit at random, which would make
	// the control shot crit occasionally and blur the sampled means.
	b.MinAttackCritChance = 0
	b.MinDefenseCritChance = 0
	b.CritDamageBase = 2.0
	b.CritDamagePerSkill = 0.05
	b.SurpriseRangedStrikeMultiplier = configs.ConfigFloat(surpriseRangedKnob)
	b.SurpriseOpeningStrikeMultiplier = configs.ConfigFloat(surpriseMeleeKnobUnused)
	b.RangedShotScale = 1.0
	// Neutral here on purpose. These fixtures fire into an empty room, so every
	// shot is unengaged and the factor cancels out of the ratios below -- but
	// pinning it keeps the surprise arithmetic independent of a knob this file
	// is not about. combat_fire_unengaged_test.go overrides it deliberately.
	b.RangedUnengagedDamageMultiplier = 1.0
	b.MeleeDamageScale = 0.30
	b.GlobalDamageMultiplier = 1.0
	b.SkillMultiplierBase = 1.0
	b.SkillMultiplierMax = 3.0
	b.SkillSoftCap = 50
	b.PhysicalMitigationCap = 0.75
	b.StaminaPenaltyMax = 0.28
	b.HealthPenaltyMax = 0.28
	b.ResourcePenaltyCurve = 2.0
	b.ShootBaseStaminaCost = 2
	b.SpecialMoveCooldown = 4
	// Live progression would move the ranks mid-run and slide the multipliers.
	cfg.GamePlay.UseSkillProgression = false
	configs.SetConfigForTest(t, cfg)
}

// newSurpriseShooter builds a loaded shooter with pinned ranged and
// skullduggery ranks and a free special-move cooldown.
func newSurpriseShooter(hidden bool) *characters.Character {
	char := fireAttacker()
	char.Skills[string(skills.RangedCombat)] = surpriseRangedRank
	char.Skills[string(skills.Skullduggery)] = surpriseSkullduggery
	char.Cooldowns = characters.Cooldowns{}
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	if hidden {
		addHiddenBuff(char)
	}
	return char
}

// pinOrdinaryContestWin installs a deterministic contest that resolves as an
// ordinary attack WIN: normalized margin 0.5 (well under any crit bar here),
// attacker z 0.5 (well clear of the fumble bar).
func pinOrdinaryContestWin(t *testing.T) {
	t.Helper()
	restore := combat.SetChannelAttackContestRunnerForTest(
		func(atkScore float64, entries []contest.Entry) contest.Result {
			return tauntDeterministicRunner(t, 0.5, 0.5, -0.5)(atkScore, entries)
		})
	t.Cleanup(restore)
}

// sampleSurpriseShotDamage fires n shots from a FRESH shooter each time (the
// special-move cooldown and the weapon's load are both one-shot) and returns
// the mean damage applied.
func sampleSurpriseShotDamage(t *testing.T, hidden bool, n int) float64 {
	t.Helper()
	target := mobs.GetInstance(500)
	require.NotNil(t, target)

	total := 0
	for i := 0; i < n; i++ {
		target.Character.Health = surpriseMobHealth
		char := newSurpriseShooter(hidden)
		res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")
		require.True(t, res.Executed, "sample %d: the shot must execute", i)
		require.True(t, res.MoveResult.Hit, "sample %d: the deterministic win must land", i)
		require.Equal(t, hidden, res.MoveResult.Crit,
			"sample %d: a stealth shot must crit and an ordinary one must not", i)
		total += res.MoveResult.Damage
	}
	return float64(total) / float64(n)
}

// ---------------------------------------------------------------------------
// 1. A same-room shot from stealth crits AND stacks
// ---------------------------------------------------------------------------

// The magnitude assertion is the point. Asserting only "hidden hurts more"
// would pass with ANY bonus multiplier, including the melee knob (twice the
// intended stack) — so the sampled ratio is pinned to the ranged knob's
// expected value within a narrow band instead.
func TestFireSurprise_SameRoomStealthShotCritsAndStacks(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	hiddenMean := sampleSurpriseShotDamage(t, true, surpriseDamageSamples)
	plainMean := sampleSurpriseShotDamage(t, false, surpriseDamageSamples)

	require.Greater(t, plainMean, 0.0, "the control shot must do real damage")

	// Expected stack: the crit's own worth (ranged rank) times the opening
	// strike's worth (skullduggery rank x the RANGED knob).
	cfg := configs.GetBalanceConfig()
	probe := newSurpriseShooter(true)
	wantRatio := combat.CritDamageMultiplier(surpriseRangedRank) *
		combat.OpeningStrikeMultiplier(probe, float64(cfg.SurpriseRangedStrikeMultiplier))
	require.Greater(t, wantRatio, 1.0, "fixture sanity: the ambush must be worth something")

	gotRatio := hiddenMean / plainMean
	assert.InEpsilon(t, wantRatio, gotRatio, surpriseRatioTolerance,
		"the stealth shot's sampled mean (%.1f) against the ordinary shot's (%.1f) "+
			"must sit at the skullduggery-scaled ranged opening-strike multiplier",
		hiddenMean, plainMean)
}

// ---------------------------------------------------------------------------
// 2. The surprise shot reveals the shooter — EXPLICITLY
// ---------------------------------------------------------------------------

// FIXTURE: char.Aggro is pre-set, so ExecuteFire's
// `if !crossRoom && char.Aggro == nil { char.SetAggro(...) }` never runs and
// the SetAggro -> TransitionToEngaging -> Awareness cascade cannot be what
// clears Hidden. Delete the explicit TransitionToRevealing call and this test
// fails.
//
// Two honest caveats, so nobody over-reads this fixture:
//
// A fixture with NO prior aggro would also fail against that mutant HERE, but
// only because internal/hooks (which registers the Awareness cascade) is not
// linked into this test binary. The pre-set aggro is what keeps the test
// honest if it ever is.
//
// And the pre-set aggro is NOT the production justification for the explicit
// call. Sneak (sneak.go:60-62) is the only entry into awareness.Hidden and
// refuses while Aggro != nil, so an already-engaged hidden shooter cannot
// exist. The real reachable cases are the paths where SetAggro returns before,
// or loses, its phase transition: the grace-period guard, the taunt-hold
// guard, and a vetoed TransitionToEngaging. See the comment on the call site.
func TestFireSurprise_RevealsAnAlreadyEngagedShooter(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := newSurpriseShooter(true)
	char.Aggro = &characters.Aggro{MobInstanceId: 500}
	require.True(t, char.IsHidden(), "precondition: the shooter starts hidden")
	require.NotNil(t, char.Aggro,
		"precondition: already engaged, so ExecuteFire must NOT call SetAggro")

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.True(t, res.Executed)
	require.True(t, res.IsSneaking,
		"the sneaking flag must be captured before anything can clear it")
	assert.True(t, res.Revealed, "firing from stealth must reveal the shooter")
	assert.False(t, char.IsHidden(), "the shooter must no longer be hidden")
}

// ---------------------------------------------------------------------------
// 3. The surprise shot burns the shared special-move cooldown
// ---------------------------------------------------------------------------

func TestFireSurprise_BurnsTheSpecialMoveCooldown(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := newSurpriseShooter(true)
	require.Zero(t, char.Cooldowns["special-move"],
		"precondition: the cooldown starts free")

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.True(t, res.Executed)
	require.False(t, res.SurpriseOnCooldown, "the opener was available")
	assert.Positive(t, char.Cooldowns["special-move"],
		"the surprise shot must claim the shared special-move cooldown")
	assert.False(t, char.Cooldowns.Try("special-move", "4 rounds"),
		"a second special move must be refused in the same window")
}

// ---------------------------------------------------------------------------
// 4. Control: an ordinary shot leaves the cooldown alone
// ---------------------------------------------------------------------------

// This is today's shipped contract (reload owns the timer, firing does not),
// and it is what makes case 3 evidence of the AMBUSH rather than of shooting.
func TestFireSurprise_OrdinaryShotDoesNotBurnTheCooldown(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := newSurpriseShooter(false)
	require.False(t, char.IsHidden(), "precondition: an ordinary, visible shooter")

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.True(t, res.Executed)
	assert.False(t, res.SurpriseOnCooldown, "nothing was refused: there was no opener")
	assert.False(t, res.Revealed, "a visible shooter has nothing to reveal")
	assert.Zero(t, char.Cooldowns["special-move"],
		"an ordinary shot must NOT consume the special-move cooldown")
	assert.True(t, char.Cooldowns.Try("special-move", "4 rounds"),
		"the shared timer must still be free after an ordinary shot")
}

// ---------------------------------------------------------------------------
// 5. A cross-room shot from stealth is entirely ordinary
// ---------------------------------------------------------------------------

// Cross-room is excluded deliberately: it never SetAggro's, is reach-gated out
// of counterattacks, and narrates anonymously. Delete `!crossRoom` from the
// surprise decision and every assertion below flips.
func TestFireSurprise_CrossRoomShotGetsNothing(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 2, 1) // the mob lives one room north
	defer cleanup()

	char := newSurpriseShooter(true)
	require.True(t, char.IsHidden(), "precondition: the shooter starts hidden")

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton north")

	require.True(t, res.Executed)
	require.True(t, res.CrossRoom, "fixture sanity: this must be the cross-room path")
	require.True(t, res.IsSneaking, "the shooter really was sneaking")
	assert.False(t, res.MoveResult.Crit,
		"a cross-room shot must not be upgraded to a crit")
	assert.False(t, res.Revealed, "a cross-room shot must not reveal the shooter")
	assert.True(t, char.IsHidden(),
		"the cross-room shooter stays hidden: nothing on that path reveals them")
	assert.Zero(t, char.Cooldowns["special-move"],
		"a cross-room shot must not claim the special-move cooldown")
	assert.False(t, res.SurpriseOnCooldown,
		"nothing was refused: a cross-room shot never asks for the opener")
	assert.Nil(t, char.Aggro, "a cross-room shot stays one-shot and aggro-free")
}

// ---------------------------------------------------------------------------
// 6. A claimed special-move cooldown denies the opener, and says so
// ---------------------------------------------------------------------------

// Not an edge case: a loaded bow implies a recent reload, and reload burns this
// same timer, so the natural "reload, sneak, shoot" sequence denies the opener
// whenever the reload was inside SpecialMoveCooldown rounds. The player has to
// be told, or the ambush silently does nothing.
func TestFireSurprise_ClaimedCooldownDeniesTheOpener(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := newSurpriseShooter(true)
	require.True(t, char.Cooldowns.Try("special-move", fmt.Sprintf("%d rounds",
		int(configs.GetBalanceConfig().SpecialMoveCooldown))),
		"precondition: something else (a reload) claims the timer first")
	claimed := char.Cooldowns["special-move"]
	require.Positive(t, claimed)

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.True(t, res.Executed, "the shot itself still happens")
	assert.True(t, res.SurpriseOnCooldown,
		"the denied opener must be reported so the wrapper can speak it")
	assert.False(t, res.MoveResult.Crit, "a denied opener is an ordinary shot")
	assert.False(t, res.Revealed, "a denied opener does not reveal the shooter")
	assert.Equal(t, claimed, char.Cooldowns["special-move"],
		"a refused claim must not extend the running cooldown")
}

// ---------------------------------------------------------------------------
// 7. A landed surprise shot trains skullduggery (ranged equivalent of the
// melee ambush award in internal/hooks/NewRound_DoCombat_unified.go)
// ---------------------------------------------------------------------------

// Assertions are on the USE COUNTER (GetSkillUseCount), never on whether a
// rank moved: progression is probabilistic and pinRangedSurpriseBalance turns
// live progression off (UseSkillProgression = false) so the ranged damage
// assertions elsewhere in this file stay stable. TrackSkillUse (called
// unconditionally by OnSkillUseScaled, live-progression knob or not) is what
// moves the counter, so it is the correct deterministic signal here.
func TestFireSurprise_LandedShotTrainsSkullduggery(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := newSurpriseShooter(true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton")

	require.True(t, res.Executed)
	require.True(t, res.MoveResult.Hit, "fixture sanity: the deterministic win must land")

	// U10b-1 Task 11 changed what this test can observe, and improved it.
	//
	// A landed ambush no longer takes a skullduggery award of its own: it
	// CONTESTS the shot's single progression event against ranged-combat, so
	// counting skill uses would be counting a coin flip. What the ambush
	// actually guarantees is that skullduggery is OFFERED as a candidate, and
	// that is what this now asserts -- through the Actor seam's recorder, which
	// is deterministic and needs no rank-rigging fixture.
	//
	// It is also a stronger assertion than the old one. The counter could be
	// satisfied by any code path that happened to train skullduggery; this
	// pins that the shot produced exactly ONE award and that the ambush put
	// skullduggery into it.
	require.Len(t, actor.awards, 1, "a resolved shot must produce exactly one progression award")
	require.True(t, actor.awards[0].won, "a landed shot is a win")

	_, n := actor.awardedCandidate(string(skills.Skullduggery))
	assert.Equal(t, 1, n, "a landed surprise shot must offer skullduggery as a candidate exactly once")

	// ranged-combat contests the SAME award rather than taking its own, and it
	// is offered to a NON-PLAYER shooter too.
	//
	// That second half is the point of the assertion. An earlier draft gated
	// this candidate on actor.IsPlayer() to preserve the gap left by
	// mobcommands/shoot.go awarding no progression, which made the firing rule
	// depend on what kind of thing was acting. Whether a mob progresses is
	// already decided centrally by MobProgressionEnabled and MobProgressionRate
	// inside the chance functions, so the gate was a second, invisible copy of
	// an existing policy. This stub reports IsPlayer() false, so it fails the
	// moment such a gate comes back.
	require.False(t, actor.IsPlayer(), "fixture premise: this stub is not a player")
	_, nRanged := actor.awardedCandidate(string(skills.RangedCombat))
	assert.Equal(t, 1, nRanged,
		"ranged-combat must contest the same award, for a non-player shooter as much as a player")
}

// ---------------------------------------------------------------------------
// 8. Control: a surprise shot that MISSES does not train skullduggery
// ---------------------------------------------------------------------------

// The defence wins by 1 sigma, cleanly (no defensive crit) — the same
// deterministic runner TestFireSeam_DefendedShotUsesContestNotAddend uses to
// pin a defended (non-crit) outcome. The shooter is still hidden and still
// pays the special-move cooldown; only the contest result differs from case 7.
func TestFireSurprise_MissedShotDoesNotTrainSkullduggery(t *testing.T) {
	pinRangedSurpriseBalance(t)
	restore := combat.SetChannelAttackContestRunnerForTest(
		tauntDeterministicRunner(t, -1.0, -0.5, 0.5))
	defer restore()
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := newSurpriseShooter(true)
	before := char.GetSkillUseCount(string(skills.Skullduggery))

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.True(t, res.Executed)
	require.False(t, res.MoveResult.Hit, "fixture sanity: the deterministic defence must win")
	assert.Equal(t, before, char.GetSkillUseCount(string(skills.Skullduggery)),
		"a missed surprise shot must not train skullduggery")
}

// ---------------------------------------------------------------------------
// 9. Control: an ordinary (unhidden) shot never trains skullduggery
// ---------------------------------------------------------------------------

// Distinguishes the award from "any landed shot trains skullduggery" — the
// shooter here is visible (hidden=false), so surpriseShot is false even
// though the contest is pinned to the same deterministic win as case 7.
func TestFireSurprise_OrdinaryShotDoesNotTrainSkullduggery(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := newSurpriseShooter(false)
	require.False(t, char.IsHidden(), "precondition: an ordinary, visible shooter")
	before := char.GetSkillUseCount(string(skills.Skullduggery))

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.True(t, res.Executed)
	require.True(t, res.MoveResult.Hit, "fixture sanity: the deterministic win must land")
	assert.Equal(t, before, char.GetSkillUseCount(string(skills.Skullduggery)),
		"an ordinary shot must not train skullduggery")
}
