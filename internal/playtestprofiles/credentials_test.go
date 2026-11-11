package playtestprofiles

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestGenerateCredentialsShapeAndHash(t *testing.T) {
	_ = configs.AddOverlayOverrides(map[string]any{
		"Validation.PasswordSizeMin": 4,
		"Validation.PasswordSizeMax": 16,
		"Validation.NameSizeMin":     1,
		"Validation.NameSizeMax":     80,
		"Validation.NameRejectRegex": `^[a-zA-Z0-9_]+$`,
	})
	u := &users.UserRecord{
		Role:      users.RoleUser,
		Character: &characters.Character{Name: "Tester"},
	}
	username, password, err := GenerateCredentials(u, "specialist-caster")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(username, "pt_specialist_caster_"), "got %q", username)
	require.NoError(t, users.ValidateName(username))
	require.GreaterOrEqual(t, len(password), 4)
	require.LessOrEqual(t, len(password), 16)
	require.Equal(t, username, u.Username)
	require.NotEqual(t, password, u.Password)
	require.True(t, u.PasswordMatches(password))
}

// The shipped config.yaml bans "*admin*" (and "*moderator*") for player-chosen
// names. The harness generates the admin profile's username itself, and login
// of an existing account never re-checks BannedNames, so the generator must not
// apply that policy or the admin profile can never start.
func TestGenerateCredentialsAdminProfileSurvivesShippedBannedNames(t *testing.T) {
	_ = configs.AddOverlayOverrides(map[string]any{
		"Validation.NameSizeMin":     2,
		"Validation.NameSizeMax":     28, // shipped since plan 3b-3 (was 32)
		"Validation.NameRejectRegex": `^[a-zA-Z0-9_]+$`,
		"Validation.BannedNames":     []string{"*admin*", "*moderator*"},
	})
	t.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"Validation.BannedNames": []string{}})
	})
	_, banned := configs.GetConfig().IsBannedName("pt_admin_abc123")
	require.True(t, banned, "precondition: the banned list must be live or this test cannot fail")

	u := &users.UserRecord{
		Role:      users.RoleAdmin,
		Character: &characters.Character{Name: "Tester"},
	}
	username, _, err := GenerateCredentials(u, "admin")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(username, "pt_admin_"), "got %q", username)
}

// Every tracked profile's generated username (pt_<profile>_<6 hex>) fits the
// shipped name limit, 28 since plan 3b-3 lowered NameSizeMax from 32. The
// longest, pt_specialist_caster_xxxxxx, is 27; a longer profile id would make
// GenerateCredentials fail every attempt and the profile could never start.
func TestGenerateCredentialsFitsTheShippedNameLimitForEveryProfile(t *testing.T) {
	_ = configs.AddOverlayOverrides(map[string]any{
		"Validation.PasswordSizeMin": 4,
		"Validation.PasswordSizeMax": 16,
		"Validation.NameSizeMin":     2,
		"Validation.NameSizeMax":     28,
		"Validation.NameRejectRegex": `^[a-zA-Z0-9_]+$`,
	})
	for _, id := range KnownTemplateIDs {
		u := &users.UserRecord{Role: users.RoleUser, Character: &characters.Character{Name: "Tester"}}
		username, _, err := GenerateCredentials(u, id)
		require.NoError(t, err, "profile %q", id)
		require.LessOrEqual(t, len(username), 28, "profile %q: %q", id, username)
	}
}

func TestSanitizeProfileToken(t *testing.T) {
	require.Equal(t, "specialist_caster", sanitizeProfileToken("specialist-caster"))
	require.Equal(t, "mid", sanitizeProfileToken("mid"))
}

func TestGenerateCredentialsRejectsHyphenShape(t *testing.T) {
	_ = configs.AddOverlayOverrides(map[string]any{
		"Validation.NameRejectRegex": `^[a-zA-Z0-9_]+$`,
		"Validation.NameSizeMin":     2,
		"Validation.NameSizeMax":     28, // shipped since plan 3b-3 (was 32)
	})
	require.Error(t, users.ValidateName("pt-fresh-abc123"))
	require.NoError(t, users.ValidateName("pt_fresh_abc123"))
}
