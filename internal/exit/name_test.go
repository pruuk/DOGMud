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
	if err := CheckName("résumé"); err != nil { // 6 letters, 8 bytes
		t.Errorf("CheckName(résumé) = %v, want nil", err)
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

// Owner ruling 18: a player's instance portal is keyed "<zone>-<userId>",
// and its deck button reads the zone.
func TestInstancePortalKeyAndZone(t *testing.T) {
	key := InstancePortalKey("arena", 42)
	if key != "arena-42" {
		t.Fatalf("InstancePortalKey = %q, want arena-42", key)
	}
	if zone, ok := InstancePortalZone(key, TemporaryRoomExit{UserId: 42}); !ok || zone != "arena" {
		t.Fatalf("InstancePortalZone(%q) = %q, %v; want arena, true", key, zone, ok)
	}
	for _, c := range []struct {
		key string
		uid int
	}{
		{"arena-42", 0}, // no opener: the ferry plank, a mob rift, the instance return
		{"arena-42", 7}, // a number that is not the exit's opener
		{"return", 0},
		{"-42", 42}, // no zone
		{"plank", 42},
	} {
		if zone, ok := InstancePortalZone(c.key, TemporaryRoomExit{UserId: c.uid}); ok {
			t.Errorf("InstancePortalZone(%q, uid %d) = %q, true; want false", c.key, c.uid, zone)
		}
	}
}
