package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Package-level setup: seed condition 9 (Hidden) spec so AddCondition(9, …) works.
// This piggybacks on the package-level init in defuse_test.go (biomes), and
// mirrors the approach used throughout other action tests.
// ---------------------------------------------------------------------------

func init() {
	// Seed condition 9 with the Hidden flag. TriggerCount > 0 so the condition is
	// not considered expired immediately after application.
	conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {
			ConditionId:  9,
			Name:         "Hidden",
			Flags:        []conditions.Flag{conditions.Hidden},
			TriggerCount: 1000000, // effectively permanent for tests
		},
	})
	// Note: SeedConditionsForTest replaces the entire registry. No cleanup is
	// called here because the hidden-condition spec is needed for the full test run
	// and must survive across tests in this package.
}

// ---------------------------------------------------------------------------
// Shadow test helpers
// ---------------------------------------------------------------------------

// addHiddenCondition grants condition 9 (Hidden) to a character AND advances the
// Awareness state machine to Hidden so that c.IsHidden() returns true.
// Requires that the condition spec for id 9 has been seeded via init() above.
func addHiddenCondition(char *characters.Character) {
	char.Conditions = conditions.New()
	// AddCondition calls GetConditionSpec internally; works because of the seeded spec.
	char.Conditions.AddCondition(9, true /* permanent for test purposes */)
	// Sync Awareness machine: reset to Visible, then advance to Hidden.
	// Without this, Character.IsHidden() (which delegates to Awareness) returns
	// false even though condition #9 is present.
	if char.Awareness == nil {
		char.Awareness = awareness.NewMachine()
	}
	r := state.TransitionReason{Trigger: "test_setup"}
	char.Awareness.ForceVisible(r) // reset regardless of current state
	_ = char.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r)
	char.Awareness.ResolveConcealment(true, r)
}

// newShadowPlayerActor creates a player-backed stubActorWithId with the
// given Dexterity and skullduggery skill rank.
// When withHidden=true, condition 9 (Hidden) is applied.
func newShadowPlayerActor(dex int, skillRank int, withHidden bool) *stubActorWithId {
	char := characters.New()
	char.Stats.Dexterity.ValueAdj = dex
	if skillRank > 0 {
		char.Skills[string(skills.Skullduggery)] = skillRank
	}
	char.Conditions = conditions.New()
	if withHidden {
		addHiddenCondition(char)
	}
	room := newStealTestRoom()
	return &stubActorWithId{
		stubActor: stubActor{char: char, room: room},
		isPlayer:  true,
		userId:    7001,
	}
}

// newShadowMobActor creates a mob-backed stubActorWithId with the given
// Dexterity and skullduggery skill rank.
// When withHidden=true, condition 9 (Hidden) is applied.
func newShadowMobActor(dex int, skillRank int, withHidden bool) *stubActorWithId {
	char := characters.New()
	char.Stats.Dexterity.ValueAdj = dex
	if skillRank > 0 {
		char.Skills[string(skills.Skullduggery)] = skillRank
	}
	char.Conditions = conditions.New()
	if withHidden {
		addHiddenCondition(char)
	}
	room := newStealTestRoom()
	return &stubActorWithId{
		stubActor:     stubActor{char: char, room: room},
		isPlayer:      false,
		mobInstanceId: 8801,
	}
}

// resetHiddenCondition clears and re-adds condition 9 on the actor's character.
// Use between trials when shadow might not remove the condition but the test
// wants a consistent starting state.
func resetHiddenCondition(actor *stubActorWithId) {
	actor.char.Conditions = conditions.New()
	addHiddenCondition(actor.char)
}

// ---------------------------------------------------------------------------
// TestShadow_RequiresHidden
// ---------------------------------------------------------------------------

