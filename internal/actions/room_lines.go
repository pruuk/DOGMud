package actions

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// The shared room-line senders of sight gates slice 5b. Every speech and
// emote wrapper, player and mob, sends its room line through one of these, so
// the name each listener reads follows that listener's sight, and the deafen
// filter applies to a player's free text only (owner rulings 3, 6 and 7).
// speech_wrapper_guard_test.go stops a wrapper sending its own.

// SendHeard sends a line the actor made heard (a rally, a warcry) to everyone
// else in its room, whatever they can see: the actor's name at clear sight,
// "a figure" at shapes, "something" when the listener sees nothing. It is
// authored text, not chatter, so the deafen filter does not apply. A player
// actor does not read its own room line.
func SendHeard(actor Actor, cat messaging.Category, text string) {
	room := actor.GetRoom()
	if room == nil {
		return
	}
	room.SendTextHidingNames(cat, text, []string{actor.GetName()}, messaging.HideNames, actor.GetUserId())
}

// SendSeen is SendHeard's visual twin, for an emote: the name at clear sight,
// "a figure" at shapes (a bare mention in the text included), and nothing for
// a listener who cannot see. chatter marks a player's free text, which keeps
// the deafen filter; a mob's line never meets it, whatever chatter says
// (owner ruling 6).
//
// A hidden actor's emote reaches no one (#274, owner R3, 2026-10-08): an
// emote would give its hider away, so it is silence rather than an anonymous
// line. The player emote command tells its own hider why.
func SendSeen(actor Actor, cat messaging.Category, text string, chatter bool) {
	room := actor.GetRoom()
	if room == nil {
		return
	}
	if c := actor.GetCharacter(); c != nil && c.IsHidden() {
		return
	}
	names := []string{actor.GetName()}
	if chatter && actor.IsPlayer() {
		room.SendVisualCommunicationHidingNames(cat, text, names, actor.GetUserId())
		return
	}
	room.SendTextVisualHidingNames(cat, text, names, actor.GetUserId())
}

// sendSpoken sends a speech line (say, shout) the actor spoke to everyone else
// in room. Every listener hears the words; the speaker's name reads "A
// figure" at shapes and "Someone" when the listener sees nothing (ruling 3).
// A player's words are chatter and keep the deafen filter; an NPC's are
// authored and do not (ruling 6). A speaker still hidden after the reveal is
// unseen by everyone.
func sendSpoken(actor Actor, room *rooms.Room, cat messaging.Category, line string, stillHidden bool) {
	if room == nil {
		return
	}
	names := []string{actor.GetName()}
	if stillHidden {
		line = messaging.HideSpeakerNames(line, names, messaging.SightNone)
	}
	if actor.IsPlayer() {
		room.SendCommunicationHidingNames(cat, line, names, actor.GetUserId())
		return
	}
	room.SendTextHidingNames(cat, line, names, messaging.HideSpeakerNames)
}
