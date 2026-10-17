package weather

import (
	"regexp"
	"strings"
	"testing"
)

// TestFrontLineShowsNoIdOrNumbers pins #561: the player view of a front names
// its type, a strength word and a distance in words, never the front id, raw
// intensity numbers, an em dash or "zone(s)".
func TestFrontLineShowsNoIdOrNumbers(t *testing.T) {
	digits := regexp.MustCompile(`[0-9#]`)
	for hops := 0; hops <= 15; hops++ {
		line := frontLine("storm", "strong", hops)
		if digits.MatchString(line) {
			t.Errorf("hops %d: line shows a number or id: %q", hops, line)
		}
		if strings.Contains(line, "—") || strings.Contains(line, "zone(s)") {
			t.Errorf("hops %d: line has debug punctuation: %q", hops, line)
		}
		if !strings.Contains(line, "strong storm system") {
			t.Errorf("hops %d: line lost the type or strength: %q", hops, line)
		}
	}
	if got := frontLine("fog", "mild", 0); got != "  A mild fog system is right over you." {
		t.Errorf("hops 0: got %q", got)
	}
	if got := frontLine("rain", "mild", 2); got != "  A mild rain system is two regions away." {
		t.Errorf("hops 2: got %q", got)
	}
}

func TestFrontStrengthWordBandsOnThreshold(t *testing.T) {
	if got := frontStrengthWord(0.49, 0.5); got != "mild" {
		t.Errorf("below threshold: got %q, want mild", got)
	}
	if got := frontStrengthWord(0.5, 0.5); got != "strong" {
		t.Errorf("at threshold: got %q, want strong", got)
	}
}
