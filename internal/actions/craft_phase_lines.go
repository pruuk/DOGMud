package actions

import (
	"fmt"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// NarrateCraftPhase sends the room's lines for one moment of a craft, start,
// success or failure, for a player or a mob crafter (messaging M6 row 7,
// owner rulings R1 to R3). It sends ROOM lines only: the crafter's own lines
// stay with the sites, from the recipe file.
//
// Each player in the room but the crafter reads one line, by sight:
//   - full sight: the skill's seen line, crafter named, {item} the output's
//     name;
//   - shapes only: the same variant, crafter hidden, {item} "something";
//   - no sight (dark, blinded, asleep): the skill's heard line, the start
//     sound at start and the finish sound at success and failure alike;
//   - a reader who does not perceive a hidden crafter: the heard line,
//     whatever their sight (R3 and R5 of M6 slice 4, as for spells).
func NarrateCraftPhase(crafter Actor, recipe *crafting.RecipeSpec, phase crafting.Phase, room *rooms.Room) {
	if crafter == nil || recipe == nil || room == nil {
		return
	}
	cc := crafter.GetCharacter()
	if cc == nil {
		return
	}
	actorName, actorPlain := cc.GetCharacterName(true), cc.GetCharacterName(false)
	// One wrapped category for every room line about a crafter, player or
	// mob (owner ruling C, 2026-10-13). CategoryEmote, which a player
	// crafter's lines rode, never wraps.
	cat := messaging.CategoryCraftSeen
	crafterUserId := 0
	switch a := crafter.(type) {
	case *UserActor:
		if a.User == nil {
			return
		}
		crafterUserId = a.User.UserId
	case *MobActor:
		if a.Mob == nil {
			return
		}
		actorName = MobSeenName(a.Mob, room, 0)
	}

	item := ""
	if name := items.SpecDisplayName(recipe.Output.ItemId); name != "" {
		item = fmt.Sprintf(`<ansi fg="itemname">%s</ansi>`, name)
	}
	full, vague := crafting.CraftRoomLines(recipe.Skill, phase,
		textutil.TokenContext{ActorName: actorName, ActorPlainName: actorPlain}, item)
	heard := crafting.CraftHeardLine(recipe.Skill, phase)

	unseenBy := UnseenBy(room, cc)
	// notFull and notShapes are the exclusion lists that narrow each visual
	// send to its own band: everyone outside the band, the crafter and the
	// non-perceivers included.
	var notFull, notShapes []int
	for _, uid := range room.GetPlayers() {
		u := users.GetByUserId(uid)
		sight := messaging.SightNone
		if u != nil && uid != crafterUserId && !slices.Contains(unseenBy, uid) {
			sight = messaging.ReaderSight(u.Character, room)
		}
		if sight != messaging.SightFull {
			notFull = append(notFull, uid)
		}
		if sight != messaging.SightShapes {
			notShapes = append(notShapes, uid)
		}
	}
	// A seen line may use {actor_plain}, which the tag anonymizer cannot catch.
	names := []string{actorName, actorPlain}
	if full != "" {
		room.SendTextVisualHidingNames(cat, full, names, notFull...)
	}
	if vague != "" {
		room.SendTextVisualHidingNames(cat, vague, names, notShapes...)
	}

	if heard == "" {
		return
	}
	// The sound channel reaches every reader who sees nothing; the
	// non-perceivers are sent their line one by one below, so the channel
	// skips them to send each exactly one.
	room.SendTextUnsighted(cat, heard, append(slices.Clone(unseenBy), crafterUserId)...)
	for _, uid := range unseenBy {
		if uid == crafterUserId {
			continue
		}
		if u := users.GetByUserId(uid); u != nil {
			u.SendText(cat, heard)
		}
	}
}
