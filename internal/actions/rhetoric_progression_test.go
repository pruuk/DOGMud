package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingActor wraps an Actor implementation and records OnSkillUse calls,
// so these tests assert the progression hook actually fires rather than
// asserting on a probabilistic skill-rank increase.
type recordingActor struct {
	Actor
	char       *characters.Character
	skillsUsed []string
}

type rhetoricActionOutcome struct {
	cost          characters.CostCommitResult
	executed      bool
	onCooldown    bool
	invalid       bool
	selfBuffID    int
	selfCondition characters.ConditionType
}

type rhetoricActionCase struct {
	name       string
	action     costs.Action
	execute    func(Actor) rhetoricActionOutcome
	invalidate func(*characters.Character)
}

var rhetoricTargetID = 9700

func rhetoricActionCases() []rhetoricActionCase {
	return []rhetoricActionCase{
		{
			name:   "taunt",
			action: costs.ActionTaunt,
			execute: func(actor Actor) rhetoricActionOutcome {
				result := ExecuteTaunt(actor)
				return rhetoricActionOutcome{cost: result.Cost, executed: result.Executed, onCooldown: result.OnCooldown, invalid: result.NoTarget}
			},
			invalidate: func(char *characters.Character) { char.EndAggro() },
		},
		{
			name:   "rally",
			action: costs.ActionRally,
			execute: func(actor Actor) rhetoricActionOutcome {
				result := ExecuteRally(actor)
				return rhetoricActionOutcome{cost: result.Cost, executed: result.Executed, onCooldown: result.OnCooldown, invalid: result.AlreadyActive, selfBuffID: 80, selfCondition: characters.ConditionRally}
			},
			invalidate: func(char *characters.Character) { char.AddBuff(80, false) },
		},
		{
			name:   "warcry",
			action: costs.ActionWarcry,
			execute: func(actor Actor) rhetoricActionOutcome {
				result := ExecuteWarcry(actor)
				return rhetoricActionOutcome{cost: result.Cost, executed: result.Executed, onCooldown: result.OnCooldown, invalid: result.AlreadyActive, selfBuffID: 79, selfCondition: characters.ConditionWarcry}
			},
			invalidate: func(char *characters.Character) { char.AddBuff(79, false) },
		},
	}
}

func newRhetoricActor(t *testing.T, player bool, conviction, rhetoric int) (*recordingActor, *characters.Character, *characters.Character) {
	t.Helper()
	rhetoricTargetID++

	target := characters.New()
	target.Name = "Rhetoric Target"
	target.RoomId = 1
	target.Conviction = 1_000_000
	target.ConvictionMax.Base = 1_000_000
	target.ConvictionMax.Recalculate()
	target.Stats.Willpower.ValueAdj = 1_000_000
	target.Buffs = buffs.New()
	targetMob := &mobs.Mob{InstanceId: rhetoricTargetID, Character: *target}
	targetMob.Character.MobInstanceId = targetMob.InstanceId
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	t.Cleanup(func() { mobs.SetInstanceForTest(targetMob.InstanceId, nil) })

	char := characters.New()
	char.Name = "Rhetoric Actor"
	char.RoomId = 1
	char.Conviction = conviction
	char.ConvictionMax.Base = 100
	char.ConvictionMax.Recalculate()
	char.Stats.Charisma.ValueAdj = 1
	char.Skills[string(skills.Rhetoric)] = rhetoric
	char.Buffs = buffs.New()
	char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
	room := &rooms.Room{RoomId: 1}

	var base Actor
	if player {
		user := &users.UserRecord{UserId: rhetoricTargetID + 10_000, Character: char}
		base = &UserActor{User: user, Room: room}
	} else {
		mob := &mobs.Mob{InstanceId: rhetoricTargetID + 20_000, Character: *char}
		char = &mob.Character
		base = &MobActor{Mob: mob, Room: room}
	}
	return &recordingActor{Actor: base, char: char}, char, &targetMob.Character
}

func hideRhetoricActor(t *testing.T, char *characters.Character) {
	t.Helper()
	char.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "rhetoric_admission_test"}
	require.NoError(t, char.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	char.Awareness.ResolveConcealment(true, reason)
	require.Equal(t, awareness.Hidden, char.Awareness.State())
}

