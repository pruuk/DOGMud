package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// loadFightWorld loads the shipped config and the data a fight read needs:
// conditions, items, species, spells and the mob templates, seeded so
// mobs.NewMobByIdFresh can spawn them. Shared by the PowerScore table and
// the pinned consider reads (#301).
func loadFightWorld(t *testing.T) map[int]*mobs.Mob {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(spells.SeedSpellsForTest(nil))
	// Spawn applies SpawnMutations (e.g. apex-predator on mobs 223 and 9538),
	// which change the spawned character's stats and so its PowerScore.
	t.Cleanup(mutations.SeedMutationsForTest(nil))
	mutations.LoadMutationFiles()
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	species.LoadDataFiles()
	spells.LoadSpellFiles()

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", dataPath, err)
	}
	t.Cleanup(mobs.SeedMobsForTest(templates, map[int]*mobs.Mob{}))
	return templates
}

// fixtureAdventurer is a bare-handed player with every stat at stat and
// unarmed combat and search at rank, pools sized from stat. Not a shipped
// character: a fixed yardstick for tables and pinned reads.
func fixtureAdventurer(t *testing.T, name string, stat, rank int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Name = name
	c.RoomId = 1
	for _, s := range []*int{&c.Stats.Strength.Base, &c.Stats.Dexterity.Base, &c.Stats.Perception.Base,
		&c.Stats.Vitality.Base, &c.Stats.Willpower.Base, &c.Stats.Charisma.Base} {
		*s = stat
	}
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stats.Perception.Recalculate()
	c.Stats.Vitality.Recalculate()
	c.Stats.Willpower.Recalculate()
	c.Stats.Charisma.Recalculate()
	if err := c.Validate(); err != nil {
		t.Fatalf("validate %s: %v", name, err)
	}
	c.Skills[string(skills.UnarmedCombat)] = rank
	c.Skills[string(skills.Search)] = rank
	c.HealthMax.Value = 2 * stat
	c.Health = c.HealthMax.Value
	c.StaminaMax.Value = 2 * stat
	c.Stamina = c.StaminaMax.Value
	c.ConvictionMax.Value = stat
	c.Conviction = stat
	return c
}
