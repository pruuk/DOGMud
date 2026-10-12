package combat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"gopkg.in/yaml.v2"
)

// seedTauntMessages replaces the package store for one test and returns a
// restore func. In-package so it can touch the unexported registry directly,
// which keeps these tests hermetic: they never read _datafiles, so they do not
// depend on the config data path a test binary cannot resolve.
func seedTauntMessages(t *testing.T, groups map[string]*TauntMessageGroup) {
	t.Helper()
	prev := tauntMessages
	tauntMessages = groups
	t.Cleanup(func() { tauntMessages = prev })
}

// tauntFixture builds a rhetoric group whose variants are position-labelled,
// so a test can tell WHICH index each role was rendered from.
func tauntFixture(n int) map[string]*TauntMessageGroup {
	mk := func(prefix string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = prefix + string(rune('0'+i))
		}
		return out
	}
	opts := map[TauntIntensity]*TauntMessages{}
	for _, band := range []TauntIntensity{TauntHit, TauntMiss, TauntCritical, TauntFumble} {
		opts[band] = &TauntMessages{
			ToAttacker: mk("A"),
			ToDefender: mk("D"),
			ToRoom:     mk("R"),
		}
	}
	return map[string]*TauntMessageGroup{"rhetoric": {OptionId: "rhetoric", Options: opts}}
}

// TestGetTauntTriadUsesOneCoordinatedIndex is the regression test for the
// defect this change fixes: usercommands/taunt.go called GetTauntMessage three
// times, once per perspective, so the three viewpoints of ONE taunt came from
// three unrelated indices and described three different moments.
//
// The probe is a SINGLE SHARED SequencePicker. If the renderer picks once, all
// three roles read index 0 and the fixture returns A0/D0/R0. If it picks three
// times, the shared sequence hands out 0, 1, 2 and the fixture returns
// A0/D1/R2. The two outcomes are distinguishable by construction, which is
// what makes this test capable of failing rather than merely green.
func TestGetTauntTriadUsesOneCoordinatedIndex(t *testing.T) {
	seedTauntMessages(t, tauntFixture(3))

	got := GetTauntTriad(TauntHit, "Source", "Target", "User", "Mob", "Dmg", narration.SequencePicker())

	if got.ToAttacker != "A0" || got.ToDefender != "D0" || got.ToRoom != "R0" {
		t.Fatalf("roles came from different indices: attacker=%q defender=%q room=%q; want A0/D0/R0 (three independent picks would give A0/D1/R2)",
			got.ToAttacker, got.ToDefender, got.ToRoom)
	}
}

// TestGetTauntTriadCoordinatesAtANonZeroIndex guards the case index 0 cannot
// see: a renderer that ignored the picker and always returned the first
// variant would pass the test above.
func TestGetTauntTriadCoordinatesAtANonZeroIndex(t *testing.T) {
	seedTauntMessages(t, tauntFixture(3))

	always2 := func(n int) int { return 2 }
	got := GetTauntTriad(TauntHit, "Source", "Target", "User", "Mob", "Dmg", always2)

	if got.ToAttacker != "A2" || got.ToDefender != "D2" || got.ToRoom != "R2" {
		t.Fatalf("picker index not honoured across all roles: attacker=%q defender=%q room=%q; want A2/D2/R2",
			got.ToAttacker, got.ToDefender, got.ToRoom)
	}
}

// TestGetTauntTriadSubstitutesTokensInEveryRole guards a role being rendered
// without its token pass, which the old per-perspective call could not get
// wrong because every role went through the same function body.
func TestGetTauntTriadSubstitutesTokensInEveryRole(t *testing.T) {
	opts := map[TauntIntensity]*TauntMessages{
		TauntHit: {
			ToAttacker: []string{"atk {actee} {damage}"},
			ToDefender: []string{"def {actor} {damage}"},
			ToRoom:     []string{"room {actor} {actee}"},
		},
	}
	seedTauntMessages(t, map[string]*TauntMessageGroup{"rhetoric": {OptionId: "rhetoric", Options: opts}})

	got := GetTauntTriad(TauntHit, "Alice", "Bob", "User", "Mob", "Wounds", narration.SequencePicker())

	for role, text := range map[string]string{
		"attacker": got.ToAttacker,
		"defender": got.ToDefender,
		"room":     got.ToRoom,
	} {
		if strings.Contains(text, "{") {
			t.Errorf("%s line has an unsubstituted token: %q", role, text)
		}
	}
	if got.ToAttacker != "atk Bob Wounds" {
		t.Errorf("attacker = %q", got.ToAttacker)
	}
	if got.ToDefender != "def Alice Wounds" {
		t.Errorf("defender = %q", got.ToDefender)
	}
	if got.ToRoom != "room Alice Bob" {
		t.Errorf("room = %q", got.ToRoom)
	}
}

