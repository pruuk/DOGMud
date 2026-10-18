package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

var partyInviteTag = regexp.MustCompile(`</?ansi[^>]*>`)

// The invitation rides CategoryGroup, which the pipeline does not wrap, and
// the 2026-10-17 newbie-duo playtest read "Quillabee invited you to their
// party. Type party accept or party decline to respond." at 84 columns. The
// prompt now goes on its own line.
func TestPartyInvite_InvitationFitsIn80Columns(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	invited := users.GetByUserId(2)
	require.NotNil(t, invited)
	for _, id := range []int{user.UserId, invited.UserId} {
		if p := parties.Get(id); p != nil {
			p.Disband()
		}
	}
	t.Cleanup(func() {
		if p := parties.Get(user.UserId); p != nil {
			p.Disband()
		}
	})
	origName := user.Character.Name
	user.Character.Name = "Quillabee"
	t.Cleanup(func() { user.Character.Name = origName })
	events.DrainQueuedMessagesForTest(invited.UserId)

	handled, err := Party("invite "+invited.Character.Name, user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	seen := false
	for _, msg := range events.DrainQueuedMessagesForTest(invited.UserId) {
		plain := partyInviteTag.ReplaceAllString(msg, "")
		if !strings.Contains(plain, "invited you") {
			continue
		}
		seen = true
		for _, line := range strings.Split(strings.ReplaceAll(plain, "\r\n", "\n"), "\n") {
			require.LessOrEqual(t, util.VisibleWidth(line), 80, "invitation line wider than 80: %q", line)
		}
		require.Contains(t, plain, "party accept")
	}
	require.True(t, seen, "the invited player must read the invitation for the probe to mean anything")
}
