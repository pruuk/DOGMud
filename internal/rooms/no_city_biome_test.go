package rooms

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestNoDogmudRoomOrZoneDefaultsToCityBiome guards plan 3c-2b's deletion of
// the `city` biome from the dogmud world. Every room that was `biome: city`
// has been reclassified into a tier (`city_thoroughfare`, `city_backstreet`,
// `interior`, `dungeon`, ...), and every zone-config that defaulted new rooms
// to `city` now defaults to `city_backstreet`. `city.yaml` no longer exists,
// and an unknown biome does not fail at boot: the room silently falls back to
// the synthetic `default` biome. This guard is the only thing that says so.
//
// Reads files as text and matches any line that YAML would read as the value
// `city` (trailing spaces, quotes and a trailing comment included), anchored
// so `city_backstreet` / `city_thoroughfare` never match: those are the tiers
// this guard exists to protect.
var cityBiomeLine = regexp.MustCompile(`^(default)?biome:\s*["']?city["']?\s*(#.*)?$`)

func TestNoDogmudRoomOrZoneDefaultsToCityBiome(t *testing.T) {
	root := "../../_datafiles/world/dogmud/rooms"

	var badRooms []string
	var badZones []string

	err := util.WalkDirTolerant(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		for line := range strings.SplitSeq(string(b), "\n") {
			m := cityBiomeLine.FindStringSubmatch(strings.TrimRight(line, "\r"))
			switch {
			case m == nil:
			case m[1] == "default":
				badZones = append(badZones, rel)
			default:
				badRooms = append(badRooms, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	if len(badRooms) == 0 && len(badZones) == 0 {
		return
	}

	sort.Strings(badRooms)
	sort.Strings(badZones)

	var msg strings.Builder
	if len(badRooms) > 0 {
		msg.WriteString("rooms still authored biome: city:\n")
		for _, r := range badRooms {
			msg.WriteString("  " + r + "\n")
		}
	}
	if len(badZones) > 0 {
		msg.WriteString("zone-configs still default to city:\n")
		for _, z := range badZones {
			msg.WriteString("  " + z + "\n")
		}
	}
	t.Errorf("the dogmud world no longer ships a city biome; reclassify these into a tier:\n%s", msg.String())
}
