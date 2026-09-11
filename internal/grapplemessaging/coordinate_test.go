package grapplemessaging

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

// TestPickIndexIsCooldownAware pins the behaviour PickTemplate had, expressed
// over indices instead of template strings so a whole triad can share one pick.
//
// Selecting by INDEX is what makes coordination possible at all. Filtering each
// role's own pool by its own cooldowns removes different entries from each,
// so position N no longer means the same moment in all three.
func TestPickIndexIsCooldownAware(t *testing.T) {
	cd := map[string]bool{}

	// A fresh SequencePicker walks 0,1,2, and each pick marks its own cooldown,
	// so three calls must return three DIFFERENT indices.
	seen := map[int]bool{}
	pick := narration.SequencePicker()
	for i := 0; i < 3; i++ {
		idx := PickIndex(3, cd, "k", pick)
		if idx < 0 || idx >= 3 {
			t.Fatalf("index %d out of range", idx)
		}
		if seen[idx] {
			t.Fatalf("index %d repeated while uncooled alternatives remained", idx)
		}
		seen[idx] = true
	}
}

// TestPickIndexResetsWhenAllCooled pins PickTemplate's wrap-around: once every
// variant has been used the map is cleared and selection starts over, rather
// than the event falling silent.
func TestPickIndexResetsWhenAllCooled(t *testing.T) {
	cd := map[string]bool{"k:0": true, "k:1": true, "k:2": true}

	idx := PickIndex(3, cd, "k", narration.SequencePicker())

	if idx < 0 || idx >= 3 {
		t.Fatalf("a fully cooled pool must still yield a usable index, got %d", idx)
	}
}

// TestPickIndexEmptyPool guards the degenerate input.
func TestPickIndexEmptyPool(t *testing.T) {
	if got := PickIndex(0, map[string]bool{}, "k", narration.SequencePicker()); got != -1 {
		t.Fatalf("an empty pool must report -1, got %d", got)
	}
}

// TestPickIndexNilCooldownMap: a caller with no cooldown state must not panic.
func TestPickIndexNilCooldownMap(t *testing.T) {
	if got := PickIndex(3, nil, "k", narration.SequencePicker()); got < 0 || got >= 3 {
		t.Fatalf("nil cooldowns must still yield a usable index, got %d", got)
	}
}

// TestRenderTriadCoordinatesAllThreeRoles is the regression test for the defect
// this change fixes: internal/hooks called PickTemplate once per role, so the
// controller, the controlled and the room each drew an INDEPENDENT index and
// were told three different moments of one grapple exchange.
//
// The authored data pairs by index and does so carefully. In clinch_to_mount,
// variant 1 is "a snap-down sets it up" for the controller and "You feel the
// snap-down TOO LATE" for the controlled: the same instant from two sides. With
// three variants per role, three independent picks agreed one time in nine.
//
// The probe is a SINGLE SHARED SequencePicker: coordinating yields index 0 for
// every role, while picking per role advances the shared sequence and yields
// three different ones. Verified red against a per-role sabotage on 2026-09-09,
// which produced c2/d2/o0.
func TestRenderTriadCoordinatesAllThreeRoles(t *testing.T) {
	tri := TemplateTriad{
		Controller: []string{"c0", "c1", "c2"},
		Controlled: []string{"d0", "d1", "d2"},
		Observers:  []string{"o0", "o1", "o2"},
	}

	got := RenderTriad(tri, "Ctrl", "Cd", map[string]bool{}, "k", narration.SequencePicker())

	if got.Controller != "c0" || got.Controlled != "d0" || got.Observers != "o0" {
		t.Fatalf("roles came from different indices: %+v; each role drew its own index instead of sharing one", got)
	}
}

// TestRenderTriadSubstitutesNamesInEveryRole guards a role rendered without its
// token pass.
func TestRenderTriadSubstitutesNamesInEveryRole(t *testing.T) {
	tri := TemplateTriad{
		Controller: []string{"you take {controlledName}"},
		Controlled: []string{"{controllerName} takes you"},
		Observers:  []string{"{controllerName} takes {controlledName}"},
	}

	got := RenderTriad(tri, "Alice", "Bob", map[string]bool{}, "k", narration.SequencePicker())

	if got.Controller != "you take Bob" {
		t.Errorf("controller = %q", got.Controller)
	}
	if got.Controlled != "Alice takes you" {
		t.Errorf("controlled = %q", got.Controlled)
	}
	if got.Observers != "Alice takes Bob" {
		t.Errorf("observers = %q", got.Observers)
	}
}

