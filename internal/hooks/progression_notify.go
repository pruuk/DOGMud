package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ProgressionNotifyCallback delivers a progression line through the messaging
// pipeline on CategorySkillProgress, the category quest skill-up lines already
// use. Registered from main via characters.SetProgressionNotifier, because
// characters cannot import messaging or users.
//
// render lays the line out for a reader width columns wide; it is called with
// the user's WrapWidth, since this callback is the one place holding the user
// (the SKILL ADVANCEMENT and STATISTIC INCREASED banners draw their rule to
// it, plan 3b-3). Nothing is measured: the banners go out on a per-round path.
//
// The category's color stage wraps the whole line in skill-progress (alias 179),
// so an untagged banner renders gold: an owner-approved change (M3 item 6 spec).
// A user who is not online gets nothing, as the old raw Message listener did.
func ProgressionNotifyCallback(userId int, render func(width int) string) {
	if user := users.GetByUserId(userId); user != nil {
		user.SendText(messaging.CategorySkillProgress, render(user.WrapWidth()))
	}
}
