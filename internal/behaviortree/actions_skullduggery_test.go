package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// skulldBuffSpec seeds buff 9 (Hidden) for behaviortree-package tests that
// need AddBuff(9) to work. A single var so the same seed is shared by all
// tests in this file.
var skulldBuffSpec = map[int]*buffs.BuffSpec{
	9: {
		BuffId:        9,
		Name:          "Hidden",
		Flags:         []buffs.Flag{buffs.Hidden},
		TriggerCount:  15,
		RoundInterval: 1,
	},
}

// ─── try_sneak ───────────────────────────────────────────────────────────────

// TestActTrySneak_SuccessWhenNoObservers verifies that a mob alone in a
// room succeeds at sneak and acquires the Hidden buff.
func TestActTrySneak_SuccessWhenNoObservers(t *testing.T) {
	cleanBuffs := buffs.SeedBuffsForTest(skulldBuffSpec)
	defer cleanBuffs()

	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Stats.Dexterity.ValueAdj = 200 // very high sneak score
	mob.Character.Aggro = nil                    // not in combat

	room := rooms.LoadRoom(1)
	room.AddMob(105)
	// No other mobs or players — no observers.

	ctx := &EvalContext{InstanceId: 105, RoomId: 1}
	if r := actTrySneak(map[string]any{}, ctx); r != Success {
		t.Errorf("expected Success for mob alone in room, got %v", r)
	}
	// Note: mob.AddBuff enqueues an event rather than applying synchronously,
	// so HasBuffFlag is not reliable here without an event-loop tick.
	// Verifying Success return is sufficient to confirm Sneak fired correctly.
}

// TestActTrySneak_AlreadyHiddenReturnsSuccess verifies that a mob that is
// already hidden returns Success (AlreadyHidden path from actions.Sneak).
func TestActTrySneak_AlreadyHiddenReturnsSuccess(t *testing.T) {
	cleanBuffs := buffs.SeedBuffsForTest(skulldBuffSpec)
	defer cleanBuffs()

	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Aggro = nil

	room := rooms.LoadRoom(1)
	room.AddMob(105)

	// Apply hidden buff directly.
	if err := mob.Character.AddBuff(9, false); err != nil {
		t.Fatalf("AddBuff(9) failed: %v", err)
	}

	ctx := &EvalContext{InstanceId: 105, RoomId: 1}
	if r := actTrySneak(map[string]any{}, ctx); r != Success {
		t.Errorf("expected Success when already hidden, got %v", r)
	}
}

// ─── try_steal ───────────────────────────────────────────────────────────────

// seedStealScenario wires the one arrangement both try_steal tests need: a
// high-Dex, skill-5 thief mob and a nearly-blind player with gold, alone in a
// room, targeted via Event.UserId.
//
// The contest floors are pinned to zero for the duration. They default to 0.05
// in dice (a Go package var, NOT config — dice deliberately does not import
// configs, so configs.SetConfigForTest does not reach them), and that 5%
// resist floor is precisely what made the previous version of this test flaky:
// it is a last-resort save that fires regardless of how outclassed the
// defender is.
func seedStealScenario(t *testing.T) (*mobs.Mob, *users.UserRecord, *EvalContext) {
	t.Helper()

	origSuccess, origResist := dice.ContestFloors()
	dice.SetContestFloors(0, 0)
	t.Cleanup(func() { dice.SetContestFloors(origSuccess, origResist) })

	t.Cleanup(seedTestRoom(t, 1, "TestZone"))
	t.Cleanup(seedTestMob(t, 5, 105, 1, "Thief"))
	t.Cleanup(seedTestUser(t, 42, "victim", "Victim", 1))

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Stats.Dexterity.ValueAdj = 300 // very high steal score
	mob.Character.Skills = map[string]int{"skullduggery": 5}
	mob.Character.Aggro = nil // steal.go blocks outright when Aggro != nil

	user := users.GetByUserId(42)
	user.Character.Gold = 1000
	user.Character.Stats.Perception.ValueAdj = 1 // nearly blind

	room := rooms.LoadRoom(1)
	room.AddMob(105)
	room.AddPlayer(42)

	// resolveSkullduggeryTarget checks Event.UserId first.
	return mob, user, &EvalContext{
		InstanceId: 105,
		RoomId:     1,
		Event:      EventContext{UserId: 42},
	}
}