// TestRenderTriadUnequalRolesRenderNothing: the authored store is equal-length
// in all 41 keys, so an unequal triad means the data broke. Rendering a partial
// triad would tell some audiences about an event and leave others silent, which
// is the defect this package is being migrated to prevent.
func TestRenderTriadUnequalRolesRenderNothing(t *testing.T) {
	tri := TemplateTriad{
		Controller: []string{"c0", "c1", "c2"},
		Controlled: []string{"d0"},
		Observers:  []string{"o0", "o1", "o2"},
	}

	got := RenderTriad(tri, "Ctrl", "Cd", map[string]bool{}, "k", narration.SequencePicker())

	if got != (RenderedTriad{}) {
		t.Fatalf("an unequal triad must render nothing, got %+v", got)
	}
}

// TestRenderTriadMismatchBurnsNothing is the answer to a blind adversarial
// review finding. The first version picked an index BEFORE checking that the
// roles agreed, so a mismatched triad rendered nothing while still drawing from
// the engine's global randomness and writing a cooldown key. That shifts every
// subsequent draw in the process and strands an entry for an event nobody was
// ever told about.
func TestRenderTriadMismatchBurnsNothing(t *testing.T) {
	tri := TemplateTriad{
		Controller: []string{"c0", "c1", "c2", "c3"},
		Controlled: []string{"d0", "d1", "d2"},
		Observers:  []string{"o0", "o1", "o2"},
	}
	cd := map[string]bool{}
	calls := 0
	counting := func(n int) int { calls++; return 0 }

	got := RenderTriad(tri, "Ctrl", "Cd", cd, "k", counting)

	if got != (RenderedTriad{}) {
		t.Fatalf("a mismatched triad must render nothing, got %+v", got)
	}
	if calls != 0 {
		t.Errorf("a mismatched triad must not consume a pick, consumed %d", calls)
	}
	if len(cd) != 0 {
		t.Errorf("a mismatched triad must not write a cooldown key, wrote %v", cd)
	}
}

// TestRenderGradientMismatchBurnsNothing is the same guard on the gradient
// renderer, which had the identical ordering bug.
func TestRenderGradientMismatchBurnsNothing(t *testing.T) {
	tri := GradientTriad{
		Self:      []string{"s0", "s1"},
		Partner:   []string{"p0"},
		Observers: []string{"o0", "o1"},
	}
	cd := map[string]bool{}
	calls := 0
	counting := func(n int) int { calls++; return 0 }

	got := RenderGradient(tri, "Self", "Partner", cd, "k", counting)

	if got != (RenderedGradient{}) {
		t.Fatalf("a mismatched gradient must render nothing, got %+v", got)
	}
	if calls != 0 || len(cd) != 0 {
		t.Errorf("a mismatched gradient must burn nothing: calls=%d cooldowns=%v", calls, cd)
	}
}

// TestValidateCompletenessRejectsUnequalRoles is what turns the above from a
// silent no-op into a BOOT FAILURE. The validator used to check each role
// against the minimum independently and never that the three agreed, so an
// authoring slip (one extra controller variant, no partner for it) passed boot
// and then stopped narrating that key forever, with nothing logged.
func TestValidateCompletenessRejectsUnequalRoles(t *testing.T) {
	lib, err := Load("../../_datafiles/world/dogmud/messaging/grapple_outcomes.yaml")
	if err != nil {
		t.Fatalf("load production library: %v", err)
	}
	if errs := ValidateCompleteness(lib); len(errs) != 0 {
		t.Fatalf("production library must validate clean, got %v", errs)
	}

	// Break one key the way an author plausibly would.
	key := RequiredAdvancementKeys[0]
	tri := lib.Advancements[key]
	tri.Controller = append(append([]string{}, tri.Controller...), "one extra line with no partner")
	lib.Advancements[key] = tri

	errs := ValidateCompleteness(lib)
	if len(errs) == 0 {
		t.Fatal("a triad whose roles disagree in length must NOT validate")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), key) && strings.Contains(e.Error(), "EQUAL") {
			found = true
		}
	}
	if !found {
		t.Errorf("the error should name the key and say the roles must be equal, got %v", errs)
	}
}
