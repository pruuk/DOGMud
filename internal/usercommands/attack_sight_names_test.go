package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #214: attack named its target to an attacker who could not see it. The
// room's "prepares to fight" line was already sight-gated; the attacker's own
// lines were raw SendText with the mob's display name, so in pitch dark the
// attacker read "You prepare to enter into mortal combat with Skeleton." The
// same raw name rode the companion refusal and the can't-attack refusal, and
// the PvP lines named each side to the other at any sight.

// darkenTestRoom1 turns the seeded room 1 (city, lamp 60) pitch dark for a
// normal-sighted reader: the seeded cave biome has no sky light, and with no
// lamp nothing else lights it.
func darkenTestRoom1(t *testing.T) {
	t.Helper()
	_, room := getTestUserAndRoom(t)
	room.Biome = "cave"
	room.Lamp = nil
	require.Equal(t, 0, room.LightLevel(), "fixture room must be pitch dark")
}

func TestAttack_PitchDark_DoesNotNameTheMobToTheAttacker(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	darkenTestRoom1(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "mortal combat", "the engagement line must still be sent: %q", out)
	require.NotContains(t, strings.ToLower(out), "skeleton",
		"an attacker who sees nothing must not learn the target's name: %q", out)
}

func TestAttack_PitchDark_CompanionRefusalDoesNotNameTheMob(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	darkenTestRoom1(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()

	// Charm the skeleton to user 2, so user 1's attack is refused as
	// someone else's companion.
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Charmed = nil
	other := users.GetByUserId(2)
	require.NotNil(t, other)
	m.Character.Charm(other.UserId, -1, "")
	require.Equal(t, mobs.HarmBlockedCompanion, mobs.CheckPlayerHarm(m), "fixture: the skeleton must be a companion")
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "companion", "the refusal must still be sent: %q", out)
	require.NotContains(t, strings.ToLower(out), "skeleton",
		"an attacker who sees nothing must not learn the companion's name: %q", out)
}

func TestAttack_Lit_StillNamesTheMob(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "Skeleton", "a sighted attacker reads the name: %q", out)
}

// The PvP target's "prepares to fight you!" is judged at the TARGET's sight,
// not the attacker's: a blinded target must not learn who is swinging at them
// while the attacker, in a lit room, sees everything.
func TestAttack_PvP_BlindedTargetDoesNotReadTheAttackersName(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	target := users.GetByUserId(2)
	require.NotNil(t, target)
	require.Equal(t, user.Character.RoomId, target.Character.RoomId, "fixture: same room")
	user.Character.EndAggro()

	orig := target.Character.Perception
	t.Cleanup(func() { target.Character.Perception = orig })
	target.Character.Perception = characters.New().Perception
	require.NoError(t, target.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))

	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedMessagesForTest(target.UserId)

	handled, err := Attack(target.Character.Name, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	attackerOut := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, attackerOut, target.Character.Name, "the sighted attacker reads the name: %q", attackerOut)

	out := strings.Join(events.DrainQueuedMessagesForTest(target.UserId), "\n")
	require.Contains(t, out, "prepares to fight you", "the line must still be sent: %q", out)
	require.NotContains(t, out, user.Character.Name,
		"a blinded target must not learn the attacker's name: %q", out)
}
