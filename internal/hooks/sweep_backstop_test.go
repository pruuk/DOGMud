package hooks

import "testing"

// The distinction the whole design rests on. A character that is DYING but was
// never QUEUED reached 0 HP outside ApplyHarm — that is exactly what the
// backstop exists for. A sweep that skips on health instead of DeathQueued
// skips its own purpose and nothing ever dies on the non-harm paths.
func TestSweepReapsDyingButUnqueuedCharacter(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)
	mob.Character.DeathQueued = false

	if !shouldSweepReap(&mob.Character) {
		t.Fatal("sweep skipped a dying, unqueued character — the backstop is dead")
	}
}

// The sweep must not reap a victim whose attributed death is already in flight.
// Die is idempotent, so the attributed event would then be a silent no-op and
// the killer would be lost while everything still looked correct.
func TestSweepSkipsQueuedCharacter(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)
	mob.Character.DeathQueued = true

	if shouldSweepReap(&mob.Character) {
		t.Fatal("sweep pre-empted a queued attributed death; attribution is lost")
	}
}

func TestSweepIgnoresHealthyCharacter(t *testing.T) {
	mob := newRouteDeathTestMob(t, 50)

	if shouldSweepReap(&mob.Character) {
		t.Fatal("sweep reaped a living character")
	}
}

// Exactly zero counts as dying: every death gate in the engine tests <= 0 or
// < 1, and a mob clamped to exactly 0 must still be reaped.
func TestSweepReapsCharacterAtExactlyZero(t *testing.T) {
	mob := newRouteDeathTestMob(t, 0)

	if !shouldSweepReap(&mob.Character) {
		t.Fatal("a character at exactly 0 health was not reaped")
	}
}

func TestSweepIgnoresNilCharacter(t *testing.T) {
	if shouldSweepReap(nil) {
		t.Fatal("nil character reported as reapable")
	}
}
