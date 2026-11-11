package characters

import "github.com/GoMudEngine/GoMud/internal/banner"

// progressionNotifyFn delivers a progression line (banner, regen gain, crit or
// fumble lesson) to a player. It is registered from main at boot, because this
// package cannot import messaging or users: messaging imports characters.
// Follows SetUserUntargetableCheck. nil = no delivery, the safe default for
// tests.
//
// It is handed a render function, not text: render(width) lays the line out
// for a reader width columns wide, and only the callback, which holds the
// user, knows that width. The banners use it (plan 3b-3); the other lines
// ignore the width and the pipeline wraps them.
//
// 🔴 A missing registration silences ALL progression text while every unit
// test stays green. The guard internal/guards/progression_notifier_guard_test.go asserts
// main.go registers it.
var progressionNotifyFn func(userId int, render func(width int) string)

// SetProgressionNotifier registers the progression delivery callback.
// Repeated registrations overwrite; pass nil to disable.
func SetProgressionNotifier(fn func(userId int, render func(width int) string)) {
	progressionNotifyFn = fn
}

// notifyProgression sends a line that reads no width. It and
// notifyProgressionBanner are the ONLY ways progression text leaves this
// package, both through notifyProgressionAt. Text carries no trailing
// newline; the pipeline's send adds one. Mobs (userId <= 0) have no client
// and are never notified.
func notifyProgression(userId int, text string) {
	notifyProgressionAt(userId, func(int) string { return text })
}

// notifyProgressionBanner sends the SKILL ADVANCEMENT or STATISTIC INCREASED
// banner, drawn for the reader's width.
func notifyProgressionBanner(userId int, kind banner.Kind, name string, tier *banner.TierChange) {
	notifyProgressionAt(userId, func(width int) string { return banner.Format(kind, name, tier, width) })
}

// notifyProgressionAt sends what render lays out for the reader's width.
func notifyProgressionAt(userId int, render func(width int) string) {
	if userId <= 0 || progressionNotifyFn == nil {
		return
	}
	progressionNotifyFn(userId, render)
}
