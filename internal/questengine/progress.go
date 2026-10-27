package questengine

import (
	"fmt"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// defaultProgressLabel names a progress line whose trigger sets no label.
const defaultProgressLabel = "Progress"

// StepProgress is the progress of a step reached by a count or distinct
// trigger (#375 phase 3b slice 4), for the quest log and the web tracker:
// the first trigger of the quest that needs the step's token and counts
// (count: N) or explores (distinct). have is the step's saved count, read
// through counts with the step's token as the key, or the number of the
// area's rooms on the player's map, never more than need. ok is false for a
// step no such trigger leads from, and for an explore step when the player
// state cannot read a map.
func (e *Engine) StepProgress(questId int, step string, player PlayerState, counts func(key string) int) (label string, have, need int, ok bool) {
	q := e.quests[questId]
	if q == nil {
		return "", 0, 0, false
	}
	token := quests.PartsToToken(questId, step)
	for i := range q.Triggers {
		t := &q.Triggers[i]
		if (t.Count < 2 && t.Distinct == "") || !slices.Contains(t.Conditions.Has, token) {
			continue
		}
		label = t.Label
		if label == "" {
			label = defaultProgressLabel
		}
		if t.Distinct == "" {
			return label, min(counts(token), t.Count), t.Count, true
		}
		mr, isMap := player.(MapReader)
		rs := areas.Rooms(t.Area)
		n, valid := quests.DistinctNeed(t.Distinct, len(rs))
		if !isMap || len(rs) == 0 || !valid {
			return "", 0, 0, false
		}
		return label, min(mr.MappedCount(rs), n), n, true
	}
	return "", 0, 0, false
}

// ProgressLine is the player-facing progress line: "Progress: 2 of 5".
func ProgressLine(label string, have, need int) string {
	return fmt.Sprintf("%s: %d of %d", label, have, need)
}

// UserStepProgress is the progress line of a player's step of questId, or ""
// when the step has none. It reads the character's saved counts and map.
func (e *Engine) UserStepProgress(user *users.UserRecord, questId int, step string) string {
	label, have, need, ok := e.StepProgress(questId, step, NewGameBridge(user, user.Character.RoomId), user.Character.QuestStepCount)
	if !ok {
		return ""
	}
	return ProgressLine(label, have, need)
}