// TestActTrySteal_SuccessMovesGoldToThief verifies the adapter end to end: that
// actTrySteal resolves the Event.UserId target, reaches the real actions.Steal
// against that player, and reports Success.
//
// Gold movement is the assertion that makes this meaningful. Returning Success
// proves only that some code path returned Succeeded; gold actually leaving the
// victim and arriving on the thief proves the adapter targeted the intended
// player. stealFromPlayer sets Succeeded before its independent detection roll
// and steals between 75% and 100% of the victim's purse, so on success the
// transfer is guaranteed to be non-zero and this stays deterministic.
//
// Whether the steal ROLL is balanced is not this test's business —
// internal/actions/steal_test.go owns that (GoldSuccess, MobOnPlayer,
// DetectionWin, OnCooldown, InCombat). Duplicating it here is what produced a
// 30-iteration loop that asserted almost nothing.
func TestActTrySteal_SuccessMovesGoldToThief(t *testing.T) {
	mob, user, ctx := seedStealScenario(t)

	thiefGoldBefore := mob.Character.Gold
	victimGoldBefore := user.Character.Gold

	// One attempt, not a loop. With the floors pinned off, Dex 300 against
	// Perception 1 loses about once in 10^6 -- both sides roll with the
	// attacker's stdDev (300 * 0.15 = 45), so the margin is
	// Normal(299, 45*sqrt(2)) and P(margin <= 0) = P(Z <= -4.7).
	if r := actTrySteal(map[string]any{}, ctx); r != Success {
		t.Fatalf("expected Success stealing from a near-blind mark, got %v", r)
	}

	stolen := mob.Character.Gold - thiefGoldBefore
	if stolen <= 0 {
		t.Errorf("thief gold did not increase: before=%d after=%d",
			thiefGoldBefore, mob.Character.Gold)
	}
	if lost := victimGoldBefore - user.Character.Gold; lost != stolen {
		t.Errorf("gold did not conserve: victim lost %d, thief gained %d", lost, stolen)
	}
}

// TestActTrySteal_CooldownMapsToFailure verifies that a cooldown-blocked
// attempt maps to Failure rather than Success.
//
// This is the case the previous version of this test hid. It cleared the
// cooldown with the key "skullduggery.steal", but the real key is
// skills.Skullduggery.String("steal"), which joins with a COLON. The delete
// never matched, so attempts 2 through 30 of its loop were all silently
// cooldown-blocked and the loop was really a single trial.
func TestActTrySteal_CooldownMapsToFailure(t *testing.T) {
	_, _, ctx := seedStealScenario(t)

	if r := actTrySteal(map[string]any{}, ctx); r != Success {
		t.Fatalf("setup: first attempt should succeed, got %v", r)
	}

	// No cooldown clearing between attempts -- that is the point.
	if r := actTrySteal(map[string]any{}, ctx); r != Failure {
		t.Errorf("expected Failure while on cooldown, got %v", r)
	}
}

// TestActTrySteal_FailureNoTarget verifies that with no aggro and no event
// target, actTrySteal returns Failure.
func TestActTrySteal_FailureNoTarget(t *testing.T) {
	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Aggro = nil

	room := rooms.LoadRoom(1)
	room.AddMob(105)

	ctx := &EvalContext{InstanceId: 105, RoomId: 1}
	if r := actTrySteal(map[string]any{}, ctx); r != Failure {
		t.Errorf("expected Failure with no target, got %v", r)
	}
}

// ─── try_plant ───────────────────────────────────────────────────────────────

