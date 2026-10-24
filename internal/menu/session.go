package menu

// Session is what a caller hands internal/users when it opens a game menu.
type Session struct {
	// Topic names the conversation in interrupt and follow lines
	// ("talk with Kael"). "" gives the generic wording.
	Topic string
	// TopicNames are the names inside Topic, hidden per reader by the
	// sight rules in follow lines.
	TopicNames []string
	OnChoose   func(e Entry)
	OnClose    func(r CloseReason)
	// StillValid re-checks the caller's context before a choice acts
	// (phase 1: the NPC is still here and visible by name). nil means valid.
	StillValid func() bool
	// ShowLast sends the menu after every line already in flight on this
	// event pass, including those queued while it runs (a quest banner and
	// its rewards, printed when the grant is processed), so the menu is the
	// last thing on screen. The menu is open for input at once either way.
	ShowLast bool
	// MobInstanceId is the NPC this menu talks with, or 0. The menu closes,
	// with a line, when that NPC leaves the room or dies.
	MobInstanceId int
}
