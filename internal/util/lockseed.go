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
	lockSeed = randomHex(32)
	// bootID names this process's lock seed without revealing it. A
	// character's keyring records it beside the solutions it learned, so a
	// later process can tell they are stale (characters.DropStaleLockSolutions).
	bootID = randomHex(8)
)

// randomHex returns n random bytes as hex. It panics if the system has no
// randomness, since a guessable lock seed would defeat the point.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("util: no system randomness for the lock seed: " + err.Error())
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
	lockSeed, bootID = randomHex(32), randomHex(8)
	lockSeedMu.Unlock()
	return func() {
		lockSeedMu.Lock()
		lockSeed, bootID = oldSeed, oldBoot
		lockSeedMu.Unlock()
	}
}
