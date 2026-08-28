package configs

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// shippedConfigSource reads _datafiles/config.yaml as TEXT, anchored on this
// file's own location -- test binaries do not reliably start in the package
// dir, and they never load config.yaml through the normal path either.
//
// Note this reads the file ON DISK, which carries skip-worktree and so may
// differ from the committed blob. That is fine for asserting a key is NAMED
// (both have it); do not use it to assert a key's VALUE.
func shippedConfigSource(t *testing.T) []byte {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "_datafiles", "config.yaml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return src
}

// bytesContainsKey looks for a YAML key at the start of a line, ignoring
// indentation, so a mention inside a comment does not count as naming it.
func bytesContainsKey(src []byte, key string) bool {
	for _, line := range bytes.Split(src, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte(key)) {
			return true
		}
	}
	return false
}

// OWNER RULING 2026-08-29: "yes a fumble should out-pay a win. You learn from
// your mistakes unless you're a fool."
//
// This pins the ruling as a RELATIONSHIP between two knobs rather than as a
// pair of magic numbers, because that is the form the ruling actually takes and
// the form a retune can silently break.
//
// What an actor collects from one resolved action, in units of a full ordinary
// progression event (see internal/progression/event.go):
//
//	plain win       1.0                                    (ordinary, tracked)
//	plain loss      ProgressionFailureFraction             (ordinary, tracked)
//	FUMBLE          ProgressionFailureFraction + CritProgressionBonus
//	                                       (ordinary + a bonus roll, untracked)
//
// So the ruling holds exactly while
// ProgressionFailureFraction + CritProgressionBonus > 1.0, and at the shipped
// 0.35 + 2.0 a fumble pays 2.35x a plain win.
//
// ⚠️ THIS IS NOT THE SAME CLAIM as "failing beats succeeding", which
// validateProgression deliberately rejects by capping ProgressionFailureFraction
// at 1.0 (see config_progressionfailure_test.go). An ORDINARY loss must stay
// worth less than a win. A FUMBLE is the ~2.3% critical-failure tail and earns a
// separate bonus roll on top, and it is only that tail the ruling is about.
// Do not "fix" one by changing the other.
//
// The fumble bonus also does not TRACK (progression.ClassFumble is in the
// do-not-track set in applyBonusProgression), so it does not advance the
// virtual rank that decays later chances. A fumble therefore pays more than a
// win AND costs less rank than a win. That is deliberate.
func TestFumbleOutPaysAWin_AtTheShippedTuning(t *testing.T) {
	const doc = `
Balance:
  ProgressionFailureFraction: 0.35
  CritProgressionBonus: 2.0
  ObservedCritProgressionBonus: 0.5
`
	cfg, err := loadConfig([]byte(doc))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()

	win := 1.0
	fumble := float64(cfg.Balance.ProgressionFailureFraction) + float64(cfg.Balance.CritProgressionBonus)

	if fumble <= win {
		t.Fatalf("owner ruling: a fumble must out-pay a win. fumble=%v (ProgressionFailureFraction %v + CritProgressionBonus %v), win=%v",
			fumble, cfg.Balance.ProgressionFailureFraction, cfg.Balance.CritProgressionBonus, win)
	}
}

// 🔴 THE RULING DEPENDS ON config.yaml NAMING THE KEY. It is not carried by the
// Go defaults, and this test exists to state that out loud rather than let the
// next reader assume it.
//
// CritProgressionBonus is guarded with `< 0`, not `<= 0`, so that an explicit 0
// stays a usable off-switch. The documented cost is that an ABSENT key also
// reads as 0 -- so a config.yaml omitting CritProgressionBonus silently pays
// NOTHING for crits and fumbles alike, and the ruling above quietly stops
// holding. (Both crit knobs were in exactly that inert state until 81061c6b4,
// 2026-08-19.) The field comment on CritProgressionBonus says "default 2.0",
// which is true only of an explicitly negative value, never of an absent one.
//
// ProgressionFailureFraction solved the same problem with a -1 absence sentinel
// seeded in newUnloadedConfig; CritProgressionBonus did not get that treatment.
// Whether it should is a config.yaml-audit question, not this slice's.
func TestCritProgressionBonus_AbsentKeyIsInert_SoTheRulingNeedsTheShippedKey(t *testing.T) {
	cfg, err := loadConfig([]byte("Balance:\n  RollSpread: 0.15\n"))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()

	if float64(cfg.Balance.CritProgressionBonus) != 0 {
		t.Fatalf("an absent CritProgressionBonus is documented to read as 0 (the `< 0` guard idiom); got %v -- if this changed, the ruling is now carried by the defaults and the comments here need updating",
			cfg.Balance.CritProgressionBonus)
	}

	fumble := float64(cfg.Balance.ProgressionFailureFraction) + float64(cfg.Balance.CritProgressionBonus)
	if fumble > 1.0 {
		t.Fatalf("premise broken: with no crit bonus a fumble cannot out-pay a win, got %v", fumble)
	}
}

// The shipped config.yaml must therefore actually name the key. This is the
// assertion that would catch someone deleting the line.
func TestShippedConfigNamesTheCritProgressionBonus(t *testing.T) {
	src := shippedConfigSource(t)
	if !bytesContainsKey(src, "CritProgressionBonus:") {
		t.Fatal("config.yaml must name CritProgressionBonus explicitly: an absent key reads as 0 and silently disables the crit AND fumble progression bonus, breaking the owner ruling that a fumble out-pays a win")
	}
}

// Proof the invariant is a real gate and not arithmetic that cannot fail: a
// legal, validator-accepted tuning exists that BREAKS the ruling. Both values
// below survive Validate() (0 is the documented off-switch for the bonus, and
// 0.35 is the default fraction), so nothing but this test would catch it.
func TestFumbleOutPaysAWin_IsAGateThatCanFail(t *testing.T) {
	const doc = `
Balance:
  ProgressionFailureFraction: 0.35
  CritProgressionBonus: 0
`
	cfg, err := loadConfig([]byte(doc))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()

	if float64(cfg.Balance.CritProgressionBonus) != 0 {
		t.Fatalf("fixture premise: CritProgressionBonus 0 is the documented off-switch and must survive validation, got %v",
			cfg.Balance.CritProgressionBonus)
	}
	fumble := float64(cfg.Balance.ProgressionFailureFraction) + float64(cfg.Balance.CritProgressionBonus)
	if fumble > 1.0 {
		t.Fatalf("fixture premise broken: this tuning is supposed to VIOLATE the ruling, got fumble=%v", fumble)
	}
}
