package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// ---------------------------------------------------------------------------
// Sleep test helpers
// ---------------------------------------------------------------------------

// sleepFakeActor is a minimal Actor for sleep tests. It exposes the full
// Actor interface with a real *characters.Character so that
// Character.AddCondition / Character.HasConditionFlag work correctly without a real
// server event queue. SendText messages are captured for assertion.
type sleepFakeActor struct {
	awardRecorder // records Actor.AwardResolved calls
	char          *characters.Character
	room          *rooms.Room
	isPlayer      bool
	userId        int
	sent          []string
}

func (a *sleepFakeActor) GetCharacter() *characters.Character { return a.char }
func (a *sleepFakeActor) GetRoom() *rooms.Room                { return a.room }
func (a *sleepFakeActor) GetName() string                     { return a.char.Name }
func (a *sleepFakeActor) IsPlayer() bool                      { return a.isPlayer }
func (a *sleepFakeActor) GetUserId() int                      { return a.userId }
func (a *sleepFakeActor) GetMobInstanceId() int               { return 0 }
func (a *sleepFakeActor) AddCondition(_ int, _ string)        {} // no-op: Sleep calls c.AddCondition directly
func (a *sleepFakeActor) OnSkillUse(_ string) bool            { return false }
func (a *sleepFakeActor) OnStatUse(_ string) bool             { return false }
func (a *sleepFakeActor) SendText(_ messaging.Category, msg string) {
	a.sent = append(a.sent, msg)
}

func (a *sleepFakeActor) SendBlankLine(_ messaging.Category) {
	a.sent = append(a.sent, "")
}

// newSleepActor builds a minimal actor for sleep tests.
// inCombat=true sets Aggro on the character to simulate combat.
func newSleepActor(t *testing.T, inCombat bool, isPlayer bool) *sleepFakeActor {
	t.Helper()
	c := characters.New()
	c.Name = "Sleeper"
	c.Conditions = conditions.New()
	if inCombat {
		// SetAggro requires a valid target; use the compat helper.
		c.SetAggro(0, 9001, characters.DefaultAttack)
	}
	room := &rooms.Room{RoomId: 9900}
	return &sleepFakeActor{
		char:     c,
		room:     room,
		isPlayer: isPlayer,
		userId:   1,
	}
}

// seedSleepCondition seeds condition spec 15 (Sleeping) so that Character.AddCondition(15,
// false) succeeds. Also re-seeds condition 9 (Hidden) to avoid breaking shadow
// tests that share the same test binary. Returns a cleanup func.
func seedSleepCondition(t *testing.T) func() {
	t.Helper()
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {
			ConditionId:  9,
			Name:         "Hidden",
			Flags:        []conditions.Flag{conditions.Hidden},
			TriggerCount: 1000000,
		},
		15: {
			ConditionId:  15,
			Name:         "Sleeping",
			Flags:        []conditions.Flag{conditions.Sleeping, conditions.CancelOnDamage},
			TriggerCount: 1000000, // effectively infinite — governed by cancel flags
		},
	})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSleep_AppliesConditionOnSuccess verifies that Sleep returns Success and
// sets the Sleeping condition flag on the character.
func TestSleep_AppliesConditionOnSuccess(t *testing.T) {
	cleanup := seedSleepCondition(t)
	defer cleanup()

	actor := newSleepActor(t, false /* notInCombat */, true)

	res := Sleep(actor, SleepOptions{})

	if !res.Success {
		t.Fatalf("expected Success, got %+v", res)
	}
	if !actor.char.HasConditionFlag(conditions.Sleeping) {
		t.Errorf("expected Sleeping flag applied after Sleep()")
	}
}

// TestSleep_ConditionUnavailable_NoRawErrorLeak verifies that when the Sleeping
// condition spec can't be applied (e.g. the condition is missing from the world data),
// the player gets a clean message rather than the raw internal AddCondition error
// that leaks the condition id ("...conditionId: 15"). Regression guard for the dogmud
// world shipping without condition 15.
func TestSleep_ConditionUnavailable_NoRawErrorLeak(t *testing.T) {
	// Deliberately seed an EMPTY condition registry so AddCondition(15) fails.
	cleanup := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{})
	defer cleanup()

	actor := newSleepActor(t, false /* notInCombat */, true)

	res := Sleep(actor, SleepOptions{})

	if res.Success {
		t.Fatalf("expected failure when the Sleeping condition is unavailable, got %+v", res)
	}
	if len(actor.sent) == 0 {
		t.Fatal("expected a player-facing message on sleep failure")
	}
	joined := strings.Join(actor.sent, " ")
	if strings.Contains(joined, "conditionId") || strings.Contains(joined, "failed to add") {
		t.Errorf("player message leaked the raw internal error: %q", joined)
	}
}

