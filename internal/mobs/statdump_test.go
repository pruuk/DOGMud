package mobs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// TestDumpMobStats writes every mob template's six effective stat values, plus
// its three pool maxima, to the file named by MOB_STAT_DUMP. It skips unless
// that variable is set, so it costs a CI run nothing.
//
// This is the value-neutrality harness for U10b-0's data moves: check out the
// before tree, run it, check out the after tree, run it, diff. Phase A used it
// to prove the training-to-base fold left all 641 templates identical, and
// Phase C needs the same evidence when the spawn pool moves from Training to
// Base. Keep it until that phase lands.
//
//	MOB_STAT_DUMP=/tmp/before.txt go test ./internal/mobs/ -run TestDumpMobStats
func TestDumpMobStats(t *testing.T) {
	out := os.Getenv("MOB_STAT_DUMP")
	if out == "" {
		t.Skip("set MOB_STAT_DUMP to a file path to run this dump")
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(filepath.Join(cwd, "..", "..")); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	species.LoadDataFiles()
	LoadDataFiles()

	ids := []int{}
	mobsMu.RLock()
	for id := range mobs {
		ids = append(ids, id)
	}
	mobsMu.RUnlock()
	sort.Ints(ids)

	fh, err := os.Create(out)
	if err != nil {
		t.Fatalf("create %s: %v", out, err)
	}
	defer fh.Close()

	for _, id := range ids {
		mobsMu.RLock()
		m := mobs[id]
		mobsMu.RUnlock()
		s := m.Character.Stats
		fmt.Fprintf(fh, "%d\t%s\tstr=%d\tdex=%d\tper=%d\tvit=%d\twil=%d\tcha=%d\thp=%d\tsp=%d\tcp=%d\n",
			id, m.Character.Name,
			s.Strength.Value, s.Dexterity.Value, s.Perception.Value,
			s.Vitality.Value, s.Willpower.Value, s.Charisma.Value,
			m.Character.HealthMax.Value, m.Character.StaminaMax.Value, m.Character.ConvictionMax.Value)
	}
	t.Logf("wrote %d mobs to %s", len(ids), out)
}
