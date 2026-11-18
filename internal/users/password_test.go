package users

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// An empty password never matches: not as input, and not as what is stored,
// whichever format the stored value would otherwise be read as.
func TestPasswordMatchesRefusesEmpty(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("real-password"), bcrypt.MinCost)
	require.NoError(t, err)

	assert.False(t, (&UserRecord{Password: ""}).PasswordMatches(""), "empty stored, empty input")
	assert.False(t, (&UserRecord{Password: ""}).PasswordMatches("anything"), "empty stored, some input")
	assert.False(t, (&UserRecord{Password: string(hash)}).PasswordMatches(""), "bcrypt stored, empty input")
	assert.False(t, (&UserRecord{Password: "plain-password"}).PasswordMatches(""), "plain text stored, empty input")
	assert.False(t, (&UserRecord{Password: util.Hash("")}).PasswordMatches(""), "hash of empty stored, empty input")
}

// writeLegacyUser puts an account with a legacy SHA-256 password into the
// temp world from withUserDataDir and returns its file path.
func writeLegacyUser(t *testing.T, dir string, userId int, username, password string) string {
	t.Helper()
	idx := NewUserIndex()
	require.NoError(t, idx.Create())
	require.NoError(t, idx.AddUser(userId, username))
	save := "userid: " + strconv.Itoa(userId) + "\nusername: " + username +
		"\npassword: " + util.Hash(password) + "\ncharacter:\n  name: " + username + "\n"
	path := filepath.Join(dir, "users", strconv.Itoa(userId)+".yaml")
	require.NoError(t, os.WriteFile(path, []byte(save), 0o644))
	return path
}

// An offline account's upgraded hash is written to its file.
func TestSavePasswordUpgradeWritesTheFile(t *testing.T) {
	dir := withUserDataDir(t)
	t.Cleanup(SeedUsersForTest(map[int]*UserRecord{}))
	writeLegacyUser(t, dir, 41, "legacyoffline", "old-password")

	checked, err := LoadUser("legacyoffline", true)
	require.NoError(t, err)
	before := checked.Password
	require.True(t, checked.PasswordMatches("old-password"))

	saved, err := SavePasswordUpgrade(checked, before)
	require.NoError(t, err)
	require.True(t, saved)

	onDisk, err := LoadUser("legacyoffline", true)
	require.NoError(t, err)
	assert.True(t, isBcryptHash(onDisk.Password), "file still holds %q", onDisk.Password)
	assert.True(t, onDisk.PasswordMatches("old-password"))
}

// An account in the world takes the upgraded hash on its live record, which
// is saved, so the live user's next save does not write the old hash back.
func TestSavePasswordUpgradeUpdatesTheLiveUser(t *testing.T) {
	dir := withUserDataDir(t)
	writeLegacyUser(t, dir, 42, "legacyonline", "old-password")

	live, err := LoadUser("legacyonline", true)
	require.NoError(t, err)
	t.Cleanup(SeedUsersForTest(map[int]*UserRecord{live.UserId: live}))

	checked, err := LoadUser("legacyonline", true)
	require.NoError(t, err)
	before := checked.Password
	require.True(t, checked.PasswordMatches("old-password"))

	saved, err := SavePasswordUpgrade(checked, before)
	require.NoError(t, err)
	require.True(t, saved)
	assert.Equal(t, checked.Password, live.Password, "the live user kept the old hash")

	onDisk, err := LoadUser("legacyonline", true)
	require.NoError(t, err)
	assert.Equal(t, checked.Password, onDisk.Password, "the file was not saved")
}

// A password changed between the check and the save is never overwritten,
// and a check that upgraded nothing writes nothing.
func TestSavePasswordUpgradeLeavesOtherPasswordsAlone(t *testing.T) {
	dir := withUserDataDir(t)
	writeLegacyUser(t, dir, 43, "legacychanged", "old-password")

	live, err := LoadUser("legacychanged", true)
	require.NoError(t, err)
	newer, err := bcrypt.GenerateFromPassword([]byte("newer-password"), bcrypt.MinCost)
	require.NoError(t, err)
	live.Password = string(newer)
	t.Cleanup(SeedUsersForTest(map[int]*UserRecord{live.UserId: live}))

	checked, err := LoadUser("legacychanged", true)
	require.NoError(t, err)
	before := checked.Password
	require.True(t, checked.PasswordMatches("old-password"))

	saved, err := SavePasswordUpgrade(checked, before)
	require.NoError(t, err)
	assert.False(t, saved)
	assert.Equal(t, string(newer), live.Password, "a newer password was overwritten")

	unchanged := &UserRecord{UserId: 43, Username: "legacychanged", Password: string(newer)}
	saved, err = SavePasswordUpgrade(unchanged, string(newer))
	require.NoError(t, err)
	assert.False(t, saved, "a check that upgraded nothing reported a save")
}

// SpendPasswordCheck does the bcrypt work it exists to spend.
func TestSpendPasswordCheckSpendsBcrypt(t *testing.T) {
	count, restore := CountSpentPasswordChecksForTest()
	defer restore()
	SpendPasswordCheck("anything")
	assert.Equal(t, 1, *count)
	assert.NotEmpty(t, dummyHash, "no bcrypt hash was made to compare against")
}
