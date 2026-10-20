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
}
