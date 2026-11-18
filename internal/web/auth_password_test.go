package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// writeStoredAuthTestUser overwrites a test user's file with its password
// stored exactly as given: plain text, empty, or a legacy hash.
func writeStoredAuthTestUser(t *testing.T, userID int, username, role, stored string) {
	t.Helper()

	u := users.NewUserRecord(userID, 0)
	u.Username = username
	u.Role = role
	u.Password = stored

	data, err := yaml.Marshal(u)
	if err != nil {
		t.Fatalf("Marshal user %q: %v", username, err)
	}
	userPath := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "users", strconv.Itoa(userID)+".yaml")
	if err := os.WriteFile(userPath, data, 0600); err != nil {
		t.Fatalf("WriteFile(%q): %v", userPath, err)
	}
}

// An admin whose stored password is plain text is refused: /admin cannot
// force a password change, so the account must change it in game first.
func TestDoBasicAuthRefusesPlaintextPassword(t *testing.T) {
	setupAuthTestUsers(t, "unused-password", map[string]string{"adminuser": users.RoleAdmin})
	writeStoredAuthTestUser(t, 1, "adminuser", users.RoleAdmin, "plain-password")
	resetAuthStateForTest()

	if code, called := doAuthRequest(t, "adminuser", "plain-password", "203.0.113.20:5000"); code != http.StatusUnauthorized || called {
		t.Fatalf("plain-text admin: status=%d called=%t, want 401/false", code, called)
	}
}

// An admin whose stored password is empty is refused with an empty password.
func TestDoBasicAuthRefusesEmptyPassword(t *testing.T) {
	setupAuthTestUsers(t, "unused-password", map[string]string{"adminuser": users.RoleAdmin})
	writeStoredAuthTestUser(t, 1, "adminuser", users.RoleAdmin, "")
	resetAuthStateForTest()

	if code, called := doAuthRequest(t, "adminuser", "", "203.0.113.21:5000"); code != http.StatusUnauthorized || called {
		t.Fatalf("empty-password admin: status=%d called=%t, want 401/false", code, called)
	}
}

// An admin with a legacy SHA-256 hash gets in, the upgraded bcrypt hash is
// saved, and the cached grant survives revalidation against the file.
func TestDoBasicAuthSavesUpgradedLegacyHash(t *testing.T) {
	const password = "legacy-password"
	setupAuthTestUsers(t, "unused-password", map[string]string{"adminuser": users.RoleAdmin})
	writeStoredAuthTestUser(t, 1, "adminuser", users.RoleAdmin, util.Hash(password))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{}))
	resetAuthStateForTest()

	if code, called := doAuthRequest(t, "adminuser", password, "203.0.113.22:5000"); code != http.StatusOK || !called {
		t.Fatalf("legacy-hash admin: status=%d called=%t, want 200/true", code, called)
	}

	onDisk, err := users.LoadUser("adminuser", true)
	if err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if !strings.HasPrefix(onDisk.Password, "$2") {
		t.Fatalf("stored password after login = %q, want a bcrypt hash", onDisk.Password)
	}

	forceAuthRevalidation()
	if code, called := doAuthRequest(t, "adminuser", password, "203.0.113.22:5000"); code != http.StatusOK || !called {
		t.Fatalf("revalidated grant: status=%d called=%t, want 200/true", code, called)
	}
}

// An unknown username and a refused plain-text account each spend one
// bcrypt comparison, as a wrong password for a real account does.
func TestDoBasicAuthSpendsPasswordCheckWithNoAccountToCheck(t *testing.T) {
	setupAuthTestUsers(t, "unused-password", map[string]string{"plainuser": users.RoleAdmin})
	writeStoredAuthTestUser(t, 1, "plainuser", users.RoleAdmin, "plain-password")
	resetAuthStateForTest()

	for _, name := range []string{"nosuchuser", "plainuser"} {
		count, restore := users.CountSpentPasswordChecksForTest()
		code, _ := doAuthRequest(t, name, "some-password", "203.0.113.23:5000")
		restore()
		if code != http.StatusUnauthorized {
			t.Fatalf("%s: status=%d, want 401", name, code)
		}
		if *count != 1 {
			t.Fatalf("%s: spent %d password checks, want 1", name, *count)
		}
	}
}
