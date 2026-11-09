package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Checks whether their level is too high for a guide
func Message_SendMessage(e events.Event) events.ListenerReturn {

	message, typeOk := e.(events.Message)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "Message", "Actual Type", e.Type())
		return events.Continue
	}

	if message.UserId > 0 {

		if user := users.GetByUserId(message.UserId); user != nil {

			// If they are deafened, they cannot hear user communications
			if message.HiddenFromDeafened(user.Deafened) {
				return events.Continue
			}

			textOut := templates.AnsiParse(message.Text)
			if user.ScreenReader {
				textOut = util.StripCharsForScreenReaders(textOut)
			}
			connections.SendTo([]byte(term.AnsiMoveCursorColumn.String()+term.AnsiEraseLine.String()+textOut), user.ConnectionId())

			events.AddToQueue(events.RedrawPrompt{UserId: user.UserId}, 100)

		}
	}

	// RoomId branch: post-T9, Room.SendText/SendTextVisual fan out
	// per-recipient (UserId events above), so this branch serves only the
	// remaining RoomId-keyed emitters: Room.SendTextCommunication (a player's
	// adjacent-room shout and the AI companion's `ask` line),
	// Room.SendTextToExits, and any other direct events.Message{RoomId}
	// construction. Player speech and free-form emotes arrive per user with
	// IsCommunication set, so the per-user check above carries the Deafened
	// filter for them (sight gates slice 5b). The IsQuiet / SuperHearing
	// filter below is reached by SendTextToExits(txt, true), but no dogmud
	// condition grants superhearing, so those lines reach nobody.
	if message.RoomId > 0 {

		room := rooms.LoadRoom(message.RoomId)
		if room == nil {
			return events.Continue
		}

		for _, userId := range room.GetPlayers() {
			skip := false

			if message.UserId == userId {
				continue
			}

			exLen := len(message.ExcludeUserIds)
			if exLen > 0 {
				for _, excludeId := range message.ExcludeUserIds {
					if excludeId == userId {
						skip = true
						break
					}
				}
			}

			if skip {
				continue
			}

			if user := users.GetByUserId(userId); user != nil {

				// If they are deafened, they cannot hear user communications
				if message.HiddenFromDeafened(user.Deafened) {
					continue
				}

				// If this is a quiet message, make sure the player can hear it
				if message.IsQuiet {
					if !user.Character.HasConditionFlag(conditions.SuperHearing) {
						continue
					}
				}

				textOut := templates.AnsiParse(roomMessageTextFor(message, user))
				if user.ScreenReader {
					textOut = util.StripCharsForScreenReaders(textOut)
				}

				connections.SendTo([]byte(term.AnsiMoveCursorColumn.String()+term.AnsiEraseLine.String()+textOut), user.ConnectionId())

				events.AddToQueue(events.RedrawPrompt{UserId: user.UserId}, 100)

			}
		}

	}
	return events.Continue

}

// roomMessageTextFor is what user reads of a RoomId-keyed message. Player
// chatter on this branch (a shout from next door, the AI companion's `ask`
// line) arrives unrendered, so it wraps here, once per reader. Every other
// RoomId message keeps the lines it was sent with.
func roomMessageTextFor(message events.Message, user *users.UserRecord) string {
	if !message.IsCommunication {
		return message.Text
	}
	return messaging.WrapAnsi(message.Text, user.WrapWidth())
}
