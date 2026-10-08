package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Emote sends the player's emote to the room through actions.SendSeen. An
// emote is seen, not heard: the name follows each onlooker's sight and a
// listener who cannot see gets nothing (owner ruling 7, sight gates slice
// 5b). Only the free-form line (and its @ form) is chatter, spared a
// deafened player; the empty and alias lines are pre-written and reach
// everyone who can see.
func Emote(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	actor := &actions.UserActor{User: user, Room: room}

	if len(rest) == 0 {
		user.SendText(messaging.CategoryEmote, "You emote.")
		actions.SendSeen(actor, messaging.CategoryEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> emotes.`, user.Character.Name),
			false,
		)
		hiddenEmoteNote(user)
		return true, nil
	}

	// Emote aliases bypass mute/deafen checks (pre-written, not free-form communication).
	result := actions.Emote(rest)
	if result.IsAlias {
		aliasMsg := actions.FormatEmoteText(user.Character.Name, result.AliasText, "username")
		user.SendText(messaging.CategoryEmote, fmt.Sprintf(`You Emote: %s`, aliasMsg))
		actions.SendSeen(actor, messaging.CategoryEmote, aliasMsg, false)
		hiddenEmoteNote(user)
		events.AddToQueue(events.Emote{UserId: user.UserId, RoomId: room.RoomId, Text: result.AliasText})
		return true, nil
	}

	if user.Muted {
		user.SendText(messaging.CategoryWarning, `You are <ansi fg="alert-5">MUTED</ansi>. You can only send <ansi fg="command">whisper</ansi>'s to Admins and Moderators.`)
		return true, nil
	}

	// Neutralise <ansi> markup before interpolation. Only the free-form path
	// is escaped; result.AliasText above comes from the server-side
	// EmoteAliases table and its markup is legitimate.
	rest = util.EscapeAnsiTags(rest)

	if rest[0] == '@' && len(rest) > 1 {
		rest = rest[1:]
	} else {
		emoteMsg := actions.FormatEmoteText(user.Character.Name, rest, "username")
		user.SendText(messaging.CategoryEmote, fmt.Sprintf(`You Emote: %s`, emoteMsg))
	}

	// Free text is chatter: true keeps the deafen filter.
	actions.SendSeen(actor, messaging.CategoryEmote,
		actions.FormatEmoteText(user.Character.Name, rest, "username"),
		true,
	)
	hiddenEmoteNote(user)
	events.AddToQueue(events.Emote{UserId: user.UserId, RoomId: room.RoomId, Text: rest})

	return true, nil
}

// hiddenEmoteNote tells a hidden player that their emote reached no one.
// actions.SendSeen sends nothing for a hidden actor (#274, owner R3,
// 2026-10-08), and without this line the silence would be a mystery.
func hiddenEmoteNote(user *users.UserRecord) {
	if user.Character.IsHidden() {
		user.SendText(messaging.CategoryEmote, `No one sees it; you are hidden.`)
	}
}
