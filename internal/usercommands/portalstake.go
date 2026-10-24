package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Registered here rather than in the userCommands literal: PortalStake
// reads userCommands (isKnownCommandLine), so a literal entry would be an
// initialization cycle. Not admin; allowed in combat, as ask is (the topic
// that opens the prompt); refused while downed, like ask.
func init() {
	userCommands[behaviortree.PortalStakeCommand] = CommandAccess{PortalStake, false, true, false}
}

// PortalStake is the hidden command behind the instance-zone amount prompt
// (#375 R14; behaviortree/portal_stake.go). The prompt re-runs it with each
// answer. It is not for typing: with no stake prompt open it reports
// unhandled, so the player sees the usual "not recognized" line.
//
// A free-text prompt takes any typed line as its answer (#641). This one
// closes on any answer that is not a whole number, and a line TryCommand
// would run (isKnownCommandLine: a registered command, an emote, a known
// spell, the say shortcut, an exit or a compass direction) then runs, so
// "north" or "smile" at the question still does what it says.
func PortalStake(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	outcome, answer := behaviortree.AnswerStake(user)
	switch outcome {
	case behaviortree.StakeNoPrompt:
		return false, nil
	case behaviortree.StakeNotAmount:
		if isKnownCommandLine(user, room, answer) {
			user.Command(answer)
		}
	}
	return true, nil
}
