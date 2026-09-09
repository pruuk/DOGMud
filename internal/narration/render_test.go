package narration

import (
	"strings"
	"testing"
)

// TestRenderCoordinatesOneIndexAcrossRoles is THE test. The whole reason this
// core exists is that variant N of every role describes the same moment, so a
// renderer that picks per role narrates a different event to each audience.
//
// The probe is a SINGLE SHARED SequencePicker: coordinating yields a0/b0/c0/d0,
// while picking per role advances the shared sequence and yields four DIFFERENT
// indices. The two outcomes are distinguishable by construction, which is what
// makes this test capable of failing rather than merely green.
//
// Verified red against a per-role sabotage on 2026-09-09, which produced
// a1/b2/c3/d0 (offset because the coordinated pick consumes index 0 first).
func TestRenderCoordinatesOneIndexAcrossRoles(t *testing.T) {
	v := Variants{
		Actor:         []string{"a0", "a1", "a2", "a3"},
		Actee:         []string{"b0", "b1", "b2", "b3"},
		Observer:      []string{"c0", "c1", "c2", "c3"},
		ActeeObserver: []string{"d0", "d1", "d2", "d3"},
	}

	got := Render(v, nil, SequencePicker())

	want := Roles{Actor: "a0", Actee: "b0", Observer: "c0", ActeeObserver: "d0"}
	if got != want {
		t.Fatalf("roles came from different indices:\n got %+v\nwant %+v\n(per-role picking would give a0/b1/c2/d3)", got, want)
	}
}

// TestRenderHonoursANonZeroIndex guards the case index 0 cannot see: a
// renderer that ignored the picker entirely and always returned the first
// variant would pass the test above.
func TestRenderHonoursANonZeroIndex(t *testing.T) {
	v := Variants{
		Actor:    []string{"a0", "a1", "a2"},
		Observer: []string{"c0", "c1", "c2"},
	}
	always2 := func(n int) int { return 2 }

	got := Render(v, nil, always2)

	if got.Actor != "a2" || got.Observer != "c2" {
		t.Fatalf("picker index not applied to every role: %+v", got)
	}
}

// TestRenderSkipsEmptyRoles is the Kind B case: buffs, spells, quests and
// crafting hold a single authored string and have no actee, so most roles are
// absent. An absent role must render empty rather than panicking or borrowing
// another role's text.
func TestRenderSkipsEmptyRoles(t *testing.T) {
	v := Variants{Actor: []string{"only the actor"}}

	got := Render(v, nil, SequencePicker())

	if got.Actor != "only the actor" {
		t.Errorf("actor = %q", got.Actor)
	}
	if got.Actee != "" || got.Observer != "" || got.ActeeObserver != "" {
		t.Errorf("absent roles must render empty, got %+v", got)
	}
}

// TestRenderRejectsUnequalRoles: unequal pools cannot be coordinated, because
// index N would name a line in one role and nothing in another. Returning a
// zero Roles matches items.DefenseOptions.RenderTriad's existing behaviour.
func TestRenderRejectsUnequalRoles(t *testing.T) {
	v := Variants{
		Actor:    []string{"a0", "a1", "a2"},
		Observer: []string{"c0", "c1"},
	}

	if got := Render(v, nil, SequencePicker()); got != (Roles{}) {
		t.Fatalf("unequal role pools must render a zero Roles, got %+v", got)
	}
}

// TestRenderNoRolesAtAll pins the other degenerate input.
func TestRenderNoRolesAtAll(t *testing.T) {
	if got := Render(Variants{}, nil, SequencePicker()); got != (Roles{}) {
		t.Fatalf("empty Variants must render a zero Roles, got %+v", got)
	}
}

// TestRenderIndexOverrideStillConsumesAPick is bug-compatibility with
// items.DefenseOptions.RenderTriad, and it is NOT a detail.
//
// That function calls pick(n) FIRST and only then overwrites the index from
// indexOverride, so the draw is discarded rather than skipped. DefaultPicker
// routes through util.Rand, which is GLOBAL engine randomness, so skipping the
// call would consume one fewer random number and shift every subsequent draw
// in the process.
func TestRenderIndexOverrideStillConsumesAPick(t *testing.T) {
	v := Variants{Actor: []string{"a0", "a1", "a2"}}

	calls := 0
	counting := func(n int) int { calls++; return 0 }

	got := Render(v, nil, counting, 2)

	if got.Actor != "a2" {
		t.Errorf("override should select index 2, got %q", got.Actor)
	}
	if calls != 1 {
		t.Errorf("pick must still be called exactly once when overridden, called %d times", calls)
	}
}

