package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
)

// withTestRoles installs a Roles config for the calling test only.
func withTestRoles(t *testing.T, roles configs.Roles) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Roles = roles
	configs.SetConfigForTest(t, cfg)
}

// roleUser is a user holding the named role.
func roleUser(role string) *UserRecord {
	return &UserRecord{Role: role, Character: &characters.Character{}}
}

// A granted key permits itself and every key below it on a dot boundary,
// and never a parent, a sibling, or a key that merely shares its spelling.
func TestHasRolePermission_GrantedKeyCoversOnlyItselfAndBelow(t *testing.T) {
	withTestRoles(t, configs.Roles{
		`builder`: {`room.info`, `build`},
		`roomer`:  {`room`},
	})
	builder := roleUser(`builder`)
	roomer := roleUser(`roomer`)

	assert.True(t, builder.HasRolePermission(`room.info`), "the granted key itself")
	assert.True(t, builder.HasRolePermission(`room.info.detail`), "a key below the granted one")
	assert.True(t, builder.HasRolePermission(`build`, true), "a whole-command key, quiet")

	assert.False(t, builder.HasRolePermission(`room`), "the parent of a granted key")
	assert.False(t, builder.HasRolePermission(`room`, true), "the parent of a granted key, quiet")
	assert.False(t, builder.HasRolePermission(`room.edit`), "a sibling of a granted key")
	assert.False(t, builder.HasRolePermission(`room.infox`), "a key that only shares the spelling")
	assert.False(t, builder.HasRolePermission(`buildx`, true), "a command that only shares the spelling")
	assert.False(t, builder.HasRolePermission(`roo`, true), "a prefix of a granted key")

	assert.True(t, roomer.HasRolePermission(`room.info`), "a parent grant covers its sub-keys")
	assert.True(t, roomer.HasRolePermission(`room.edit.exits`), "and their sub-keys")
	assert.False(t, roomer.HasRolePermission(`roomx`), "but not a longer command name")
}

// HasRolePermissionBelow answers for a key strictly below the asked one, so
// a command that checks its own sub-keys can admit a partial role.
func TestHasRolePermissionBelow(t *testing.T) {
	withTestRoles(t, configs.Roles{
		`builder`: {`room.info`, `build`},
	})
	builder := roleUser(`builder`)

	assert.True(t, builder.HasRolePermissionBelow(`room`), "room.info is below room")
	assert.False(t, builder.HasRolePermissionBelow(`room.info`), "a key is not below itself")
	assert.False(t, builder.HasRolePermissionBelow(`build`), "build has nothing below it")
	assert.False(t, builder.HasRolePermissionBelow(`roo`), "no dot boundary, no match")

	assert.True(t, roleUser(RoleAdmin).HasRolePermissionBelow(`anything`), "admin holds every key")
	assert.False(t, roleUser(RoleUser).HasRolePermissionBelow(`room`), "a plain user holds none")
	assert.False(t, roleUser(`undefined-role`).HasRolePermission(`room`), "a role the config lacks holds none")
}