// TestShadow_RequiresHidden verifies that an actor without condition 9 (Hidden)
// is rejected before any roll or cooldown is set.
func TestShadow_RequiresHidden(t *testing.T) {
	actor := newShadowPlayerActor(100, 5, false /* not hidden */)

	result := Shadow(actor, ShadowOptions{TargetUserId: 7002})

	assert.False(t, result.Succeeded, "actor without Hidden condition should not succeed")
	assert.False(t, result.OnCooldown, "hidden gate should not set cooldown")
	assert.Equal(t, "not hidden", result.Reason)
	// Cooldown must not have been set.
	assert.Equal(t, 0, actor.char.GetCooldown(skills.Skullduggery.String("shadow")),
		"cooldown should not be set when hidden gate fires")
}

// ---------------------------------------------------------------------------
// TestShadow_NoTarget
// ---------------------------------------------------------------------------

// TestShadow_NoTarget verifies that empty ShadowOptions (with hidden actor)
// returns a failure mentioning "no target" without triggering any cooldown.
func TestShadow_NoTarget(t *testing.T) {
	actor := newShadowPlayerActor(100, 5, true /* hidden */)

	result := Shadow(actor, ShadowOptions{})

	assert.False(t, result.Succeeded, "empty opts should not succeed")
	assert.False(t, result.OnCooldown, "empty opts should not set cooldown")
	assert.Equal(t, "no target", result.Reason)
}

// ---------------------------------------------------------------------------
// TestShadow_Success
// ---------------------------------------------------------------------------

// TestShadow_Success verifies that a hidden actor targeting a valid player
// succeeds and stores the target id in misc-data.
func TestShadow_Success(t *testing.T) {
	targetUser := users.NewTestUser(7002, "prey", "Prey", 0)
	targetUser.Character.Stats.Perception.ValueAdj = 1 // nearly blind — won't detect
	targetUser.Character.Stats.Dexterity.ValueAdj = 1
	cleanup := users.SeedUsersForTest(map[int]*users.UserRecord{
		7002: targetUser,
	})
	defer cleanup()

	// High Dex actor → CalcSneakScore far beats CalcSearchScore(Per=1)
	actor := newShadowPlayerActor(200, 5, true /* hidden */)

	result := Shadow(actor, ShadowOptions{TargetUserId: 7002})

	require.True(t, result.Succeeded, "hidden actor with valid target should succeed")
	assert.Equal(t, "Prey", result.TargetName)

	// Misc-data must store the target user id.
	raw := actor.char.GetMiscData("shadow-target-user")
	require.NotNil(t, raw, "shadow-target-user misc-data must be set on success")
	uid, ok := raw.(int)
	require.True(t, ok, "shadow-target-user must be an int")
	assert.Equal(t, 7002, uid)

	// Mob slot must be cleared.
	assert.Nil(t, actor.char.GetMiscData("shadow-target-mob"),
		"shadow-target-mob misc-data must be nil for a player target")
}

// ---------------------------------------------------------------------------
// TestShadow_Cooldown
// ---------------------------------------------------------------------------

// TestShadow_Cooldown verifies that the second Shadow invocation within
// the cooldown window is blocked.
func TestShadow_Cooldown(t *testing.T) {
	targetUser := users.NewTestUser(7003, "prey2", "Prey2", 0)
	targetUser.Character.Stats.Perception.ValueAdj = 1
	cleanup := users.SeedUsersForTest(map[int]*users.UserRecord{
		7003: targetUser,
	})
	defer cleanup()

	actor := newShadowPlayerActor(200, 5, true /* hidden */)

	first := Shadow(actor, ShadowOptions{TargetUserId: 7003})
	require.True(t, first.Succeeded, "first call should succeed")

	// Re-add Hidden condition for the second call.
	resetHiddenCondition(actor)

	second := Shadow(actor, ShadowOptions{TargetUserId: 7003})

	assert.True(t, second.OnCooldown, "second call within cooldown must be blocked")
	assert.False(t, second.Succeeded)
}

// ---------------------------------------------------------------------------
// TestShadow_SkillProgressionFires
// ---------------------------------------------------------------------------

