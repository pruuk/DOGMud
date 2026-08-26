package configs

import (
	"testing"

	"gopkg.in/yaml.v2"
)

// ProgressionFailureFraction is the fraction of a full progression event that a
// LOST resolved action awards under the U10b-1 best-of firing convention.
//
// It is the one knob in the balance config whose zero value is BOTH a legal
// explicit setting ("failure teaches nothing", i.e. the pre-U10b-1 behaviour)
// AND what an absent YAML key unmarshals to. That collision is why neither of
// the two idioms used by its neighbours works here:
//
//   - `<= 0` (the common idiom) would silently restore the default whenever an
//     author deliberately set 0, making the off-switch impossible to reach.
//   - `< 0` (the CritProgressionBonus idiom) preserves an explicit 0, but then
//     an ABSENT key also reads as 0 -- so a config.yaml that never mentions the
//     knob would get "off" instead of the intended default.
//
// The fix is a pre-unmarshal sentinel: newUnloadedConfig() seeds -1 before the
// document is decoded, so "still negative after the unmarshal" means "absent"
// and nothing else. These tests pin both halves of that behaviour.

func TestProgressionFailureFraction_SentinelGetsTheDefault(t *testing.T) {
	b := Balance{ProgressionFailureFraction: -1}
	b.Validate()
	if b.ProgressionFailureFraction != 0.35 {
		t.Fatalf("the -1 absent-key sentinel must validate to the 0.35 default, got %v", b.ProgressionFailureFraction)
	}
}

// An explicit 0 is the documented off-switch: "a lost action teaches nothing".
// A `<= 0` guard here would make that configuration unreachable, which is
// exactly the bug this knob's sentinel exists to avoid.
func TestProgressionFailureFraction_ExplicitZeroSurvives(t *testing.T) {
	b := Balance{ProgressionFailureFraction: 0}
	b.Validate()
	if b.ProgressionFailureFraction != 0 {
		t.Fatalf("an explicit 0 is the off-switch and must survive validation, got %v", b.ProgressionFailureFraction)
	}
}

func TestProgressionFailureFraction_LegalValueSurvives(t *testing.T) {
	b := Balance{ProgressionFailureFraction: 0.5}
	b.Validate()
	if b.ProgressionFailureFraction != 0.5 {
		t.Fatalf("a legal fraction must survive validation, got %v", b.ProgressionFailureFraction)
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

// ── BEHAVIOURAL: through the real unmarshal ─────────────────────────────────
//
// The two tests below decode YAML into the SAME seeded struct production uses
// (newUnloadedConfig, called by ReloadConfig) rather than hand-setting the
// field. Hand-setting -1 only proves validateProgression reads a sentinel; it
// does not prove anything ever WRITES one, nor that yaml.v2 leaves an absent
// key untouched instead of zeroing it. Both facts are load-bearing.
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
	cfg := newUnloadedConfig()
	if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
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
	cfg := newUnloadedConfig()
	if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
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
	cfg := newUnloadedConfig()
	if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.Validate()
	if cfg.Balance.ProgressionFailureFraction != 0.2 {
		t.Fatalf("an authored ProgressionFailureFraction must load unchanged, got %v",
			cfg.Balance.ProgressionFailureFraction)
	}
}

// The seeding itself, stated once: newUnloadedConfig must hand back a struct
// whose ProgressionFailureFraction is already negative, or the two behavioural
// tests above are testing a coincidence.
func TestNewUnloadedConfig_SeedsTheAbsenceSentinel(t *testing.T) {
	cfg := newUnloadedConfig()
	if cfg.Balance.ProgressionFailureFraction >= 0 {
		t.Fatalf("newUnloadedConfig must pre-seed a negative absence sentinel, got %v",
			cfg.Balance.ProgressionFailureFraction)
	}
}
