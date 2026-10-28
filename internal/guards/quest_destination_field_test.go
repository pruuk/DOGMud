package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// #375 phase 4: map_target and map_target_mob became the destination block.
// A quest file still carrying either key would be read as an unknown key and
// silently lose its minimap destination.
func TestQuestFilesCarryNoMapTargetKey(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "quests", "*.yaml")) // TestMain runs from the repo root
	if err != nil || len(files) < 70 {
		t.Fatalf("found %d quest files (%v); the guard would pass vacuously", len(files), err)
	}
	key := regexp.MustCompile(`(?m)^\s*map_target(_mob)?\s*:`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if key.Match(b) {
			t.Errorf("%s still has a map_target key; use destination:", filepath.Base(f))
		}
	}
}
