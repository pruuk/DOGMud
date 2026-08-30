package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Playtest 2026-08-19 (u6b flip sweep, BUG-4): named attacks failing with no
// output at all while `look` still listed the target. Two holes combined:
//
//  1. A stale id in r.players made rooms.findPlayerByName panic AFTER the mob
//     had already been resolved; events.invokeListenerSafely swallowed the
//     panic, so the whole attack command died silently. Fixed in
//     internal/rooms (see rooms/respawn_targeting_test.go); the first test
//     locks the fix end-to-end at the attack layer.
//  2. attack's own resolved-id-but-no-record branches (mob instance gone, or
//     `attack @id` resolving a player id with no user behind it) returned
//     without sending ANY text. A failed resolution must always message the
//     player.

// A stale player id in the room must not prevent attacking a mob by name.
// Pre-fix this panicked inside room.FindByName (swallowed in production).
func TestAttack_StalePlayerIdInRoom_StillEngagesNamedMob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	room.AddPlayer(999) // no user record behind this id
	defer room.RemovePlayer(999)
	user.Character.EndAggro()

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Attack panicked with a stale player id in the room (swallowed silently in production): %v", rec)
		}
	}()

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	require.True(t, user.Character.IsInCombat(), "attack must engage the named mob despite the stale player id")
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId)
}

// The same stale-player-id poison silenced EVERY command that resolves names
// through room.FindByName (23 call sites: taunt and the melee specials via
// StageMeleeTarget/ResolveTargetActor, cast's four target lookups, consider,
// look...). Lock the taunt path too: it must survive and say something.
func TestTaunt_StalePlayerIdInRoom_StillMessages(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	room.AddPlayer(999)
	defer room.RemovePlayer(999)
	user.Character.EndAggro()

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Taunt panicked with a stale player id in the room (swallowed silently in production): %v", rec)
		}
	}()

	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Taunt("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	msgs := events.DrainQueuedMessagesForTest(user.UserId)
	assert.NotEmpty(t, msgs, "taunt must never complete without messaging the player")
}

// `attack @<id>` (the party auto-attack form) resolving an id with no user
// record must message the player, not return silently.
func TestAttack_StaleTargetPlayerId_MessagesInsteadOfSilence(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	room.AddPlayer(999) // resolvable via @999, but users.GetByUserId(999) is nil
	defer room.RemovePlayer(999)
	user.Character.EndAggro()

	events.DrainQueuedMessagesForTest(user.UserId) // discard fixture noise

	handled, err := Attack("@999", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	msgs := events.DrainQueuedMessagesForTest(user.UserId)
	require.NotEmpty(t, msgs, "a failed resolution must always message the player")
	assert.True(t, strings.Contains(strings.Join(msgs, "\n"), "don't see them"),
		"expected the not-found family message, got: %q", msgs)
	assert.False(t, user.Character.IsInCombat(), "no engagement may happen on a vanished target")
}

// A defended taunt must still tell the taunter something, even if the defence
// message registry has no entry for the defence type that stopped it.
//
// This pins the invariant structurally rather than leaving it to data luck.
// Before the fix, the hit line was skipped because the target defended and
// sendChannelDefenceMessages returned silently on an empty triad, so the player
// got nothing at all. Shipped data covers every defence type, so the only way
// to reach that state deliberately is to empty the registry -- which is exactly
// the state a future defence type shipped without a message file would create.
func TestTaunt_DefendedWithNoDefenceMessage_StillMessagesAttacker(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	// Empty the registry AFTER seedAllRegistries has populated it.
	restoreEmpty := items.SeedDefenseMessagesForTest(nil)
	defer restoreEmpty()

	user, room := getTestUserAndRoom(t)
	target := users.GetByUserId(2)

	originalAction := executeTauntAction
	executeTauntAction = func(actions.Actor) actions.TauntResult {
		return actions.TauntResult{
			Executed: true, Hit: true,
			Target:  actions.AggroTarget{Char: target.Character, Name: target.Character.Name, UserId: target.UserId, Found: true},
			Defence: combat.ChannelDefenceResult{DefenceType: characters.DefenseDefy, Defended: true, DamageMultiplier: 0},
		}
	}
	t.Cleanup(func() { executeTauntAction = originalAction })

	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Taunt(target.Character.Name, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	msgs := events.DrainQueuedMessagesForTest(user.UserId)
	assert.NotEmpty(t, msgs, "a defended taunt must never leave the taunter with no message at all")
}
