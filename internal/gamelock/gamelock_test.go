package gamelock

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func TestLock_SetLockedIncrementsRotationSeed(t *testing.T) {
	l := Lock{Difficulty: 6}
	if l.RotationSeed != 0 {
		t.Fatalf("RotationSeed = %d, want 0 default", l.RotationSeed)
	}
	l.SetLocked()
	if l.RotationSeed != 1 {
		t.Fatalf("RotationSeed after first SetLocked = %d, want 1", l.RotationSeed)
	}
	l.SetUnlocked()
	l.SetLocked()
	if l.RotationSeed != 2 {
		t.Fatalf("RotationSeed after second SetLocked = %d, want 2", l.RotationSeed)
	}
}

func TestLock_RotationSeedDefaultsZero(t *testing.T) {
	var l Lock
	if l.RotationSeed != 0 {
		t.Errorf("zero-value Lock RotationSeed = %d, want 0", l.RotationSeed)
	}
}

// relockRound is the first round at which a lock unlocked at unlockedAt
// should read locked again.
func relockRound(unlockedAt uint64, interval string) uint64 {
	if interval == `` {
		interval = DefaultRelockTime
	}
	return gametime.GetDate(unlockedAt).AddPeriod(interval)
}

func TestLock_RelocksAfterInterval(t *testing.T) {
	defer util.ResetRoundCountForTest()

	// Test binaries load Go defaults, not config.yaml; pin the shipped
	// timing so a game hour is a real span of rounds.
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)

	for _, interval := range []string{``, `2 hours`} {
		start := uint64(util.RoundCountMinimum) + 1000
		util.SetRoundCountForTest(start)

		l := Lock{Difficulty: 6, RelockInterval: interval}
		l.SetUnlocked()

		relock := relockRound(start, interval)
		if relock <= start+1 {
			t.Fatalf("interval %q: relock round %d is not after unlock round %d", interval, relock, start)
		}

		util.SetRoundCountForTest(relock - 1)
		if l.IsLocked() {
			t.Errorf("interval %q: locked one round before the interval ends", interval)
		}

		util.SetRoundCountForTest(relock)
		if !l.IsLocked() {
			t.Errorf("interval %q: still unlocked once the interval has passed", interval)
		}
	}
}
