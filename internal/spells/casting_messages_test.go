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
	castingMessages, castingMessagesLoaded = cm, cm != nil
	t.Cleanup(func() { castingMessages, castingMessagesLoaded = prevStore, prevLoaded })
}

func shippedShape() *CastingMessages {
	return &CastingMessages{
		AlreadyCasting:       []string{"a", "b", "c"},
		CastStarted:          []string{"a", "b", "c"},
		CastContinuing:       []string{"a", "b", "c", "d"},
		ConcentrationSlipped: []string{"a", "b", "c"},
		CastStartedRoom:      []string{"{actor} a.", "{actor} b.", "{actor} c."},
		CastContinuingRoom:   []string{"{actor} a.", "{actor} b.", "{actor} c."},
		CastStartedUnseen:    []string{"a", "b", "c"},
	}
}

// Messaging M6 slice 4, R7: the generic room lines are data, with a fallback
// only when no world is loaded (unit tests), as GetCastMessage has.
func TestCastRoomPoolReadsTheStoreThenFallsBack(t *testing.T) {
	cm := shippedShape()
	cm.CastStartedRoom = []string{"{actor} a.", "{actor} b.", "{actor} c."}
	seedCastingMessages(t, cm)
	if got := CastRoomPool(PhaseCast); len(got) != 3 || got[0] != "{actor} a." {
		t.Fatalf("cast pool: %v", got)
	}
	if got := CastRoomPool(PhaseMagic); got != nil {
		t.Fatalf("magic has no generic line, got %v", got)
	}
}

func TestCastRoomPoolFallbackWhenUnloaded(t *testing.T) {
	seedCastingMessages(t, nil)
	if got := CastRoomPool(PhaseWait); len(got) != 1 || got[0] != "{actor} weaves magic with focused intent." {
		t.Fatalf("wait fallback: %v", got)
	}
	if got := CastRoomPool(PhaseCast); len(got) != 1 || got[0] != "{actor} begins weaving a spell." {
		t.Fatalf("cast fallback: %v", got)
	}
	if CastStartedUnseenLine() == "" {
		t.Fatal("unseen line must never be empty")
	}
}

func TestValidateRejectsAShortUnseenPool(t *testing.T) {
	cm := shippedShape()
	cm.CastStartedUnseen = cm.CastStartedUnseen[:1]
	if err := cm.Validate(); err == nil || !strings.Contains(err.Error(), "cast_started_unseen") {
		t.Fatalf("got %v", err)
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
