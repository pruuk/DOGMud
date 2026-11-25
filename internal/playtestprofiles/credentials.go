package playtestprofiles

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	usernameMaxAttempts = 8
	passwordHexLen      = 12 // hex digits; util.RandomHex takes bytes, half this
)

// GenerateCredentials creates a unique username and plaintext password, hashes
// the password onto u, and returns the plaintext for the creds artifact.
func GenerateCredentials(u *users.UserRecord, profileID string) (username, password string, err error) {
	if u == nil {
		return "", "", fmt.Errorf("playtestprofiles: nil user")
	}
	// Underscores only: Validation.NameRejectRegex rejects hyphens
	// (^[a-zA-Z0-9_]+$). Spec's pt-<profile>-<suffix> shape is preserved
	// with underscores substituted for the separators.
	base := "pt_" + sanitizeProfileToken(profileID)
	for range usernameMaxAttempts {
		candidate := base + "_" + util.RandomHex(3)
		// BannedNames is a policy on names a player chooses (shipped config
		// bans "*admin*"), and login of an existing account never re-checks
		// it. The harness picks this name itself, so skipping the banned
		// check is what lets the admin profile start. ForbiddenIdentity
		// below still keeps real prod identities out.
		if err := users.ValidateActorName(candidate, users.ValidateActorOpts{SkipBannedCheck: true}); err != nil {
			continue
		}
		if idx := users.NewUserIndex(); idx.Exists() {
			if _, found := idx.FindByUsername(candidate); found {
				continue
			}
		}
		if err := ForbiddenIdentity(candidate); err != nil {
			continue
		}
		username = candidate
		break
	}
	if username == "" {
		return "", "", fmt.Errorf("playtestprofiles: could not allocate unique username for %q", profileID)
	}
	password = util.RandomHex(passwordHexLen / 2)
	u.Username = username
	if err := u.SetPassword(password); err != nil {
		return "", "", fmt.Errorf("playtestprofiles: set password: %w", err)
	}
	return username, password, nil
}

func sanitizeProfileToken(profileID string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(profileID) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "profile"
	}
	return out
}
