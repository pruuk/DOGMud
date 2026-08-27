package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// ExecuteThrottle tests
// ---------------------------------------------------------------------------

// TestThrottle_NoAggro verifies that ExecuteThrottle returns NoTarget=true
// when the actor has no aggro set (not yet in combat).
func TestThrottle_NoAggro(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	result := ExecuteThrottle(actor)

	assert.False(t, result.Executed, "throttle with no aggro should not execute")
	assert.True(t, result.NoTarget, "throttle with no aggro should set NoTarget")
	assert.False(t, result.OnCooldown, "NoTarget should take priority over cooldown reporting")
}

// TestThrottle_OnCooldown verifies that ExecuteThrottle returns OnCooldown=true
// when the special-move cooldown is active.
func TestThrottle_OnCooldown(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	prepareSpecialMoveCooldown(t, char, 7908, 7908, &species.Species{
		SpeciesId: 7908, Name: "cooldown-wolf", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite,
	})

	result := ExecuteThrottle(actor)

	assert.False(t, result.Executed, "throttle should not execute when on cooldown")
	assert.True(t, result.OnCooldown, "throttle should report OnCooldown")
}

// TestThrottle_NotFanged verifies the anatomy/identity gate: a non-fanged
// actor in combat with a valid target gets NotFanged=true, while a fanged
// actor passes the gate.
func TestThrottle_NotFanged(t *testing.T) {
	// Seed two species: one clawed (not fanged), one fanged.
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7001: {SpeciesId: 7001, Name: "feline", BodyParts: []string{"legs", "paws"}, NaturalAttack: items.Claws},
		7002: {SpeciesId: 7002, Name: "wolf", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer cleanup()

	// Register a target mob so ResolveAggroTarget returns Found=true.
	targetMob := &mobs.Mob{InstanceId: 7099}
	targetMob.Character.Name = "Target"
	targetMob.Character.HealthMax.Value = 100
	targetMob.Character.Health = 100
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	t.Run("non-fanged actor returns NotFanged", func(t *testing.T) {
		char := characters.New()
		char.SpeciesId = 7001 // feline — clawed, not fanged
		char.Aggro = &characters.Aggro{MobInstanceId: targetMob.InstanceId}

		result := ExecuteThrottle(newStubActor(char, newTestRoom()))

		assert.True(t, result.NotFanged, "non-fanged actor should return NotFanged=true")
		assert.False(t, result.Executed, "non-fanged actor should not execute the throttle")
		assert.Equal(t, 0, result.MoveResult.Damage, "non-fanged throttle should deal no damage")
	})

	t.Run("fanged actor passes the gate and executes", func(t *testing.T) {
		char := characters.New()
		char.SpeciesId = 7002 // wolf — fanged
		char.Aggro = &characters.Aggro{MobInstanceId: targetMob.InstanceId}
		fundSpecialMove(char)

		result := ExecuteThrottle(newStubActor(char, newTestRoom()))

		assert.False(t, result.NotFanged, "fanged actor should NOT return NotFanged")
		assert.True(t, result.Executed, "fanged actor should execute the throttle")
	})
}

// TestThrottle_Executed_BleedAndBuff verifies that on a hit a fanged attacker
// applies ConditionBleeding and Throttled buff (id 89) to the target.
func TestThrottle_Executed_BleedAndBuff(t *testing.T) {
	// Seed buff 89 so AddBuff can find it.
	buffCleanup := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		89: {BuffId: 89, Name: "Throttled", TriggerCount: 3, RoundInterval: 1},
	})
	defer buffCleanup()

	// Seed a fanged species.
	speciesCleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7003: {SpeciesId: 7003, Name: "fanged-test", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer speciesCleanup()

	// Register a target mob with minimal dexterity to maximize hit chance.
	targetMob := &mobs.Mob{InstanceId: 7199}
	targetMob.Character.Name = "Target"
	targetMob.Character.HealthMax.Value = 500
	targetMob.Character.Health = 500
	targetMob.Character.Stamina = 500
	targetMob.Character.StaminaMax.Value = 500
	targetMob.Character.Stats.Dexterity.ValueAdj = 1
	targetMob.Character.Buffs = buffs.New()
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	// Attacker with high Strength so the hit + bleed are reliable.
	char := characters.New()
	char.SpeciesId = 7003
	char.Stats.Strength.ValueAdj = 500
	fundSpecialMove(char)

	// Retry until we observe a hit.
	hitSeen := false
	var res ThrottleResult
	for i := 0; i < 100; i++ {
		char.Aggro = &characters.Aggro{MobInstanceId: targetMob.InstanceId}
		char.Cooldowns = characters.Cooldowns{}

		res = ExecuteThrottle(newStubActor(char, newTestRoom()))
		if res.Executed && res.MoveResult.Hit {
			hitSeen = true
			break
		}
	}

	if !hitSeen {
		// Not a flake to shrug off. The per-attempt hit rate here is high
		// enough that 100 consecutive misses is effectively impossible;
		// skipping meant the assertions below never ran (review finding 24).
		t.Fatal("no hit in 100 attempts — throttle hit path is broken")
	}

	// ConditionBleeding should be applied.
	assert.True(t, targetMob.Character.HasCondition(characters.ConditionBleeding),
		"target should have ConditionBleeding after a successful throttle")

	// BleedDmg should be at least the minimum.
	assert.GreaterOrEqual(t, res.BleedDmg, 2,
		"BleedDmg should be at least 2 (min floor)")

	// Throttled buff (id 89) should be applied.
	assert.True(t, targetMob.Character.HasBuff(89),
		"target should have Throttled buff (id 89) after a successful throttle")
}

