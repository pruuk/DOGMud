package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func charmDurationTestConfig(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.CharmDurationMinRounds = 30
	cfg.Balance.CharmDurationMaxRounds = 450

	// SetConfigForTest replaces the WHOLE config, and a test binary never loads
	// config.yaml, so anything a charm fixture depends on has to be pinned here
	// or it arrives as zero. Conviction is the one that bites: the pool is
	// ConvictionBase + Cha*perCha + Wil*perWil, and with those at zero
	// ConvictionMax floors to 1, the flat 280 reserve breaches the cap, and
	// every charm is silently REFUSED -- so duration assertions read 0 and look
	// like the margin is broken.
	cfg.Balance.ConvictionBase = 5
	cfg.Balance.ConvictionPerCharisma = 3
	cfg.Balance.ConvictionPerWillpower = 1
	cfg.Balance.PoolReservationCapPct = 0.66
	cfg.Balance.CompanionReserveDefault = 280
	cfg.Balance.CompanionSoftCap = 5

	configs.SetConfigForTest(t, cfg)
}

// A bigger margin must NEVER buy a shorter bond. This is the test that catches
// an inverted margin sign, which contest.Result.Margin's own docs warn compiles
// cleanly and silently puts the outcome on the losing side. Assert the
// DIRECTION, not just the endpoints.
func TestCharmDurationFor_IsMonotonicInMargin(t *testing.T) {
	charmDurationTestConfig(t)

	prev := 0
	for _, m := range []float64{0, 0.1, 0.25, 0.5, 1.0, 1.5, 1.9} {
		got := charmDurationFor(m)
		if got < prev {
			t.Errorf("margin %v gave %d rounds, less than a smaller margin's %d", m, got, prev)
		}
		prev = got
	}
}

func TestCharmDurationFor_ClampsBothEnds(t *testing.T) {
	charmDurationTestConfig(t)

	if got := charmDurationFor(0); got != 30 {
		t.Errorf("margin 0 = %d, want the 30 minimum", got)
	}
	// A floored win reports margin 0 and must take exactly Min: a mercy-granted
	// success is not a dominant one.
	if got := charmDurationFor(-5); got != 30 {
		t.Errorf("negative margin = %d, want the 30 minimum", got)
	}
	if got := charmDurationFor(2.0); got != 450 {
		t.Errorf("two sigma = %d, want the 450 maximum", got)
	}
	if got := charmDurationFor(50); got != 450 {
		t.Errorf("absurd margin = %d, want the 450 maximum", got)
	}
}

// Assert the RELATIONSHIP, not the shipped tuning. Hardcoding 240 here would
// break on any retune while testing nothing about the curve's shape.
func TestCharmDurationFor_MidMarginIsMidRange(t *testing.T) {
	charmDurationTestConfig(t)

	lo, hi := charmDurationFor(0), charmDurationFor(2.0)
	if got, want := charmDurationFor(1.0), lo+(hi-lo)/2; got != want {
		t.Errorf("margin 1.0 = %d, want the midpoint %d", got, want)
	}
}

// CharmPermanent is -1 and means never expires. charmDurationFor must never
// return it, or 0, either of which makes the bond permanent or instant.
func TestCharmDurationFor_NeverReturnsSentinelOrZero(t *testing.T) {
	charmDurationTestConfig(t)

	for _, m := range []float64{-100, -1, 0, 0.001, 1, 100} {
		if got := charmDurationFor(m); got < 1 {
			t.Errorf("margin %v returned %d; must always be >= 1 round", m, got)
		}
	}
}

// A misconfigured pair must not invert the mechanic. The config validator
// collapses Max to Min, and the function guards independently in case it is
// called against a config that never went through validation.
func TestCharmDurationFor_InvertedConfigCollapsesRatherThanInverting(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.CharmDurationMinRounds = 400
	cfg.Balance.CharmDurationMaxRounds = 100
	configs.SetConfigForTest(t, cfg)

	for _, m := range []float64{0, 1, 2, 5} {
		if got := charmDurationFor(m); got != 400 {
			t.Errorf("inverted config at margin %v gave %d, want a flat 400", m, got)
		}
	}
}
