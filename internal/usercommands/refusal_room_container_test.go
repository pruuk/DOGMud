package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
)

// M7 slice 3 (#371): a refusal about a room container never names it, since
// the player found it by looking (spec section 2).
func TestRoomContainerRefusalsNeverNameIt(t *testing.T) {
	for _, band := range []aimBand{aimFull, aimDark} {
		user, room := aimScene(t, band)
		room.IsStorage = false
		room.Containers = map[string]rooms.Container{
			"chest": {Lock: gamelock.Lock{Difficulty: 5}, Recipes: map[int][]int{1: {2}}},
		}
		events.DrainQueuedMessagesForTest(1)
		_, _ = Use("chest", user, room, events.EventFlag(0))
		_, _ = Storage("", user, room, events.EventFlag(0))
		_, _ = Put("rock in chest", user, room, events.EventFlag(0))
		told := strings.ToLower(aimTold(1))
		assert.Equal(t, 2, strings.Count(told, "that is locked."), "band %d: premise, use and put both refused the lock", band)
		assert.NotContains(t, told, "chest", "band %d", band)

		// Unlocked, but the recipe's ingredients are not inside: use's
		// "missing something" branch.
		room.Containers = map[string]rooms.Container{
			"chest": {Lock: gamelock.Lock{}, Recipes: map[int][]int{1: {2}}},
		}
		events.DrainQueuedMessagesForTest(1)
		_, _ = Use("chest", user, room, events.EventFlag(0))
		told = strings.ToLower(aimTold(1))
		assert.Contains(t, told, "missing something", "band %d: premise, the recipe branch ran", band)
		assert.NotContains(t, told, "chest", "band %d", band)
		room.Containers = nil
	}
}

// The storage hint writes raw angle brackets inside the command tag, as every
// other refusal hint does; an escaped &lt; would reach the player literally.
func TestStorageHintWritesRawBrackets(t *testing.T) {
	user, room := aimScene(t, aimFull)
	room.IsStorage = false
	room.Containers = map[string]rooms.Container{"chest": {}}
	events.DrainQueuedMessagesForTest(1)
	_, _ = Storage("", user, room, events.EventFlag(0))
	raw := strings.Join(events.DrainQueuedMessagesForTest(1), "")
	assert.Contains(t, raw, "put <item> <container>")
	assert.NotContains(t, raw, "&lt;")
	room.Containers = nil
}

// put echoes the item words the player typed: the item is their own.
func TestPutNotCarryingEchoesTheItem(t *testing.T) {
	user, room := aimScene(t, aimFull)
	room.IsStorage = false
	room.Containers = map[string]rooms.Container{"chest": {}}
	events.DrainQueuedMessagesForTest(1)
	_, _ = Put("zzyzx in chest", user, room, events.EventFlag(0))
	raw := strings.Join(events.DrainQueuedMessagesForTest(1), "")
	assert.Contains(t, raw, refusal.NotCarryingText("zzyzx"))
	room.Containers = nil
}
