package actions

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

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
		rolledAgainstSomething = true
		roll := dice.RollStat(searchScore)
		if roll.Value >= 125.0 {
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
		roll := dice.RollStat(searchScore)
		if roll.Value >= 125.0 {
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
		roll := dice.RollStat(searchScore)
		if roll.Value >= 135.0 {
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

	// NOTE(unassigned, see UNIFIED_RESOLUTION_ROADMAP "Category B"): the six
	// dice.RollStat threshold checks in this file are the LAST uncertain
	// outcomes off the contest core. The two below are the sharpest problem:
	// they answer "does the observer spot the hider?" with a flat 135 threshold
	// that never reads the hider's sneak score, while
	// usercommands/go.go resolves the SAME question as an opposed contest
	// (observerScore vs hiddenScore). A hider's skill decides the outcome in one
	// path and is ignored in the other. Mobs reach this path too, via
	// behaviortree/actions_scout.go's actTrySearch, gated by the cheap
	// condRoomHasHiddenEntity pre-check in conditions_scout.go.
	//
	// U4 migrated go.go's opposed version and deliberately did NOT touch these:
	// converting a flat threshold into a contest is a behaviour change, and
	// U1-U5 are provable no-ops. Whichever chunk claims them must reconcile the
	// two implementations, not just move one.

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
		roll := dice.RollStat(searchScore)
		if roll.Value >= 135.0 {
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
		roll := dice.RollStat(searchScore)
		if roll.Value >= 135.0 {
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
		roll := dice.RollStat(searchScore)
		if roll.Value >= 175.0 {
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

	return result
}
