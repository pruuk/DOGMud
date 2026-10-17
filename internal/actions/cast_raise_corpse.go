package actions

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Raise-corpse selection, shared by both raise gates: cast start
// (usercommands.Cast) and resolution (hooks.resolveCompanionSummon). Selection
// spends nothing, so both gates run it before the reservation gate and before
// any component is consumed (#282): a raise with no usable corpse names the
// corpse, not a reservation the player would free up only to fail again.

// RaiseSkipReason explains why no corpse could be raised, so the refusal can
// be specific and actionable instead of a blanket "no remains".
type RaiseSkipReason int

const (
	RaiseNoMatch    RaiseSkipReason = iota // no name/type-matching corpse present
	RaiseWasCharmed                        // matched, but it was already a companion
	RaiseTooWeak                           // matched, but its essence is too faint
	RaiseHasLoot                           // matched, but it still holds loot
)

// raisePriority ranks reasons by how actionable they are; the most actionable
// (HasLoot: loot it and retry) wins when several near-matches apply.
func raisePriority(r RaiseSkipReason) int {
	switch r {
	case RaiseHasLoot:
		return 3
	case RaiseTooWeak:
		return 2
	case RaiseWasCharmed:
		return 1
	default:
		return 0
	}
}

func worseRaiseReason(a, b RaiseSkipReason) RaiseSkipReason {
	if raisePriority(b) > raisePriority(a) {
		return b
	}
	return a
}

// corpseRaisePool is the stat metric the raise spell scores a corpse by, the
// same number `assess` reports, so the two agree on a corpse's worth. Both
// call StatPoolTotal rather than keeping two copies of the same sum in step.
func corpseRaisePool(c rooms.Corpse) int {
	return c.Character.StatPoolTotal()
}

// ParseRaiseTarget splits a raise spell's target text into a name filter and
// a 1-based index among valid corpses: "corpse", "2.corpse", "corpse#2". A
// generic word (corpse, remains, body, bones) matches by index, not by name.
// The filter is the word with its index stripped, so "2.goblin" filters on
// "goblin" (it used to filter on "2.goblin", which no corpse name holds).
func ParseRaiseTarget(spellRest string) (nameFilter string, targetIndex int) {
	targetWord, targetIndex := util.GetMatchNumber(spellRest)
	genericTargets := map[string]bool{"corpse": true, "remains": true, "body": true, "corpses": true, "bones": true, "": true}
	if genericTargets[targetWord] {
		return "", targetIndex
	}
	return targetWord, targetIndex
}

// SelectRaiseCorpse picks the targetIndex-th valid mob corpse to raise (1-based
// among valid candidates). Returns (-1, 0, reason) when none qualifies, where
// reason describes why the best name/type-matching corpse was rejected, so the
// caller can show a helpful refusal. minPool <= 0 disables the stat-pool gate.
func SelectRaiseCorpse(corpses []rooms.Corpse, nameFilter string, targetIndex, minPool int) (idx int, pool int, reason RaiseSkipReason) {
	reason = RaiseNoMatch
	validCount := 0
	for i, c := range corpses {
		// Not a raisable mob corpse at all: never counts as a near-match.
		if c.Prunable || c.UserId != 0 {
			continue
		}
		// Name filter: a non-matching corpse isn't what the player asked for.
		if nameFilter != "" && !strings.Contains(strings.ToLower(c.Character.Name), strings.ToLower(nameFilter)) {
			continue
		}
		// From here the corpse matched name/type; rejections carry a reason.
		if c.WasCharmed {
			reason = worseRaiseReason(reason, RaiseWasCharmed)
			continue
		}
		if c.HasLoot() {
			reason = worseRaiseReason(reason, RaiseHasLoot)
			continue
		}
		p := corpseRaisePool(c)
		if minPool > 0 && p < minPool {
			reason = worseRaiseReason(reason, RaiseTooWeak)
			continue
		}
		validCount++
		if validCount == targetIndex {
			return i, p, RaiseNoMatch
		}
	}
	return -1, 0, reason
}

// RaiseCorpseRefusal is the refusal line for a raise that found no corpse.
func RaiseCorpseRefusal(reason RaiseSkipReason, nameFilter string) string {
	switch reason {
	case RaiseHasLoot:
		return refusal.RemainsHoldLootLine
	case RaiseTooWeak:
		return refusal.RemainsTooFaintLine
	case RaiseWasCharmed:
		return refusal.RemainsAlreadyRaisedLine
	default:
		if nameFilter != "" {
			return refusal.NoRemainsMatchingLine
		}
		return refusal.NoRemainsLine
	}
}

// FindRaiseCorpse is the one corpse gate both raise sites run. It returns the
// chosen corpse's index in room.Corpses and its pool, or idx -1 and the
// refusal line to send. It changes nothing; the caller removes the corpse
// only once every other gate has passed.
func FindRaiseCorpse(room *rooms.Room, spellInfo *spells.SpellData, spellRest string) (idx int, pool int, refusalLine string) {
	var corpses []rooms.Corpse
	if room != nil {
		corpses = room.Corpses
	}
	nameFilter, targetIndex := ParseRaiseTarget(spellRest)
	idx, pool, reason := SelectRaiseCorpse(corpses, nameFilter, targetIndex, int(spellInfo.SummonMinCorpsePool))
	if idx < 0 {
		return -1, 0, RaiseCorpseRefusal(reason, nameFilter)
	}
	return idx, pool, ""
}
