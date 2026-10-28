package quests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"gopkg.in/yaml.v2"
)

// Every way the map can show a destination has at least one live step in
// shipped content (#375 phase 4 slice 3), so the playtests and screenshots
// that prove each render keep a step to stand on. A kind counts whether it
// is the destination or a mob's fallback: a zone fallback draws the same
// banner as a zone. Any non-secret step of the kind will do.
func TestShippedContent_EveryDestinationKindHasALiveStep(t *testing.T) {
	chdirRepoRootForTest(t)

	dir := configs.GetFilePathsConfig().DataFiles.String() + `/quests`
	files, err := filepath.Glob(dir + `/*.yaml`)
	if err != nil || len(files) == 0 {
		t.Fatalf("no quest files found under %s: %v", dir, err)
	}

	seen := map[string]string{}
	note := func(kind, where string) {
		if _, ok := seen[kind]; !ok {
			seen[kind] = where
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var q Quest
		if err := yaml.Unmarshal(data, &q); err != nil {
			t.Fatalf("%s: parse: %v", f, err)
		}
		if q.Secret {
			continue
		}
		for _, s := range q.Steps {
			d := s.Destination
			where := fmt.Sprintf("quest %d step %s", q.QuestId, s.Id)
			if d.Anywhere {
				note("anywhere", where)
			}
			if len(d.Mobs) > 0 {
				note("mob", where)
			}
			if d.Room > 0 {
				note("room", where)
			}
			if d.Area != "" {
				note("area", where)
			}
			if d.Zone != "" {
				note("zone", where)
			}
		}
	}
	for _, kind := range []string{"anywhere", "room", "mob", "area", "zone"} {
		if where, ok := seen[kind]; ok {
			t.Logf("%s: %s", kind, where)
		} else {
			t.Errorf("no live non-secret step has destination kind %q", kind)
		}
	}
}
