package movenarration

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

func TestEventVariantsRoundTrip(t *testing.T) {
	g := &MoveNarrationGroup{
		MoveId: "kick",
		Events: map[EventKey]*EventMessages{
			"standard_hit": {
				Actor:    []string{`You kick {actee} hard! ({damage})`},
				Actee:    []string{`{actor} kicks you hard! ({damage})`},
				Observer: []string{`{actor} kicks {actee}!`},
			},
		},
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	v, ok := g.Variants("standard_hit")
	if !ok {
		t.Fatal(`Variants("standard_hit") not found`)
	}
	if v.Len() != 1 {
		t.Fatalf("Len = %d, want 1", v.Len())
	}
	roles := narration.Render(v, map[string]string{
		narration.TokenActor: `<ansi fg="mobname">Goblin</ansi>`,
		narration.TokenActee: `<ansi fg="username">Kesh</ansi>`,
		TokenDamage:          `<ansi fg="damage">a solid hit</ansi>`,
	}, narration.FirstPicker)
	want := `<ansi fg="mobname">Goblin</ansi> kicks you hard! (<ansi fg="damage">a solid hit</ansi>)`
	if roles.Actee != want {
		t.Errorf("Actee =\n%q\nwant\n%q", roles.Actee, want)
	}
}

func TestValidateRejectsRaggedPools(t *testing.T) {
	g := &MoveNarrationGroup{
		MoveId: "kick",
		Events: map[EventKey]*EventMessages{
			"standard_hit": {
				Actee:    []string{`a`, `b`},
				Observer: []string{`c`},
			},
		},
	}
	if err := g.Validate(); err == nil {
		t.Fatal("Validate accepted ragged pools; it must reject them")
	}
}

func TestVariantsMissingEventReportsNotFound(t *testing.T) {
	g := &MoveNarrationGroup{MoveId: "kick", Events: map[EventKey]*EventMessages{}}
	if _, ok := g.Variants("nope"); ok {
		t.Fatal(`Variants("nope") reported found for an absent event`)
	}
}

func TestValidateRejectsUnknownToken(t *testing.T) {
	g := &MoveNarrationGroup{
		MoveId: "shoot",
		Events: map[EventKey]*EventMessages{
			"hit": {
				Actee: []string{`{actor} shoots {exit_name}!`},
			},
		},
	}
	err := g.Validate()
	if err == nil {
		t.Fatal("Validate accepted an unknown token; nothing downstream would replace it")
	}
	if !strings.Contains(err.Error(), "{exit_name}") {
		t.Errorf("error should name the offending token, got: %v", err)
	}
}

func TestValidateAcceptsEveryShippedToken(t *testing.T) {
	// Proves the guard is not simply rejecting everything: each token this
	// store declares must pass.
	for tok := range allowedTokens {
		g := &MoveNarrationGroup{
			MoveId: "probe",
			Events: map[EventKey]*EventMessages{"hit": {Actee: []string{"x " + tok + " y"}}},
		}
		if err := g.Validate(); err != nil {
			t.Errorf("declared token %s was rejected: %v", tok, err)
		}
	}
}

// Within one role of one event, every variant must carry the same SET of
// value tokens. Variant N of every role describes one moment, and a variant
// that drops {damage} (or {heal}, {position}, ...) silently loses that part
// of the report whenever the picker lands on it. Shoot hit and partial once
// shipped a first variant without {damage}.
func TestShippedVariantsCarryTheSameValueTokens(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "narration", "special-moves")
	prev := loadedMoves
	t.Cleanup(func() { loadedMoves = prev })
	if err := LoadFrom(dir); err != nil {
		t.Fatalf("LoadFrom(%s): %v", dir, err)
	}
	if len(loadedMoves) == 0 {
		t.Fatal("loaded no moves; the check would compare nothing")
	}
	valueTokens := []string{
		TokenDamage, TokenHeal, TokenPosition, TokenWeapon, TokenItem,
		TokenExitName, TokenLabel, TokenWith, TokenVerb,
	}
	// Bash's observer variants are alternative phrasings of the species
	// instrument ("{actor} {verb} {actee} {with}" against "{actor}'s {label}
	// catches {actee}"), not one variant dropping a report, so {label},
	// {with} and {verb} are not compared there.
	phrasing := map[string]bool{TokenLabel: true, TokenWith: true, TokenVerb: true}
	setOf := func(moveId, role, line string) string {
		var have []string
		for _, tok := range valueTokens {
			if moveId == "bash" && role == "observer" && phrasing[tok] {
				continue
			}
			if strings.Contains(line, tok) {
				have = append(have, tok)
			}
		}
		return strings.Join(have, ",")
	}
	compared := 0
	for moveId, g := range loadedMoves {
		for key, ev := range g.Events {
			roles := map[string][]string{
				"actor": ev.Actor, "actee": ev.Actee,
				"observer": ev.Observer, "remote_observer": ev.RemoteObserver,
			}
			for role, lines := range roles {
				if len(lines) < 2 {
					continue
				}
				compared++
				want := setOf(moveId, role, lines[0])
				for i, line := range lines[1:] {
					if got := setOf(moveId, role, line); got != want {
						t.Errorf("%s %s %s: variant %d carries [%s] but variant 0 carries [%s]: %s",
							moveId, key, role, i+1, got, want, line)
					}
				}
			}
		}
	}
	if compared == 0 {
		t.Fatal("compared no multi-variant roles")
	}
}