func assertRhetoricRefusalCarryPreserved(t *testing.T, actor Actor, char *characters.Character, action costs.Action) {
	t.Helper()
	char.Conviction = 100
	base := float64(configs.GetBalanceConfig().RhetoricActionBaseConvictionCost)
	first := admitFullCost(actor, action, characters.PoolConviction, base)
	second := admitFullCost(actor, action, characters.PoolConviction, base)
	require.Equal(t, 4, first.Charged)
	require.Equal(t, 4, second.Charged, "refused quote must not bank fractional carry")
}

func TestTauntRallyWarcryRefusalIsAtomicForPlayerAndMob(t *testing.T) {
	cleanup := seedBuffsForTest()
	defer cleanup()

	for _, tc := range rhetoricActionCases() {
		for _, player := range []bool{true, false} {
			kind := "mob"
			if player {
				kind = "player"
			}
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				actor, char, target := newRhetoricActor(t, player, 0, 0)
				hideRhetoricActor(t, char)
				char.Cooldowns = characters.Cooldowns{"expired": -2, "other": 7}
				cooldownsBefore := char.GetAllCooldowns()
				target.SetAggro(404, 0, characters.DefaultAttack)
				target.SetRoundsWaiting(6)
				targetAggroBefore := target.CurrentCombatTarget()
				targetConviction := target.Conviction
				targetBuffs := len(target.Buffs.GetBuffs())

				result := tc.execute(actor)

				require.False(t, result.executed)
				require.Equal(t, characters.CostRefused, result.cost.Status)
				require.Equal(t, characters.PoolConviction, result.cost.Pool)
				require.Zero(t, result.cost.Charged)
				require.Zero(t, char.Conviction)
				require.Equal(t, awareness.Hidden, char.Awareness.State())
				require.Equal(t, cooldownsBefore, char.GetAllCooldowns())
				require.Zero(t, char.RoundsWaiting())
				require.Equal(t, targetConviction, target.Conviction)
				require.Equal(t, targetAggroBefore, target.CurrentCombatTarget())
				require.Len(t, target.Buffs.GetBuffs(), targetBuffs)
				require.Empty(t, actor.skillsUsed)
				if result.selfBuffID != 0 {
					require.False(t, char.HasBuff(result.selfBuffID))
					require.False(t, char.HasCondition(result.selfCondition))
				}
				assertRhetoricRefusalCarryPreserved(t, actor, char, tc.action)
			})
		}
	}
}

func TestTauntRallyWarcryReadOnlyGatesPreserveHiddenState(t *testing.T) {
	cleanup := seedBuffsForTest()
	defer cleanup()

	for _, tc := range rhetoricActionCases() {
		t.Run(tc.name+"/invalid", func(t *testing.T) {
			actor, char, target := newRhetoricActor(t, false, 10, 0)
			hideRhetoricActor(t, char)
			tc.invalidate(char)
			targetConviction := target.Conviction
			result := tc.execute(actor)

			require.True(t, result.invalid)
			require.False(t, result.executed)
			require.Equal(t, characters.CostNoCharge, result.cost.Status)
			require.Equal(t, 10, char.Conviction)
			require.Equal(t, awareness.Hidden, char.Awareness.State())
			require.Empty(t, char.Cooldowns)
			require.Equal(t, targetConviction, target.Conviction)
			require.Empty(t, actor.skillsUsed)
		})

		t.Run(tc.name+"/cooldown", func(t *testing.T) {
			actor, char, target := newRhetoricActor(t, false, 10, 0)
			hideRhetoricActor(t, char)
			char.Cooldowns = characters.Cooldowns{"special-move": 3, "other": 8}
			before := char.GetAllCooldowns()
			targetConviction := target.Conviction
			result := tc.execute(actor)

			require.True(t, result.onCooldown)
			require.False(t, result.executed)
			require.Equal(t, characters.CostNoCharge, result.cost.Status)
			require.Equal(t, 10, char.Conviction)
			require.Equal(t, awareness.Hidden, char.Awareness.State())
			require.Equal(t, before, char.GetAllCooldowns())
			require.Zero(t, char.RoundsWaiting())
			require.Equal(t, targetConviction, target.Conviction)
			require.Empty(t, actor.skillsUsed)
			if result.selfBuffID != 0 {
				require.False(t, char.HasBuff(result.selfBuffID))
				require.False(t, char.HasCondition(result.selfCondition))
			}
		})
	}
}

