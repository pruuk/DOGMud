package darkcombat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func shippedDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "narration", "dark-combat")
}

func TestShippedStoreLoads(t *testing.T) {
	t.Cleanup(UnloadForTest)
	if err := LoadFrom(shippedDir(t)); err != nil {
		t.Fatalf("shipped dark-combat store: %v", err)
	}
	for _, v := range Viewpoints() {
		for _, o := range Outcomes() {
			if n := len(Pool(v, o)); n < MinVariants {
				t.Fatalf("%s/%s holds %d lines, want at least %d", v, o, n, MinVariants)
			}
		}
	}
}

func TestLine_UnloadedIsSilent(t *testing.T) {
	UnloadForTest()
	if _, ok := Line(Attacker, Hit, func(int) int { return 0 }); ok {
		t.Fatal("an unloaded store must say nothing, so callers keep their composed lines")
	}
}

func TestLine_PicksFromThePool(t *testing.T) {
	t.Cleanup(UnloadForTest)
	if err := LoadFrom(shippedDir(t)); err != nil {
		t.Fatal(err)
	}
	got, ok := Line(Defender, Miss, func(int) int { return 0 })
	if !ok || !strings.Contains(got, "You hear something whoosh past!") {
		t.Fatalf("Line(defender, miss, first) = %q, %v", got, ok)
	}
}

// Each validation rule must be able to fire. One bad file per rule.
func TestValidate_Rejects(t *testing.T) {
	good := func() *ViewpointGroup {
		g := &ViewpointGroup{Viewpoint: Attacker, Outcomes: map[Outcome][]string{}}
		for _, o := range Outcomes() {
			g.Outcomes[o] = []string{"You hit a.", "You hit b.", "You hit c.", "You hit d."}
		}
		return g
	}
	if err := good().Validate(); err != nil {
		t.Fatalf("control: a well-formed group failed: %v", err)
	}
	cases := map[string]func(g *ViewpointGroup){
		"unknown viewpoint": func(g *ViewpointGroup) { g.Viewpoint = "watcher" },
		"missing outcome":   func(g *ViewpointGroup) { delete(g.Outcomes, Crit) },
		"unknown outcome":   func(g *ViewpointGroup) { g.Outcomes["parry"] = g.Outcomes[Hit] },
		"too few":           func(g *ViewpointGroup) { g.Outcomes[Hit] = g.Outcomes[Hit][:3] },
		"token":             func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit {actee}." },
		"number":            func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit 3 times." },
		"em dash":           func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit — hard." },
		"spaced hyphen":     func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit - hard." },
		"double hyphen":     func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit -- hard." },
		"too wide":          func(g *ViewpointGroup) { g.Outcomes[Hit][1] = strings.Repeat("a", 81) },
	}
	for name, mutate := range cases {
		g := good()
		mutate(g)
		if err := g.Validate(); err == nil {
			t.Errorf("%s: Validate passed a bad group", name)
		}
	}
}

func TestLoadFrom_MissingViewpointFile(t *testing.T) {
	t.Cleanup(UnloadForTest)
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(shippedDir(t), "attacker.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "attacker.yaml"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadFrom(dir); err == nil {
		t.Fatal("a store with no defender file must fail to load")
	}
}
