package seeders

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/goals"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

const (
	couleeDaceMobId  = 9126
	couleeDummyMobId = 9127
)

// loadCouleeProp reads a shipped mob from the Coulee's own file, and points
// the data folder at a temp dir before returning, so a goal the test seeds
// never lands in the repo.
func loadCouleeProp(t *testing.T, mobId int) *mobs.Mob {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repo := filepath.Join(filepath.Dir(here), "..", "..")
	world := filepath.Join(repo, "_datafiles", "world", "dogmud")

	t.Chdir(repo)
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(world)
	configs.SetConfigForTest(t, cfg)

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{}))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{}))
	conditions.LoadDataFiles()
	species.LoadDataFiles()
	items.LoadDataFiles()
	mobs.LoadDataFiles()
	require.NoError(t, factions.LoadAllDefinitions())

	m := mobs.NewMobByIdFresh(mobs.MobId(mobId), 5218)
	require.NotNil(t, m, "mob %d must load from the shipped data", mobId)

	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	configs.SetConfigForTest(t, cfg)
	name := util.ConvertForFilename(m.Character.Name)
	t.Cleanup(func() { _ = goals.Clear(mobId, name) })
	return m
}

// A successful theft seeds a revenge goal on every combat-capable mark, and
// goals are saved by template, so a Pocket Dummy kept hunting its student
// across every respawn: three talk menus and a lockpick game broke off in one
// playtest. The dummy is a prop that holds no grudge. A caught student is
// still swung at (actions.markAttacksThief, the rule every mark shares), but
// that fight ends with the dummy.
func TestPocketDummyHoldsNoGrudge(t *testing.T) {
	dummy := loadCouleeProp(t, couleeDummyMobId)

	require.False(t, dummy.AutoAggro, "a Pocket Dummy must not start fights on its own")
	require.Equal(t, ResponseReportOnly, classifyWitnessResponse(dummy),
		"a robbed Pocket Dummy must seed no revenge goal")

	const student = 7
	seedWitnessResponse(dummy, student, theftVictimRevengePriority)
	name := util.ConvertForFilename(dummy.Character.Name)
	require.Empty(t, goals.GoalsOf(int(dummy.MobId), name),
		"nothing may carry a grudge over to the next dummy")
}

// Proves the test above can fail: the same mob without its data marker hunts
// the student, which is the bug the playtest saw.
func TestPocketDummyHoldsNoGrudge_GuardCanFail(t *testing.T) {
	dummy := loadCouleeProp(t, couleeDummyMobId)
	dummy.Groups = []string{"construct"}

	require.Equal(t, ResponseRevenge, classifyWitnessResponse(dummy))
	const student = 7
	seedWitnessResponse(dummy, student, theftVictimRevengePriority)
	name := util.ConvertForFilename(dummy.Character.Name)
	require.Len(t, goals.GoalsOf(int(dummy.MobId), name), 1,
		"without the marker a theft seeds the goal that follows the dummy across respawns")
}

// Dace is the Shadows line's hand-in and teaches beside the dummy, so a theft
// on the jetty must not frighten her off it. A noncombatant witness takes
// alarmReaction, one step through a random exit; a "guard" marker takes no
// reaction at all.
func TestCutpurseDaceStaysOnHerJetty(t *testing.T) {
	dace := loadCouleeProp(t, couleeDaceMobId)

	require.Equal(t, 0, dace.MaxWander)
	require.True(t, dace.IsNonCombatant(), "the hand-in giver must stay safe from harm")
	require.Equal(t, ResponseReportOnly, classifyWitnessResponse(dace),
		"a theft on the jetty must not send her through an exit")
}

// Proves the test above can fail: the same mob without its marker recoils and
// leaves, which is the bug the playtest saw.
func TestCutpurseDaceStaysOnHerJetty_GuardCanFail(t *testing.T) {
	dace := loadCouleeProp(t, couleeDaceMobId)
	dace.Groups = []string{"humanoid", "coulee_folk"}

	require.Equal(t, ResponseAlarm, classifyWitnessResponse(dace))
}

// "guard" is read twice in the engine: here, and to run law enforcement for
// the factions the mob belongs to (justice.RunGuardEnforcement, which stops at
// once when there are none). Neither prop may belong to a faction, or the
// marker would make a straw dummy or a cutpurse start arresting people.
func TestCouleePropsEnforceNoLaw(t *testing.T) {
	for _, id := range []int{couleeDaceMobId, couleeDummyMobId} {
		m := loadCouleeProp(t, id)
		require.Empty(t, factions.FactionsForMob(m),
			"mob %d carries the guard marker, so it must belong to no faction", id)
	}
}
