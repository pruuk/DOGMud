package users

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/presence"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// WakePresence moves an Idle or AFK player back to Active and, when the
// AFK was manual, says so. usercommands.TryCommand calls it for every
// command but `afk` (which sets its own state); NoteActivity calls it for a
// line a game menu consumed.
func (u *UserRecord) WakePresence() {
	if u.Character == nil || u.Character.Presence == nil {
		return
	}
	switch u.Character.Presence.State() {
	case presence.Idle, presence.AFK:
		d, hadData := u.Character.Presence.AFKData()
		_ = u.Character.Presence.TransitionTo(presence.Active,
			state.TransitionReason{Trigger: presence.TriggerInputReceived})
		if hadData && d.Manual {
			u.SendText(messaging.CategorySystem, `<ansi fg="8">You are no longer AFK.</ansi>`)
		}
	}
}

// NoteActivity records real player input that never reaches TryCommand (a
// line a game menu consumed, #375): it stamps the last input round and
// wakes Idle or AFK, so a player working through a menu is not idled out.
func (u *UserRecord) NoteActivity() {
	u.SetLastInputRound(util.GetRoundCount())
	u.WakePresence()
}
