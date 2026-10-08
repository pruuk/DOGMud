package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Shout keeps the player's own concerns (mute, uppercase, drunk text,
// escaping, the self line) and hands the rest to actions.Shout: the reveal,
// the room line with the name by each listener's sight, the line next door,
// and waking the room (sight gates slice 5b).
func Shout(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if user.Muted {
		user.SendText(messaging.CategoryWarning, `You are <ansi fg="alert-5">MUTED</ansi>. You can only send <ansi fg="command">whisper</ansi>'s to Admins and Moderators.`)
		return true, nil
	}

	// Nothing to shout: refuse rather than broadcast a blank line to this
	// room and the next (#260).
	if strings.TrimSpace(rest) == `` {
		user.SendText(messaging.CategorySystem, `Shout what?`)
		return true, nil
	}

	rest = strings.ToUpper(rest)

	if user.Character.HasConditionFlag(conditions.Drunk) {
		// modify the text to look like it's the speech of a drunk person
		rest = drunkify(rest)
	}

	// Neutralise <ansi> markup before interpolation. ToUpper above happens to
	// break the parser's byte-exact lowercase match, but that is an accident of
	// the current parser, not a defence, so escape explicitly. Shout crosses
	// room boundaries, so a forged tag here reaches players who never opted in.
	rest = util.EscapeAnsiTags(rest)

	actions.Shout(&actions.UserActor{User: user, Room: room}, rest)

	selfMsg := fmt.Sprintf(`You shout, "<ansi fg="yellow">%s</ansi>"`, rest)
	user.SendText(messaging.CategoryShout, util.SplitStringNL(selfMsg, 80))

	return true, nil
}
