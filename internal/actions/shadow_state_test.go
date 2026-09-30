package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// seedShadowingCondition makes condition 87 appliable for one test.
// SeedConditionsForTest replaces the whole registry, and its returned func
// restores the package's own seed (condition 9) afterwards.
func seedShadowingCondition(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		ShadowingConditionId: {ConditionId: ShadowingConditionId, Name: "Shadowing", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25},
	}))
}

// shadowingChar is a character shadowing player 7320 with condition 87 held.
func shadowingChar(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	c.SetMiscData(shadowTargetUserKey, 7320)
	if err := c.AddCondition(ShadowingConditionId, false); err != nil {
		t.Fatalf("add condition 87: %v", err)
	}
	return c
}

// shadowConditionEnded reports whether condition 87 is gone once the turn's
// prune runs. RemoveCondition only expires a condition: HasCondition stays
// true until Conditions.Prune drops it (hooks/NewTurn_PruneConditions.go).
func shadowConditionEnded(c *characters.Character) bool {
	c.Conditions.Prune()
	return !c.HasCondition(ShadowingConditionId)
}

// ShadowTargetOf reads whichever key is set, and zeros for none.
func TestShadowTargetOf_ReadsEitherKey(t *testing.T) {
	c := characters.New()
	if u, m := ShadowTargetOf(c); u != 0 || m != 0 {
		t.Errorf("no shadow: got (%d, %d), want (0, 0)", u, m)
	}
	c.SetMiscData(shadowTargetUserKey, 7330)
	if u, m := ShadowTargetOf(c); u != 7330 || m != 0 {
		t.Errorf("player quarry: got (%d, %d), want (7330, 0)", u, m)
	}
	c.SetMiscData(shadowTargetUserKey, nil)
	c.SetMiscData(shadowTargetMobKey, 88330)
	if u, m := ShadowTargetOf(c); u != 0 || m != 88330 {
		t.Errorf("mob quarry: got (%d, %d), want (0, 88330)", u, m)
	}
	if u, m := ShadowTargetOf(nil); u != 0 || m != 0 {
		t.Errorf("nil character: got (%d, %d), want (0, 0)", u, m)
	}
}

// Shadow stores what ShadowTargetOf reads back.
func TestShadowTargetOf_ReadsWhatShadowStored(t *testing.T) {
	target := users.NewTestUser(7331, "stored", "Stored", 0)
	cleanup := users.SeedUsersForTest(map[int]*users.UserRecord{7331: target})
	defer cleanup()
	actor := newShadowPlayerActor(100, 5, true)

	if result := Shadow(actor, ShadowOptions{TargetUserId: 7331}); !result.Succeeded {
		t.Fatalf("shadow did not start: %+v", result)
	}
	if u, m := ShadowTargetOf(actor.char); u != 7331 || m != 0 {
		t.Errorf("ShadowTargetOf = (%d, %d), want (7331, 0)", u, m)
	}
}

// ClearShadow is the silent drop: the target and condition 87 go, and no
// cooldown starts (the stale guard and the death and logoff cleanups).
func TestClearShadow_DropsTargetAndConditionWithoutCooldown(t *testing.T) {
	seedShadowingCondition(t)
	c := shadowingChar(t)
	c.SetMiscData(shadowTargetMobKey, 88320)

	ClearShadow(c)

	if userId, mobInstanceId := ShadowTargetOf(c); userId != 0 || mobInstanceId != 0 {
		t.Errorf("ShadowTargetOf after ClearShadow = (%d, %d), want (0, 0)", userId, mobInstanceId)
	}
	if !shadowConditionEnded(c) {
		t.Error("condition 87 survived ClearShadow")
	}
	if got := c.GetCooldown(skills.Skullduggery.String("shadow")); got != 0 {
		t.Errorf("ClearShadow started a %d round cooldown; it must start none", got)
	}
}

// EndShadow is ClearShadow plus the cooldown and the reason line.
func TestEndShadow_ClearsStartsTheCooldownAndTellsTheActor(t *testing.T) {
	seedShadowingCondition(t)
	const userId = 7321
	u := users.NewTestUser(userId, "ender", "Ender", 0)
	u.Character = shadowingChar(t)
	events.DrainQueuedMessagesForTest(userId)

	EndShadow(NewUserActor(u), "You stop shadowing your target.")

	if tu, tm := ShadowTargetOf(u.Character); tu != 0 || tm != 0 {
		t.Errorf("ShadowTargetOf after EndShadow = (%d, %d), want (0, 0)", tu, tm)
	}
	if !shadowConditionEnded(u.Character) {
		t.Error("condition 87 survived EndShadow")
	}
	// The test binary reads the Go default ShadowCooldown (0), which
	// Cooldowns.Try rounds up to one round; production ships 5. Either way
	// a cooldown is running.
	if got := u.Character.GetCooldown(skills.Skullduggery.String("shadow")); got <= 0 {
		t.Errorf("EndShadow left cooldown %d, want a running cooldown", got)
	}
	msgs := events.DrainQueuedMessagesForTest(userId)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "You stop shadowing your target.") {
		t.Errorf("actor messages %q, want the one reason line", msgs)
	}
}

// An empty reason sends nothing.
func TestEndShadow_EmptyReasonIsSilent(t *testing.T) {
	seedShadowingCondition(t)
	const userId = 7322
	u := users.NewTestUser(userId, "quiet", "Quiet", 0)
	u.Character = shadowingChar(t)
	events.DrainQueuedMessagesForTest(userId)

	EndShadow(NewUserActor(u), "")

	if msgs := events.DrainQueuedMessagesForTest(userId); len(msgs) != 0 {
		t.Errorf("an empty reason sent %q", msgs)
	}
}