func TestRallyWarcryPaidBuffsChargeOnceBeforeEffects(t *testing.T) {
	cleanup := seedBuffsForTest()
	defer cleanup()

	for _, tc := range rhetoricActionCases()[1:] {
		for _, player := range []bool{true, false} {
			kind := "mob"
			if player {
				kind = "player"
			}
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				actor, char, _ := newRhetoricActor(t, player, 10, 0)
				hideRhetoricActor(t, char)

				result := tc.execute(actor)

				require.True(t, result.executed)
				require.Equal(t, characters.CostPaid, result.cost.Status)
				require.Equal(t, characters.PoolConviction, result.cost.Pool)
				require.Equal(t, 4, result.cost.Charged)
				require.Equal(t, 6, char.Conviction)
				require.Equal(t, awareness.Visible, char.Awareness.State())
				require.Greater(t, char.Cooldowns["special-move"], 0)
				require.Equal(t, 1, char.RoundsWaiting())
				require.True(t, char.HasBuff(result.selfBuffID))
				require.True(t, char.HasCondition(result.selfCondition))
				require.Equal(t, []string{string(skills.Rhetoric)}, actor.skillsUsed)
			})
		}
	}
}

// This test used to demand an ordinary paid MISS. The U6b Task 5 collapse
// deleted the miss outcome with the gate: a taunt the defender out-rolls is
// now a DEFENDED taunt (Hit=true, Defence.Defended=true, partial damage),
// exactly like a defended spell cast. What the test actually pinned — a paid,
// noisy, cooldown-consuming attempt that does not land cleanly still pays
// exactly once, reveals the actor, and fires exactly one skill use — survives
// on the defended outcome, so it is re-pinned there. The hopeless attacker
// (Cha 1 vs Willpower 1,000,000) makes the defence win a decisive defensive
// crit, so the target's conviction moves ONLY by the admitted defy cost.
func TestTauntAffordableDefendedPaysOnceAndConsumesNoisyAction(t *testing.T) {
	pinTauntContestKnobs(t)

	// Retried rather than seeded, to skip the ~2.3% self-relative fumbles.
	// rand.Seed has been a no-op since Go 1.20 unless GODEBUG=randseednop=0 is
	// set, which this file does not set, so each iteration is an independent
	// draw -- which is what makes retrying until a non-fumble work.
	for attempt := 0; attempt < 20; attempt++ {
		actor, char, target := newRhetoricActor(t, false, 10, 0)
		hideRhetoricActor(t, char)
		startTargetConviction := target.Conviction
		result := ExecuteTaunt(actor)
		if result.Fumble {
			continue
		}

		require.True(t, result.Executed)
		require.True(t, result.Hit, "a non-fumble taunt is always delivered since the collapse")
		require.True(t, result.Defence.Defended, "a Cha-1 taunter cannot out-roll a Willpower-1e6 defy")
		require.True(t, result.Defence.DefensiveCrit, "the hopeless gap makes every defy win decisive")
		require.Zero(t, result.Damage, "a defensive crit fully negates")
		require.Equal(t, characters.CostPaid, result.Cost.Status)
		require.Equal(t, 4, result.Cost.Charged)
		require.Equal(t, 6, char.Conviction)
		require.Equal(t, startTargetConviction-result.Defence.Cost.Charged, target.Conviction,
			"only the admitted defy cost may move the defender's conviction")
		require.Equal(t, awareness.Visible, char.Awareness.State())
		require.Greater(t, char.Cooldowns["special-move"], 0)
		require.Equal(t, 1, char.RoundsWaiting())
		require.Equal(t, []string{string(skills.Rhetoric)}, actor.skillsUsed)
		return
	}
	t.Fatal("twenty hopeless taunts all fumbled; the retry loop is broken")
}

