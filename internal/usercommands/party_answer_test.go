package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A party member who was never invited (or already answered) typing
// `party accept` or `party decline` is told they have no invitation. It is
// not a server fault, so nothing may go out as "That didn't work".
func TestPartyAnswer_MemberWithNoInvitationIsRefused(t *testing.T) {
	for _, cmd := range []string{`accept`, `join`, `decline`} {
		t.Run(cmd, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			user, room := getTestUserAndRoom(t)
			if p := parties.Get(user.UserId); p != nil {
				p.Disband()
			}
			p := parties.New(user.UserId)
			t.Cleanup(func() { p.Disband() })
			require.True(t, p.IsMember(user.UserId))
			events.DrainQueuedMessagesForTest(user.UserId)

			handled, err := Party(cmd, user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)

			out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
			assert.Contains(t, out, refusal.NoPartyInviteLine)
			assert.NotContains(t, out, refusal.FailedText(""), "not a server fault")
			assert.True(t, p.IsMember(user.UserId), "still a member")
		})
	}
}
