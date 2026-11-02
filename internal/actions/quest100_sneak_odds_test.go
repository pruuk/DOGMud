package actions

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/require"
)

// Quest 100 (From the Dark) asks a rank-1 student to walk hidden into the
// Tunnel Brute's room. Its text says the hood is the better way in and that a
// few tries will do, so this holds the sump's odds to that, from the shipped
// config and the real mob file. A retune that makes the walk rare fails here,
// and the quest's text and the mine both need another look.
//
// Measured when written (2026-11-01, Dexterity 100, rank 1): 73.5% hooded,
// 53.5% with the lantern open. Nothing rolls again while the student stands
// there: the only rolls on a hider are at the door (EntryDetection) or from a
// newcomer or a searcher, and this brute is neither.
func TestQuest100_RankOneWalksPastTheBruteOftenEnough(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repo := filepath.Join(filepath.Dir(here), "..", "..")
	t.Chdir(repo)
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repo, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{}))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{}))
	conditions.LoadDataFiles()
	species.LoadDataFiles()
	items.LoadDataFiles()
	mobs.LoadDataFiles()

	const (
		bruteMobId   = 9121
		lanternLight = 54 // condition 127 at full strength
		trials       = 4000
	)
	student := characters.New()
	student.Stats.Dexterity.ValueAdj = 100
	student.SetSkill(string(skills.Skullduggery), 1)
	base := student.SneakBaseScore()
	require.InDelta(t, 105.0, base, 0.001, "Dexterity 100 plus one rank at SkillWeight 5")

	bal := configs.GetBalanceConfig()
	rate := func(light int, modifier float64) float64 {
		won := 0
		for i := 0; i < trials; i++ {
			brute := mobs.NewMobByIdFresh(bruteMobId, 5257)
			require.NotNil(t, brute)
			observer, _ := sneakObserverScore(&brute.Character, messaging.FixedLight(light))
			if combat.RunContest(base*modifier, []contest.Entry{{Score: observer}}).Success {
				won++
			}
		}
		return float64(won) / trials
	}

	// Hooded in a sealed cave the brute sees nothing and only hears.
	brute := mobs.NewMobByIdFresh(bruteMobId, 5257)
	_, sight := sneakObserverScore(&brute.Character, messaging.FixedLight(0))
	require.Equal(t, messaging.SightNone, sight, "a hooded student gives the brute nothing to see")

	hooded := rate(0, 1.0)
	open := rate(lanternLight, float64(bal.SneakModEmitsLightLitRoom))
	t.Logf("hooded %.3f, open %.3f", hooded, open)

	require.GreaterOrEqual(t, hooded, 0.60, "the text promises a rank-1 student a few tries at most")
	require.Less(t, open, hooded-0.10, "the text says the closed hood is the better way in")
}