// TestRenderOverrideWraps pins RenderTriad's existing modulo behaviour,
// including its handling of a negative override.
func TestRenderOverrideWraps(t *testing.T) {
	v := Variants{Actor: []string{"a0", "a1", "a2"}}

	if got := Render(v, nil, SequencePicker(), 4); got.Actor != "a1" {
		t.Errorf("override 4 of 3 should wrap to index 1, got %q", got.Actor)
	}
	if got := Render(v, nil, SequencePicker(), -1); got.Actor != "a2" {
		t.Errorf("override -1 of 3 should wrap to index 2, got %q", got.Actor)
	}
}

// TestRenderNilPickerIsProduction pins the repo-wide contract that a nil
// Picker means DefaultPicker rather than a panic.
func TestRenderNilPickerIsProduction(t *testing.T) {
	v := Variants{Actor: []string{"only"}}

	if got := Render(v, nil, nil); got.Actor != "only" {
		t.Errorf("nil picker should still render, got %q", got.Actor)
	}
}

// TestRenderSubstitutesTokensInEveryRole guards a role rendered without its
// token pass, which is easy to introduce when roles are separate struct fields.
func TestRenderSubstitutesTokensInEveryRole(t *testing.T) {
	v := Variants{
		Actor:         []string{"you hit {target}"},
		Actee:         []string{"{source} hits you"},
		Observer:      []string{"{source} hits {target}"},
		ActeeObserver: []string{"{target} is hit by {source}"},
	}

	got := Render(v, map[string]string{"{source}": "Alice", "{target}": "Bob"}, SequencePicker())

	for role, text := range map[string]string{
		"actor": got.Actor, "actee": got.Actee,
		"observer": got.Observer, "acteeObserver": got.ActeeObserver,
	} {
		if strings.Contains(text, "{") {
			t.Errorf("%s has an unsubstituted token: %q", role, text)
		}
	}
	if got.Observer != "Alice hits Bob" {
		t.Errorf("observer = %q", got.Observer)
	}
	if got.ActeeObserver != "Bob is hit by Alice" {
		t.Errorf("acteeObserver = %q", got.ActeeObserver)
	}
}

// TestRenderTokenSubstitutionIsSinglePass: a token VALUE that happens to
// contain a token spelling must not be substituted again by a later pass. A
// name is player-supplied in principle, so this is the safe behaviour.
func TestRenderTokenSubstitutionIsSinglePass(t *testing.T) {
	v := Variants{Actor: []string{"{source} waves"}}

	got := Render(v, map[string]string{"{source}": "{target}", "{target}": "Bob"}, SequencePicker())

	if got.Actor != "{target} waves" {
		t.Errorf("substitution must be single pass, got %q", got.Actor)
	}
}

// TestRenderTokenSubstitutionIsDeterministic guards against Go's randomised
// map iteration order leaking into player-visible output. Rendering the same
// input many times must give the same answer every time.
func TestRenderTokenSubstitutionIsDeterministic(t *testing.T) {
	v := Variants{Actor: []string{"{a}{b}{ab}"}}
	tokens := map[string]string{"{a}": "1", "{b}": "2", "{ab}": "3"}

	first := Render(v, tokens, SequencePicker()).Actor
	for range 200 {
		if got := Render(v, tokens, SequencePicker()).Actor; got != first {
			t.Fatalf("token substitution is order-dependent: got %q then %q", first, got)
		}
	}
}

