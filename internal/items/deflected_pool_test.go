package items

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
)

func deflectedFixture(tag string, n int) *DeflectedPool {
	p := &DeflectedPool{}
	for i := 0; i < n; i++ {
		p.Actor = append(p.Actor, tag+"-atk-"+string(rune('0'+i))+" {actee} {damage}")
		p.Actee = append(p.Actee, tag+"-def-"+string(rune('0'+i))+" {actor} {damage}")
	}
	return p
}

func TestDeflectedPoolValidation(t *testing.T) {
	good := &DefenseMessageGroup{OptionId: DefencePoolDeflected, Deflected: deflectedFixture("g", 4)}
	if err := good.Validate(); err != nil {
		t.Fatalf("a bands-free group with a 4-variant deflected pool was rejected: %v", err)
	}

	short := &DefenseMessageGroup{OptionId: DefencePoolDeflected, Deflected: deflectedFixture("g", 3)}
	if err := short.Validate(); err == nil || !strings.Contains(err.Error(), "at least") {
		t.Errorf("a 3-variant pool must be rejected, got %v", err)
	}

	skew := deflectedFixture("g", 4)
	skew.Actee = skew.Actee[:3]
	if err := (&DefenseMessageGroup{OptionId: DefencePoolDeflected, Deflected: skew}).Validate(); err == nil || !strings.Contains(err.Error(), "equal") {
		t.Errorf("unequal actor and actee lists must be rejected, got %v", err)
	}

	blank := deflectedFixture("g", 4)
	blank.Actee[2] = "  "
	if err := (&DefenseMessageGroup{OptionId: DefencePoolDeflected, Deflected: blank}).Validate(); err == nil {
		t.Error("a blank line must be rejected")
	}

	// A group with bands AND a pool still has its bands checked.
	withBands := validDefenseMessageGroup()
	withBands.Deflected = deflectedFixture("g", 4)
	delete(withBands.Options, Normal)
	if err := withBands.Validate(); err == nil || !strings.Contains(err.Error(), "missing option") {
		t.Errorf("a missing band must still be rejected beside a deflected pool, got %v", err)
	}

	// No bands and no pool is still invalid.
	if err := (&DefenseMessageGroup{OptionId: DefencePoolDeflected}).Validate(); err == nil {
		t.Error("a group with neither bands nor a deflected pool must be rejected")
	}
}

func TestRenderDeflectedSwingPicksPoolPerDefenceAndPairsRoles(t *testing.T) {
	restore := SeedDefenseMessagesForTest(map[DefencePool]*DefenseMessageGroup{
		DefencePoolFor(combatvocab.DefenceDodge): {OptionId: DefencePoolFor(combatvocab.DefenceDodge), Deflected: deflectedFixture("dodge", 4)},
		DefencePoolFor(combatvocab.DefenceParry): {OptionId: DefencePoolFor(combatvocab.DefenceParry), Deflected: deflectedFixture("parry", 4)},
		DefencePoolDeflected:                     {OptionId: DefencePoolDeflected, Deflected: deflectedFixture("generic", 4)},
	})
	defer restore()

	toks := map[TokenName]string{TokenActor: "Rurik", TokenActee: "Selka", TokenDamage: "light wounds"}
	cases := []struct {
		name string
		d    combatvocab.Defence
		want string
	}{
		{"dodge", combatvocab.DefenceDodge, "dodge"},
		{"parry", combatvocab.DefenceParry, "parry"},
		{"block has no pool of its own", combatvocab.DefenceBlock, "generic"},
		{"none", combatvocab.DefenceNone, "generic"},
	}
	for _, tc := range cases {
		for i := 0; i < 4; i++ {
			a, d, ok := RenderDeflectedSwing(tc.d, toks, i)
			if !ok {
				t.Fatalf("%s: not ok", tc.name)
			}
			wantA := tc.want + "-atk-" + string(rune('0'+i)) + " Selka light wounds"
			wantD := tc.want + "-def-" + string(rune('0'+i)) + " Rurik light wounds"
			if string(a) != wantA || string(d) != wantD {
				t.Errorf("%s index %d: got %q / %q, want %q / %q", tc.name, i, a, d, wantA, wantD)
			}
		}
	}

	// Unpicked: both roles still come from one index.
	for n := 0; n < 40; n++ {
		a, d, _ := RenderDeflectedSwing(combatvocab.DefenceDodge, toks)
		if a[len(a)-len(" Selka light wounds")-1] != d[len(d)-len(" Rurik light wounds")-1] {
			t.Fatalf("roles drew different variants: %q / %q", a, d)
		}
	}
}

func TestRenderDeflectedSwingNotOkWhenNothingLoaded(t *testing.T) {
	restore := SeedDefenseMessagesForTest(nil)
	defer restore()
	if _, _, ok := RenderDeflectedSwing(combatvocab.DefenceDodge, nil); ok {
		t.Error("an empty store must report not ok so the caller keeps its literal text")
	}
}

// The shipped files load, validate, and keep variant 0 equal to the sentence
// the game wrote before pools existed.
func TestShippedDeflectedPools(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "defense-messages")
	groups, err := fileloader.LoadAllFlatFiles[DefencePool, *DefenseMessageGroup](dir)
	if err != nil {
		t.Fatalf("load %s: %v", dir, err)
	}
	restore := SeedDefenseMessagesForTest(groups)
	defer restore()

	const dmg = `(<ansi fg="damage">light wounds</ansi>)`
	toks := map[TokenName]string{TokenActor: "Rurik", TokenActee: "Selka", TokenDamage: "light wounds"}
	cases := []struct {
		d         combatvocab.Defence
		atk, defn string
	}{
		{combatvocab.DefenceDodge, "Selka dodges your swing, but you still connect! " + dmg, "You dodge Rurik's swing, but it still catches you! " + dmg},
		{combatvocab.DefenceParry, "Selka parries your swing, but you still connect! " + dmg, "You parry Rurik's swing, but it still catches you! " + dmg},
		{combatvocab.DefenceBlock, "Selka blocks your swing, but you still connect! " + dmg, "You block Rurik's swing, but it still catches you! " + dmg},
		{combatvocab.DefenceNone, "Selka turns your swing aside, but you still connect! " + dmg, "You turn Rurik's swing aside, but it still catches you! " + dmg},
	}
	for _, tc := range cases {
		a, d, ok := RenderDeflectedSwing(tc.d, toks, 0)
		if !ok {
			t.Fatalf("%q: no pool", tc.d)
		}
		if string(a) != tc.atk || string(d) != tc.defn {
			t.Errorf("%q variant 0 changed:\n got %q / %q\nwant %q / %q", tc.d, a, d, tc.atk, tc.defn)
		}
	}
	for _, name := range []DefencePool{"dodge", "parry", "block", DefencePoolDeflected} {
		g := groups[name]
		if g == nil || g.Deflected == nil || len(g.Deflected.Actor) < minDeflectedVariants {
			t.Errorf("%s has no usable deflected pool", name)
		}
	}
}
