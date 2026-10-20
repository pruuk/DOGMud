package menu

// CloseReason says why an open menu closed.
type CloseReason int

const (
	CloseNone         CloseReason = iota // nothing closed it (gate: no rule holds)
	CloseChosen                          // the player chose an entry
	CloseLeft                            // the player left
	CloseOtherCommand                    // a typed line that was not an answer ran as a command
	CloseReplaced                        // another menu opened
	CloseCombat
	CloseBusy // casting, crafting or salvaging
	CloseDead // dead or respawning
	CloseAsleep
	CloseOffFeet
	CloseControlled
	CloseQuitting
	CloseRoomChange
	CloseFollow // pulled away by a leader; FollowLines speaks instead
	CloseContextLost
)

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

var interruptTail = map[CloseReason]string{
	CloseCombat:      " as the fight begins",
	CloseBusy:        " to focus on your work",
	CloseOffFeet:     " as you go down",
	CloseControlled:  " as you are seized",
	CloseAsleep:      " as you fall asleep",
	CloseQuitting:    " to meditate",
	CloseRoomChange:  " as you leave",
	CloseContextLost: "",
}

// InterruptLine is the one line the player reads when an interrupt closes
// their menu, or "" when the close is silent (their own choice, a command
// whose output speaks for itself, death, a follow pull).
func InterruptLine(r CloseReason, topic string) string {
	tail, ok := interruptTail[r]
	if !ok {
		return ""
	}
	subject := ""
	if topic != "" {
		subject = " your " + topic
	}
	return "You break off" + subject + tail + "."
}

// FollowLines are the three lines when a leader pulls a follower out of a
// menu: to the follower, to the leader, and to the room left behind. Names
// arrive formatted; the caller hides them per reader.
func FollowLines(follower, leader, topic string) (toFollower, toLeader, toRoom string) {
	if topic == "" {
		return "You stop what you were doing to follow " + leader + ".",
			follower + " stops what they were doing to follow you.",
			follower + " stops what they were doing and follows " + leader + "."
	}
	return "You break off your " + topic + " to follow " + leader + ".",
		follower + " breaks off a " + topic + " to follow you.",
		follower + " breaks off a " + topic + " and follows " + leader + "."
}
