package textutil

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

func TestTokensCarriesAllFourKeysEvenWhenEmpty(t *testing.T) {
	m := TokenContext{SourceName: "S", SourcePlainName: "Sp"}.Tokens()
	want := map[string]string{"{source}": "S", "{source_plain}": "Sp", "{target}": "", "{target_plain}": ""}
	if len(m) != len(want) {
		t.Fatalf("got %d keys, want %d: %v", len(m), len(want), m)
	}
	for k, v := range want {
		got, ok := m[k]
		if !ok || got != v {
			t.Fatalf("key %q = %q (present %v), want %q", k, got, ok, v)
		}
	}
}

func TestPoolIsNilForEmptyAndOneVariantOtherwise(t *testing.T) {
	if got := Pool(""); got != nil {
		t.Fatalf("Pool(\"\") = %v, want nil", got)
	}
	if got := Pool("  "); len(got) != 1 || got[0] != "  " {
		t.Fatalf("Pool(whitespace) = %v, want the whitespace kept so a validator can refuse it", got)
	}
	if got := Pool("x"); len(got) != 1 || got[0] != "x" {
		t.Fatalf("Pool(x) = %v", got)
	}
}

func TestNarrateSubstitutesEveryRoleFromTheOneVariant(t *testing.T) {
	ctx := TokenContext{SourceName: `<ansi fg="username">Kael</ansi>`, SourcePlainName: "Kael", TargetName: "Goblin", TargetPlainName: "Goblin"}
	roles := Narrate(narration.Variants{
		Actor:    Pool("You hex {target}."),
		Actee:    Pool("{source} hexes you."),
		Observer: Pool("{source_plain}'s hex lands on {target_plain}."),
	}, ctx)
	if roles.Actor != "You hex Goblin." {
		t.Fatalf("actor: %q", roles.Actor)
	}
	if roles.Actee != `<ansi fg="username">Kael</ansi> hexes you.` {
		t.Fatalf("actee: %q", roles.Actee)
	}
	if roles.Observer != "Kael's hex lands on Goblin." {
		t.Fatalf("observer: %q", roles.Observer)
	}
	if roles.ActeeObserver != "" {
		t.Fatalf("acteeObserver should be empty, got %q", roles.ActeeObserver)
	}
}

func TestNarrateRendersNothingForNoVariants(t *testing.T) {
	if roles := Narrate(narration.Variants{}, TokenContext{}); roles != (narration.Roles{}) {
		t.Fatalf("got %+v, want zero Roles", roles)
	}
}

func TestSubstituteTokensAndNarrateAgree(t *testing.T) {
	ctx := TokenContext{SourceName: "A", SourcePlainName: "a", TargetName: "B", TargetPlainName: "b"}
	for _, text := range []string{
		"{source} at {target}; {source_plain}/{target_plain}; {unknown} stays",
		"no tokens",
		"{source}{source}",
	} {
		if got, want := SubstituteTokens(text, ctx), Narrate(narration.Variants{Actor: Pool(text)}, ctx).Actor; got != want {
			t.Fatalf("%q: SubstituteTokens %q != Narrate %q", text, got, want)
		}
	}
}
