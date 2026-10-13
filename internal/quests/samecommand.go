package quests

import (
	"fmt"
	"slices"
	"strings"
)

// validateSameCommandChains refuses a quest in which one trigger hands out
// what a LATER trigger on the same event requires, when one player command
// can match both.
//
// The engine runs every trigger an event matches, in file order, and judges
// each trigger's conditions against the state as it stands when that trigger
// is reached, so after the earlier ones have acted. A grant in trigger 3 that
// trigger 5 checks with `has` fires trigger 5 in the very command that made
// the grant: quest 14's strongbox handed over the ledger and then said "You
// have already taken the ledger" (#495). The fix is to put the "already done"
// trigger first, where it is judged before the grant.
//
// Checked pairs: grant then has, give_item then has_item, consume_item then
// missing_item, set_flag then has_flag. Only a trigger's own actions count; a
// sequence's on_complete runs rounds later, not in the same command.
func (r *Quest) validateSameCommandChains() error {
	var problems []string
	for i := range r.Triggers {
		a := &r.Triggers[i]
		for j := i + 1; j < len(r.Triggers); j++ {
			b := &r.Triggers[j]
			if a.Event != b.Event || !triggersCanShareAnEvent(a, b) || sameCommandBlocks(a, b.Conditions) {
				continue
			}
			for _, why := range sameCommandEnables(a.Actions, b.Conditions) {
				problems = append(problems, fmt.Sprintf(
					"trigger %d %s, which trigger %d on the same %s requires, so both fire in one command; put trigger %d first",
					i, why, j, a.Event, j))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("quest %d (%s): %s", r.QuestId, r.Name, strings.Join(problems, "; "))
	}
	return nil
}

// triggersCanShareAnEvent reports whether one event can match both triggers'
// filters: for each filter, the two agree or at least one leaves it open.
func triggersCanShareAnEvent(a, b *TriggerDef) bool {
	ints := [][2]int{{a.Room, b.Room}, {a.Mob, b.Mob}, {a.Item, b.Item}}
	for _, p := range ints {
		if p[0] > 0 && p[1] > 0 && p[0] != p[1] {
			return false
		}
	}
	strs := [][2]string{
		{a.Skill, b.Skill}, {a.Command, b.Command}, {a.Topic, b.Topic},
		{a.QuestToken, b.QuestToken}, {a.Noun, b.Noun}, {a.Verb, b.Verb},
	}
	for _, p := range strs {
		if p[0] != "" && p[1] != "" && p[0] != p[1] {
			return false
		}
	}
	return true
}

// sameCommandBlocks reports whether the later trigger cannot fire in the same
// command as a: a's own conditions contradict later's, or a's actions falsify
// one of later's conditions (quest 14's locked-strongbox line needs the key
// gone, which the open trigger consumes, but also needs the ledger not yet
// granted, which the same trigger grants).
func sameCommandBlocks(a *TriggerDef, later Conditions) bool {
	for _, tok := range a.Conditions.Has {
		if slices.Contains(later.Missing, tok) {
			return true
		}
	}
	for _, tok := range a.Conditions.Missing {
		if slices.Contains(later.Has, tok) && !grantsToken(a.Actions, tok) {
			return true
		}
	}
	for _, act := range a.Actions {
		if act.Grant != "" && slices.Contains(later.Missing, act.Grant) {
			return true
		}
		if act.GiveItem > 0 && act.GiveItem == later.MissingItem {
			return true
		}
		if act.ConsumeItem > 0 && act.ConsumeItem == later.HasItem {
			return true
		}
		if act.SetFlag != nil {
			if v, ok := later.MissingFlag[act.SetFlag.Key]; ok && v == act.SetFlag.Value {
				return true
			}
			if v, ok := later.HasFlag[act.SetFlag.Key]; ok && v != act.SetFlag.Value {
				return true
			}
		}
	}
	return false
}

func grantsToken(actions []ActionDef, tok string) bool {
	for _, act := range actions {
		if act.Grant == tok {
			return true
		}
	}
	return false
}

// sameCommandEnables lists what in actions satisfies a condition in later.
func sameCommandEnables(actions []ActionDef, later Conditions) []string {
	var out []string
	for _, act := range actions {
		for _, tok := range later.Has {
			if act.Grant != "" && act.Grant == tok {
				out = append(out, fmt.Sprintf("grants %q", tok))
			}
		}
		if act.GiveItem > 0 && act.GiveItem == later.HasItem {
			out = append(out, fmt.Sprintf("gives item %d", act.GiveItem))
		}
		if act.ConsumeItem > 0 && act.ConsumeItem == later.MissingItem {
			out = append(out, fmt.Sprintf("consumes item %d", act.ConsumeItem))
		}
		if act.SetFlag != nil {
			if v, ok := later.HasFlag[act.SetFlag.Key]; ok && v == act.SetFlag.Value {
				out = append(out, fmt.Sprintf("sets flag %s=%s", act.SetFlag.Key, v))
			}
		}
	}
	return out
}
