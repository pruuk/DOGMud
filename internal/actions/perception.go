package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The one answer to "who perceives this character" and "what does a reader
// call this mob" for every line about an act (messaging M6 slice 4). The
// hooks package's mobDisplayName, mobSeenName, mobHiddenFrom,
// playersNotPerceiving and conditionLineUnseenBy delegate here.

// MobDisplayName returns the formatted display name for a mob in combat text,
// including duplicate index coloring when multiple mobs share the same name.
//
// A mob hidden from the viewer reads as "something" (lowercase: the line may
// put it mid-sentence). Viewer 0 is a room-wide line, which has no one reader
// to ask Perceives of, so a hidden mob is unseen in it by every reader, as a
// hidden caster is in its spell-channel lines (#274, owner R3).
// GetMobNameIndexed alone named it to every faces reader as "Skeleton
// (hidden)" (#382).
//
// It is a NARRATION name, as characters.GetCharacterName(true) is (#453): the
// identity tag with its duplicate index and suffix colour, but no adjective
// span, quest star or " and <pet>". Every caller puts it in a line about what
// the mob did or had done to it (recoil, spell, condition and defence
// lines), where "You recoil from striking Skeleton (charmed)!" told a state
// tag (#454 review).
func MobDisplayName(mob *mobs.Mob, room *rooms.Room, viewingUserId int) string {
	if MobHiddenFrom(mob, viewingUserId) {
		return messaging.UnseenFigure(messaging.SightNone)
	}
	return MobSeenName(mob, room, viewingUserId)
}

// MobSeenName is MobDisplayName for a reader known to perceive the mob, so it
// names a hidden mob too. A condition line is the caller: it goes only to the
// readers who perceive its holder (UnseenBy, #458).
func MobSeenName(mob *mobs.Mob, room *rooms.Room, viewingUserId int) string {
	dupIdx := room.GetMobDuplicateIndex(mob.InstanceId)
	f := mob.Character.GetMobNameIndexed(viewingUserId, dupIdx)
	f.Adjectives = nil
	f.QuestAlert = false
	f.PetName = ``
	return f.String()
}

// MobHiddenFrom reports whether a line about mob must not name it to
// viewingUserId: the mob is hidden and the viewer does not Perceive it. Viewer
// 0 (a room-wide line) perceives no hidden mob.
func MobHiddenFrom(mob *mobs.Mob, viewingUserId int) bool {
	if !mob.Character.IsHidden() {
		return false
	}
	if viewingUserId == 0 {
		return true
	}
	u := users.GetByUserId(viewingUserId)
	return u == nil || !u.Character.Perceives(&mob.Character)
}

// PlayersNotPerceiving lists the players in room who cannot make out c
// (characters.Character.Perceives: c is hidden and they have no see-hidden).
func PlayersNotPerceiving(room *rooms.Room, c *characters.Character) []int {
	var ids []int
	for _, uid := range room.GetPlayers() {
		if v := users.GetByUserId(uid); v != nil && !v.Character.Perceives(c) {
			ids = append(ids, uid)
		}
	}
	return ids
}

// UnseenBy lists the players in room who must not read a line about an act of
// c's own: those who do not perceive c (PlayersNotPerceiving), by the rule the
// quit line uses. Every condition room line skips them: start (judged before
// the condition lands), trigger and end (#458), and a hidden mob's own acts
// skip the same readers. nil for a visible c, which costs no room walk, and
// for a nil room (owner ruling R3, #274).
func UnseenBy(room *rooms.Room, c *characters.Character) []int {
	if room == nil || c == nil || !c.IsHidden() {
		return nil
	}
	return PlayersNotPerceiving(room, c)
}
