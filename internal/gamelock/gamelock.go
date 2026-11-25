package gamelock

import (
	"encoding/binary"
	"hash/fnv"

	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	DefaultRelockTime = `1 hour`
)

type Lock struct {
	Difficulty       uint8  `yaml:"difficulty,omitempty"`            // 0 - no lock. greater than zero = difficulty to unlock.
	UnlockedRound    uint64 `yaml:"-"`                               // What round it was unlocked at, when util.GetRoundCount() > UnlockedUntil, it is relocked (set to zero).
	RelockInterval   string `yaml:"relockinterval,omitempty"`        // How long until it relocks if unlocked?
	TrapConditionIds []int  `yaml:"trapconditionids,omitempty,flow"` // if lockpick is failed, a message is displayed about a trap and these are applied.
	// RotationSeed rotates the lock combination on every SetLocked
	// call. Mixed into util.GetLockSequence so cached keyring entries
	// become invalid after the lock re-locks. Default 0 = back-compat
	// (sequence derivation unchanged when seed is zero). Readers use
	// Rotation, never this field: a relock on the timer rotates too.
	RotationSeed uint64 `yaml:"rotationseed,omitempty"`
}

func (l Lock) IsLocked() bool {

	if l.Difficulty == 0 {
		return false
	}

	if l.UnlockedRound == 0 {
		return true
	}

	return l.timerRelocked()
}

// timerRelocked reports whether the lock was opened and its relock interval
// has since run out. The relock clock runs from the round the lock was
// opened, not from now: measured from now, the deadline always lies ahead
// and an opened lock never relocked until a restart.
func (l Lock) timerRelocked() bool {
	if l.Difficulty == 0 || l.UnlockedRound == 0 {
		return false
	}

	gd := gametime.GetDate(l.UnlockedRound)

	if l.RelockInterval == `` {
		return util.GetRoundCount() >= gd.AddPeriod(DefaultRelockTime)
	}

	return util.GetRoundCount() >= gd.AddPeriod(l.RelockInterval)
}

// Rotation is the rotation the lock's combination is built from now. Every
// relock gets a new combination (owner ruling 2026-11-25), whatever caused
// it. SetLocked bumps RotationSeed. A relock on the timer writes nothing (a
// Lock is copied by value in many places, and IsLocked works the relock out
// lazily), so the rotation is worked out from the lock's state instead: once
// the timer has relocked an opened lock, the rotation mixes RotationSeed with
// the round it was opened, so each open-then-relock cycle gets its own. The
// mixed value has its top bit set, so it never equals a plain RotationSeed
// that SetLocked could reach. A lock still open keeps RotationSeed.
func (l Lock) Rotation() uint64 {
	if !l.timerRelocked() {
		return l.RotationSeed
	}
	h := fnv.New64a()
	var b [16]byte
	binary.LittleEndian.PutUint64(b[:8], l.RotationSeed)
	binary.LittleEndian.PutUint64(b[8:], l.UnlockedRound)
	h.Write(b[:])
	return h.Sum64() | 1<<63
}

// Sequence is the lock's combination now: util.LockSequence at the lock's
// difficulty and current Rotation. Every reader of a combination (a key ring
// check, picklock, the key ring view, the admin views) calls it, so no
// relock path is missed.
func (l Lock) Sequence(lockId string) string {
	return util.LockSequence(lockId, int(l.Difficulty), l.Rotation())
}

func (l *Lock) SetUnlocked() {
	if l.Difficulty > 0 {
		l.UnlockedRound = util.GetRoundCount()
	}
}

func (l *Lock) SetLocked() {
	l.UnlockedRound = 0
	l.RotationSeed++
}
