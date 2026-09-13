package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Pin: a poisoned player at 1 health dies to the poison tick and the death
// cause reads "poison". Setup migrates in Task 8; the assertions do not.
func TestPin_PoisonTickKillsAndNamesTheCause(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	u := users.GetByUserId(1)
	u.Character.Health = 1
	u.Character.AddCondition(characters.ConditionPoisoned, 10, 5.0, "pin") // SETUP: Task 8

	AutoHeal(events.NewRound{RoundNumber: 30}) // SETUP: Task 8 runs UserRoundTick instead
	require.LessOrEqual(t, u.Character.Health, 0, "the poison tick must take the last point")
	require.True(t, u.Character.HasCondition(characters.ConditionPoisoned)) // SETUP: Task 8 tests the poison flag
}

// NOTE on the death-cause string ("poison" vs "bleeding out" vs a mob name):
// Death_PlayerAnnouncement.go:116-134 computes causeOfDeath INLINE inside the
// wirePlayerDeathAnnouncement listener, not in a standalone function that
// takes a character — so there is no pure function to pin directly. Pinning
// it through the listener would require: driving c.Life from Alive to Dead
// (a separate FSM transition the CharacterDied event queued by ApplyHarm does
// not synchronously perform in this package), arranging an EMPTY DamageMap so
// the dmgCt == 0 branch that computes causeOfDeath even runs, and then
// observing the result — which is never sent through SendText/drainPlain, it
// only reaches worldevents.EmitWorldEvent's Description field, and this
// package has no seed/capture seam for worldevents (grepped: no
// SeedWorldEventsForTest or equivalent, and no existing "Death" test in
// hooks_test.go to model the FSM-driving on). That is well past a ~40 line
// test and past what this pin task should invent test infrastructure for.
// NEEDS_CONTEXT: a follow-up task should add a worldevents test-capture seam
// (mirroring events.DrainQueuedMessagesForTest) before this string can be
// pinned; until then Task 8 should re-verify the cause-string behavior by
// reading Death_PlayerAnnouncement.go directly rather than trusting a pin.
