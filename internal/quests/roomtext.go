package quests

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// RoomTextProblems returns every way a quest room_text breaks the quest
// convention, or nil.
//
// Every quest room_text is something the room watches the triggering player
// do, so it must name them with {source}. Until 2026-09-11 twenty-one lines
// were written as subjectless fragments ("unlocks the strongbox") and one used
// {source} that nothing filled in.
//
// It is stricter than buffs and spells, which only WARN on an unknown token.
// A new rule with no shipped violations can refuse at no cost; upgrading buffs
// and spells would change what is allowed to boot, so that is filed.
func RoomTextProblems(text string) []string {
	var problems []string
	if !strings.Contains(text, "{source}") {
		problems = append(problems, "must name the acting player with {source}; the room is watching them act")
	}
	for _, token := range []string{"{target}", "{target_plain}"} {
		if strings.Contains(text, token) {
			problems = append(problems, token+" is not available: a quest has no target, so it would render empty")
		}
	}
	if strings.Contains(text, "{source_plain}") {
		problems = append(problems, "{source_plain} is an untagged name that cannot be anonymized in the dark; use {source}")
	}
	problems = append(problems, textutil.ValidateTokens(text)...)
	return problems
}

// validateRoomText applies RoomTextProblems to every room_text in the quest,
// including actions nested in a sequence's on_complete, which the engine
// executes and narrates too.
//
// It lives in Validate on purpose. Validate runs on every quest file parse at
// boot AND before an admin editor save (modules/gmcp buildQuestUpdate), so a
// bad line is refused at save with a reply. The first version of this rule was
// a questengine boot check the editor never ran: a bad line saved to disk, the
// reindex panicked into a recovered listener with no reply, and the next cold
// boot failed.
func (r *Quest) validateRoomText() error {
	var problems []string
	var walk func(where string, actions []ActionDef, depth int)
	walk = func(where string, actions []ActionDef, depth int) {
		for j, a := range actions {
			aw := fmt.Sprintf("%s action %d", where, j)
			if a.RoomText != "" {
				for _, p := range RoomTextProblems(a.RoomText) {
					problems = append(problems, aw+" room_text: "+p)
				}
			}
			// The engine nests exactly one level; the guard mirrors validate_refs.go.
			if a.Sequence != nil && depth < 2 {
				walk(aw+" sequence on_complete", a.Sequence.OnComplete, depth+1)
			}
		}
	}
	for i, t := range r.Triggers {
		walk(fmt.Sprintf("trigger %d", i), t.Actions, 0)
	}
	if len(problems) > 0 {
		return fmt.Errorf("quest %d (%s): %s", r.QuestId, r.Name, strings.Join(problems, "; "))
	}
	return nil
}
