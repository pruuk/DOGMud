package gamelock

import (
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
	// (sequence derivation unchanged when seed is zero).
	RotationSeed uint64 `yaml:"rotationseed,omitempty"`
}

func (l Lock) IsLocked() bool {

	if l.Difficulty == 0 {
		return false
	}

	if l.UnlockedRound == 0 {
		return true
	}

	// The relock clock runs from the round the lock was opened, not from
	// now: measured from now, the deadline always lies ahead and an opened
	// lock never relocked until a restart.
	gd := gametime.GetDate(l.UnlockedRound)

	if l.RelockInterval == `` {
		return util.GetRoundCount() >= gd.AddPeriod(DefaultRelockTime)
	}

	return util.GetRoundCount() >= gd.AddPeriod(l.RelockInterval)
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
