package combat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v2"
)

func newDyingTestChar(health int) *characters.Character {
	c := characters.New()
	c.Name = "Target-Dummy"
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = health
	c.Buffs = buffs.New()
	return c
}

// "Dying" is health-based, not DeathQueued-based: it is what the renderer and
// combat targeting key off, and it is true the instant the lethal blow lands.
func TestIsDying(t *testing.T) {
	if !IsDying(newDyingTestChar(-5)) {
		t.Error("a live character below 1 health is dying")
	}
	if !IsDying(newDyingTestChar(0)) {
		t.Error("a live character at exactly 0 health is dying")
	}
	if IsDying(newDyingTestChar(1)) {
		t.Error("a character at 1 health is not dying")
	}
	if IsDying(nil) {
		t.Error("nil is not dying")
	}
}

// The recursion guard. GetPreAttackMessage falls back to Generic, and before
// U5c that fallback called itself with no base case: an intensity Generic did
// not define looped until the stack overflowed and took the server with it.
// Adding a new intensity is exactly what makes that reachable.
//
// This runs without loaded data on purpose — with the message registry empty
// EVERY lookup takes the fallback path, so it is the strongest possible version
// of this test.
func TestUnknownIntensityOnGenericDoesNotRecurse(t *testing.T) {
	opts := items.GetPreAttackMessage(items.Generic, items.Intensity("no-such-intensity"))

	if len(opts.Together.ToAttacker.Beginner) != 0 {
		t.Error("expected an empty result for an undefined intensity")
	}
}

func TestUnknownSubtypeFallsBackWithoutRecursing(t *testing.T) {
	opts := items.GetPreAttackMessage(items.ItemSubType("no-such-weapon"), items.CoupDeGrace)

	if len(opts.Together.ToAttacker.Beginner) != 0 {
		t.Error("expected an empty result when neither the subtype nor Generic is loaded")
	}
}

// generic.yaml MUST define a coupdegrace block: it is the fallback every weapon
// without its own block lands on. Since the recursion guard above makes a
// missing block degrade to SILENCE rather than a crash, nothing else would
// catch its absence.
//
// This asserts the DATA rather than the loader, deliberately. Loading the real
// registry needs configs plus the whole item tree, which is far too heavy for a
// data-shape check, and the actual risk here is a missing or misindented block.
func TestGenericYamlDefinesCoupDeGrace(t *testing.T) {
	path := filepath.Join(repoRootForTest(t),
		"_datafiles", "world", "dogmud", "combat-messages", "generic.yaml")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generic.yaml: %v", err)
	}

	var parsed struct {
		Options map[string]struct {
			Together map[string]map[string][]string `yaml:"together"`
			Separate map[string]map[string][]string `yaml:"separate"`
		} `yaml:"options"`
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse generic.yaml: %v", err)
	}

	cdg, ok := parsed.Options[string(items.CoupDeGrace)]
	if !ok {
		t.Fatalf("generic.yaml defines no %q block; every weapon would fall back to silence",
			items.CoupDeGrace)
	}

	// Mirror the keys the renderer actually reads, so a block that parses but
	// is missing a branch still fails here rather than rendering nothing.
	for _, key := range []string{"toattacker", "todefender", "toroom"} {
		tiers, ok := cdg.Together[key]
		if !ok {
			t.Errorf("coupdegrace.together is missing %q", key)
			continue
		}
		if len(tiers["beginner"]) == 0 {
			t.Errorf("coupdegrace.together.%s has no beginner messages", key)
		}
	}

	for _, key := range []string{"toattacker", "todefender"} {
		tiers, ok := cdg.Separate[key]
		if !ok {
			t.Errorf("coupdegrace.separate is missing %q", key)
			continue
		}
		if len(tiers["beginner"]) == 0 {
			t.Errorf("coupdegrace.separate.%s has no beginner messages", key)
		}
	}
}

// repoRootForTest walks up to the directory holding go.mod. Test binaries run
// with CWD set to their own package directory.
func repoRootForTest(t *testing.T) string {
	t.Helper()

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		root = filepath.Dir(root)
	}

	t.Fatal("could not locate repo root (no go.mod within 6 levels)")
	return ""
}
