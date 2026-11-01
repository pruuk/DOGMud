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
	CloseControlled
	CloseQuitting
	CloseRoomChange
	CloseFollow // pulled away by a leader; FollowLines speaks instead
	CloseContextLost
	ClosePartnerLeft // the NPC the menu talks with left the room or died
)

var interruptTail = map[CloseReason]string{
	CloseCombat:      " as the fight begins",
	CloseBusy:        " to focus on your work",
	CloseControlled:  " as you are seized",
	CloseAsleep:      " as you fall asleep",
	CloseQuitting:    " to meditate",
	CloseRoomChange:  " as you leave",
	CloseContextLost: "",
}

// InterruptLine is the one line the player reads when an interrupt closes
// their menu, or "" when the close is silent (their own choice, death, a
// follow pull, a typed command closing a menu with no topic).
func InterruptLine(r CloseReason, topic string) string {
	if r == CloseOtherCommand {
		// A typed command closed the menu (owner ruling 2026-10-30, #690):
		// talk menus number lines by position, so name the conversation the
		// command ended. A menu with no topic has none to name.
		if topic == "" {
			return ""
		}
		return "You break off your " + topic + "."
	}
	if r == ClosePartnerLeft {
		// The player broke nothing off: the other side went away.
		if topic == "" {
			return "The conversation ends because they left."
		}
		return "Your " + topic + " ends because they left."
	}
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
