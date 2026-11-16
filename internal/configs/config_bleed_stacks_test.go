package configs

import "testing"

// Slice 1b (2026-09-14): a bleed is a stack per landed hit. These knobs set
// each move's stack length and per-round amount. See the "BLEED STACKS" block
// in config.yaml for the equilibrium they are tuned to.

var bleedStackKeys = []string{
	"RakeBleedRounds", "RakeBleedStrengthDivisor", "RakeBleedMin",
	"MaulBleedRounds", "MaulBleedStrengthDivisor", "MaulBleedMin",
	"HamstringBleedRounds", "HamstringBleedStrengthDivisor", "HamstringBleedMin",
	"DrainBleedRounds", "DrainBleedStrengthDivisor", "DrainBleedMin",
	"ThrottleBleedRounds", "ThrottleBleedStrengthDivisor", "ThrottleBleedMin",
}

// bleedStackDefaults is the shipped default for each key in bleedStackKeys,
// in the same order. Shared by the absent-keys and negative-values tests so
// the fifteen literals are typed once.
var bleedStackDefaults = []ConfigInt{
	10, 50, 1, // rake: rounds, strength divisor, min
	12, 35, 1, // maul
	12, 50, 1, // hamstring
	10, 50, 1, // drain
	8, 33, 1, // throttle
}

// bleedStackValues reads the fifteen bleed stack fields off b in the same
// order as bleedStackKeys.
func bleedStackValues(b Balance) []ConfigInt {
	return []ConfigInt{
		b.RakeBleedRounds, b.RakeBleedStrengthDivisor, b.RakeBleedMin,
		b.MaulBleedRounds, b.MaulBleedStrengthDivisor, b.MaulBleedMin,
		b.HamstringBleedRounds, b.HamstringBleedStrengthDivisor, b.HamstringBleedMin,
		b.DrainBleedRounds, b.DrainBleedStrengthDivisor, b.DrainBleedMin,
		b.ThrottleBleedRounds, b.ThrottleBleedStrengthDivisor, b.ThrottleBleedMin,
	}
}

// legalBleedBalance sets every one of the fifteen bleed stack fields to one
// of three uniform values, applied by role (rounds, divisor, min) across all
// five moves.
func legalBleedBalance(rounds, divisor, min ConfigInt) Balance {
	return Balance{
		RakeBleedRounds: rounds, RakeBleedStrengthDivisor: divisor, RakeBleedMin: min,
		MaulBleedRounds: rounds, MaulBleedStrengthDivisor: divisor, MaulBleedMin: min,
		HamstringBleedRounds: rounds, HamstringBleedStrengthDivisor: divisor, HamstringBleedMin: min,
		DrainBleedRounds: rounds, DrainBleedStrengthDivisor: divisor, DrainBleedMin: min,
		ThrottleBleedRounds: rounds, ThrottleBleedStrengthDivisor: divisor, ThrottleBleedMin: min,
	}
}

// An absent key reads 0 and must take the default: a zero stack length never
// ticks, a zero divisor divides by zero, a zero floor lets a stack tick for
// nothing. The defaults equal the shipped values, so test binaries (which
// never load config.yaml) see the shipped tuning.
func TestBleedStackKnobs_AbsentKeysTakeTheDefaults(t *testing.T) {
	b := Balance{}
	b.Validate()
	got := bleedStackValues(b)
	for i, g := range got {
		if g != bleedStackDefaults[i] {
			t.Errorf("%s = %d after Validate on an empty Balance, want default %d", bleedStackKeys[i], g, bleedStackDefaults[i])
		}
	}
}

// A negative value is not absent, but the guard is `< 1`, so it must be
// rejected exactly like an absent key and fall back to the same default.
func TestBleedStackKnobs_NegativeValuesTakeTheDefaults(t *testing.T) {
	b := legalBleedBalance(-1, -1, -1)
	b.Validate()
	got := bleedStackValues(b)
	for i, g := range got {
		if g != bleedStackDefaults[i] {
			t.Errorf("%s = %d after Validate with every knob at -1, want default %d (the guard is `< 1`, so negative must fall back same as absent)", bleedStackKeys[i], g, bleedStackDefaults[i])
		}
	}
}

