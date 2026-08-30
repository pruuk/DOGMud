package combat

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MobDamageMultiplier is the NPC-only physical dial, and it must reach EVERY
// NPC physical damage path.
//
// ⚠️ It used to be applied at exactly one site -- the mob melee auto-attack in
// combat_helpers.go -- despite its name and its config comment claiming to
// govern NPC physical damage generally. Every mob special move (all 16), the
// dodge-crit counter-sweep and every mob ranged shot route through
// skill_moves.go and silently ignored it, so a mob's bash was scaled
// differently from its own punch.
//
// That mattered the moment MeleeDamageScale became the PLAYER-side dial on
// 2026-08-30: the two share the physical channel, and NPCs are held down only
// by this multiplier. A path that misses it gets player-level damage.
func TestMobDamageMultiplierReachesBothPhysicalPaths(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	// Both files must consult the knob.
	for _, name := range []string{"combat_helpers.go", "skill_moves.go"} {
		src, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		assert.Contains(t, string(src), "MobDamageMultiplier",
			"%s computes NPC physical damage and must apply MobDamageMultiplier. "+
				"Without it, NPCs deal PLAYER-level damage on that path, because "+
				"MeleeDamageScale is the player dial and this knob is the only "+
				"thing holding NPCs down.", name)
	}
}

// The multiplier must be gated on the attacker actually being an NPC, or it
// would quietly nerf players too.
func TestMobDamageMultiplierIsGatedOnIsMob(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "skill_moves.go"))
	require.NoError(t, err)

	// The apply site must sit inside a mob check.
	window := regexp.MustCompile(`(?s)IsMob\s*\{[^}]*MobDamageMultiplier`)
	assert.True(t, window.Match(src),
		"skill_moves.go must apply MobDamageMultiplier only when the attacker "+
			"IsMob; applying it unconditionally would nerf players by the NPC dial")
	assert.False(t, strings.Contains(string(src), "srcType == Mob"),
		"skill_moves.go has no srcType; it gates on Attacker.IsMob")
}
