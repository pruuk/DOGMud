package spells

import (
	"strings"
	"testing"
)

// The per-round "still folding" line used to reuse the cast_started pool, so a
// multi-fold spell told the player the cast was *beginning* on every round of
// it. cast_continuing exists to say the true thing instead.
func TestGetCastMessage_CastContinuingIsItsOwnPool(t *testing.T) {
	msg := GetCastMessage("cast_continuing", "Conviction Spike")

	if msg == "" {
		t.Fatal("cast_continuing returned an empty message")
	}
	if strings.Contains(msg, "{spell}") {
		t.Errorf("cast_continuing left the {spell} token unsubstituted: %q", msg)
	}
	if !strings.Contains(msg, "Conviction Spike") {
		t.Errorf("cast_continuing did not substitute the spell name: %q", msg)
	}
	if strings.Contains(msg, "Something stirs with") {
		t.Errorf("cast_continuing fell through to the unknown-category fallback: %q", msg)
	}
}

// Every continuing message must read as mid-cast. "begins"/"begin" is the
// giveaway that a start-of-cast line leaked back into the pool.
func TestCastContinuingMessages_DoNotClaimTheCastIsStarting(t *testing.T) {
	cm := loadCastingMessages()

	if len(cm.CastContinuing) == 0 {
		t.Fatal("no cast_continuing messages loaded")
	}

	for _, raw := range cm.CastContinuing {
		lower := strings.ToLower(raw)
		if strings.Contains(lower, "begin") {
			t.Errorf("cast_continuing message reads as a cast START: %q", raw)
		}
		if !strings.Contains(raw, "{spell}") {
			t.Errorf("cast_continuing message has no {spell} token: %q", raw)
		}
	}
}

func TestGetCastMessage_UnknownCategoryStillReturnsSomething(t *testing.T) {
	if got := GetCastMessage("no-such-category", "Foo"); got == "" {
		t.Error("unknown category returned an empty string")
	}
}