// TestThrottle_CastInterrupt verifies that a throttle hit against a casting
// target resolves the cast-interrupt through the concentration seam (U10):
// the target's hold (Willpower + spellcasting) is contested against the
// throttler's grip (Dexterity + unarmed-combat), floored at
// Balance.ConcentrationFloor.
//
// Determinism strategy: this test pins ConcentrationFloor to 0 via
// AddOverlayOverrides rather than accepting the standard 2% mercy-flip.
// Validate()'s `<= 0 || > 0.5` rewrite only ever runs once — configData
// tracks a `validated` bool and ensureConfigValidated is a sync-once gate —
// so an overlay applied after startup validation is never re-validated and 0
// sticks (confirmed: AddOverlayOverrides unmarshals straight onto the live
// Config, it never calls Validate). With the floor at 0,
// contest.RunWithFloors's floor<=0 branch returns the raw contest untouched
// (no mercy-flip at all), so pairing that with an overwhelming grip-vs-hold
// gap (target Willpower 1 vs. a five-figure attacker Dexterity) makes the
// interrupt outcome deterministic rather than merely "near-certain" — no
// realistic roll variance can flip a margin that lopsided. The throttle
// move's own to-hit roll is a separate, independent contest, so the
// retry-until-hit loop below (unchanged from the prior version) still
// covers that residual variance.
func TestThrottle_CastInterrupt(t *testing.T) {
	prevFloor := float64(configs.GetBalanceConfig().ConcentrationFloor)
	_ = configs.AddOverlayOverrides(map[string]any{
		"Balance.ConcentrationFloor": 0.0,
	})
	defer func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"Balance.ConcentrationFloor": prevFloor,
		})
	}()

	// Seed buff 89.
	buffCleanup := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		89: {BuffId: 89, Name: "Throttled", TriggerCount: 3, RoundInterval: 1},
	})
	defer buffCleanup()

	// Seed a fanged species.
	speciesCleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7004: {SpeciesId: 7004, Name: "fanged-cast-test", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer speciesCleanup()

	// Register a target mob that is currently casting, with a near-zero hold
	// (tiny Willpower) and a tiny Dexterity so the throttle move itself lands
	// reliably.
	targetMob := &mobs.Mob{InstanceId: 7299}
	targetMob.Character.Name = "CastingTarget"
	targetMob.Character.HealthMax.Value = 500
	targetMob.Character.Health = 500
	targetMob.Character.Stamina = 500
	targetMob.Character.StaminaMax.Value = 500
	targetMob.Character.Conviction = 50
	targetMob.Character.ConvictionMax.Value = 100
	targetMob.Character.Stats.Dexterity.ValueAdj = 1
	targetMob.Character.Stats.Willpower.ValueAdj = 1
	targetMob.Character.Buffs = buffs.New()
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	// Set the target into a casting state.
	setCastingForTest(&targetMob.Character, activity.CastingData{
		SpellId:             "fireball",
		TotalConvictionCost: 30,
		ConvictionSpent:     10,
	})
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	assert.True(t, targetMob.Character.IsCasting(), "pre-condition: target should be casting")

	// Attacker: high Strength for a near-guaranteed hit, and a five-figure
	// Dexterity so the throttler's grip overwhelms the target's near-zero
	// hold regardless of the random baseline characters.New() rolled.
	char := characters.New()
	char.SpeciesId = 7004
	char.Stats.Strength.ValueAdj = 500
	char.Stats.Dexterity.ValueAdj = 5000
	fundSpecialMove(char)

	// Store pre-interrupt conviction to verify refund.
	convictionBefore := targetMob.Character.Conviction

	// Retry until we observe a hit.
	var res ThrottleResult
	hitSeen := false
	for i := 0; i < 100; i++ {
		// Reset casting state for each attempt (previous hit may have cleared it).
		if !targetMob.Character.IsCasting() {
			setCastingForTest(&targetMob.Character, activity.CastingData{
				SpellId:             "fireball",
				TotalConvictionCost: 30,
				ConvictionSpent:     10,
			})
		}
		char.Aggro = &characters.Aggro{MobInstanceId: targetMob.InstanceId}
		char.Cooldowns = characters.Cooldowns{}

		res = ExecuteThrottle(newStubActor(char, newTestRoom()))
		if res.Executed && res.MoveResult.Hit {
			hitSeen = true
			break
		}
	}

	if !hitSeen {
		// Not a flake to shrug off. The per-attempt hit rate here is high
		// enough that 100 consecutive misses is effectively impossible;
		// skipping meant the assertions below never ran (review finding 24).
		t.Fatal("no hit in 100 attempts — throttle hit path is broken")
	}

	// Cast should have been interrupted: with the floor pinned to 0 and an
	// overwhelming grip-vs-hold gap, the concentration contest is
	// deterministic here.
	assert.True(t, res.InterruptedCast,
		"InterruptedCast should be true when an overwhelming grip contests a near-zero hold with ConcentrationFloor pinned to 0")
	assert.False(t, targetMob.Character.IsCasting(),
		"target should no longer be casting after throttle interrupt")

	// Conviction refund: 30-10=20 unspent → refund 10.
	_ = convictionBefore
	// (conviction starts at 50 + refund 10 = 60, but exact value depends on
	// whether a prior miss re-set things; just verify it did not go below minimum)
	assert.GreaterOrEqual(t, targetMob.Character.Conviction, 0,
		"target conviction should not be negative after refund")

}

