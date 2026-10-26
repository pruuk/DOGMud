package questengine

import "sort"

// WouldFire reports whether Notify(eventType, d, player, ...) would fire at
// least one trigger, without running anything: no action executes, no token
// is granted, no text is sent and no quest_granted chain is followed. It uses
// the same match step as Notify (fieldMatches, then EvalConditions) against
// the player's current state.
//
// It answers for the first trigger to fire only. A trigger that would become
// eligible only after an earlier one's actions changed the player's state is
// not seen, which is the right answer for a gate asked before the event.
func (e *Engine) WouldFire(eventType string, d EventDetails, player PlayerState) bool {
	for _, t := range e.fieldMatches(eventType, d) {
		if EvalConditions(t.def.Conditions, player) {
			return true
		}
	}
	return false
}

// SearchFeatures returns the sorted, distinct nouns of room_interact triggers
// with `verb: search` whose Room is roomId. These are the things a player can
// search in that room for a quest, whatever the triggers' conditions.
func (e *Engine) SearchFeatures(roomId int) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range e.triggerIndex["room_interact"] {
		if t.def.Verb != "search" || t.def.Room != roomId || t.def.Noun == "" {
			continue
		}
		if seen[t.def.Noun] {
			continue
		}
		seen[t.def.Noun] = true
		out = append(out, t.def.Noun)
	}
	sort.Strings(out)
	return out
}
