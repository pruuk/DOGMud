package actions

import (
	"fmt"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ShoutResult reports a shout to its wrapper.
type ShoutResult struct {
	Text string
}

// Shout is the one shout body for a player and a mob (sight gates slice 5b):
// it reveals a hidden shouter (rider 5), sends the room line through
// sendSpoken (the name by each listener's sight, the words always), sends
// the anonymous line with the words to every adjacent room, and wakes every
// sleeper in the room but the shouter.
//
// The player wrapper keeps mute, uppercase, drunk text, escaping and the
// self line; the mob wrapper keeps nothing but this call.
func Shout(actor Actor, text string) ShoutResult {
	char := actor.GetCharacter()

	// Shouting is a noisy action: reveal if hidden.
	if char.IsHidden() {
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger:  awareness.TriggerNoisyAction,
			Metadata: map[string]any{"command": "shout"},
		})
	}
	isSneaking := char.IsHidden()

	room := actor.GetRoom()
	if room == nil {
		return ShoutResult{Text: text}
	}

	nameColor, textColor := "mobname", "saytext-mob"
	if actor.IsPlayer() {
		nameColor, textColor = "username", "yellow"
	}
	sendSpoken(actor, room, messaging.CategoryShout,
		formatShoutText(actor.GetName(), text, nameColor, textColor), isSneaking)

	// The web client's Chat window shows shouts too (owner 2026-11-07); gmcp
	// builds each listener's line from this.
	events.AddToQueue(events.Communication{
		SourceUserId:        actor.GetUserId(),
		SourceMobInstanceId: actor.GetMobInstanceId(),
		CommType:            `shout`,
		Name:                actor.GetName(),
		Message:             text,
		SpeakerHidden:       isSneaking,
	})

	// Next door hears the words and no name, from either speaker. A player's
	// line stays player chatter (deafen-filtered, wrapped per reader by the listener);
	// a mob's is authored and unfiltered. Standard, temporary and
	// mutator-added exits, each neighbour once.
	//
	// #781 ruling 16: through a secret exit, a listener who has not found it
	// hears "Someone shouts from nearby" and learns nothing; one who has
	// reads the exit's name; an ordinary exit sends one line
	// (Room.HeardLines). A player's queued line carries the allow-list, so a
	// walk-in reads only "nearby"; a mob's goes per listener at once, where
	// the line's excludes are exact.
	room.ForEachAdjacentRoom(func(otherRoom *rooms.Room, back rooms.ExitTo) {
		for _, line := range otherRoom.HeardLines(back,
			fmt.Sprintf(`Someone shouts from the <ansi fg="exit">%s</ansi> direction, "<ansi fg="%s">%s</ansi>"`,
				back.Name, textColor, text),
			fmt.Sprintf(`Someone shouts from %s, "<ansi fg="%s">%s</ansi>"`,
				rooms.HeardFromNearby, textColor, text),
			nil) {
			if actor.IsPlayer() {
				otherRoom.SendTextCommunicationTo(line.Text, line.Only, slices.Concat(line.Exclude, []int{actor.GetUserId()})...)
				continue
			}
			otherRoom.SendText(messaging.CategoryShout, line.Text, line.Exclude...)
		}
	})

	wakeSleepers(actor, room)

	return ShoutResult{Text: text}
}

// formatShoutText is the room line of a shout, left unwrapped:
// CategoryShout wraps per reader in the pipeline.
func formatShoutText(name, text, nameColor, textColor string) string {
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi> shouts, "<ansi fg="%s">%s</ansi>"`,
		nameColor, name, textColor, text)
}

// wakeSleepers wakes every sleeper in room but the actor. Chunk 3.3: a shout
// wakes the room it is shouted in; next door is out of scope.
func wakeSleepers(actor Actor, room *rooms.Room) {
	for _, uid := range room.GetPlayers() {
		if uid == actor.GetUserId() {
			continue
		}
		if other := users.GetByUserId(uid); other != nil && other.Character.HasConditionFlag(conditions.Sleeping) {
			other.Character.CancelConditionsWithFlag(conditions.Sleeping)
			mobs.OnSleeperWoken(other.Character)
		}
	}
	for _, instId := range room.GetMobs() {
		if instId == actor.GetMobInstanceId() {
			continue
		}
		if m := mobs.GetInstance(instId); m != nil && m.Character.HasConditionFlag(conditions.Sleeping) {
			m.Character.CancelConditionsWithFlag(conditions.Sleeping)
			mobs.OnSleeperWoken(&m.Character)
		}
	}
}
