package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// #454: party auto-assist types `attack #<id>` for every member in the room.
// A member who sees nothing cannot pick the mob out, so assisting would only
// tell them "You don't see them here." every round. They are skipped; a member
// who can see still assists (the control).
func TestPartyAutoAssist_SkipsAMemberWhoSeesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	p := parties.New(1)
	t.Cleanup(p.Disband)
	p.InvitePlayer(2)
	p.AcceptInvite(2)
	mob := mobs.GetInstance(100)
	defender := users.GetByUserId(1)

	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	events.DrainQueuedUserInputsForTest(2)
	handlePartyAutoAttack(mob, defender)
	assert.Equal(t, []string{"attack #100"}, events.DrainQueuedUserInputsForTest(2), "a member who can see assists")

	rooms.LoadRoom(1).Lamp = nil
	darken(t, 1)
	handlePartyAutoAttack(mob, defender)
	assert.Empty(t, events.DrainQueuedUserInputsForTest(2), "a member who sees nothing was sent to attack")
}
