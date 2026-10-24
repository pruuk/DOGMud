package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
)

func shippedStepTokens(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "quests", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("found no quest files (err %v): the guard cannot run", err)
	}
	var qs []quests.Quest
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var q quests.Quest
		if err := yaml.Unmarshal(data, &q); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		qs = append(qs, q)
	}
	toks := quests.StepTokens(qs)
	sort.Strings(toks)
	return toks
}

// TestStepIdManifest: saved characters hold their current step id, so a
// shipped step id may leave content only through a "# retired" or
// "# renamed" line, and a new step id must be added to the manifest.
func TestStepIdManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "quests", "step-ids.txt"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := quests.ParseStepManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	missing, unlisted := m.Diff(shippedStepTokens(t))
	for _, k := range missing {
		t.Errorf("step %s is gone from quest content: add it under \"# retired\" or \"# renamed\" (old -> new)", k)
	}
	for _, k := range unlisted {
		t.Errorf("step %s is not in step-ids.txt: append it", k)
	}
}

// TestStepIdManifest_GuardCanFail proves the guard sees a removed step.
func TestStepIdManifest_GuardCanFail(t *testing.T) {
	toks := shippedStepTokens(t)
	m, _ := quests.ParseStepManifest([]byte("0-removed_on_purpose\n"))
	m.Keys = append(m.Keys, toks...)
	missing, _ := m.Diff(toks)
	if len(missing) != 1 || missing[0] != "0-removed_on_purpose" {
		t.Fatalf("guard did not see the removed step: %v", missing)
	}
}
