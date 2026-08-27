package configs

import "testing"

// ProgressionFailureFraction is the fraction of a full progression event that a
// LOST resolved action awards. It is the one balance knob whose zero value is
// both a legal explicit setting and what an absent YAML key decodes to, so it
// is defaulted by a pre-unmarshal sentinel rather than by a guard predicate.
// The reasoning lives on newUnloadedConfig in configs.go.

func TestProgressionFailureFraction_SentinelGetsTheDefault(t *testing.T) {
	b := Balance{ProgressionFailureFraction: -1}
	b.Validate()
	if b.ProgressionFailureFraction != 0.35 {
		t.Fatalf("the -1 absent-key sentinel must validate to the 0.35 default, got %v", b.ProgressionFailureFraction)
	}
}

// An explicit 0 is the documented off-switch: "a lost action teaches nothing".
// A `<= 0` guard here would make that configuration unreachable, which is
// exactly the bug the sentinel exists to avoid.
func TestProgressionFailureFraction_ExplicitZeroSurvives(t *testing.T) {
	b := Balance{ProgressionFailureFraction: 0}
	b.Validate()
	if b.ProgressionFailureFraction != 0 {
		t.Fatalf("an explicit 0 is the off-switch and must survive validation, got %v", b.ProgressionFailureFraction)
	}
}

// Above 1.0 would make failing worth MORE than succeeding, inverting the whole
// point of the convention. Fall back to the default rather than clamp to 1.0:
// a fraction of exactly 1.0 (failure is worth as much as success) is almost
// certainly not what the author meant either.
func TestProgressionFailureFraction_AboveOneIsRejected(t *testing.T) {
	b := Balance{ProgressionFailureFraction: 1.5}
	b.Validate()
	if b.ProgressionFailureFraction != 0.35 {
		t.Fatalf("a fraction above 1.0 must fall back to the 0.35 default, got %v", b.ProgressionFailureFraction)
	}
}

// ── BEHAVIOURAL: through the real load path ─────────────────────────────────
//
// These go through loadConfig, the function ReloadConfig itself calls, so the
// WIRING is under test and not just the guard. Hand-seeding a struct would
// prove validateProgression reads a sentinel without proving anything ever
// writes one; asserting against the shipped config.yaml (which now names the
// key) would be a gate that cannot fail.
//
// They come as a pair on purpose: without the explicit-zero half, an
// implementation that simply always defaulted would still pass the absent-key
// half.
//
// Config.Validate() is safe to call in a test binary: every subsection
// validator only clamps its own fields, and none of them touch the filesystem
// or panic (verified by grepping internal/configs/config.*.go for panic( and
// os. -- no hits).

func TestProgressionFailureFraction_AbsentKeyLoadsTheDefault(t *testing.T) {
	const doc = `
Balance:
  CritProgressionBonus: 2.0
  ObservedCritProgressionBonus: 0.5
`
	cfg, err := loadConfig([]byte(doc))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.ProgressionFailureFraction != 0.35 {
		t.Fatalf("a config omitting ProgressionFailureFraction must load the 0.35 default, got %v",
			cfg.Balance.ProgressionFailureFraction)
	}
}

func TestProgressionFailureFraction_ExplicitZeroLoadsAsZero(t *testing.T) {
	const doc = `
Balance:
  CritProgressionBonus: 2.0
  ProgressionFailureFraction: 0
`
	cfg, err := loadConfig([]byte(doc))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.ProgressionFailureFraction != 0 {
		t.Fatalf("an explicitly-zero ProgressionFailureFraction must load as 0, got %v",
			cfg.Balance.ProgressionFailureFraction)
	}
}

// A non-zero authored value must round-trip through the real load path too.
func TestProgressionFailureFraction_ExplicitValueLoadsAsAuthored(t *testing.T) {
	const doc = `
Balance:
  ProgressionFailureFraction: 0.2
`
	cfg, err := loadConfig([]byte(doc))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.ProgressionFailureFraction != 0.2 {
		t.Fatalf("an authored ProgressionFailureFraction must load unchanged, got %v",
			cfg.Balance.ProgressionFailureFraction)
	}
}

// The seeding itself, stated once: newUnloadedConfig must hand back a struct
// whose ProgressionFailureFraction is already negative, or the behavioural
// tests above are testing a coincidence.
func TestNewUnloadedConfig_SeedsTheAbsenceSentinel(t *testing.T) {
	cfg := newUnloadedConfig()
	if cfg.Balance.ProgressionFailureFraction >= 0 {
		t.Fatalf("newUnloadedConfig must pre-seed a negative absence sentinel, got %v",
			cfg.Balance.ProgressionFailureFraction)
	}
}
