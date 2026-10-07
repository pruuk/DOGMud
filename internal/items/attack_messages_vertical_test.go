package items

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// loadShippedGenericForTest parses the shipped dogmud generic.yaml, the
// fallback block every weapon without its own separate lines lands on.
func loadShippedGenericForTest(t *testing.T) WeaponAttackMessageGroup {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "combat-messages", "generic.yaml"))
	if err != nil {
		t.Fatalf("read generic.yaml: %v", err)
	}
	var g WeaponAttackMessageGroup
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse generic.yaml: %v", err)
	}
	return g
}

// A cross-room blow through a vertical exit used to read "A shot from the up
// finds you" and "prepares to attack towards the up" (#430). Every authored
// separate line of the shipped generic block is rendered for a shot fired up
// (attacker's exit "up", target's entrance "down") and for one fired north
// (exit "north", entrance "south"): the vertical render never says "the up"
// or "the down", and the cardinal one still names its direction.
func TestSeparateRender_VerticalExitsReadAsDirections(t *testing.T) {
	g := loadShippedGenericForTest(t)
	checked := 0
	for intensity, opts := range g.Options {
		v := opts.Separate
		n := len(v.ToAttacker.PoolFor(100))
		for _, pool := range [][]string{v.ToDefender.PoolFor(100), v.ToAttackerRoom.PoolFor(100), v.ToDefenderRoom.PoolFor(100)} {
			if len(pool) > n {
				n = len(pool)
			}
		}
		for i := 0; i < n; i++ {
			pick := func(int) int { return i }
			up := v.Render(100, map[TokenName]string{TokenExitName: "up", TokenEntranceName: "down",
				TokenActor: "Orc", TokenActee: "Bob"}, pick)
			north := v.Render(100, map[TokenName]string{TokenExitName: "north", TokenEntranceName: "south",
				TokenActor: "Orc", TokenActee: "Bob"}, pick)
			for _, line := range []string{up.Actor, up.Actee, up.Observer, up.ActeeObserver} {
				plain := strings.NewReplacer(`<ansi fg="exit">`, "", `</ansi>`, "").Replace(line)
				if strings.Contains(plain, "the up") || strings.Contains(plain, "the down") {
					t.Errorf("%s[%d] still reads as a place: %q", intensity, i, line)
				}
				checked++
			}
			for _, line := range []string{north.Actor, north.Actee, north.Observer, north.ActeeObserver} {
				if strings.Contains(line, "above") || strings.Contains(line, "below") ||
					strings.Contains(line, "upward") || strings.Contains(line, "downward") {
					t.Errorf("%s[%d] cardinal line was rewritten: %q", intensity, i, line)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no separate lines rendered; the probe could not fail")
	}
}

// The coup de grace actee line, rendered exactly, for both directions.
func TestSeparateRender_CoupDeGraceShotFromAboveAndFromTheNorth(t *testing.T) {
	g := loadShippedGenericForTest(t)
	cdg := g.Options[CoupDeGrace].Separate
	first := func(int) int { return 0 }
	up := cdg.Render(0, map[TokenName]string{TokenExitName: "down", TokenEntranceName: "up"}, first)
	if want := "A shot from above finds you as you fall."; up.Actee != want {
		t.Errorf("up: got %q, want %q", up.Actee, want)
	}
	north := cdg.Render(0, map[TokenName]string{TokenExitName: "south", TokenEntranceName: "north"}, first)
	if want := "A shot from the north finds you as you fall."; north.Actee != want {
		t.Errorf("north: got %q, want %q", north.Actee, want)
	}
}
