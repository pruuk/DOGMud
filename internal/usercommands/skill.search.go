package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// searchTooDarkLine answers a search that would have moved a quest step in a
// room where the player sees nothing at all.
const searchTooDarkLine = `It is too dark to make anything out.`

// Search is a thin wrapper over actions.Search. The action handles
// all tier rolls, template rendering, cooldown gating, and skill
// progression.
//
// `search` searches the room; `search <feature>` searches one thing in it
// (a noun, a discovered hidden noun or a visible container). See
// actions/search_feature.go.
func Search(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `search`) {
		return true, nil
	}
	actor := &actions.UserActor{User: user, Room: room}
	spots := questSearchSpots(user, room)
	step := questSearchStep(user, room)
	// Owner ruling 2026-11-02: a search where the player sees nothing at all
	// counts for no quest. The game's own sight decision settles it (night
	// vision and infravision count through it), the same test `look` refuses
	// on. Quest spots are not rolled then, and the step is answered below.
	//
	// The cause is told apart as `look` tells it (actions.ResolveLook and
	// noSightRefusal, #364): light helps a searcher in the dark and does
	// nothing for a blinded one, so the blinded one is not told to find light.
	lookKind := actions.ResolveLook(actor, ``).Kind
	sightLine, dark := noSightRefusal(lookKind)
	if lookKind == actions.LookTooDark {
		sightLine = searchTooDarkLine
	}
	questSpots := spots
	if dark {
		questSpots = nil
	}
	// A search refused for cooldown never ran, so it sends no quest event: a
	// quest step advances only when the action succeeds (owner, 2026-10-17).
	// QuestStep also holds back "You find nothing of interest." when the dark
	// refusal below answers in its place.
	result := actions.Search(actor, actions.SearchOptions{
		Feature:       rest,
		QuestFeatures: questSpots,
		QuestStep:     step || (dark && len(spots) > 0),
	})
	if result.OnCooldown {
		return true, nil
	}
	if dark {
		// Say so when a search here would have moved a step or found a spot,
		// which includes a quest that a search itself starts, and only if the
		// search turned up nothing else (a bauble, say). Otherwise the player
		// reads the ordinary search output and nothing extra.
		if (step || len(spots) > 0) && !result.FoundAnything() {
			user.SendText(messaging.CategoryRefusal, sightLine)
		}
		return true, nil
	}

	// #375 phase 3a: `search <feature>` raises the search-family
	// room_interact after the search has run, so a `verb: search` trigger on
	// that feature fires with no roll. Only a resolved feature counts (a room
	// noun, a discovered hidden noun or a visible container, as the search
	// itself resolved it). A plain search raises it for each quest search
	// spot it uncovered at the room difficulty.
	if result.Feature != `` {
		notifyRoomInteract(user, room, quests.VerbSearch, rest, result.Feature)
	}
	for _, spot := range result.QuestFeaturesFound {
		notifyRoomInteract(user, room, quests.VerbSearch, spot, spot)
	}

	// Quest engine: command notification, so a quest can gate a step on the
	// `search` command in a room (#375 phase 7: quests 49 and 58 in the
	// Reliquary, 76 in Maren's Cottage). Mirrors forage/drink/throw/cast.
	questengine.NotifyCommand(user, room.RoomId, "search")

	return true, nil
}

// questSearchSpots lists the room's quest search spots (the features of its
// `verb: search` room_interact triggers, matched on the template room) whose
// trigger would fire for this player now. Only these are rolled by a plain
// search: a spot of a quest the player is not on, or a step already done,
// is never rolled, so the thorough-search line cannot hint at it.
func questSearchSpots(user *users.UserRecord, room *rooms.Room) []string {
	matchRoom, _ := rooms.OriginalRoomId(room.RoomId)
	engine := questengine.GetEngine()
	var spots []string
	for _, spot := range engine.SearchFeatures(matchRoom) {
		bridge := questengine.NewGameBridge(user, room.RoomId)
		if engine.WouldFire("room_interact", questengine.EventDetails{
			UserId:   user.UserId,
			RoomId:   matchRoom,
			Verb:     quests.VerbSearch,
			Noun:     spot,
			RoomNoun: spot,
		}, bridge) {
			spots = append(spots, spot)
		}
	}
	return spots
}

// questSearchStep reports whether a `command: search` quest trigger in this
// room would fire for this player now. Its own text says what the search
// turned up, so actions.Search must not first say it found nothing.
func questSearchStep(user *users.UserRecord, room *rooms.Room) bool {
	d := questengine.CommandDetails(user.UserId, room.RoomId, "search", questengine.NoTarget())
	return questengine.GetEngine().WouldFire("command", d, questengine.NewGameBridge(user, room.RoomId))
}
