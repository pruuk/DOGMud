package species

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Slice 1 of the combat reachability spec (#541): creatures PR #208 found on
// the wrong species are the species they look like. Owner decisions D1 to D8
// are recorded in docs/superpowers/plans/completed/2026-10-16-combat-reachability-slice1-species.md.
func TestCreaturesAreTheSpeciesTheyLookLike(t *testing.T) {
	want := map[string]int{
		"amber_valley/9407-a_sun_lizard.yaml":               21,
		"amber_valley/9408-a_pale_cave_crawler.yaml":        12,
		"amber_valley/9409-a_cave_lurker.yaml":              12,
		"ashwick/263-forest_fox.yaml":                       2,
		"ashwick/264-timber_wolf.yaml":                      2,
		"ashwick/265-briar_hawk.yaml":                       9,
		"ashwick/266-village_chicken.yaml":                  25,
		"ashwick/267-cottage_mouse.yaml":                    10,
		"dustwalk_road/82-scavenger_bird.yaml":              25,
		"east_road_to_greenford/9498-a_wheeling_hawk.yaml":  9,
		"east_road_to_greenford/9499-a_jackrabbit.yaml":     10,
		"greenford/9507-a_grey_heron.yaml":                  25,
		"greenford/9508-a_river_otter.yaml":                 24,
		"greywater_flats/9194-a_grey_heron.yaml":            25,
		"ironwind_steppe/201-dust_crow.yaml":                25,
		"ironwind_steppe/214-sage_grouse.yaml":              25,
		"ironwind_steppe/233-carrion_crow.yaml":             25,
		"labyrinth_of_low_tunnels/77-fungal_grub.yaml":      18,
		"labyrinth_of_low_tunnels/78-spore_crawler.yaml":    12,
		"marches_spur_road/255-scrub_coyote.yaml":           2,
		"marches_spur_road/256-feral_hog.yaml":              6,
		"marches_spur_road/257-field_sparrow.yaml":          25,
		"marches_spur_road/258-farm_cat.yaml":               11,
		"new_plymouth_old_quarter/9393-a_canal_lurker.yaml": 12,
		"north_road/326-river_rat.yaml":                     10,
		"north_road/327-wild_dog.yaml":                      2,
		"river_road/9416-a_grey_heron.yaml":                 25,
		"river_road/9417-a_river_otter.yaml":                24,
		"river_road/9418-a_river_gar.yaml":                  13,
		"thornwall_city/375-hob.yaml":                       26,
		"thornwall_city/376-bran.yaml":                      26,
		"watchers_crossing/87-river_lurker.yaml":            21,
	}
	require.Len(t, want, 32)
	species := dogmudSpecies(t)
	dir := filepath.Join("..", "..", "_datafiles", "world", "dogmud", "mobs")
	for rel, id := range want {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		require.NoError(t, err, rel)
		var mob struct {
			Character struct {
				SpeciesId int `yaml:"speciesid"`
			} `yaml:"character"`
		}
		require.NoError(t, yaml.Unmarshal(data, &mob), rel)
		require.Equal(t, id, mob.Character.SpeciesId, "%s", rel)
		require.NotNil(t, species[id], "%s names species %d, which does not exist", rel, id)
	}
}
