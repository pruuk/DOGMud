package usercommands

// skill_skullduggery_cooldown_message_test.go pins the player-facing text of
// the steal and plant cooldown refusal. It used to read "You need to wait 15
// rounds remaining before you can do that again." -- awkward, and it handed
// the player a raw round count. dogmud-player-copy forbids showing raw
// numbers for durations, so this test fails on the old line and only passes
// once the refusal is rewritten in plain words with no number.
//
// Steal and Plant share one cooldown (skullduggeryCooldownKey in
// internal/actions/steal.go), and both usercommand wrappers build the same
// refusal text, so both are pinned here.

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skullduggeryCooldownRefusal is the exact player-facing line a thief sees
// when the shared steal/plant cooldown refuses another attempt.
const skullduggeryCooldownRefusal = "You need a moment before you can try that again."

// armSkullduggeryCooldown gives the actor rank enough to try, then pre-arms
// the shared steal/plant cooldown so the very next attempt is refused.
func armSkullduggeryCooldown(user *users.UserRecord) {
	if user.Character.Skills == nil {
		user.Character.Skills = map[string]int{}
	}
	user.Character.Skills[string(skills.Skullduggery)] = 5
	if user.Character.Cooldowns == nil {
		user.Character.Cooldowns = characters.Cooldowns{}
	}
	user.Character.Cooldowns[skills.Skullduggery.String("steal")] = 15
}

func TestSteal_OnCooldown_TellsThePlayerPlainly(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	armSkullduggeryCooldown(user)
	room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}

	events.DrainQueuedMessagesForTest(user.UserId) // discard fixture noise

	handled, err := Steal("chest", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.Contains(t, out, skullduggeryCooldownRefusal)
	assert.NotContains(t, out, "rounds remaining",
		"no raw round count belongs in player-facing text")
}

func TestPlant_OnCooldown_TellsThePlayerPlainly(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	armSkullduggeryCooldown(user)
	room.Containers = map[string]rooms.Container{"chest": {Gold: 100}}
	user.Character.StoreItem(items.Item{ItemId: 1})

	events.DrainQueuedMessagesForTest(user.UserId) // discard fixture noise

	handled, err := Plant("!1 chest", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.Contains(t, out, skullduggeryCooldownRefusal)
	assert.NotContains(t, out, "rounds remaining",
		"no raw round count belongs in player-facing text")
}
