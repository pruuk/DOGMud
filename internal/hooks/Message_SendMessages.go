package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
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
			if message.Reply {
				// #842: part of the reply to this player's typed line. Held
				// until the line's InputDone (Pager_ReplyEnd), which sends it
				// whole or a page at a time.
				user.HoldReply(textOut)
			} else {
				// No paging in combat (owner ruling 2026-11-18): a fight that
				// started while pages wait sends them before its first line.
				if rest := user.FlushPagesInCombat(); rest != `` {
					user.SendReplacingPrompt([]byte(rest))
				}
				user.SendReplacingPrompt([]byte(textOut))

				events.AddToQueue(events.RedrawPrompt{UserId: user.UserId}, 100)
			}

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

		// The players in the room now: one who left since queueing reads
		// nothing, and OnlyUserIds (#781 D43) keeps a walk-in out of a line
		// addressed to others (events.Message.AddressedTo).
		for _, userId := range room.GetPlayers() {
			if message.UserId == userId {
				continue
			}

			if !message.AddressedTo(userId) {
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

				user.SendReplacingPrompt([]byte(textOut))

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