func TestTauntRefusalDoesNotCommitStagedAggression(t *testing.T) {
	base, char, target := newRhetoricActor(t, true, 0, 0)
	char.EndAggro()
	target.EndAggro()
	commits := 0
	staged := &stagedMeleeActor{
		Actor:  base,
		target: AggroTarget{Char: target, Name: target.Name, MobInstanceId: target.MobInstanceId, Found: true},
		commit: func() {
			commits++
			char.SetAggro(0, target.MobInstanceId, characters.DefaultAttack)
			target.SetAggro(base.GetUserId(), 0, characters.DefaultAttack)
		},
	}

	result := ExecuteTaunt(staged)

	require.Equal(t, characters.CostRefused, result.Cost.Status)
	require.Zero(t, commits)
	require.False(t, char.IsInCombat())
	require.False(t, target.IsInCombat())
}

type staleRhetoricTargetActor struct {
	Actor
	targetID       int
	characterCalls int
}

func (a *staleRhetoricTargetActor) GetCharacter() *characters.Character {
	a.characterCalls++
	char := a.Actor.GetCharacter()
	if a.characterCalls == 2 {
		mobs.SetInstanceForTest(a.targetID, nil)
	}
	return char
}

func TestTauntPaidStaleTargetPreservesEffectsAndEngagement(t *testing.T) {
	actor, char, target := newRhetoricActor(t, false, 10, 0)
	hideRhetoricActor(t, char)
	targetID := char.CurrentCombatTarget().MobInstanceId
	target.SetAggro(505, 0, characters.DefaultAttack)
	target.SetRoundsWaiting(9)
	targetAggro := target.CurrentCombatTarget()
	targetConviction := target.Conviction
	stale := &staleRhetoricTargetActor{Actor: actor, targetID: targetID}

	result := ExecuteTaunt(stale)

	require.True(t, result.NoTarget)
	require.False(t, result.Executed)
	require.Equal(t, characters.CostPaid, result.Cost.Status)
	require.Equal(t, 4, result.Cost.Charged)
	require.Equal(t, 6, char.Conviction)
	require.Equal(t, awareness.Hidden, char.Awareness.State())
	require.Empty(t, char.Cooldowns)
	require.Zero(t, char.RoundsWaiting())
	require.Equal(t, targetConviction, target.Conviction)
	require.Equal(t, targetAggro, target.CurrentCombatTarget())
	require.Empty(t, actor.skillsUsed)
}

type rhetoricAdmissionRaceActor struct {
	Actor
	characterCalls int
	onAdmission    func()
}

func (a *rhetoricAdmissionRaceActor) GetCharacter() *characters.Character {
	a.characterCalls++
	char := a.Actor.GetCharacter()
	if a.characterCalls == 2 && a.onAdmission != nil {
		a.onAdmission()
	}
	return char
}

func TestTauntPaidMovedTargetPreservesEffectsAndEngagement(t *testing.T) {
	actor, char, target := newRhetoricActor(t, false, 10, 0)
	hideRhetoricActor(t, char)
	target.SetAggro(606, 0, characters.DefaultAttack)
	target.SetRoundsWaiting(8)
	targetAggro := target.CurrentCombatTarget()
	targetConviction := target.Conviction
	targetConditions := len(target.Conditions)
	targetBuffs := len(target.Buffs.GetBuffs())
	race := &rhetoricAdmissionRaceActor{Actor: actor, onAdmission: func() {
		target.RoomId = 2
	}}

	result := ExecuteTaunt(race)

	require.True(t, result.NoTarget)
	require.False(t, result.Executed)
	require.Equal(t, characters.CostPaid, result.Cost.Status)
	require.Equal(t, 4, result.Cost.Charged)
	require.Equal(t, 6, char.Conviction)
	require.Equal(t, awareness.Hidden, char.Awareness.State())
	require.Empty(t, char.Cooldowns)
	require.Equal(t, 2, target.RoomId)
	require.Zero(t, char.RoundsWaiting())
	require.Equal(t, targetConviction, target.Conviction)
	require.Equal(t, targetAggro, target.CurrentCombatTarget())
	require.Len(t, target.Conditions, targetConditions)
	require.Len(t, target.Buffs.GetBuffs(), targetBuffs)
	require.Empty(t, actor.skillsUsed)
}

