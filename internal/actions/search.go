package actions

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// secretExitDiscoveryKey namespaces a secret exit's discovery record.
//
// Discoveries share one key space per room (Character.Discoveries is
// map[roomId][]string), and container names and hidden-noun keys are authored
// strings that could collide with a direction. The prefix keeps an exit named
// "gate" from being confused with a hidden container of the same name.
func secretExitDiscoveryKey(exitName string) string {
	return "exit:" + exitName
}

// spotsHider resolves "does the observer spot this hider?" as an OPPOSED
// contest, the same way usercommands/go.go does on room entry.
//
// Deliberately mirrors go.go rather than inventing a variant: the two paths
// answer the identical question and disagreed for four slices, which is the
// defect Phase C exists to close.
//
// Scores follow the convention U6b Task 16 set — CalcDetectionScore for the
// opposed observer side, CalcSneakScoreVsObserver for the hider, which folds in
// per-observer lighting (the observer's own sight decides whether the room
// reads as lit for it).
//
// room is the observer's room, for the observer's sight ramp inside
// CalcDetectionScore and the lit test inside CalcSneakScoreVsObserver. The
// caller passes messaging.FixedLight so the room's light is composed once,
// not once per occupant.
func spotsHider(observer *characters.Character, hider *characters.Character, room messaging.RoomVisibility) bool {
	return combat.RunContest(
		CalcDetectionScore(observer, room),
		[]contest.Entry{{Score: CalcSneakScoreVsObserver(hider, observer, room)}},
	).Success
}

// foundHiderEntry is one line of a searcher's find, as the searcher's sight in
// the room lets them read it (lighting plan 5c): the name and the hiding mark
// with faces, the anonymous figure HideNames writes below that. kind is the
// FormattedName type, "username" or "mob".
func foundHiderEntry(name, kind string, d messaging.SightDecision) string {
	if d != messaging.SightFull {
		return messaging.UnseenFigure(d)
	}
	return characters.FormattedName{Name: name + ` <ansi fg="black-bold">(hiding)</ansi>`, Type: kind, Suffix: "hidden"}.String()
}

// showFoundHiders renders a searcher's find through the room roster template.
// Only the hiders found are listed: GetDetails fills both lists with everyone
// already visible, and the find used to reset one list and keep the other, so
// a found player was reported alongside every visible mob. Below full sight
// the entries go in the player list, which the template does not color, so
// the color cannot tell a mob from a player either.
func showFoundHiders(actor Actor, room *rooms.Room, names []string, kind string) {
	d := messaging.ParticipantSight(actor.GetCharacter(), room)
	details := rooms.GetDetails(room, users.GetByUserId(actor.GetUserId()))
	details.VisiblePlayers = []string{}
	details.VisibleMobs = []string{}
	for _, name := range names {
		entry := foundHiderEntry(name, kind, d)
		if kind == "mob" && d == messaging.SightFull {
			details.VisibleMobs = append(details.VisibleMobs, entry)
		} else {
			details.VisiblePlayers = append(details.VisiblePlayers, entry)
		}
	}
	actor.SendText(messaging.CategorySystem, RenderRoster(details, actor.GetUserId()))
}

// SearchOptions selects what is searched.
type SearchOptions struct {
	// Feature is what a player typed after `search` (`search bookshelf`,
	// `search under the table`). Empty searches the whole room. Only players
	// search features; a mob's search ignores it. See search_feature.go.
	Feature string
}

// SearchStashedItem represents a stashed item discovered by Tier 2.
type SearchStashedItem struct {
	ItemId      int
	DisplayName string
}

// SearchResult is the structured outcome.
type SearchResult struct {
	HiddenExitsFound      []string // Tier 1 — player flavor
	HiddenContainersFound []string // Tier 1 — player flavor
	StashedItemsFound     []SearchStashedItem
	HiddenPlayersFound    []int    // Tier 2 — user ids
	HiddenMobsFound       []int    // Tier 2 — mob instance ids
	HiddenNounsFound      []string // Tier 3 — player flavor
	BaubleFound           bool     // Tier 4: a bauble is on its way to the player (see search_bauble.go)

	// Feature is the room's name for what `search <feature>` searched;
	// empty for a plain search. FeatureNotFound is words that name no
	// feature (the search is then a plain one, as `search <anything>`
	// always was). FeatureSearched is the feature's bauble roll already
	// spent this BaubleFeatureWindowMinutes (the room's roll was taken
	// instead). A feature search is ALWAYS the whole room search as well.
	Feature         string
	FeatureNotFound bool
	FeatureSearched bool

	OnCooldown bool
	Reason     string
}

