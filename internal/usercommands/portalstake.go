package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
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
// closes on any answer that is not a whole number, and a line that is a
// known command (a registered command after alias expansion, or an exit
// here) is then run, so "north" at the question still walks north.
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

// isKnownCommandLine reports whether line starts with a command the
// dispatcher would run: a registered command after alias expansion, or an
// exit of room.
func isKnownCommandLine(user *users.UserRecord, room *rooms.Room, line string) bool {
	verb, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
	if verb == `` {
		return false
	}
	cmd, _ := actions.ExpandAliases(verb, rest, user.TryCommandAlias)
	if _, ok := userCommands[cmd]; ok && cmd != behaviortree.PortalStakeCommand {
		return true
	}
	if room != nil {
		if exitName, _ := room.FindExitByName(cmd); exitName != `` {
			return true
		}
	}
	return false
}
