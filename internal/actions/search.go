package actions

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/templates"
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
// per-observer lighting (NightVision counts as lit for that observer alone).
func spotsHider(observer *characters.Character, hider *characters.Character, room *rooms.Room) bool {
	return combat.RunContest(
		CalcDetectionScore(observer),
		[]contest.Entry{{Score: CalcSneakScoreVsObserver(hider, observer, room)}},
	).Success
}

// SearchOptions is intentionally empty v1 — in-room search is the only
// mode. Reserved for future "search container" path.
type SearchOptions struct{}

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

	OnCooldown bool
	Reason     string
}

// FoundAnything reports whether the search turned up ANY of its six kinds of
// discovery. It is the win/lose input to U10b-1's progression award.
//
// Derived from the result rather than tracked by a flag set beside each of the
// six append sites: one predicate in one place cannot fall out of step with
// five of its six siblings. ⚠️ A NEW TIER MUST ADD ITS SLICE HERE. That is the
// one thing this shape does not make automatic, and forgetting it makes the new
// tier's finds read as failures -- the search would award the loss fraction
// while telling the player it found something.
func (r SearchResult) FoundAnything() bool {
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

	if !char.TryCooldown("search", "2 rounds") {
		result.OnCooldown = true
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				fmt.Sprintf("You need to wait %d more rounds to do that again.",
					char.GetCooldown("search")))
		}
		return result
	}

	searchScore := CalcSearchScore(char)

	if actor.IsPlayer() {
		actor.SendText(messaging.CategorySystem, "You snoop around for a bit...\n")
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is snooping around.`, char.Name),
			actor.GetUserId(),
		)
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
		details := map[string]any{
			"GroundStuff": stashedNames,
			"IsDark":      room.GetBiome().IsDark(),
			"IsNight":     gametime.IsNight(),
		}
		text, _ := templates.Process("descriptions/ontheground", details, actor.GetUserId())
		actor.SendText(messaging.CategorySystem, text)
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
		if spotsHider(char, p.Character, room) {
			result.HiddenPlayersFound = append(result.HiddenPlayersFound, pId)
			if actor.IsPlayer() {
				hiddenPlayerNames = append(hiddenPlayerNames,
					p.Character.Name+` <ansi fg="black-bold">(hiding)</ansi>`)
			}
		}
	}
	if actor.IsPlayer() && len(hiddenPlayerNames) > 0 {
		details := rooms.GetDetails(room, users.GetByUserId(actor.GetUserId()))
		details.VisiblePlayers = []string{}
		for _, name := range hiddenPlayerNames {
			details.VisiblePlayers = append(details.VisiblePlayers,
				characters.FormattedName{Name: name, Type: "username", Suffix: "hidden"}.String())
		}
		text, _ := templates.Process("descriptions/who", details, actor.GetUserId())
		actor.SendText(messaging.CategorySystem, text)
	}

	// ── Tier 2 (target 135): Hidden mobs ────────────────────────
	hiddenMobNames := []string{}
	for _, mId := range room.GetMobs() {
		m := mobs.GetInstance(mId)
		if m == nil || !m.Character.IsHidden() {
			continue
		}
		rolledAgainstSomething = true
		if spotsHider(char, &m.Character, room) {
			result.HiddenMobsFound = append(result.HiddenMobsFound, mId)
			if actor.IsPlayer() {
				hiddenMobNames = append(hiddenMobNames,
					m.Character.Name+` <ansi fg="black-bold">(hiding)</ansi>`)
			}
		}
	}
	if actor.IsPlayer() && len(hiddenMobNames) > 0 {
		details := rooms.GetDetails(room, users.GetByUserId(actor.GetUserId()))
		details.VisibleMobs = []string{}
		for _, name := range hiddenMobNames {
			details.VisibleMobs = append(details.VisibleMobs,
				characters.FormattedName{Name: name, Type: "mob", Suffix: "hidden"}.String())
		}
		text, _ := templates.Process("descriptions/who", details, actor.GetUserId())
		actor.SendText(messaging.CategorySystem, text)
	}

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
	if rolledAgainstSomething {
		actor.AwardResolved(result.FoundAnything(), char.CandidateFor(string(skills.Search)))
	}

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
