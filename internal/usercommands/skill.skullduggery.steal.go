package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

/*
Skullduggery Skill
Level 2 - Steal: attempt to steal from a mob or room container using
an opposed Dex+skill vs Perception roll.
*/
func Steal(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	skillLevel := user.Character.GetSkillLevel(skills.Skullduggery)

	// Requires skullduggery rank 1 to even see the command. Rank 2
	// is required for the actual steal — that gate lives inside
	// actions.Steal AFTER target validation, so immune-mob targets get
	// the canonical rebuff regardless of the player's skill level
	// (otherwise a rank-1 thief targeting a forager would see
	// "not advanced enough" — a misleading hint that the command would
	// work with more skill).
	if skillLevel < 1 {
		return false, nil
	}

	if refuseWhileBusy(user, `steal`) {
		return true, nil
	}

	args := util.SplitButRespectQuotes(strings.ToLower(rest))

	if len(args) == 0 {
		user.SendText(messaging.CategorySystem, "Steal from whom?")
		return true, nil
	}

	opts := parseStealArgs(args, room, user)
	if opts == nil {
		// parseStealArgs sent the appropriate message already.
		return true, nil
	}

	actor := &actions.UserActor{User: user, Room: room}
	result := actions.Steal(actor, *opts)

	// actions.Steal emits player-facing text for every outcome except
	// the cooldown case (which returns without calling SendText).
	if result.OnCooldown {
		user.SendText(messaging.CategorySystem,
			"You need a moment before you can try that again.")
	}

	return true, nil
}

// parseStealArgs resolves CLI arguments to StealOptions.
// Args are already lowercased. Accepts:
//
//	steal <mob|container>
//	steal from <mob|container>
//	steal <household bauble>   (a find that belongs to this room's household)
//
// Returns nil when the target cannot be resolved (message already sent).
// The "can't steal from players" guard lives here since the player
// command explicitly forbids PvP; actions.Steal supports it for mobs.
func parseStealArgs(args []string, room *rooms.Room, user *users.UserRecord) *actions.StealOptions {
	// Support optional "from" preposition: `steal from <target>`
	targetNoun := args[0]
	if targetNoun == "from" && len(args) > 1 {
		targetNoun = args[1]
	}

	// Try mob/player resolution first.
	target, err := actions.ResolveTargetActor(room, targetNoun, actions.ResolveTargetOptions{Viewer: user.Character})
	if err == nil {
		if target.IsPlayer() {
			user.SendText(messaging.CategorySystem, "You can't steal from other players.")
			return nil
		}
		return &actions.StealOptions{
			TargetMobInstanceId: target.(*actions.MobActor).Mob.InstanceId,
		}
	}

	// Try room container.
	containerName := room.FindContainerByName(targetNoun)
	if containerName != "" {
		return &actions.StealOptions{
			ContainerNoun: containerName,
		}
	}

	// Try a bauble on the floor that belongs to this room's household
	// (found by searching their home). Only those: anything else on the
	// floor is simply picked up with get. Every word is used, so a bauble
	// is found by any word of its name ("steal small doll").
	if itm, ok := householdBaubleNamed(room, strings.Join(args, " ")); ok {
		return &actions.StealOptions{HouseholdItem: itm}
	}

	// A fixture is part of the room: nothing to steal (lighting 5e). Named
	// only to one who can see the floor, as `get` does (TooDarkToGet).
	if !actions.TooDarkToGet(&actions.UserActor{User: user, Room: room}) {
		if itm, ok := actions.FindTakeableOnFloor(room, strings.Join(args, " "), false); ok && itm.IsFixture() {
			fixedInPlace(user, itm)
			return nil
		}
	}

	user.SendText(messaging.CategorySystem, "Steal from whom?")
	return nil
}

// householdBaubleNamed finds, among the floor's baubles that belong to this
// room's household, the one the words name.
func householdBaubleNamed(room *rooms.Room, words string) (items.Item, bool) {
	var theirs []items.Item
	for _, itm := range room.Items {
		if itm.BaubleBelongsTo(room.RoomId) {
			theirs = append(theirs, itm)
		}
	}
	if len(theirs) == 0 {
		return items.Item{}, false
	}
	closeMatch, exact := items.FindMatchIn(words, theirs...)
	if exact.ItemId > 0 {
		return exact, true
	}
	if closeMatch.ItemId > 0 {
		return closeMatch, true
	}
	return items.Item{}, false
}
