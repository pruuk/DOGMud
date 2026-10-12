package items

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
)

// The five defence pools are keyed by the defence's own name, so the YAML
// files under defense-messages/ (dodge.yaml ...) do not move.
func TestDefencePoolForIsTheDefenceName(t *testing.T) {
	for _, d := range combatvocab.Defences() {
		if got := DefencePoolFor(d); string(got) != string(d) {
			t.Errorf("DefencePoolFor(%s) = %q", d, got)
		}
	}
	if got := DefencePoolFor(combatvocab.DefenceNone); got != "" {
		t.Errorf("DefencePoolFor(none) = %q, want empty", got)
	}
}

// The five counter constants are exactly the counter-* files that ship, and
// each is what CounterPoolFor names for its defence. Read from disk so a
// renamed or missing file turns this red.
func TestCounterPoolConstantsAreTheShippedFilesAndTheConversion(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "defense-messages")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	shipped := map[DefencePool]bool{}
	for _, e := range entries {
		if name := e.Name(); strings.HasPrefix(name, "counter-") && strings.HasSuffix(name, ".yaml") {
			shipped[DefencePool(strings.TrimSuffix(name, ".yaml"))] = true
		}
	}
	constants := map[DefencePool]combatvocab.Defence{
		CounterPoolDodge: combatvocab.DefenceDodge, CounterPoolParry: combatvocab.DefenceParry,
		CounterPoolBlock: combatvocab.DefenceBlock, CounterPoolQuell: combatvocab.DefenceQuell,
		CounterPoolDefy: combatvocab.DefenceDefy,
	}
	for pool, d := range constants {
		if !shipped[pool] {
			t.Errorf("constant %q has no shipped file", pool)
		}
		if got := CounterPoolFor(d); got != pool {
			t.Errorf("CounterPoolFor(%s) = %q, want the constant %q", d, got, pool)
		}
	}
	// Variant pools ship beside the per-defence ones but are not a
	// CounterPoolFor result.
	variants := map[DefencePool]bool{CounterPoolDefyWordless: true}
	for pool := range variants {
		if !shipped[pool] {
			t.Errorf("variant constant %q has no shipped file", pool)
		}
	}
	for pool := range shipped {
		if _, ok := constants[pool]; !ok && !variants[pool] {
			t.Errorf("shipped file %q has no constant", pool)
		}
	}
	for _, d := range combatvocab.Defences() {
		if !shipped[CounterPoolFor(d)] {
			t.Errorf("defence %s has no shipped counter pool", d)
		}
	}
}
