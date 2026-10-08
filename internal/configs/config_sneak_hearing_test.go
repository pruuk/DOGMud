package configs

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// Sight gates close-out (#333, owner 2026-10-08): an observer who sees
// nothing gets a hearing roll against a sneak, at SneakHearingMult of its
// detection score. The owner set 0.75: opposed rolls make a lower value a
// severe penalty.

// An absent key reads 0 and must take the default, so test binaries (which
// never load config.yaml) see the shipped tuning.
func TestSneakHearingMult_AbsentTakesTheDefault(t *testing.T) {
	b := Balance{}
	b.Validate()
	if b.SneakHearingMult != 0.75 {
		t.Fatalf("SneakHearingMult = %v after Validate on an empty Balance, want 0.75", b.SneakHearingMult)
	}
}

// Zero, a negative and anything above 1 (the ear beating the eye) are
// rejected like an absent key.
func TestSneakHearingMult_OutOfRangeTakesTheDefault(t *testing.T) {
	for _, v := range []ConfigFloat{-0.5, 0, 1.5} {
		b := Balance{SneakHearingMult: v}
		b.Validate()
		if b.SneakHearingMult != 0.75 {
			t.Errorf("SneakHearingMult %v validated to %v, want the 0.75 default", v, b.SneakHearingMult)
		}
	}
}

// A legal value is kept, so the guard is not simply overwriting everything.
func TestSneakHearingMult_InRangeIsKept(t *testing.T) {
	for _, v := range []ConfigFloat{0.5, 1} {
		b := Balance{SneakHearingMult: v}
		b.Validate()
		if b.SneakHearingMult != v {
			t.Errorf("SneakHearingMult %v validated to %v, want it kept", v, b.SneakHearingMult)
		}
	}
}

// The shipped config.yaml names the key and ships the owner's 0.75. An
// absent key would still read 0.75 through the default, but the owner tunes
// this in config.yaml, so it must be there to tune.
func TestSneakHearingMult_ShippedConfigShips075(t *testing.T) {
	src := shippedConfigSource(t)
	for _, line := range bytes.Split(src, []byte("\n")) {
		trimmed := strings.TrimSpace(string(line))
		if !strings.HasPrefix(trimmed, "SneakHearingMult:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "SneakHearingMult:"))
		if i := strings.Index(val, "#"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		got, err := strconv.ParseFloat(val, 64)
		if err != nil {
			t.Fatalf("SneakHearingMult value %q does not parse: %v", val, err)
		}
		if got != 0.75 {
			t.Fatalf("config.yaml ships SneakHearingMult %v, want 0.75", got)
		}
		return
	}
	t.Fatal("config.yaml does not name SneakHearingMult")
}
