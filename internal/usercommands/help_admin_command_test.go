package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #296: admin help lives in admincommands/help/command.<name>, but `help
// <name>` only looked in help/<name>, so `help build` found nothing.
func TestHelp_AdminCommandShowsAdminHelp(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	oldRole := user.Role
	t.Cleanup(func() { user.Role = oldRole })

	want, err := templates.Process("admincommands/help/command.build", nil, user.UserId)
	require.NoError(t, err)
	require.NotEmpty(t, strings.TrimSpace(want))

	user.Role = users.RoleAdmin
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err = Help("build", user, room, 0)
	require.NoError(t, err)
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.NotContains(t, out, "No help found")
	assert.Contains(t, out, strings.TrimSpace(strings.SplitN(want, "\n", 2)[0]))

	user.Role = users.RoleUser
	events.DrainQueuedMessagesForTest(user.UserId)
	Help("build", user, room, 0)
	assert.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"),
		`No help found for "build"`, "a player is not shown admin help")
}