// TestSleep_BlocksInCombat verifies that Sleep returns Failure and sends
// a user-facing message when the actor is in combat.
func TestSleep_BlocksInCombat(t *testing.T) {
	cleanup := seedSleepCondition(t)
	defer cleanup()

	actor := newSleepActor(t, true /* inCombat */, true)

	res := Sleep(actor, SleepOptions{})

	if res.Success {
		t.Errorf("expected failure when in combat, got %+v", res)
	}
	if actor.char.HasConditionFlag(conditions.Sleeping) {
		t.Errorf("expected Sleeping NOT applied during combat")
	}
	if len(actor.sent) == 0 {
		t.Errorf("expected player-facing message on combat block, got none")
	}
}

// TestSleep_MobActorSilentInCombat verifies that a mob actor in combat
// receives no message (mob.SendText is a no-op, actor.sent stays empty).
func TestSleep_MobActorSilentInCombat(t *testing.T) {
	cleanup := seedSleepCondition(t)
	defer cleanup()

	actor := newSleepActor(t, true /* inCombat */, false /* mob, not player */)

	res := Sleep(actor, SleepOptions{})

	if res.Success {
		t.Errorf("expected failure for mob in combat, got %+v", res)
	}
	if len(actor.sent) != 0 {
		t.Errorf("mob actor should be silent; got messages: %v", actor.sent)
	}
}

// TestSleep_IdempotentWhenAlreadySleeping verifies that calling Sleep on a
// character that is already sleeping returns Success without error.
func TestSleep_IdempotentWhenAlreadySleeping(t *testing.T) {
	cleanup := seedSleepCondition(t)
	defer cleanup()

	actor := newSleepActor(t, false, true)

	// First call applies the condition.
	first := Sleep(actor, SleepOptions{})
	if !first.Success {
		t.Fatalf("first Sleep() expected Success, got %+v", first)
	}

	// Second call should be a no-op success.
	second := Sleep(actor, SleepOptions{})
	if !second.Success {
		t.Errorf("idempotent re-sleep expected Success, got %+v", second)
	}
}

// TestSleep_PlayerReadsTheStartLine pins that the sleeper is told they went to
// sleep. Condition 15 is flagged silent-start because Sleep applies it
// synchronously and so it never travels the event that narrates a condition start;
// the flag makes the line the applier's to send, and before this it was sent
// by nobody at all. A mob holder has no client, so it gets nothing.
func TestSleep_PlayerReadsTheStartLine(t *testing.T) {
	const startLine = "You lie down and let sleep take you."

	cleanup := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {
			ConditionId:  9,
			Name:         "Hidden",
			Flags:        []conditions.Flag{conditions.Hidden},
			TriggerCount: 1000000,
		},
		15: {
			ConditionId:   15,
			Name:          "Sleeping",
			Flags:         []conditions.Flag{conditions.SilentStart, conditions.Sleeping, conditions.CancelOnDamage},
			TriggerCount:  1000000,
			StartUserText: startLine,
		},
	})
	defer cleanup()

	t.Run("player reads it exactly once", func(t *testing.T) {
		actor := newSleepActor(t, false /* notInCombat */, true /* isPlayer */)

		if res := Sleep(actor, SleepOptions{}); !res.Success {
			t.Fatalf("expected Success, got %+v", res)
		}

		count := 0
		for _, msg := range actor.sent {
			if strings.Contains(msg, startLine) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("player read the sleep start line %d times, want exactly 1; sent: %#v", count, actor.sent)
		}
	})

	t.Run("a mob reads nothing", func(t *testing.T) {
		actor := newSleepActor(t, false /* notInCombat */, false /* isPlayer */)

		if res := Sleep(actor, SleepOptions{}); !res.Success {
			t.Fatalf("expected Success, got %+v", res)
		}

		for _, msg := range actor.sent {
			if strings.Contains(msg, startLine) {
				t.Errorf("a mob holder has no client, so it must not be sent the start line; sent: %#v", actor.sent)
			}
		}
	})
}