func TestTauntPaidSwappedAggroPreservesBothTargetsAndRound(t *testing.T) {
	actor, char, original := newRhetoricActor(t, false, 10, 0)
	hideRhetoricActor(t, char)
	original.SetAggro(707, 0, characters.DefaultAttack)
	original.SetRoundsWaiting(7)
	originalAggro := original.CurrentCombatTarget()
	originalConviction := original.Conviction
	originalConditions := len(original.Conditions)
	originalBuffs := len(original.Buffs.GetBuffs())

	rhetoricTargetID++
	replacement := characters.New()
	replacement.Name = "Replacement Target"
	replacement.RoomId = 1
	replacement.MobInstanceId = rhetoricTargetID
	replacement.ConvictionMax.Base = 1_000_000
	replacement.ConvictionMax.Recalculate()
	replacement.Conviction = 1_000_000
	replacement.Stats.Willpower.ValueAdj = 1_000_000
	replacement.Buffs = buffs.New()
	replacement.SetAggro(808, 0, characters.DefaultAttack)
	replacement.SetRoundsWaiting(6)
	replacementMob := &mobs.Mob{InstanceId: rhetoricTargetID, Character: *replacement}
	mobs.SetInstanceForTest(replacementMob.InstanceId, replacementMob)
	t.Cleanup(func() { mobs.SetInstanceForTest(replacementMob.InstanceId, nil) })
	replacement = &replacementMob.Character
	replacementAggro := replacement.CurrentCombatTarget()
	replacementConviction := replacement.Conviction
	replacementConditions := len(replacement.Conditions)
	replacementBuffs := len(replacement.Buffs.GetBuffs())
	race := &rhetoricAdmissionRaceActor{Actor: actor, onAdmission: func() {
		char.SetAggro(0, replacementMob.InstanceId, characters.DefaultAttack)
	}}

	result := ExecuteTaunt(race)

	require.True(t, result.NoTarget)
	require.False(t, result.Executed)
	require.Equal(t, characters.CostPaid, result.Cost.Status)
	require.Equal(t, 4, result.Cost.Charged)
	require.Equal(t, 6, char.Conviction)
	require.Equal(t, awareness.Hidden, char.Awareness.State())
	require.Empty(t, char.Cooldowns)
	require.Equal(t, replacementMob.InstanceId, char.CurrentCombatTarget().MobInstanceId)
	require.Zero(t, char.RoundsWaiting())
	require.Equal(t, originalConviction, original.Conviction)
	require.Equal(t, originalAggro, original.CurrentCombatTarget())
	require.Len(t, original.Conditions, originalConditions)
	require.Len(t, original.Buffs.GetBuffs(), originalBuffs)
	require.Equal(t, replacementConviction, replacement.Conviction)
	require.Equal(t, replacementAggro, replacement.CurrentCombatTarget())
	require.Len(t, replacement.Conditions, replacementConditions)
	require.Len(t, replacement.Buffs.GetBuffs(), replacementBuffs)
	require.Empty(t, actor.skillsUsed)
}

func TestTauntRallyWarcryPaidStaleCooldownPreservesEffects(t *testing.T) {
	cleanup := seedBuffsForTest()
	defer cleanup()

	for _, tc := range rhetoricActionCases() {
		t.Run(tc.name, func(t *testing.T) {
			actor, char, target := newRhetoricActor(t, false, 10, 0)
			hideRhetoricActor(t, char)
			targetConviction := target.Conviction
			stale := &staleCooldownActor{Actor: actor}

			result := tc.execute(stale)

			require.False(t, result.executed)
			require.True(t, result.onCooldown)
			require.Equal(t, characters.CostPaid, result.cost.Status)
			require.Equal(t, 4, result.cost.Charged)
			require.Equal(t, 6, char.Conviction)
			require.Equal(t, awareness.Hidden, char.Awareness.State())
			require.Equal(t, 3, char.Cooldowns["special-move"])
			require.Zero(t, char.RoundsWaiting())
			require.Equal(t, targetConviction, target.Conviction)
			require.Empty(t, actor.skillsUsed)
			if result.selfBuffID != 0 {
				require.False(t, char.HasBuff(result.selfBuffID))
				require.False(t, char.HasCondition(result.selfCondition))
			}
		})
	}
}

