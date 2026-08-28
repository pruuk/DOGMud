package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// The regen stat lists are authored as literals at six OnRegenTick call sites
// (three player, three mob) in NewRound_AutoHeal.go, and which stats appear in
// them IS the regen faucet's shape. There is no runtime accessor to assert
// against, so this pins the literals.
//
// It exists because the mapping already drifted away from its documentation
// once: regenDamperFactor's comment claimed vitality took TWO regen rolls per
// tick, which stopped being true when U10b-1 Task 22 dropped vitality from the
// stamina row, and the claim sat there describing a faucet that no longer
// existed. U10b-2 corrected the comment; this keeps the code from moving out
// from under it again silently.
func TestRegenProgression_StatListsPerPool(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller(0) failed")
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "NewRound_AutoHeal.go"))
	require.NoError(t, err)

	// Match each call's Pool argument and its []string{...} literal, tolerating
	// the long explanatory comments that sit between them.
	re := regexp.MustCompile(`OnRegenTick\(characters\.(Pool\w+),(?s:.*?)\[\]string\{([^}]*)\}`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	require.Len(t, matches, 6, "expected six OnRegenTick call sites (three player, three mob)")

	stat := regexp.MustCompile(`"(\w+)"`)
	perStat := map[string]int{}
	byPool := map[string][]string{}
	for _, m := range matches {
		var names []string
		for _, s := range stat.FindAllStringSubmatch(m[2], -1) {
			names = append(names, s[1])
			perStat[s[1]]++
		}
		byPool[m[1]] = names
	}

	// The mapping, player and mob paths identical.
	require.Equal(t, []string{"vitality", "willpower"}, byPool["PoolHealth"])
	require.Equal(t, []string{"strength"}, byPool["PoolStamina"],
		"stamina is STRENGTH ONLY since U10b-1 Task 22; re-adding vitality here gives it two pools again")
	require.Equal(t, []string{"willpower", "charisma"}, byPool["PoolConviction"])

	// Rolls per character per tick. WILLPOWER is the only stat drawing from two
	// pools; the regenDamperFactor comment says so and this is what makes it true.
	require.Equal(t, 2, perStat["vitality"], "vitality: one roll per tick, on each of the player and mob paths")
	require.Equal(t, 4, perStat["willpower"], "willpower is the double-dipper: Health + Conviction, on both paths")
	require.Equal(t, 2, perStat["strength"])
	require.Equal(t, 2, perStat["charisma"])
}
