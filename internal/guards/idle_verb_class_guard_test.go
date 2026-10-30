package guards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// Every idle command in shipped content starts with a verb the idle roll's
// class table knows (#317). The roll splits a pool into chatter and movement
// by that table, and an unknown verb quietly rolls as chatter, which would
// move a new movement verb onto the chatter dial. Mob templates, schedule
// segments and room spawn overrides all carry pools.

func idleEntries(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, "")
		}
	}
	return out
}

func addIdlePool(pools map[string][]string, key string, v any) {
	if entries := idleEntries(v); len(entries) > 0 {
		pools[key] = entries
	}
}

// shippedIdlePools reads every idle pool in shipped content, keyed
// "<file>#<where>".
func shippedIdlePools(t *testing.T) map[string][]string {
	t.Helper()
	pools := map[string][]string{}
	root := filepath.Join("_datafiles", "world", "dogmud")
	for _, sub := range []string{"mobs", "schedules", "rooms"} {
		dir := filepath.Join(root, sub)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var doc map[string]any
			if yaml.Unmarshal(raw, &doc) != nil {
				return nil // the loaders own parse errors
			}
			rel := filepath.ToSlash(path)
			switch sub {
			case "mobs":
				addIdlePool(pools, rel+"#mob", doc["idlecommands"])
			case "schedules":
				segs, _ := doc["segments"].([]any)
				for i, seg := range segs {
					if m, ok := seg.(map[any]any); ok {
						addIdlePool(pools, fmt.Sprintf("%s#segment%d", rel, i), m["idlecommands"])
					}
				}
			case "rooms":
				spawns, _ := doc["spawninfo"].([]any)
				for i, sp := range spawns {
					if m, ok := sp.(map[any]any); ok {
						addIdlePool(pools, fmt.Sprintf("%s#spawn%d", rel, i), m["idlecommands"])
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	return pools
}

// unclassifiedIdleVerbs lists every entry whose verb the table does not know.
func unclassifiedIdleVerbs(pools map[string][]string) []string {
	keys := make([]string, 0, len(pools))
	for k := range pools {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bad []string
	for _, k := range keys {
		for _, e := range pools[k] {
			verb := mobs.IdleVerb(e)
			if verb == "" {
				continue
			}
			if _, ok := mobs.ClassifyIdleVerb(verb); !ok {
				bad = append(bad, fmt.Sprintf("%s: %q", k, e))
			}
		}
	}
	return bad
}

// chainedTextEntries lists every emote or say entry that carries a ';' and
// then a known idle verb ("emote waves;wander"). The ambient path sends an
// emote or say line whole, so the chained command would print as text.
func chainedTextEntries(pools map[string][]string) []string {
	keys := make([]string, 0, len(pools))
	for k := range pools {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bad []string
	for _, k := range keys {
		for _, e := range pools[k] {
			if v := mobs.IdleVerb(e); v != "emote" && v != "say" {
				continue
			}
			parts := strings.Split(e, ";")
			for _, p := range parts[1:] {
				if _, ok := mobs.ClassifyIdleVerb(mobs.IdleVerb(p)); ok {
					bad = append(bad, fmt.Sprintf("%s: %q", k, e))
					break
				}
			}
		}
	}
	return bad
}

func TestShippedIdleVerbsAreClassified(t *testing.T) {
	pools := shippedIdlePools(t)
	// The walk must be able to find something: a known pool with a wander.
	if _, ok := pools["_datafiles/world/dogmud/mobs/instance_planar_oasis/318-sand_elemental.yaml#mob"]; !ok {
		t.Fatal("the Sand Elemental's pool was not found: the walk is not reading shipped mob files")
	}
	if len(pools) < 500 {
		t.Fatalf("only %d idle pools found (1028 on 2026-10-30): the walk is not reading shipped content", len(pools))
	}
	for _, bad := range unclassifiedIdleVerbs(pools) {
		t.Errorf("idle command whose verb mobs.idleVerbClass does not classify: %s (add the verb to the table as chatter or movement)", bad)
	}
}

func TestShippedIdleTextEntriesChainNoCommand(t *testing.T) {
	pools := shippedIdlePools(t)
	if len(pools) < 500 {
		t.Fatalf("only %d idle pools found: the walk is not reading shipped content", len(pools))
	}
	for _, bad := range chainedTextEntries(pools) {
		t.Errorf("emote or say idle entry chains a command after ';' (the ambient path sends it whole, so it would print as text): %s", bad)
	}
}

func TestUnclassifiedIdleVerbs_CatchesAPlantedVerb(t *testing.T) {
	got := unclassifiedIdleVerbs(map[string][]string{"planted#mob": {"emote waves.", "", "dance wildly"}})
	if len(got) != 1 || !strings.Contains(got[0], "dance wildly") {
		t.Fatalf("a planted unknown verb must be reported once, got %v", got)
	}
}

func TestChainedTextEntries_CatchesAPlantedChain(t *testing.T) {
	got := chainedTextEntries(map[string][]string{"planted#mob": {
		"emote waves;wander",
		"say hello; wander",
		"emote waves. Then pauses; thinks.",
		"wander;wander",
		"",
	}})
	if len(got) != 2 {
		t.Fatalf("two planted chains must be reported, plain prose and non-text verbs must not, got %v", got)
	}
}