func TestValidateVariants(t *testing.T) {
	ok := Variants{
		Actor:    []string{"a", "b", "c", "d", "e"},
		Observer: []string{"a", "b", "c", "d", "e"},
	}
	if err := ValidateVariants(ok, 5); err != nil {
		t.Fatalf("equal-length pools should validate: %v", err)
	}

	unequal := Variants{
		Actor:    []string{"a", "b", "c", "d", "e"},
		Observer: []string{"a", "b", "c"},
	}
	if err := ValidateVariants(unequal, 5); err == nil {
		t.Error("unequal role pools must not validate")
	}

	short := Variants{Actor: []string{"a", "b"}}
	if err := ValidateVariants(short, 5); err == nil {
		t.Error("pools below the minimum must not validate")
	}

	blank := Variants{Actor: []string{"a", "   ", "c", "d", "e"}}
	if err := ValidateVariants(blank, 5); err == nil {
		t.Error("a whitespace-only variant must not validate")
	}

	if err := ValidateVariants(Variants{}, 0); err == nil {
		t.Error("a Variants with no roles at all must not validate")
	}
}

// TestValidateVariantsNamesTheOffendingRole: a boot panic that does not say
// WHICH role is wrong sends whoever hits it back to counting YAML lines.
func TestValidateVariantsNamesTheOffendingRole(t *testing.T) {
	unequal := Variants{
		Actor:    []string{"a", "b", "c", "d", "e"},
		Observer: []string{"a", "b", "c"},
	}
	err := ValidateVariants(unequal, 5)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "observer") {
		t.Errorf("error should name the offending role, got %v", err)
	}
}

// TestValidateVariantsExpectedRolesCatchesAMissingRole is the answer to a gap a
// blind adversarial review found in the first version of this core: without a
// declared role set, a store with an entirely ABSENT role validated happily and
// then rendered that audience as "" forever, while the other two got real text.
//
// That is exactly the defect this package exists to prevent, so the primitive
// has to be able to express it rather than leaving every store to re-implement
// the check. The old defence code DID re-implement it; a future migrated store
// might not.
func TestValidateVariantsExpectedRolesCatchesAMissingRole(t *testing.T) {
	// A three-role store that lost its actee pool entirely.
	v := Variants{
		Actor:    []string{"a", "b", "c", "d", "e"},
		Observer: []string{"a", "b", "c", "d", "e"},
	}

	// Without a declared role set this passes, which is the gap.
	if err := ValidateVariants(v, 5); err != nil {
		t.Fatalf("undeclared validation should still accept this shape: %v", err)
	}

	// Declaring the three roles turns it into a boot failure.
	err := ValidateVariants(v, 5, RoleActor, RoleActee, RoleObserver)
	if err == nil {
		t.Fatal("a declared-but-absent role must not validate")
	}
	if !strings.Contains(err.Error(), "actee") {
		t.Errorf("error should name the missing role, got %v", err)
	}
}

// TestValidateVariantsExpectedRolesCatchesAnUnexpectedRole guards the other
// direction: a role authored into a store that does not know how to deliver it
// would render text nobody ever sends.
func TestValidateVariantsExpectedRolesCatchesAnUnexpectedRole(t *testing.T) {
	v := Variants{
		Actor:         []string{"a", "b", "c"},
		ActeeObserver: []string{"a", "b", "c"},
	}

	err := ValidateVariants(v, 3, RoleActor)
	if err == nil {
		t.Fatal("a role the store does not expect must not validate")
	}
	if !strings.Contains(err.Error(), "acteeObserver") {
		t.Errorf("error should name the unexpected role, got %v", err)
	}
}

// TestValidateVariantsExpectedRolesAcceptsTheDeclaredShape keeps the happy path
// honest, including the single-role Kind B stores.
func TestValidateVariantsExpectedRolesAcceptsTheDeclaredShape(t *testing.T) {
	triad := Variants{
		Actor:    []string{"a", "b", "c", "d", "e"},
		Actee:    []string{"a", "b", "c", "d", "e"},
		Observer: []string{"a", "b", "c", "d", "e"},
	}
	if err := ValidateVariants(triad, 5, RoleActor, RoleActee, RoleObserver); err != nil {
		t.Errorf("a complete triad must validate: %v", err)
	}

	single := Variants{Actor: []string{"a", "b", "c"}}
	if err := ValidateVariants(single, 3, RoleActor); err != nil {
		t.Errorf("a single-role store must validate: %v", err)
	}
}
