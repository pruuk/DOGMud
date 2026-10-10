package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #459: an invite that is refused (nobody by that name here) must not leave
// the inviter leading an empty party, which then blocks `follow`.
func TestPartyInvite_RefusedInviteCreatesNoParty(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	if p := parties.Get(user.UserId); p != nil {
		p.Disband()
	}
	t.Cleanup(func() {
		if p := parties.Get(user.UserId); p != nil {
			p.Disband()
		}
	})
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Party("invite Zzyzxnobody", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Nil(t, parties.Get(user.UserId), "a refused invite must not create a party")
}
