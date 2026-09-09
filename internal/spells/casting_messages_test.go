package spells

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

// seedCastingMessages installs a store for one test without touching the
// sync.Once, and restores the previous one afterwards.
func seedCastingMessages(t *testing.T, cm *CastingMessages) {
	t.Helper()
	prevStore, prevLoaded := castingMessages, castingMessagesLoaded
	castingMessages, castingMessagesLoaded = cm, true
	t.Cleanup(func() { castingMessages, castingMessagesLoaded = prevStore, prevLoaded })
}

func shippedShape() *CastingMessages {
	return &CastingMessages{
		AlreadyCasting:       []string{"a", "b", "c"},
		CastStarted:          []string{"a", "b", "c"},
		CastContinuing:       []string{"a", "b", "c", "d"},
		ConcentrationSlipped: []string{"a", "b", "c"},
	}
}

// TestValidateAcceptsTheShippedShape is the reason the minimum is 3 and not
// defence's 5: casting-messages.yaml ships 3/3/3/4, so defence's number would
// fail boot on shipped data without improving a single line of text.
func TestValidateAcceptsTheShippedShape(t *testing.T) {
	if err := shippedShape().Validate(); err != nil {
		t.Fatalf("the shipped shape (3/3/3/4) must validate: %v", err)
	}
}

func TestValidateRejectsShortPool(t *testing.T) {
	cm := shippedShape()
	cm.AlreadyCasting = []string{"a", "b"}

	err := cm.Validate()
	if err == nil {
		t.Fatal("a pool below the minimum must not validate")
	}
	if !strings.Contains(err.Error(), "already_casting") {
		t.Errorf("error should name the offending pool, got %v", err)
	}
}

func TestValidateRejectsMissingPool(t *testing.T) {
	cm := shippedShape()
	cm.CastStarted = nil

	err := cm.Validate()
	if err == nil {
		t.Fatal("a missing pool must not validate")
	}
	if !strings.Contains(err.Error(), "cast_started") {
		t.Errorf("error should name the offending pool, got %v", err)
	}
}

func TestValidateRejectsBlankVariant(t *testing.T) {
	cm := shippedShape()
	cm.ConcentrationSlipped = []string{"a", "   ", "c"}

	if err := cm.Validate(); err == nil {
		t.Fatal("a whitespace-only variant must not validate")
	}
}

func TestGetCastMessageRendersThroughTheCore(t *testing.T) {
	seedCastingMessages(t, &CastingMessages{
		AlreadyCasting:       []string{"already {spell}"},
		CastStarted:          []string{"started {spell} 0", "started {spell} 1"},
		CastContinuing:       []string{"continuing {spell}"},
		ConcentrationSlipped: []string{"slipped {spell}"},
	})

	if got := GetCastMessage("cast_started", "Firebolt", narration.SequencePicker()); got != "started Firebolt 0" {
		t.Errorf("cast_started = %q", got)
	}
	always1 := func(n int) int { return 1 }
	if got := GetCastMessage("cast_started", "Firebolt", always1); got != "started Firebolt 1" {
		t.Errorf("cast_started at index 1 = %q", got)
	}
}

// TestGetCastMessageSubstitutesTheSpellToken pins that {spell} is replaced with
// the DISPLAY name. Passing a spellid here would leak an internal identifier
// into player output, which is what the round loop used to do.
func TestGetCastMessageSubstitutesTheSpellToken(t *testing.T) {
	seedCastingMessages(t, shippedShapeWithToken())

	got := GetCastMessage("cast_started", "Chrysalis Cocoon", narration.SequencePicker())
	if strings.Contains(got, "{spell}") {
		t.Errorf("token left unsubstituted: %q", got)
	}
	if !strings.Contains(got, "Chrysalis Cocoon") {
		t.Errorf("display name missing: %q", got)
	}
}

func shippedShapeWithToken() *CastingMessages {
	cm := shippedShape()
	cm.CastStarted = []string{"you begin {spell}", "b", "c"}
	return cm
}

// TestGetCastMessageUnknownCategoryFallsBack pins the ONE fallback that
// survives: an unrecognised category still returns a sentence rather than an
// empty string, because a caller printing "" would show the player nothing.
func TestGetCastMessageUnknownCategoryFallsBack(t *testing.T) {
	seedCastingMessages(t, shippedShape())

	got := GetCastMessage("bogus-category", "Firebolt", narration.SequencePicker())
	if got != "Something stirs with Firebolt." {
		t.Errorf("unknown category should keep its literal fallback sentence, got %q", got)
	}
}