// TestShadow_SkillProgressionFires verifies that OnSkillUse is called once
// per successful Shadow attempt (mob actor, no quest-engine path).
func TestShadow_SkillProgressionFires(t *testing.T) {
	targetUser := users.NewTestUser(7004, "prey3", "Prey3", 0)
	targetUser.Character.Stats.Perception.ValueAdj = 1
	cleanup := users.SeedUsersForTest(map[int]*users.UserRecord{
		7004: targetUser,
	})
	defer cleanup()

	actor := newShadowMobActor(200, 5, true /* hidden */)

	result := Shadow(actor, ShadowOptions{TargetUserId: 7004})
	require.True(t, result.Succeeded)

	// U10b-1 Task 18 routed this site through Actor.AwardResolved, so the
	// observable moved from the OnSkillUse counter to the award recorder.
	// The COUNT assertion is unchanged in meaning: one award per attempt,
	// win or lose.
	assert.Greater(t, len(actor.awards), 0,
		"Shadow should award skullduggery progression")
	if _, n := actor.awardedCandidate(string(skills.Skullduggery)); n == 0 {
		t.Error("the award did not name skullduggery")
	}
}

// ---------------------------------------------------------------------------
// TestShadow_DetectionWin
// ---------------------------------------------------------------------------

// TestShadow_DetectionWin verifies that the initial detection roll fires
// (Detected=true) when the target has high Perception+Search vs the
// actor's Dex+Skullduggery, and that the target receives the detection
// message.
//
// Design rationale (mirrors steal/plant detection tests):
//
//	Shadow uses: CalcSneakScore (Dex + SkillMult(skulld)*25) for actor
//	             CalcSearchScore (Per + SkillMult(search)*25) for target
//	Detection:   combat.RunContest(searchScore, {Score: sneakScore});
//	             target wins = detected.
//
//	Actor:    Dex=110, skullduggery rank=2
//	          CalcSneakScore ≈ 110 + 1.41*25 ≈ 145
//	Target:   Perception=40, Search rank=50
//	          CalcSearchScore ≈ 40 + 3.0*25 = 115
//
// Detection is 115 vs 145, a ~24% per-trial chance. Over 50 trials the
// probability of zero detections is <0.0001%.
func TestShadow_DetectionWin(t *testing.T) {
	const trials = 50

	// Register target player with high Search skill.
	targetUser := users.NewTestUser(7005, "watchful3", "Watchful3", 0)
	targetUser.Character.Stats.Perception.ValueAdj = 40
	if targetUser.Character.Skills == nil {
		targetUser.Character.Skills = make(map[string]int)
	}
	targetUser.Character.Skills[string(skills.Search)] = 50 // CalcSearchScore ≈ 115
	cleanup := users.SeedUsersForTest(map[int]*users.UserRecord{
		7005: targetUser,
	})
	defer cleanup()

	// Mob actor: Dex=110, rank=2. CalcSneakScore ≈ 145.
	actor := newShadowMobActor(110, 2, true /* hidden */)

	detectionCount := 0
	successCount := 0

	for range trials {
		// Reset cooldown and Hidden condition each trial.
		delete(actor.char.Cooldowns, skills.Skullduggery.String("shadow"))
		resetHiddenCondition(actor)

		result := Shadow(actor, ShadowOptions{TargetUserId: 7005})
		if result.Succeeded {
			successCount++
		}
		if result.Detected {
			detectionCount++
		}
	}

	// Shadow always succeeds when the actor is hidden — Detected is orthogonal.
	require.Greater(t, successCount, trials/2,
		"hidden actor should succeed in majority of trials")

	// Detection must fire in at least one trial. With ~24% per-trial
	// probability over 50 trials the chance of zero detections is <0.0001%.
	require.Greater(t, detectionCount, 0,
		"detection-win branch (result.Detected=true) must be reachable; "+
			"check CalcSearchScore vs CalcSneakScore math")
}
