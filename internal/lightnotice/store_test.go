package lightnotice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func anyPools(lines ...string) *Pools { return &Pools{Any: lines} }

func validGroup(c Cause) *CauseGroup {
	g := &CauseGroup{Cause: c, Transitions: map[Transition]*Pools{}}
	for _, tr := range Transitions() {
		g.Transitions[tr] = anyPools("The light changes around you.", "Your view of things changes.")
	}
	return g
}

func TestValidateAcceptsACompleteGroup(t *testing.T) {
	if err := validGroup(CauseSky).Validate(); err != nil {
		t.Fatalf("valid group refused: %v", err)
	}
}

func TestValidateRefuses(t *testing.T) {
	cases := map[string]func(g *CauseGroup){
		"unknown cause":        func(g *CauseGroup) { g.Cause = "moonbeam" },
		"missing transition":   func(g *CauseGroup) { delete(g.Transitions, DarkerDark) },
		"unknown transition":   func(g *CauseGroup) { g.Transitions["sideways"] = anyPools("a", "b") },
		"one variant":          func(g *CauseGroup) { g.Transitions[DarkerDark] = anyPools("Only one line here.") },
		"blank line":           func(g *CauseGroup) { g.Transitions[DarkerDark] = anyPools("Fine.", "  ") },
		"over eighty columns":  func(g *CauseGroup) { g.Transitions[DarkerDark] = anyPools("Fine.", strings.Repeat("a", 81)) },
		"a number":             func(g *CauseGroup) { g.Transitions[DarkerDark] = anyPools("Fine.", "It is 3 times darker.") },
		"an em dash":           func(g *CauseGroup) { g.Transitions[DarkerDark] = anyPools("Fine.", "Dark \u2014 very.") },
		"an en dash":           func(g *CauseGroup) { g.Transitions[DarkerDark] = anyPools("Fine.", "Dark \u2013 very.") },
		"a token":              func(g *CauseGroup) { g.Transitions[DarkerDark] = anyPools("Fine.", "{actor} dims.") },
		"any plus split":       func(g *CauseGroup) { g.Transitions[DarkerDark].Outdoor = []string{"a.", "b."} },
		"split missing indoor": func(g *CauseGroup) { g.Transitions[DarkerDark] = &Pools{Outdoor: []string{"a.", "b."}} },
		"empty pools":          func(g *CauseGroup) { g.Transitions[DarkerDark] = &Pools{} },
	}
	for name, mutate := range cases {
		g := validGroup(CauseSky)
		mutate(g)
		if err := g.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
}

func writeStore(t *testing.T, causes []Cause) string {
	t.Helper()
	dir := t.TempDir()
	for _, c := range causes {
		var b strings.Builder
		fmt.Fprintf(&b, "cause: %s\ntransitions:\n", c)
		for _, tr := range Transitions() {
			fmt.Fprintf(&b, "  %s:\n    any:\n      - 'The light changes around you.'\n      - 'Your view of things changes.'\n", tr)
		}
		if err := os.WriteFile(filepath.Join(dir, string(c)+".yaml"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadFromNeedsEveryCause(t *testing.T) {
	t.Cleanup(ResetForTest)
	if err := LoadFrom(writeStore(t, Causes())); err != nil {
		t.Fatalf("complete store refused: %v", err)
	}
	if err := LoadFrom(writeStore(t, Causes()[1:])); err == nil {
		t.Fatal("a store missing a cause file loaded")
	}
}

func TestPoolPicksSetting(t *testing.T) {
	t.Cleanup(ResetForTest)
	g := validGroup(CauseSky)
	g.Transitions[DarkerShapes] = &Pools{Outdoor: []string{"Out one.", "Out two."}, Indoor: []string{"In one.", "In two."}}
	setStoreForTest(map[string]*CauseGroup{string(CauseSky): g})

	if got := Pool(CauseSky, DarkerShapes, false); len(got) != 2 || got[0] != "Out one." {
		t.Errorf("outdoor pool = %v", got)
	}
	if got := Pool(CauseSky, DarkerShapes, true); len(got) != 2 || got[0] != "In one." {
		t.Errorf("indoor pool = %v", got)
	}
	if got := Pool(CauseSky, DarkerDark, true); len(got) != 2 {
		t.Errorf("an any pool serves both settings, got %v", got)
	}
	if got := Pool(CauseLamp, DarkerDark, true); got != nil {
		t.Errorf("absent cause = %v, want nil", got)
	}
}

func TestUnloadedStoreIsSilent(t *testing.T) {
	ResetForTest()
	if _, ok := line(CauseSky, DarkerDark, false, nil); ok {
		t.Fatal("an unloaded store produced a line")
	}
}

// TestShippedStoreLoads is the boot check for the real files.
func TestShippedStoreLoads(t *testing.T) {
	t.Cleanup(ResetForTest)
	if err := LoadFrom(shippedDir); err != nil {
		t.Fatalf("shipped light notices refused: %v", err)
	}
}

// #409: a lantern lit in a cave the player had just walked into said "you can
// make out faces again", though they never saw faces there. A carried light
// coming up is often the first light a player has in that room, so its
// lighter lines never say "again".
func TestCarriedLighterLinesNeverSayAgain(t *testing.T) {
	t.Cleanup(ResetForTest)
	if err := LoadFrom(shippedDir); err != nil {
		t.Fatalf("shipped light notices refused: %v", err)
	}
	for _, tr := range []Transition{LighterShapes, LighterFaces} {
		for _, indoor := range []bool{false, true} {
			lines := Pool(CauseCarried, tr, indoor)
			if len(lines) == 0 {
				t.Fatalf("carried %s indoor=%v: no lines; the walk tested nothing", tr, indoor)
			}
			for _, l := range lines {
				if strings.Contains(strings.ToLower(l), "again") {
					t.Errorf("carried %s line says again: %q", tr, l)
				}
			}
		}
	}
}
