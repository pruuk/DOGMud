package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestPowerScoreTable writes PowerScore for every mob whose behavior acts on a
// PowerScore ratio, against three fixture players, with each archetype's own
// decision (#301). Run before and after PowerScore moves onto the shared
// helpers and compare. Opt-in: set DOGMUD_POWERSCORE_TABLE to the output path.
//
// Decisions, from the shipped behavior trees:
//
//	lookout  acts when self/target > 1.0   (lookout.yaml:49-50)
//	thief    acts when self/target > 1.5   (thief.yaml:50-51)
//	predator picks a target when target/self < 0.85 (predator.yaml:44)
func TestPowerScoreTable(t *testing.T) {
	out := os.Getenv("DOGMUD_POWERSCORE_TABLE")
	if out == "" {
		t.Skip("set DOGMUD_POWERSCORE_TABLE=<file> to write the PowerScore table (#301)")
	}
	templates := loadFightWorld(t)

	players := []struct {
		name       string
		stat, rank int
	}{{"novice", 60, 5}, {"journeyman", 100, 20}, {"veteran", 150, 40}}
	playerPower := make([]float64, len(players))
	for i, p := range players {
		playerPower[i] = combat.PowerScore(*fixtureAdventurer(t, p.name, p.stat, p.rank))
	}

	var ids []int
	for id, m := range templates {
		switch m.BehaviorArchetype {
		case "lookout", "thief", "predator":
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	var b strings.Builder
	b.WriteString("| mob | archetype | power |")
	for i, p := range players {
		fmt.Fprintf(&b, " vs %s (%.0f) |", p.name, playerPower[i])
	}
	b.WriteString("\n|---|---|---|---|---|---|\n")
	for _, id := range ids {
		// Spawn rolls its stat pool and any bonus mutation. Seed each mob
		// by its id so the table is reproducible and a row does not depend
		// on its neighbours.
		// The random bonus mutation is still not reproducible (the weighted
		// pool is built by ranging a map), so the table leaves it out: it is
		// the template's power, curated mutations included.
		templates[id].MutationChance = 0
		restoreRand := util.SetRandForTest(int64(id))
		inst := mobs.NewMobByIdFresh(mobs.MobId(id), 1)
		restoreRand()
		if inst == nil {
			t.Fatalf("mob %d failed to spawn", id)
		}
		arch := templates[id].BehaviorArchetype
		power := combat.PowerScore(inst.Character)
		fmt.Fprintf(&b, "| %d %s | %s | %.0f |", id, inst.Character.Name, arch, power)
		for i := range players {
			ratio := power / playerPower[i]
			acts := false
			switch arch {
			case "lookout":
				acts = ratio > 1.0
			case "thief":
				acts = ratio > 1.5
			case "predator":
				acts = 1/ratio < 0.85
			}
			mark := "-"
			if acts {
				mark = "acts"
			}
			fmt.Fprintf(&b, " %.2f %s |", ratio, mark)
		}
		b.WriteString("\n")
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d rows to %s", len(ids), out)
}
