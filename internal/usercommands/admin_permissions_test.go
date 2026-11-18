package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// withAdminTestRoles installs a Roles config for the calling test only.
func withAdminTestRoles(t *testing.T, roles configs.Roles) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Roles = roles
	configs.SetConfigForTest(t, cfg)
}

// A role reaches an admin command through the command's own key or a
// parent, or through a key below it only when the command's handler checks
// its sub-keys itself.
func TestMayRunAdminCommand_SubKeyReachesOnlyGatedCommands(t *testing.T) {
	withAdminTestRoles(t, configs.Roles{
		`builder`: {`room.info`, `build`},
		`helper`:  {`teleport.playername`},
		`partial`: {`server.motd`, `zone.list`},
	})
	builder := users.NewTestUser(901, `b`, `Builda`, 0)
	builder.Role = `builder`
	helper := users.NewTestUser(902, `h`, `Helpa`, 0)
	helper.Role = `helper`
	partial := users.NewTestUser(903, `p`, `Parta`, 0)
	partial.Role = `partial`

	require.True(t, mayRunAdminCommand(builder, `room`), "room checks room.info itself")
	require.True(t, mayRunAdminCommand(builder, `build`), "its own key")
	require.False(t, mayRunAdminCommand(builder, `buil`), "a prefix of a granted key")
	require.False(t, mayRunAdminCommand(builder, `teleport`), "an unrelated command")
	require.True(t, mayRunAdminCommand(helper, `teleport`), "teleport checks its sub-keys itself")
	require.False(t, mayRunAdminCommand(partial, `server`), "server checks no sub-keys")
	require.False(t, mayRunAdminCommand(partial, `zone`), "zone checks no sub-keys")
}

// `teleport <player> <where>` moves another player only with teleport.other;
// moving oneself to a player still needs only teleport.playername.
func TestTeleport_MovingAnotherPlayerNeedsTeleportOther(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	staff, leader, _, _ := teleportPartyScene(t)
	withAdminTestRoles(t, configs.Roles{
		`helper`:      {`teleport.playername`},
		`mover`:       {`teleport.playername`, `teleport.other`},
		`teleportall`: {`teleport`},
	})

	staff.Role = `helper`
	_, err := Teleport(`Bobrick Carrow`, staff, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)
	require.Equal(t, tpRoomB, leader.Character.RoomId, "a helper must not move another player")

	staff.Role = `mover`
	_, err = Teleport(`Bobrick Carrow`, staff, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)
	require.Equal(t, tpRoomA, leader.Character.RoomId, "teleport.other moves another player")

	staff.Role = `teleportall`
	_, err = Teleport(`Bobrick 12`, staff, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)
	require.Equal(t, tpRoomC, leader.Character.RoomId, "the whole teleport key covers teleport.other")

	staff.Role = `helper`
	_, err = Teleport(`Bobrick`, staff, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)
	require.Equal(t, tpRoomC, staff.Character.RoomId, "a helper still goes to a player")
}

// roleChangeRefusal: only an admin grants admin, an admin is never changed,
// and nobody raises their own role.
func TestRoleChangeRefusal(t *testing.T) {
	mod := users.NewTestUser(911, `mod`, `Moddo`, 0)
	mod.Role = `moderator`
	tim := users.NewTestUser(912, `tim`, `Timmo`, 0)
	adm := users.NewTestUser(913, `adm`, `Admo`, 0)
	adm.Role = users.RoleAdmin
	adm2 := users.NewTestUser(914, `adm2`, `Admtwo`, 0)
	adm2.Role = users.RoleAdmin

	require.NotEmpty(t, roleChangeRefusal(mod, tim, users.RoleAdmin), "a non-admin grants admin")
	require.NotEmpty(t, roleChangeRefusal(mod, mod, `builder`), "a non-admin changes their own role")
	require.NotEmpty(t, roleChangeRefusal(mod, adm, `builder`), "a non-admin changes an admin")
	require.NotEmpty(t, roleChangeRefusal(adm, adm2, users.RoleUser), "an admin's role is not changed here")

	require.Empty(t, roleChangeRefusal(mod, tim, `builder`), "a non-admin sets another player's lesser role")
	require.Empty(t, roleChangeRefusal(mod, mod, users.RoleUser), "anyone may step down to user")
	require.Empty(t, roleChangeRefusal(adm, tim, users.RoleAdmin), "an admin grants admin")
}

// `modify role` through the command: a modify.role holder cannot make a
// player admin, nor change their own role.
func TestModifyRole_RefusesAdminGrantAndSelfRaise(t *testing.T) {
	withAdminTestRoles(t, configs.Roles{
		`moderator`: {`modify.role`},
		`builder`:   {`build`},
	})
	mod := users.NewTestUser(921, `mod`, `Moddo`, 0)
	mod.Role = `moderator`
	tim := users.NewTestUser(922, `tim`, `Timmo`, 0)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{921: mod, 922: tim}))

	_, err := Modify(`role tim admin`, mod, nil, 0)
	require.NoError(t, err)
	require.Equal(t, users.RoleUser, tim.Role, "a modify.role holder must not grant admin")

	_, err = Modify(`role mod admin`, mod, nil, 0)
	require.NoError(t, err)
	require.Equal(t, `moderator`, mod.Role, "a modify.role holder must not make themselves admin")

	_, err = Modify(`role mod builder`, mod, nil, 0)
	require.NoError(t, err)
	require.Equal(t, `moderator`, mod.Role, "a modify.role holder must not change their own role")
}

// `command` refuses an admin target unless the issuer is an admin; a
// non-admin target is still commanded.
func TestCommand_RefusesAnAdminTargetForANonAdmin(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	staff, _, _, bystander := teleportPartyScene(t) // both in room A; staff is an admin
	withAdminTestRoles(t, configs.Roles{`commander`: {`command`}})
	bystander.Role = `commander`
	events.DrainQueuedUserInputsForTest(staff.UserId)

	_, err := Command(`Adminna say hi`, bystander, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)
	require.Empty(t, events.DrainQueuedUserInputsForTest(staff.UserId), "a non-admin must not command an admin")

	staff.Role = users.RoleUser
	_, err = Command(`Adminna say hi`, bystander, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)
	require.Equal(t, []string{`say hi`}, events.DrainQueuedUserInputsForTest(staff.UserId), "a non-admin target is still commanded")

	staff.Role = users.RoleAdmin
	bystander.Role = users.RoleAdmin
	_, err = Command(`Adminna say hi`, bystander, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)
	require.Equal(t, []string{`say hi`}, events.DrainQueuedUserInputsForTest(staff.UserId), "an admin commands an admin")
}
