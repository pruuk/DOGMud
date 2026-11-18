package inputhandlers

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// withLoginWorld points DataFiles at a temp world holding one account whose
// stored password is stored as given, isolates the online-user table and the
// throttle, and registers a connection to log in on. It returns the
// connection and the path of the account's file.
func withLoginWorld(t *testing.T, connId connections.ConnectionId, userId int, username, stored string) string {
	t.Helper()

	dir := t.TempDir()
	prev := configs.GetFilePathsConfig()
	require.NoError(t, configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles":        dir,
		"FilePaths.CarefulSaveFiles": false,
	}))
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles":        prev.DataFiles.String(),
			"FilePaths.CarefulSaveFiles": bool(prev.CarefulSaveFiles),
		})
	})
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "users"), 0o755))

	idx := users.NewUserIndex()
	require.NoError(t, idx.Create())
	require.NoError(t, idx.AddUser(userId, username))

	save := "userid: " + strconv.Itoa(userId) + "\nusername: " + username +
		"\npassword: \"" + stored + "\"\ncharacter:\n  name: " + username + "\n"
	path := filepath.Join(dir, "users", strconv.Itoa(userId)+".yaml")
	require.NoError(t, os.WriteFile(path, []byte(save), 0o644))

	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{}))

	// A plain test connection, not the websocket one: a refused login writes
	// its answer, and this one fails the write instead of dereferencing an
	// undialled websocket.
	connections.RegisterTestConnection(connId)
	connections.Get(connId).SetClientIP(`198.51.100.40`)
	gameLoginThrottle.ResetForTest()
	t.Cleanup(func() {
		connections.UnregisterTestConnection(connId)
		gameLoginThrottle.ResetForTest()
	})
	return path
}

// storedPassword reads the account's password back from its file.
func storedPassword(t *testing.T, username string) string {
	t.Helper()
	u, err := users.LoadUser(username, true)
	require.NoError(t, err)
	return u.Password
}

// A game login with a legacy unsalted SHA-256 password saves the bcrypt hash
// it was upgraded to, so the account never goes back to the legacy form.
func TestFinalizeLogin_SavesUpgradedLegacyHash(t *testing.T) {
	const conn connections.ConnectionId = 97710
	withLoginWorld(t, conn, 31, `legacyhash`, util.Hash(`old-password`))

	shared := map[string]any{}
	ok := FinalizeLoginOrCreate(map[string]string{`username`: `legacyhash`, `password`: `old-password`},
		shared, &connections.ClientInput{ConnectionId: conn})
	require.True(t, ok, "the legacy password did not log in")

	stored := storedPassword(t, `legacyhash`)
	require.NotEqual(t, util.Hash(`old-password`), stored, "the legacy hash is still on disk after login")
	check := &users.UserRecord{Password: stored}
	require.True(t, check.PasswordMatches(`old-password`), "the saved hash does not match the password")

	loggedIn, _ := shared["UserObject"].(*users.UserRecord)
	require.NotNil(t, loggedIn)
	require.Equal(t, stored, loggedIn.Password, "the live user holds a different hash than the file")
}

// A legacy plain-text password still logs into the game, and stays plain
// text, so the game can force the player to change it.
func TestFinalizeLogin_PlaintextLogsInForAChange(t *testing.T) {
	const conn connections.ConnectionId = 97711
	withLoginWorld(t, conn, 32, `plainuser`, `plain-password`)

	shared := map[string]any{}
	ok := FinalizeLoginOrCreate(map[string]string{`username`: `plainuser`, `password`: `plain-password`},
		shared, &connections.ClientInput{ConnectionId: conn})
	require.True(t, ok, "the plain-text password did not log in")

	loggedIn, _ := shared["UserObject"].(*users.UserRecord)
	require.NotNil(t, loggedIn)
	require.True(t, loggedIn.HasPlaintextPassword(), "the login hid the plain-text password the game must force a change of")
}

// An account whose stored password is empty does not log in, not even with
// an empty password.
func TestFinalizeLogin_RefusesEmptyStoredPassword(t *testing.T) {
	const conn connections.ConnectionId = 97712
	withLoginWorld(t, conn, 33, `emptyuser`, ``)

	shared := map[string]any{}
	ok := FinalizeLoginOrCreate(map[string]string{`username`: `emptyuser`, `password`: ``},
		shared, &connections.ClientInput{ConnectionId: conn})
	require.False(t, ok, "an empty stored password logged in")
	require.Nil(t, shared["UserObject"])
}

// An unknown username spends one bcrypt comparison, as a wrong password for
// a real account does.
func TestFinalizeLogin_UnknownUserSpendsPasswordCheck(t *testing.T) {
	const conn connections.ConnectionId = 97713
	withLoginWorld(t, conn, 34, `realuser`, util.Hash(`whatever`))

	count, restore := users.CountSpentPasswordChecksForTest()
	ok := FinalizeLoginOrCreate(map[string]string{`username`: `nosuchuser`, `password`: `guess`},
		map[string]any{}, &connections.ClientInput{ConnectionId: conn})
	restore()
	require.False(t, ok)
	require.Equal(t, 1, *count, "an unknown username did not spend a password check")
}
