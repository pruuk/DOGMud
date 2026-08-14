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
//
// For a mob that costs the killer: it stays Dead, so the queued event no-ops.
// For a PLAYER it is a correctness bug and not merely a credit one, because Die
// cascades back to Alive and the queued event then runs the entire death
// cascade a second time. See shouldSweepReap.
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
