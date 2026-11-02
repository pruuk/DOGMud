package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-11-02: Veyra teaches the pinnacle recipes, so nobody may
// grief by keeping her dead or baiting her into a fight. Her template stays a
// non-combatant that cannot be charmed, does not start fights and has no
// combat commands, and the harm check refuses every player attack on her.
func TestVeyraCannotBeHarmed(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	require.NoError(t, err)

	veyra := templates[9584]
	require.NotNil(t, veyra, "Veyra (mob 9584) is missing from the shipped mobs")
	require.True(t, veyra.IsNonCombatant(), "non_combatant")
	require.True(t, veyra.CharmImmune, "charm_immune")
	require.False(t, veyra.LegacyHostile, "hostile: false")
	require.False(t, veyra.AutoAggro, "auto_aggro")
	require.Empty(t, veyra.CombatCommands, "no combatcommands")
	require.Equal(t, mobs.HarmBlockedNonCombatant, mobs.CheckPlayerHarm(veyra))
}

// The guard above is only worth having if it can fail: the same checks fire on
// a mob that is hostile and fights.
func TestVeyraCannotBeHarmed_GuardCanFail(t *testing.T) {
	brute := &mobs.Mob{LegacyHostile: true, AutoAggro: true, CombatCommands: []string{"attack"}}
	require.False(t, brute.IsNonCombatant())
	require.False(t, brute.CharmImmune)
	require.NotEqual(t, mobs.HarmBlockedNonCombatant, mobs.CheckPlayerHarm(brute))
}