// TestGetTauntTriadEmptyWhenStoreUnloaded pins the fallback contract
// usercommands/taunt.go relies on: it detects "no messages loaded" by testing
// the attacker line for emptiness.
func TestGetTauntTriadEmptyWhenStoreUnloaded(t *testing.T) {
	seedTauntMessages(t, map[string]*TauntMessageGroup{})

	got := GetTauntTriad(TauntHit, "Source", "Target", "User", "Mob", "Dmg", narration.SequencePicker())

	if got.ToAttacker != "" || got.ToDefender != "" || got.ToRoom != "" {
		t.Fatalf("want all-empty triad when the store is unloaded, got %+v", got)
	}
}

// TestTauntValidateRejectsUnequalRoleLengths is the validator taunt has never
// had. Defence has enforced equal lengths all along
// (items/defensive_messages.go), and it is what would have caught the shipped
// 8/8/6 gap in the hit and miss bands, where a coordinated index of 6 or 7 had
// no room line to pair with.
func TestTauntValidateRejectsUnequalRoleLengths(t *testing.T) {
	g := &TauntMessageGroup{OptionId: "rhetoric", Options: map[TauntIntensity]*TauntMessages{}}
	for _, band := range []TauntIntensity{TauntHit, TauntMiss, TauntCritical, TauntFumble} {
		g.Options[band] = &TauntMessages{
			ToAttacker: []string{"a", "b", "c", "d", "e"},
			ToDefender: []string{"a", "b", "c", "d", "e"},
			ToRoom:     []string{"a", "b", "c", "d", "e"},
		}
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("equal-length group should validate, got %v", err)
	}

	g.Options[TauntHit].ToRoom = []string{"a", "b", "c"}
	err := g.Validate()
	if err == nil {
		t.Fatal("want an error for a group whose room pool is shorter than its participant pools")
	}
	if !strings.Contains(err.Error(), "hit") {
		t.Errorf("error should name the offending band, got %v", err)
	}
}

// TestTauntValidateRejectsEmptyAndShortPools mirrors the rest of defence's
// contract: no empty strings, and enough variants that a player does not see
// the same line every other taunt.
func TestTauntValidateRejectsEmptyAndShortPools(t *testing.T) {
	build := func(mut func(*TauntMessages)) *TauntMessageGroup {
		g := &TauntMessageGroup{OptionId: "rhetoric", Options: map[TauntIntensity]*TauntMessages{}}
		for _, band := range []TauntIntensity{TauntHit, TauntMiss, TauntCritical, TauntFumble} {
			g.Options[band] = &TauntMessages{
				ToAttacker: []string{"a", "b", "c", "d", "e"},
				ToDefender: []string{"a", "b", "c", "d", "e"},
				ToRoom:     []string{"a", "b", "c", "d", "e"},
			}
		}
		mut(g.Options[TauntHit])
		return g
	}

	short := build(func(m *TauntMessages) {
		m.ToAttacker = []string{"a", "b", "c", "d"}
		m.ToDefender = []string{"a", "b", "c", "d"}
		m.ToRoom = []string{"a", "b", "c", "d"}
	})
	if err := short.Validate(); err == nil {
		t.Error("want an error for pools with fewer than 5 variants")
	}

	blank := build(func(m *TauntMessages) { m.ToRoom[2] = "   " })
	if err := blank.Validate(); err == nil {
		t.Error("want an error for a whitespace-only variant")
	}
}

// TestGetTauntTriadIsAllThreeOrNothing pins the sentinel every caller depends
// on: usercommands/taunt.go reads an empty ToAttacker as "the store said
// nothing" and falls back to its literals, and mobcommands does the same with
// ToRoom.
//
// A blind adversarial review found that the migration onto narration.Render had
// quietly dropped this. The core skips empty roles by design, because a
// single-role store like casting legitimately has them, so a band missing one
// pool rendered a PARTIAL triad that satisfied neither sentinel while narrating
// to some audiences and not others.
//
// Validate makes this unreachable through loaded world data. It is reachable
// through SeedTauntMessagesForTest, which is exactly the kind of bypass a
// future test will use.
func TestGetTauntTriadIsAllThreeOrNothing(t *testing.T) {
	seedTauntMessages(t, map[string]*TauntMessageGroup{"rhetoric": {
		OptionId: "rhetoric",
		Options: map[TauntIntensity]*TauntMessages{
			TauntHit: {
				// No ToAttacker pool at all.
				ToDefender: []string{"D0", "D1", "D2"},
				ToRoom:     []string{"R0", "R1", "R2"},
			},
		},
	}})

	got := GetTauntTriad(TauntHit, "Source", "Target", "username", "mobname", "Dmg", narration.SequencePicker())

	if got != (TauntTriad{}) {
		t.Fatalf("a band missing a role must render NOTHING, not a partial triad; got %+v", got)
	}
}

