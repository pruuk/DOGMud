package usercommands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The earnmutation admin command must be registered in the command table: a
// handler that passes a direct unit test but is missing from the table
// resolves to "unknown command" in game. An admin reaches it; a non-admin is
// refused like any other admin-only command.
func TestAdminEarnMutation_RegistrationAndAdminGate(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user := users.GetByUserId(1)
	require.NotNil(t, user, "test user 1 must exist")
	defer func() { user.Role = users.RoleUser }()

	t.Run("admin reaches the handler", func(t *testing.T) {
		user.Role = users.RoleAdmin
		handled, err := TryCommand("earnmutation", "", 1, events.CmdSkipScripts)
		assert.True(t, handled)
		assert.NoError(t, err)
	})

	t.Run("non-admin is refused", func(t *testing.T) {
		user.Role = users.RoleUser
		handled, err := TryCommand("earnmutation", "", 1, events.CmdSkipScripts)
		assert.False(t, handled, "an admin-only command must not dispatch for a non-admin")
		assert.NoError(t, err)
	})
}

// `mutate` is a player help alias for the mutations page, and player topics
// win over admin pages. The admin lever is therefore named earnmutation so
// its page is reachable: `help earnmutation` shows the admin page for an
// admin, and `help mutate` still shows the player mutations topic.
func TestHelp_EarnMutationAdminPageAndMutateAlias(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	oldRole := user.Role
	t.Cleanup(func() { user.Role = oldRole })
	user.Role = users.RoleAdmin

	// Load the shipped aliases: the seeded registries leave them empty, and
	// the `mutate` alias is the thing under test.
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "keywords.yaml"))
	require.NoError(t, err)
	var shipped keywords.Aliases
	require.NoError(t, yaml.Unmarshal(raw, &shipped))
	t.Cleanup(keywords.SeedKeywordsForTest(shipped))

	adminPage, err := templates.Process("admincommands/help/command.earnmutation", nil, user.UserId)
	require.NoError(t, err)
	adminMarker := strings.TrimSpace(strings.SplitN(adminPage, "\n", 2)[0])
	require.NotEmpty(t, adminMarker)

	playerPage, err := GetHelpContents("mutations")
	require.NoError(t, err)
	playerMarker := strings.TrimSpace(strings.SplitN(playerPage, "\n", 2)[0])
	require.NotEmpty(t, playerMarker)
	require.NotEqual(t, adminMarker, playerMarker)

	events.DrainQueuedMessagesForTest(user.UserId)
	_, err = Help("earnmutation", user, room, 0)
	require.NoError(t, err)
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.Contains(t, out, adminMarker, "help earnmutation shows the admin page")
	assert.NotContains(t, out, playerMarker)

	events.DrainQueuedMessagesForTest(user.UserId)
	_, err = Help("mutate", user, room, 0)
	require.NoError(t, err)
	out = strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	assert.Contains(t, out, playerMarker, "help mutate still resolves to the player mutations topic")
	assert.NotContains(t, out, adminMarker)
}
