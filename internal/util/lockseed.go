package util

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Lock solutions are good only until the server restarts (owner ruling
// 2026-11-18). Every lock's pin sequence comes from lockSeed, made at random
// once per process and never written anywhere, so nobody can work a
// solution out offline and a solution learned before a restart no longer
// opens anything. Server.Seed is not used for locks: it is in the shipped
// config, and it also drives the zodiac calendar, which must not change at
// every restart.
var (
	// lockSeedMu guards lockSeed and bootID.
	lockSeedMu sync.RWMutex
	// lockSeed is this process's secret lock seed.
	lockSeed = RandomHex(32)
	// bootID names this process's lock seed without revealing it. A
	// character's keyring records it beside the solutions it learned, so a
	// later process can tell they are stale (characters.DropStaleLockSolutions).
	bootID = RandomHex(8)
)

// RandomHex returns n bytes from crypto/rand as 2n hex digits: the lock seed
// and boot id here, the touch buttons' per-room ids (internal/buttons) and
// the playtest harness's credentials (internal/playtestprofiles).
// It panics if the system has no randomness, since a guessable secret would
// defeat the point.
func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("util: no system randomness: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// LockSequence is the pin sequence of a lock in this process: GetLockSequence
// with the secret per-boot lock seed. Every lock check, pick and admin view
// uses it; nothing may pass Server.Seed to GetLockSequence (guarded in
// internal/guards).
func LockSequence(lockIdentifier string, difficulty int, rotation uint64) string {
	lockSeedMu.RLock()
	seed := lockSeed
	lockSeedMu.RUnlock()
	return GetLockSequence(lockIdentifier, difficulty, seed, rotation)
}

// BootID names this process's lock seed. It changes at every restart.
func BootID() string {
	lockSeedMu.RLock()
	defer lockSeedMu.RUnlock()
	return bootID
}

// RebootLocksForTest gives the process a new lock seed and boot id, as a
// restart would, and returns a function that puts the old ones back.
func RebootLocksForTest() (restore func()) {
	lockSeedMu.Lock()
	oldSeed, oldBoot := lockSeed, bootID
	lockSeed, bootID = RandomHex(32), RandomHex(8)
	lockSeedMu.Unlock()
	return func() {
		lockSeedMu.Lock()
		lockSeed, bootID = oldSeed, oldBoot
		lockSeedMu.Unlock()
	}
}

// RebootLocksUntilRotatedForTest is RebootLocksUntilDistinctForTest for
// rotations 0 and 1, the first SetLocked.
func RebootLocksUntilRotatedForTest(difficulty int, lockIds ...string) (restore func()) {
	return RebootLocksUntilDistinctForTest(difficulty, lockIds, 0, 1)
}

// RebootLocksUntilDistinctForTest reboots the locks (RebootLocksForTest) until
// each named lock has a different sequence at every one of the given
// rotations. A short lock has few sequences (a 3-pin lock has 8), so a
// rotation can land on the same one by chance; a test of what a rotation
// changes needs it not to. It returns a function that puts the original seed
// and boot id back. Keyring solutions set before the call are stale after it.
// Two equal rotations always give the same sequence, so they panic rather than
// reboot forever.
func RebootLocksUntilDistinctForTest(difficulty int, lockIds []string, rotations ...uint64) (restore func()) {
	asked := map[uint64]bool{}
	for _, r := range rotations {
		if asked[r] {
			panic("util: RebootLocksUntilDistinctForTest given the same rotation twice; no seed can tell them apart")
		}
		asked[r] = true
	}
	restore = RebootLocksForTest()
	for {
		distinct := true
		for _, id := range lockIds {
			seen := map[string]bool{}
			for _, r := range rotations {
				s := LockSequence(id, difficulty, r)
				if seen[s] {
					distinct = false
				}
				seen[s] = true
			}
		}
		if distinct {
			return restore
		}
		RebootLocksForTest()
	}
}