func TestBleedStackKnobs_LegalValuesSurvive(t *testing.T) {
	cases := []struct {
		rounds, divisor, min ConfigInt
	}{
		{7, 41, 2},
		{1, 1, 1}, // pins the `< 1` boundary: 1 is the smallest legal value and must survive
	}
	for _, c := range cases {
		b := legalBleedBalance(c.rounds, c.divisor, c.min)
		b.Validate()
		want := []ConfigInt{c.rounds, c.divisor, c.min}
		got := bleedStackValues(b)
		for i, g := range got {
			if g != want[i%3] {
				t.Errorf("%s = %d with input (%d,%d,%d), a legal value must survive Validate (want %d)", bleedStackKeys[i], g, c.rounds, c.divisor, c.min, want[i%3])
			}
		}
	}
}

func TestShippedConfigNamesEveryBleedStackKnob(t *testing.T) {
	src := shippedConfigSource(t)
	for _, k := range bleedStackKeys {
		if !bytesContainsKey(src, k+":") {
			t.Errorf("config.yaml must name %s: the shipped tuning is the owner's, not a Go default", k)
		}
	}
}

// The slice 1b targets (spec, "Bleed record and producers"): a stack lasts 2
// to 3 special-move cooldowns, so a lone attacker keeps 2 to 3 alive, and a
// stack's total at Strength 100 is 1.5 to 3 times the single hit it replaced.
// oldHit is that single hit: one trigger of Strength/12 (rake, drain),
// /10 (hamstring, throttle), /8 (maul).
//
// The per-round arithmetic here (Strength / divisor, floored at min) mirrors
// the production formula in actions.bleedPerRound (Task 4); configs cannot
// import internal/actions (actions imports configs), so it is duplicated
// rather than shared, and the two must be kept in step by hand.
//
// This test is tuned against the shipped SpecialMoveCooldown: retuning that
// knob is expected to trip the "cooldowns" assertion below and is not itself
// a sign the bleed knobs are wrong.
//
// Reads config.yaml ON DISK (skip-worktree): the slice adds the same block to
// the disk copy and the committed blob, and CI checks out the blob.
func TestShippedBleedTuningMeetsTheSliceTargets(t *testing.T) {
	cfg, err := loadConfig(shippedConfigSource(t))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	// A misplaced YAML block (indented under the wrong key, or moved outside
	// Balance:) is silently dropped by the decoder rather than erroring, and
	// Validate() below would then paper over it with Go defaults, passing for
	// the wrong reason. Check all fifteen are set BEFORE Validate.
	for i, v := range bleedStackValues(cfg.Balance) {
		if v == 0 {
			t.Errorf("%s is 0 before Validate: config.yaml did not set it (block outside Balance:?)", bleedStackKeys[i])
		}
	}

	cfg.Balance.Validate()
	b := cfg.Balance

	const strength = 100
	cooldown := float64(b.SpecialMoveCooldown)
	rows := []struct {
		move                   string
		rounds, divisor, floor ConfigInt
		oldHit                 int
	}{
		{"rake", b.RakeBleedRounds, b.RakeBleedStrengthDivisor, b.RakeBleedMin, 8},
		{"maul", b.MaulBleedRounds, b.MaulBleedStrengthDivisor, b.MaulBleedMin, 12},
		{"hamstring", b.HamstringBleedRounds, b.HamstringBleedStrengthDivisor, b.HamstringBleedMin, 10},
		{"drain", b.DrainBleedRounds, b.DrainBleedStrengthDivisor, b.DrainBleedMin, 8},
		{"throttle", b.ThrottleBleedRounds, b.ThrottleBleedStrengthDivisor, b.ThrottleBleedMin, 10},
	}
	for _, r := range rows {
		if n := float64(r.rounds) / cooldown; n < 2 || n > 3 {
			t.Errorf("%s: a stack lasts %d rounds = %.2f cooldowns of %v; want 2 to 3 so a lone attacker keeps 2 to 3 stacks", r.move, r.rounds, n, cooldown)
		}
		perRound := max(strength/int(r.divisor), int(r.floor))
		if ratio := float64(int(r.rounds)*perRound) / float64(r.oldHit); ratio < 1.5 || ratio > 3 {
			t.Errorf("%s: a stack totals %d at Strength 100 = %.2f times the old %d hit; want 1.5 to 3", r.move, int(r.rounds)*perRound, ratio, r.oldHit)
		}
	}
}
