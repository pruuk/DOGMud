package playtestprofiles

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The slice A infrared tester is the ONLY way to stand a shapes-only player in
// a room (nothing in dogmud content grants buff 85), so its buff has to arrive
// ALIVE.
//
// Presence in the list is not enough, and asserting only that is a gate that
// cannot fail: the 2026-09-11 playtest ran with buff 85 sitting in the list
// while the character made out nothing. Buff.Expired() is TriggersLeft <= 0 and
// never consults PermaBuff, GetBuffs filters expired buffs out and Prune
// deletes them, so a profile buff with no triggersleft is born dead.
func TestSliceAInfraredProfileCarriesALiveBuff85(t *testing.T) {
	dir := filepath.Join("..", "..", "tools", "playtest", "profiles")
	u, err := LoadTemplate(dir, "slice-a-infrared")
	require.NoError(t, err)

	var found bool
	for _, b := range u.Character.Buffs.List {
		if b.BuffId != 85 {
			continue
		}
		found = true
		require.False(t, b.Expired(),
			"buff 85 loaded but is already expired (triggersleft=%d): it would be hidden by GetBuffs and deleted by Prune", b.TriggersLeft)
	}
	require.True(t, found, "the profile carries no buff 85 at all")
}