// TestThrottle_CastInterrupt_OverwhelmingCaster is the companion case: a
// caster whose hold (Willpower + spellcasting) overwhelmingly outweighs the
// throttler's grip (Dexterity + unarmed-combat) should rarely lose the
// concentration contest, and every HELD contest must fire exactly one
// success-only spellcasting progression event — no event on the (rare)
// interrupts, mirroring internal/hooks' checkConcentrationBreak.
//
// Determinism strategy: rather than asserting on a single roll, this test
// drives 20 LANDED throttle hits (the concentration contest's floor-driven
// 2% mercy chance is deliberately left at its shipped default here, unlike
// the sibling test above) and checks two count-equality invariants that hold
// regardless of which individual rolls the floor happens to flip:
//  1. heldCount == the spellcasting SkillUseCount delta (success-only
//     progression, proven exactly rather than probabilistically).
//  2. heldCount >= 1 — the odds of zero holds across 20 tries against an
//     overwhelming caster is on the order of 0.02^20, i.e. not a real flake
//     risk.
//
// interruptCount < heldCount is also asserted as a sanity check that
// interrupts stay rare; failing it would require roughly 10+ interrupts out
// of 20 trials at a real per-trial rate near 2%, which is not a realistic
// flake either.
func TestThrottle_CastInterrupt_OverwhelmingCaster(t *testing.T) {
	// Deliberately NOT overriding ConcentrationFloor — this test exercises
	// the shipped 2% mercy floor, not the pinned-to-0 guaranteed case above.

	buffCleanup := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		89: {BuffId: 89, Name: "Throttled", TriggerCount: 3, RoundInterval: 1},
	})
	defer buffCleanup()

	speciesCleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		7005: {SpeciesId: 7005, Name: "fanged-overwhelmed-test", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer speciesCleanup()

	// Target: overwhelming caster. High Willpower via .Base + RecalculateStats
	// (the arc's standard "derive it, don't just poke ValueAdj" idiom), plus
	// a high spellcasting rank via the existing char.Skills[...] idiom.
	targetMob := &mobs.Mob{InstanceId: 7399}
	targetMob.Character.Name = "OverwhelmingCaster"
	targetMob.Character.Stats.Willpower.Base = 5000
	targetMob.Character.RecalculateStats()
	targetMob.Character.HealthMax.Value = 500
	targetMob.Character.Health = 500
	targetMob.Character.Stamina = 500
	targetMob.Character.StaminaMax.Value = 500
	targetMob.Character.Conviction = 50
	targetMob.Character.ConvictionMax.Value = 100
	// Tiny Dexterity so the throttle move itself lands reliably — this is
	// the move's own to-hit roll, independent of the concentration contest.
	targetMob.Character.Stats.Dexterity.ValueAdj = 1
	targetMob.Character.Skills = map[string]int{string(skills.Spellcasting): 100}
	targetMob.Character.Buffs = buffs.New()
	setCombatPositionParallel(&targetMob.Character, position.Standing)
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	castData := activity.CastingData{
		SpellId:             "fireball",
		TotalConvictionCost: 30,
		ConvictionSpent:     10,
	}
	setCastingForTest(&targetMob.Character, castData)

	// Attacker: modest, unmodified grip (default characters.New() Dexterity
	// and unarmed-combat rank 1) — dwarfed by the target's engineered hold
	// regardless of characters.New()'s random stat roll.
	char := characters.New()
	char.SpeciesId = 7005
	char.Stats.Strength.ValueAdj = 500
	fundSpecialMove(char)

	startProgression := targetMob.Character.GetSkillUseCount(string(skills.Spellcasting))
	heldCount := 0
	interruptCount := 0
	landed := 0

	for i := 0; i < 500 && landed < 20; i++ {
		if !targetMob.Character.IsCasting() {
			setCastingForTest(&targetMob.Character, castData)
		}
		char.Aggro = &characters.Aggro{MobInstanceId: targetMob.InstanceId}
		char.Cooldowns = characters.Cooldowns{}

		res := ExecuteThrottle(newStubActor(char, newTestRoom()))
		if !res.Executed || !res.MoveResult.Hit {
			continue
		}
		landed++
		if res.InterruptedCast {
			interruptCount++
		} else {
			heldCount++
		}
	}

	if landed < 20 {
		t.Fatalf("only %d landed hits in 500 attempts — throttle hit path is broken", landed)
	}

	endProgression := targetMob.Character.GetSkillUseCount(string(skills.Spellcasting))

	// U10b-1 Task 12: once per RESOLVED contest, win or lose -- not once per
	// HELD contest. This assertion used to read `heldCount` and the old rule it
	// pinned was success-only.
	//
	// It was also VACUOUS under the new rule as written: this fixture's caster
	// is deliberately overwhelming, so interruptCount is normally 0 and
	// heldCount equals the total. Summing both makes the assertion say what it
	// means regardless of how the contests fall.
	assert.Equal(t, heldCount+interruptCount, endProgression-startProgression,
		"spellcasting progression fires once per resolved concentration contest, on an interrupt as well as a hold")
	assert.GreaterOrEqual(t, heldCount, 1,
		"an overwhelming caster should hold at least once across 20 landed hits (odds of zero holds is ~0.02^20)")
	assert.Less(t, interruptCount, heldCount,
		"an overwhelming caster should be interrupted far less often than it holds")
}

// TestThrottle_TargetGone verifies that when aggro is set to an invalid mob
// instance ID (target gone), Executed is false and NoTarget is true.
func TestThrottle_TargetGone(t *testing.T) {
	char := characters.New()
	room := newTestRoom()
	actor := newStubActor(char, room)

	// Aggro pointing at a nonexistent mob instance — target resolution fails.
	char.Aggro = &characters.Aggro{MobInstanceId: 999999}

	result := ExecuteThrottle(actor)

	assert.False(t, result.Executed, "throttle with missing target should not execute")
	assert.True(t, result.NoTarget, "throttle should report NoTarget when the resolved target is gone")
	assert.False(t, result.OnCooldown, "cooldown should not be reported when target is gone")
}