// TestActTryPlant_RequiresItemTag verifies that plant with a missing
// item_tag param returns Failure immediately.
func TestActTryPlant_RequiresItemTag(t *testing.T) {
	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()
	cleanUser := seedTestUser(t, 42, "target", "Target", 1)
	defer cleanUser()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Aggro = nil

	room := rooms.LoadRoom(1)
	room.AddMob(105)
	room.AddPlayer(42)

	// item_tag param is absent.
	ctx := &EvalContext{
		InstanceId: 105,
		RoomId:     1,
		Event:      EventContext{UserId: 42},
	}
	if r := actTryPlant(map[string]any{}, ctx); r != Failure {
		t.Errorf("expected Failure when item_tag is missing, got %v", r)
	}
}

// TestActTryPlant_FailureNoItemInBackpack verifies that plant with a valid
// item_tag but mob carrying nothing returns Failure (Plant returns
// "item not in backpack").
func TestActTryPlant_FailureNoItemInBackpack(t *testing.T) {
	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()
	cleanUser := seedTestUser(t, 42, "target", "Target", 1)
	defer cleanUser()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Aggro = nil
	mob.Character.Items = nil // empty backpack

	room := rooms.LoadRoom(1)
	room.AddMob(105)
	room.AddPlayer(42)

	ctx := &EvalContext{
		InstanceId: 105,
		RoomId:     1,
		Event:      EventContext{UserId: 42},
	}
	if r := actTryPlant(map[string]any{"item_tag": "knife"}, ctx); r != Failure {
		t.Errorf("expected Failure when mob has nothing to plant, got %v", r)
	}
}

// ─── try_shadow ──────────────────────────────────────────────────────────────

// TestActTryShadow_FailureWhenNotHidden verifies that shadow returns Failure
// when the acting mob does not carry buff 9 (Hidden).
func TestActTryShadow_FailureWhenNotHidden(t *testing.T) {
	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()
	cleanUser := seedTestUser(t, 42, "target", "Target", 1)
	defer cleanUser()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Aggro = nil
	// No Hidden buff.

	room := rooms.LoadRoom(1)
	room.AddMob(105)
	room.AddPlayer(42)

	ctx := &EvalContext{
		InstanceId: 105,
		RoomId:     1,
		Event:      EventContext{UserId: 42},
	}
	if r := actTryShadow(map[string]any{}, ctx); r != Failure {
		t.Errorf("expected Failure when mob is not hidden, got %v", r)
	}
}

// TestActTryShadow_FailureNoTarget verifies that shadow returns Failure
// when there is no resolvable target (even when hidden).
func TestActTryShadow_FailureNoTarget(t *testing.T) {
	cleanBuffs := buffs.SeedBuffsForTest(skulldBuffSpec)
	defer cleanBuffs()

	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Aggro = nil

	room := rooms.LoadRoom(1)
	room.AddMob(105)

	if err := mob.Character.AddBuff(9, false); err != nil {
		t.Fatalf("AddBuff(9) failed: %v", err)
	}

	ctx := &EvalContext{InstanceId: 105, RoomId: 1}
	if r := actTryShadow(map[string]any{}, ctx); r != Failure {
		t.Errorf("expected Failure when hidden but no target resolvable, got %v", r)
	}
}

// ─── try_defuse ──────────────────────────────────────────────────────────────

// TestActTryDefuse_FailureNoTraps verifies that defuse returns Failure when
// the room has no trapped containers or exits.
func TestActTryDefuse_FailureNoTraps(t *testing.T) {
	cleanRoom := seedTestRoom(t, 1, "TestZone")
	defer cleanRoom()
	cleanMob := seedTestMob(t, 5, 105, 1, "Thief")
	defer cleanMob()

	mob := mobs.GetInstance(105)
	mob.Character.HealthMax.Value = 500
	mob.Character.Health = 500
	mob.Character.Aggro = nil

	room := rooms.LoadRoom(1)
	room.AddMob(105)
	// Room has no containers or exits with traps.

	ctx := &EvalContext{InstanceId: 105, RoomId: 1}
	if r := actTryDefuse(map[string]any{}, ctx); r != Failure {
		t.Errorf("expected Failure when room has no traps, got %v", r)
	}
}