// FoundAnything reports whether the search turned up ANY of its kinds of
// discovery. It decides whether the player is told "You find nothing of
// interest."
//
// Derived from the result rather than tracked by a flag set beside each of the
// append sites: one predicate in one place cannot fall out of step with its
// siblings. ⚠️ A NEW TIER MUST ADD ITS SLICE HERE. That is the one thing this
// shape does not make automatic, and forgetting it makes the new tier's finds
// read as failures.
//
// ⚠️ A NEW CONTESTED TIER MUST ALSO ADD ITS SLICE TO foundByContest. The
// bauble tier is the one tier that is here and NOT there: it is not a
// contest, and it reaches the award its own way (awardSearch).
func (r SearchResult) FoundAnything() bool {
	return r.foundByContest() || r.BaubleFound
}

// foundByContest reports whether any CONTESTED tier found something.
func (r SearchResult) foundByContest() bool {
	return len(r.HiddenExitsFound) > 0 ||
		len(r.HiddenContainersFound) > 0 ||
		len(r.StashedItemsFound) > 0 ||
		len(r.HiddenPlayersFound) > 0 ||
		len(r.HiddenMobsFound) > 0 ||
		len(r.HiddenNounsFound) > 0
}

// Search rolls Perception+Search per discovery candidate in the room.
// UserActor receives template-rendered output; MobActor is silent
// (no broadcast, no template). Cooldown is shared with the player path
// (2 rounds on the "search" key).
func Search(actor Actor, opts SearchOptions) SearchResult {
	result := SearchResult{}
	char := actor.GetCharacter()
	room := actor.GetRoom()
	if char == nil || room == nil {
		return result
	}

	// `search <feature>` is this whole search, every tier as always (quests
	// hide their items behind `search shelf` and the like), plus one thing:
	// the bauble roll is that feature's own (search_feature.go). Words that
	// name no feature are a plain search, as `search <anything>` always was.
	//
	// A searcher who makes out nothing at all here (lighting plan 5c, the
	// same test `look` refuses on) cannot pick a feature out either: what
	// they typed is treated exactly like words that name nothing, so the
	// reply cannot confirm the feature exists. At shapes a feature is still
	// named, as `look <noun>` still describes one there.
	var feature SearchFeature
	hasFeature := false
	if actor.IsPlayer() && strings.TrimSpace(opts.Feature) != `` {
		if messaging.ParticipantSight(char, room) == messaging.SightNone {
			result.FeatureNotFound = true
		} else if f, ok := FindSearchFeature(char, room, opts.Feature); ok {
			feature, hasFeature = f, true
			result.Feature = f.Name
		} else {
			result.FeatureNotFound = true
		}
	}

	if !char.TryCooldown("search", "2 rounds") {
		result.OnCooldown = true
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				fmt.Sprintf("You need to wait %d more rounds to do that again.",
					char.GetCooldown("search")))
		}
		return result
	}

	// sight ramp (plan 5b): the searcher needs to see. This score feeds only
	// the static AgainstDifficulty tiers; the hidden-occupant tiers go through
	// spotsHider, whose CalcDetectionScore pays the ramp itself.
	searchScore := CalcSearchScore(char) * messaging.SightMult(char, room)

	if actor.IsPlayer() {
		if hasFeature {
			actor.SendText(messaging.CategorySystem,
				fmt.Sprintf("You search the <ansi fg=\"noun\">%s</ansi> and snoop around for a bit...\n", feature.Name))
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is searching the %s.`, char.Name, feature.Name),
				[]string{char.Name},
				actor.GetUserId(),
			)
		} else {
			actor.SendText(messaging.CategorySystem, "You snoop around for a bit...\n")
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is snooping around.`, char.Name),
				[]string{char.Name},
				actor.GetUserId(),
			)
		}
	}

	rolledAgainstSomething := false

	// ── Tier 1 (target 125): Secret exits ────────────────────────
	for exitName, exitInfo := range room.Exits {
		if !exitInfo.Secret {
			continue
		}
		// 🔴 A FOUND SECRET EXIT IS NOT ROLLED AGAIN. Until 2026-08-29 this tier
		// set rolledAgainstSomething for every secret exit in the room and never
		// skipped one already found, because secret exits recorded no discovery
		// at all — hidden containers below and hidden nouns in tier 6 both guard
		// with HasDiscovery and record on a find. A room with a secret exit was
		// therefore a PERMANENT progression candidate on a 2-round cooldown,
		// roughly 450 uses an hour, against the ~150/hr that `search`'s own
		// multiplier was solved on (the assumption is written into config.yaml
		// and skills.go).
		//
		// ⚠️ It is still REPORTED, which is where this deliberately differs from
		// the container and noun tiers that `continue` outright. A found
		// container is reachable afterwards through `get`, but a secret exit
		// stays out of the room's exit list until the player VISITS the room
		// beyond it (roomdetails.go gates on HasVisited, not on a discovery). So
		// skipping it silently would leave someone who found it and did not walk
		// through with no way to be reminded of the name. Reporting costs
		// nothing they have not already earned; the roll and the award are what
		// close the farm.
		if char.HasDiscovery(room.RoomId, secretExitDiscoveryKey(exitName)) {
			result.HiddenExitsFound = append(result.HiddenExitsFound, exitName)
			continue
		}
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 125.0).Success {
			char.AddDiscovery(room.RoomId, secretExitDiscoveryKey(exitName))
			result.HiddenExitsFound = append(result.HiddenExitsFound, exitName)
			if actor.IsPlayer() {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You found a secret exit: <ansi fg="secret-exit">%s</ansi>`, exitName))
			}
		}
	}

	// ── Tier 1 (target 125): Hidden containers ──────────────────
	for containerName, container := range room.Containers {
		if !container.Hidden {
			continue
		}
		if char.HasDiscovery(room.RoomId, containerName) {
			continue
		}
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 125.0).Success {
			char.AddDiscovery(room.RoomId, containerName)
			result.HiddenContainersFound = append(result.HiddenContainersFound, containerName)
			if actor.IsPlayer() {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You discover a hidden <ansi fg="container">%s</ansi>!`, containerName))
			}
		}
	}

	// ── Tier 2 (target 135): Stashed items ──────────────────────
	stashedNames := []string{}
	for _, item := range room.Stash {
		if !item.IsValid() {
			room.RemoveItem(item, true)
			continue
		}
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 135.0).Success {
			result.StashedItemsFound = append(result.StashedItemsFound, SearchStashedItem{
				ItemId:      item.ItemId,
				DisplayName: item.DisplayName(),
			})
			if actor.IsPlayer() {
				stashedNames = append(stashedNames, item.DisplayName()+` <ansi fg="item-stashed">(stashed)</ansi>`)
			}
		}
	}
	if actor.IsPlayer() && len(stashedNames) > 0 {
		actor.SendText(messaging.CategorySystem,
			RenderGround(stashedNames, !room.IsLit(), gametime.IsNight(), actor.GetUserId()))
	}

	// U10b-1b PHASE C: hidden detection is an OPPOSED contest, reconciled onto
	// the form usercommands/go.go already used.
	//
	// It answered "does the observer spot the hider?" with a flat 135 threshold
	// that NEVER READ THE HIDER'S SNEAK SCORE, while go.go resolved the identical
	// question as observerScore vs hiddenScore. A hider's skill decided the
	// outcome in one path and was ignored in the other. Mobs reached the broken
	// path too, via behaviortree/actions_scout.go's actTrySearch, gated by the
	// cheap condRoomHasHiddenEntity pre-check in conditions_scout.go.
	//
	// ⚠️ THIS IS THE SLICE'S ONE DELIBERATE BEHAVIOUR CHANGE. Investing in
	// stealth now works against a searcher, where before it did nothing at all.
	// U4 declined it precisely because converting a flat threshold into a contest
	// is a behaviour change and U1-U5 are contracted as provable no-ops.
	//
	// It uses combat.RunContest, NOT contest.AgainstDifficulty: there is a real
	// opponent, so it belongs on the opposed seam and takes ContestFloor like
	// every other opposed contest. The four static tiers in this file are the
	// other kind and stay on AgainstDifficulty.

	// The room's light is invariant across every hidden occupant checked
	// below, so it is composed once here rather than inside spotsHider on
	// every iteration.
	roomLight := messaging.FixedLight(room.LightLevel())

	// ── Tier 2 (target 135): Hidden players ─────────────────────
	hiddenPlayerNames := []string{}
	for _, pId := range room.GetPlayers() {
		if pId == actor.GetUserId() {
			continue
		}
		p := users.GetByUserId(pId)
		if p == nil || !p.Character.IsHidden() {
			continue
		}
		rolledAgainstSomething = true
		if spotsHider(char, p.Character, roomLight) {
			result.HiddenPlayersFound = append(result.HiddenPlayersFound, pId)
			if actor.IsPlayer() {
				hiddenPlayerNames = append(hiddenPlayerNames, p.Character.Name)
			}
		}
	}
	if actor.IsPlayer() && len(hiddenPlayerNames) > 0 {
		showFoundHiders(actor, room, hiddenPlayerNames, "username")
	}

	// ── Tier 2 (target 135): Hidden mobs ────────────────────────
	hiddenMobNames := []string{}
	for _, mId := range room.GetMobs() {
		m := mobs.GetInstance(mId)
		if m == nil || !m.Character.IsHidden() {
			continue
		}
		rolledAgainstSomething = true
		if spotsHider(char, &m.Character, roomLight) {
			result.HiddenMobsFound = append(result.HiddenMobsFound, mId)
			if actor.IsPlayer() {
				hiddenMobNames = append(hiddenMobNames, m.Character.Name)
			}
		}
	}
	if actor.IsPlayer() && len(hiddenMobNames) > 0 {
		showFoundHiders(actor, room, hiddenMobNames, "mob")
	}

	// Owner ruling 10 (follow-up slice A): a player's find ends the hider's
	// hiding for everyone, as walking in and spotting them already does.
	revealSpotted(actor, result, room)

	// ── Tier 3 (target 175): Hidden nouns ───────────────────────
	// Sort keys for deterministic output order.
	nounKeys := make([]string, 0, len(room.HiddenNouns))
	for k := range room.HiddenNouns {
		nounKeys = append(nounKeys, k)
	}
	sort.Strings(nounKeys)

	for _, nounKey := range nounKeys {
		if char.HasDiscovery(room.RoomId, nounKey) {
			continue
		}
		hiddenNoun := room.HiddenNouns[nounKey]
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 175.0).Success {
			char.AddDiscovery(room.RoomId, nounKey)
			result.HiddenNounsFound = append(result.HiddenNounsFound, nounKey)
			if actor.IsPlayer() {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You discover something: <ansi fg="noun">%s</ansi>`, nounKey))
				actor.SendText(messaging.CategorySystem, hiddenNoun.HiddenDescription)
			}
		}
	}

	// ── Tier 4 (no contest): Baubles ────────────────────────────
	// Players only. A flat chance per roll, rolls rationed per room per real
	// hour (docs/baubles). Deliberately after every contested tier, and
	// deliberately NOT a candidate for progression: see searchForBauble.
	//
	// A feature search takes the feature's roll instead (once per feature
	// per BaubleFeatureWindowMinutes, apart from the room's two). When the
	// feature's roll is spent, the room's is taken, exactly as a plain
	// search would.
	if actor.IsPlayer() {
		used := false
		if hasFeature {
			result.BaubleFound, used = searchFeatureForBauble(actor, room, feature)
			result.FeatureSearched = !used
		}
		if !used {
			result.BaubleFound = searchForBauble(actor, room)
		}
	}

	// ── Skill progression (anti-botting gate) ───────────────────
	//
	// The gate is unchanged and is NOT the firing rule: a search of an empty
	// room rolled against nothing, so no contest resolved and nothing is
	// awarded. That is what stops `search` in a bare corridor from being a
	// free progression tick. What U10b-1 Task 14 changed is the WEIGHT of the
	// award that does fire.
	//
	// ⚠️ THIS SITE IS A CUT, not a gain, and it is the first in the slice.
	// A resolved search paid a FULL event win or lose; a fruitless search now
	// pays ProgressionFailureFraction. Searching is a high-frequency action
	// against mostly-empty rooms, so most searches resolve and find nothing --
	// the common case is the one being reduced. Carry it into the re-solve.
	//
	// ONE AWARD PER SEARCH, unchanged. A room with five hidden things rolls
	// five times and still pays once: the six tiers are one resolved action,
	// not six. That was already true and is now pinned by test.
	awardSearch(actor, char, result, rolledAgainstSomething)

	// Close the loop for the player. Without this a search that finds nothing
	// prints "You snoop around for a bit..." and then NOTHING, which reads as a
	// broken or ignored command rather than a completed one. Found things
	// announce themselves individually above; this is the only path with no
	// output at all.
	//
	// ⚠️ DELIBERATELY IDENTICAL for both "there was nothing here" and "there was
	// something here and you failed to find it", and it must stay that way.
	// Splitting the two would turn `search` into an oracle for the EXISTENCE of
	// hidden content: a player could stand in a room, read the different line,
	// and know a secret exit or stash is present without ever passing the roll.
	// That would defeat every hidden thing in the world.
	//
	// The consequence is accepted and is NOT a defect to "fix" later: U10b-1
	// made a fruitless-but-resolved search pay ProgressionFailureFraction, and
	// that award is INVISIBLE here on purpose. Progression must not leak level
	// design.
	if actor.IsPlayer() && !result.FoundAnything() {
		actor.SendText(messaging.CategorySystem, "You find nothing of interest.\n")
	}

	return result
}

// revealSpotted ends the hiding of everything a PLAYER's search found, for
// everyone in the room. Owner ruling 10 (follow-up slice A): the same thing
// walking in and spotting a hider already does (usercommands/go.go). Without
// it a hidden creature that never fights could be found by search and still not
// be named. A mob's search ends nothing; mobs perceiving hidden creatures is
// slice F.
func revealSpotted(actor Actor, found SearchResult, room *rooms.Room) {
	if !actor.IsPlayer() {
		return
	}
	searcher := actor.GetName()
	for _, uid := range found.HiddenPlayersFound {
		if u := users.GetByUserId(uid); u != nil {
			endHidingOnSpot(searcher, u.Character, u, room)
		}
	}
	for _, mid := range found.HiddenMobsFound {
		if m := mobs.GetInstance(mid); m != nil {
			endHidingOnSpot(searcher, &m.Character, nil, room)
		}
	}
}

// endHidingOnSpot drives the hider's Awareness machine out of Hidden, exactly
// as go.go does for a spotted occupant; the Awareness cascade then cancels
// condition 9. A player hider is told, by name only if they can see the searcher.
func endHidingOnSpot(searcher string, hider *characters.Character, hiderUser *users.UserRecord, room *rooms.Room) {
	if hider.Awareness != nil {
		_ = hider.Awareness.TransitionToRevealing(
			state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
	}
	if hiderUser == nil {
		return
	}
	hider.SetMiscData("sneaking", nil)
	if messaging.CanSeeClearly(hider, room) {
		hiderUser.SendText(messaging.CategorySystem, fmt.Sprintf(
			"<ansi fg=\"username\">%s</ansi> searches the room and spots you!", searcher))
	} else {
		hiderUser.SendText(messaging.CategorySystem, "Someone searches the room and spots you!")
	}
}

// awardSearch pays the search's ONE progression award, if it earned one.
//
//   - A search that rolled a contest pays: a win if anything was found, the
//     failure fraction if not.
//   - A bauble find is a win too (owner ruling, 2026-09-26). The bauble chance
//     grows with search skill and a find is rare, so it is fair training.
//   - A bauble roll that found nothing pays NOTHING, and never makes a room a
//     progression candidate on its own: every room offers bauble rolls, and
//     letting a roll count would turn every room into the secret-exit farm
//     described in the Tier 1 comment above.
//
// Still one award per search, however many tiers found things.
func awardSearch(actor Actor, char *characters.Character, result SearchResult, rolledAgainstSomething bool) {
	if !rolledAgainstSomething && !result.BaubleFound {
		return
	}
	actor.AwardResolved(result.FoundAnything(), char.CandidateFor(string(skills.Search)))
}
