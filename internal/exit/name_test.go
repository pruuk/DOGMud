package exit

import (
	"strings"
	"testing"
)

// Owner ruling 18 (2026-11-26): a named exit is at most MaxNameLetters so it
// fits a 48px deck button; the ten compass words are exempt.
func TestCheckName(t *testing.T) {
	if MaxNameLetters != 6 {
		t.Fatalf("MaxNameLetters = %d, the owner ruled 6", MaxNameLetters)
	}
	for _, ok := range []string{"tower", "ledge", "plank", "rift", "shrine", "portal",
		"north", "northeast", "southwest", "up", "down"} {
		if err := CheckName(ok); err != nil {
			t.Errorf("CheckName(%q) = %v, want nil", ok, err)
		}
	}
	for _, long := range []string{"outlook", "watchtower", "overlook", "gangplank", "dark portal"} {
		err := CheckName(long)
		if err == nil {
			t.Errorf("CheckName(%q) = nil, want a refusal", long)
			continue
		}
		if !strings.Contains(err.Error(), long) {
			t.Errorf("CheckName(%q) error %q does not name the exit", long, err)
		}
	}
}

// Letters, not bytes: a six-letter name with a multibyte letter fits.
func TestCheckName_CountsLettersNotBytes(t *testing.T) {
	if err := CheckName("caféé"); err != nil {
		t.Errorf("CheckName(caféé) = %v, want nil", err)
	}
}

func TestIsCompassWord(t *testing.T) {
	for _, w := range []string{"north", "south", "east", "west", "up", "down",
		"northeast", "northwest", "southeast", "southwest"} {
		if !IsCompassWord(w) {
			t.Errorf("IsCompassWord(%q) = false", w)
		}
	}
	for _, w := range []string{"shrine", "n", "North", "out"} {
		if IsCompassWord(w) {
			t.Errorf("IsCompassWord(%q) = true", w)
		}
	}
}
