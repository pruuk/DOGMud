package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mutate admin command must be registered in the command table: a
// handler that passes a direct unit test but is missing from the table
// resolves to "unknown command" in game. An admin reaches it; a non-admin is
// refused like any other admin-only command.
func TestAdminMutate_RegistrationAndAdminGate(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user := users.GetByUserId(1)
	require.NotNil(t, user, "test user 1 must exist")
	defer func() { user.Role = users.RoleUser }()

	t.Run("admin reaches the handler", func(t *testing.T) {
		user.Role = users.RoleAdmin
		handled, err := TryCommand("mutate", "", 1, events.CmdSkipScripts)
		assert.True(t, handled)
		assert.NoError(t, err)
	})

	t.Run("non-admin is refused", func(t *testing.T) {
		user.Role = users.RoleUser
		handled, err := TryCommand("mutate", "", 1, events.CmdSkipScripts)
		assert.False(t, handled, "an admin-only command must not dispatch for a non-admin")
		assert.NoError(t, err)
	})
}
