package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
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
		told := strings.ToLower(aimTold(1))
		assert.NotContains(t, told, "chest", "band %d", band)
		room.Containers = nil
	}
}
