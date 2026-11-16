package guards

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Owner ruling (2026-10-17): a harmful condition's start lines name whoever
// laid it on. A mutation's on_hit_condition is always queued with the
// attacker as its caster (queueOnHitCondition), so every condition a
// shipped mutation applies on hit must name {actor} in both the holder's and
// the room's start line. #596 found three that did not (Venom, Ensnared,
// Rending Bleed); a fourth on-hit mutation added later is covered too.
func TestOnHitConditionsNameTheAttacker(t *testing.T) {
	world := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")

	type effect struct {
		Type  string `yaml:"type"`
		Value int    `yaml:"value"`
	}
	type mutation struct {
		MutationId string   `yaml:"mutationid"`
		Pros       []effect `yaml:"pros"`
		Cons       []effect `yaml:"cons"`
	}
	mutationFiles, err := filepath.Glob(filepath.Join(world, "mutations", "*.yaml"))
	if err != nil || len(mutationFiles) == 0 {
		t.Fatalf("no mutation files found (err %v): the guard cannot see anything", err)
	}
	onHit := map[int]string{} // condition id -> a mutation that applies it
	for _, path := range mutationFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m mutation
		if err := yaml.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, e := range append(m.Pros, m.Cons...) {
			if e.Type == "on_hit_condition" {
				onHit[e.Value] = m.MutationId
			}
		}
	}
	if len(onHit) < 4 {
		t.Fatalf("found %d on-hit conditions, want at least the 4 shipped (39, 111, 114, 115): %v", len(onHit), onHit)
	}

	type condition struct {
		StartActee    string `yaml:"start_actee"`
		StartObserver string `yaml:"start_observer"`
	}
	for id, mutationId := range onHit {
		matches, _ := filepath.Glob(filepath.Join(world, "conditions", strconv.Itoa(id)+"-*.yaml"))
		if len(matches) != 1 {
			t.Errorf("condition %d (on hit from %s): found %d files, want 1", id, mutationId, len(matches))
			continue
		}
		raw, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		var c condition
		if err := yaml.Unmarshal(raw, &c); err != nil {
			t.Fatalf("%s: %v", matches[0], err)
		}
		for role, line := range map[string]string{"start_actee": c.StartActee, "start_observer": c.StartObserver} {
			if !strings.Contains(line, "{actor") {
				t.Errorf("%s: %s %q does not name the attacker ({actor}); %s applies it on hit",
					filepath.Base(matches[0]), role, line, mutationId)
			}
		}
	}
}