func TestTauntRallyWarcryQuotesUseSkillButIgnoreEncumbrance(t *testing.T) {
	quote := func(t *testing.T, action costs.Action, rank int, laden bool) characters.CostCommitResult {
		t.Helper()
		char := characters.New()
		char.Conviction = 100
		char.ConvictionMax.Value = 100
		char.Skills[string(skills.Rhetoric)] = rank
		if laden {
			char.Items = []items.Item{{ItemId: 9701, Spec: &items.ItemSpec{ItemId: 9701, Name: "rhetoric ballast", Weight: char.CarryCapacity()}}}
		}
		return char.CommitCost(char.QuoteActionCost(characters.ActionCostRequest{
			Action: action, Pool: characters.PoolConviction, Base: 10, Modifier: 1, Units: 1,
		}), characters.CostFullOrRefuse)
	}

	for _, action := range []costs.Action{costs.ActionTaunt, costs.ActionRally, costs.ActionWarcry} {
		t.Run(string(action), func(t *testing.T) {
			novice := quote(t, action, 0, false)
			master := quote(t, action, 100, false)
			laden := quote(t, action, 0, true)
			require.Equal(t, 11, novice.Charged, "rank-zero literal quote")
			require.Equal(t, 4, master.Charged, "rank-100 literal quote")
			require.Equal(t, 11, laden.Charged, "nonphysical quote must ignore full encumbrance")
		})
	}
}

func (r *recordingActor) GetCharacter() *characters.Character { return r.char }
func (r *recordingActor) OnSkillUse(skillName string) bool {
	r.skillsUsed = append(r.skillsUsed, skillName)
	return true
}

// AwardResolved records the same way OnSkillUse does, so every skillsUsed
// assertion in this file keeps working after U10b-1 Task 18b routed the
// rhetoric sites through the Best-of seam.
//
// The recorded value is the CANDIDATE'S skill, which is what those assertions
// were ever about; the win/lose weight is asserted separately where it matters.
func (r *recordingActor) AwardResolved(won bool, cands ...progression.Candidate) {
	for _, c := range cands {
		if c.Skill != "" {
			r.skillsUsed = append(r.skillsUsed, c.Skill)
		}
	}
}

// TestRegression_WarcryRallyAwardRhetoric locks the fix for the 2026-07-20
// audit finding: warcry and rally left Rhetoric progression to their callers,
// unlike the other migrated special moves which award it inside actions/. The
// player wrappers implemented it; the mob wrappers never did, so mobs could
// warcry and rally indefinitely without ever building Rhetoric.
//
// Progression now lives in ExecuteWarcry/ExecuteRally, so every Actor
// implementation gets it. In combat it always fires, which is what these tests
// pin (the out-of-combat path is a 50% roll and is deliberately not asserted).
func TestRegression_WarcryRallyAwardRhetoric(t *testing.T) {
	newInCombatActor := func() *recordingActor {
		c := &characters.Character{
			Name:  "Test Subject",
			Aggro: &characters.Aggro{}, // non-nil Aggro => IsInCombat()
		}
		c.ConvictionMax.Base = 100
		c.Validate()
		c.Conviction = 100
		return &recordingActor{char: c}
	}

	t.Run("warcry_awards_rhetoric_in_combat", func(t *testing.T) {
		a := newInCombatActor()
		require.True(t, a.char.IsInCombat(), "precondition: actor must be in combat")

		res := ExecuteWarcry(a)
		require.True(t, res.Executed, "precondition: warcry must have executed")

		assert.Contains(t, a.skillsUsed, string(skills.Rhetoric),
			"warcry must award Rhetoric progression for every actor type")
	})

	t.Run("rally_awards_rhetoric_in_combat", func(t *testing.T) {
		a := newInCombatActor()
		require.True(t, a.char.IsInCombat(), "precondition: actor must be in combat")

		res := ExecuteRally(a)
		require.True(t, res.Executed, "precondition: rally must have executed")

		assert.Contains(t, a.skillsUsed, string(skills.Rhetoric),
			"rally must award Rhetoric progression for every actor type")
	})

	// A blocked execution must not award progression — otherwise a mob on
	// cooldown could farm Rhetoric by spamming the command.
	t.Run("blocked_warcry_awards_nothing", func(t *testing.T) {
		a := newInCombatActor()

		first := ExecuteWarcry(a)
		require.True(t, first.Executed)
		countAfterFirst := len(a.skillsUsed)

		second := ExecuteWarcry(a)
		require.False(t, second.Executed, "precondition: second warcry must be blocked")

		assert.Len(t, a.skillsUsed, countAfterFirst,
			"a blocked warcry must not award progression")
	})
}
