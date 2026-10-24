package characters

// progressionNotifyFn delivers a progression line (banner, regen gain, crit or
// fumble lesson) to a player. It is registered from main at boot, because this
// package cannot import messaging or users: messaging imports characters.
// Follows SetUserUntargetableCheck. nil = no delivery, the safe default for
// tests.
//
// 🔴 A missing registration silences ALL progression text while every unit
// test stays green. The guard internal/guards/progression_notifier_guard_test.go asserts
// main.go registers it.
var progressionNotifyFn func(userId int, text string)

// SetProgressionNotifier registers the progression delivery callback.
// Repeated registrations overwrite; pass nil to disable.
func SetProgressionNotifier(fn func(userId int, text string)) {
	progressionNotifyFn = fn
}

// notifyProgression is the ONLY way progression text leaves this package.
// text carries no trailing newline; the pipeline's send adds one. Mobs
// (userId <= 0) have no client and are never notified.
func notifyProgression(userId int, text string) {
	if userId <= 0 || progressionNotifyFn == nil {
		return
	}
	progressionNotifyFn(userId, text)
}