// loadShippedTauntPool reads one shipped taunt-messages file straight from the
// repo, validates it as the loader would, and returns it.
func loadShippedTauntPool(t *testing.T, name string) *TauntMessageGroup {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "taunt-messages", name+".yaml"))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var g TauntMessageGroup
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if g.OptionId != name {
		t.Fatalf("%s: optionid %q does not match the file name", name, g.OptionId)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("validate %s: %v", name, err)
	}
	return &g
}

// The shipped wordless pool loads and renders all three roles for every band,
// with its tokens filled.
func TestGetTauntTriadFromWordlessRendersEveryBand(t *testing.T) {
	seedTauntMessages(t, map[string]*TauntMessageGroup{
		TauntPoolRhetoric: loadShippedTauntPool(t, TauntPoolRhetoric),
		TauntPoolWordless: loadShippedTauntPool(t, TauntPoolWordless),
	})

	for _, band := range []TauntIntensity{TauntHit, TauntMiss, TauntCritical, TauntFumble} {
		got := GetTauntTriadFrom(TauntPoolWordless, band, "Gruntfang", "Target", "mob", "user", "a stinging insult", narration.SequencePicker())
		if got.ToAttacker == "" || got.ToDefender == "" || got.ToRoom == "" {
			t.Fatalf("band %s: incomplete triad %+v", band, got)
		}
		for _, line := range []string{got.ToAttacker, got.ToDefender, got.ToRoom} {
			if strings.Contains(line, "{") || strings.Contains(line, "}") {
				t.Errorf("band %s: unfilled token in %q", band, line)
			}
		}
		if !strings.Contains(got.ToRoom, "Gruntfang") && band != TauntFumble {
			t.Errorf("band %s: room line does not name the actor: %q", band, got.ToRoom)
		}
	}
}

// A named pool reads that pool, not rhetoric; an unknown pool is the zero
// triad; GetTauntTriad still reads rhetoric.
func TestGetTauntTriadFromSelectsThePool(t *testing.T) {
	seedTauntMessages(t, map[string]*TauntMessageGroup{
		TauntPoolRhetoric: relabel(tauntFixture(3)["rhetoric"], "R-"),
		TauntPoolWordless: relabel(tauntFixture(3)["rhetoric"], "W-"),
	})

	pick := narration.SequencePicker
	if got := GetTauntTriadFrom(TauntPoolWordless, TauntHit, "s", "t", "mob", "user", "d", pick()); !strings.HasPrefix(got.ToAttacker, "W-") {
		t.Errorf("wordless read %q", got.ToAttacker)
	}
	if got := GetTauntTriadFrom(TauntPoolRhetoric, TauntHit, "s", "t", "mob", "user", "d", pick()); !strings.HasPrefix(got.ToAttacker, "R-") {
		t.Errorf("rhetoric read %q", got.ToAttacker)
	}
	if got := GetTauntTriad(TauntHit, "s", "t", "mob", "user", "d", pick()); !strings.HasPrefix(got.ToAttacker, "R-") {
		t.Errorf("GetTauntTriad must stay on rhetoric, read %q", got.ToAttacker)
	}
	if got := GetTauntTriadFrom("no-such-pool", TauntHit, "s", "t", "mob", "user", "d", pick()); got != (TauntTriad{}) {
		t.Errorf("unknown pool must be the zero triad, got %+v", got)
	}
}

// relabel prefixes every line of a group, so a test can tell the pools apart.
func relabel(g *TauntMessageGroup, prefix string) *TauntMessageGroup {
	out := &TauntMessageGroup{OptionId: g.OptionId, Options: map[TauntIntensity]*TauntMessages{}}
	for band, m := range g.Options {
		p := func(in []string) []string {
			r := make([]string, len(in))
			for i, s := range in {
				r[i] = prefix + s
			}
			return r
		}
		out.Options[band] = &TauntMessages{ToAttacker: p(m.ToAttacker), ToDefender: p(m.ToDefender), ToRoom: p(m.ToRoom)}
	}
	return out
}
