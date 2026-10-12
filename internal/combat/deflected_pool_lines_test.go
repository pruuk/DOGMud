package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/items"
)

func deflectedPoolGroup(id items.DefencePool, tag string) *items.DefenseMessageGroup {
	p := &items.DeflectedPool{}
	for _, n := range []string{"0", "1", "2", "3"} {
		p.Actor = append(p.Actor, tag+"-A"+n+" {actee} {damage}")
		p.Actee = append(p.Actee, tag+"-D"+n+" {actor} {damage}")
	}
	return &items.DefenseMessageGroup{OptionId: id, Deflected: p}
}

// With the store loaded, a deflected swing reads from the pool of the defence
// that won, falls back to the generic pool, and both roles share one variant.
func TestDeflectedSwingLines_ReadsPerDefencePool(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(map[items.DefencePool]*items.DefenseMessageGroup{
		items.DefencePoolFor(combatvocab.DefenceDodge): deflectedPoolGroup(items.DefencePoolFor(combatvocab.DefenceDodge), "dodge"),
		items.DefencePoolFor(combatvocab.DefenceParry): deflectedPoolGroup(items.DefencePoolFor(combatvocab.DefenceParry), "parry"),
		items.DefencePoolFor(combatvocab.DefenceBlock): deflectedPoolGroup(items.DefencePoolFor(combatvocab.DefenceBlock), "block"),
		items.DefencePoolDeflected:                     deflectedPoolGroup(items.DefencePoolDeflected, "generic"),
	})
	defer restore()

	cases := []struct {
		defense combatvocab.Defence
		tag     string
	}{
		{combatvocab.DefenceDodge, "dodge"},
		{combatvocab.DefenceParry, "parry"},
		{combatvocab.DefenceBlock, "block"},
		{combatvocab.DefenceNone, "generic"},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			for i := 0; i < 30; i++ {
				atk, def := deflectedSwingLines(tc.defense, "Rurik", "Selka", "light wounds")
				a, d := string(atk), string(def)
				if !strings.HasPrefix(a, tc.tag+"-A") || !strings.HasPrefix(d, tc.tag+"-D") {
					t.Fatalf("lines did not come from the %s pool: %q / %q", tc.tag, a, d)
				}
				if a[len(tc.tag)+2] != d[len(tc.tag)+2] {
					t.Fatalf("roles drew different variants: %q / %q", a, d)
				}
				if !strings.Contains(a, "Selka light wounds") || !strings.Contains(d, "Rurik light wounds") {
					t.Fatalf("tokens not filled by role: %q / %q", a, d)
				}
			}
		})
	}
}

// With nothing loaded the literal sentences stand, so tests and tools that
// never load world data still read the swing.
func TestDeflectedSwingLines_UnloadedStoreKeepsLiteralText(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(nil)
	defer restore()

	atk, def := deflectedSwingLines(combatvocab.DefenceParry, "Rurik", "Selka", "light wounds")
	wantA := `Selka parries your swing, but you still connect! (<ansi fg="damage">light wounds</ansi>)`
	wantD := `You parry Rurik's swing, but it still catches you! (<ansi fg="damage">light wounds</ansi>)`
	if string(atk) != wantA || string(def) != wantD {
		t.Errorf("got %q / %q, want %q / %q", atk, def, wantA, wantD)
	}
}
