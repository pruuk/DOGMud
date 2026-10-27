package questengine

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// MapReader is implemented by player states that can read the player's map
// (#375 phase 3b slice 4): how many of these template rooms the player has
// mapped. Explore steps need it; a player state without it never satisfies
// one.
type MapReader interface {
	MappedCount(roomIds []int) int
}

// An explore step is a room_enter trigger with area and distinct. It is
// never matched on an event (matchTriggerFields): its progress is the number
// of the area's rooms on the player's map, read at two moments only, when a
// room of the area is newly mapped (CheckExplore) and when its step starts
// (exploreAtStepStart, inside the quest_granted chain). Rooms mapped before
// the quest count; a room crossed in the dark does not, since it is not on
// the map.

// exploreMet reports whether the player's map holds enough of the trigger's
// area.
func exploreMet(t *indexedTrigger, player PlayerState) bool {
	mr, ok := player.(MapReader)
	if !ok {
		return false
	}
	rs := areas.Rooms(t.def.Area)
	if len(rs) == 0 {
		return false
	}
	need, ok := quests.DistinctNeed(t.def.Distinct, len(rs))
	if !ok {
		return false
	}
	return mr.MappedCount(rs) >= need
}

// exploreReady is the cheap test first (conditions), then the map read.
func exploreReady(t *indexedTrigger, player PlayerState) bool {
	return EvalConditions(t.def.Conditions, player) && exploreMet(t, player)
}

// CheckExplore is raised when roomId was newly mapped for the player
// (NotifyRoomMapped). Every explore trigger of every area holding the room
// whose conditions pass and whose area is mapped enough fires, through the
// same path as any trigger (fire), under one fresh guard.
func (e *Engine) CheckExplore(roomId int, player PlayerState, ctx ActionContext) NotifyResult {
	roomId, _ = rooms.OriginalRoomId(roomId)
	keys := areas.AreasOf(roomId)
	if len(keys) == 0 {
		return NotifyResult{}
	}
	userId := ctx.GetUserId()
	run := beginGuarded()
	guard := NewEvalGuard(run.maxDepth)
	defer run.end(guard, "explore", userId)
	result := NotifyResult{}
	if !guard.PushDepth() {
		return result
	}
	defer guard.PopDepth()
	d := EventDetails{UserId: userId, RoomId: roomId}
	pw, _ := ctx.(ProgressWatcher)
	for _, key := range keys {
		for _, t := range e.exploreByArea[key] {
			if !EvalConditions(t.def.Conditions, player) {
				continue
			}
			// The step is live and one more of its area is mapped, so its
			// progress line moved whether or not it is met.
			if pw != nil {
				pw.QuestProgressChanged(t.questId)
			}
			if exploreMet(t, player) {
				result.merge(e.fire("explore", t, d, player, ctx, guard, nil))
			}
		}
	}
	return result
}

// ProgressWatcher is implemented by action contexts that show quest progress
// (GameBridge: the web tracker). CheckExplore tells it when a live explore
// step's mapped count moved; a count's bump is the StepCounter's own news.
type ProgressWatcher interface {
	QuestProgressChanged(questId int)
}

// exploreAtStepStart checks the granted quest's explore triggers as a step
// starts: one whose area is already mapped fires at once, telling the player
// why (its explored_line, else the default line naming the area). It runs
// inside evaluate, so the chain it may start is bounded by the guard's depth
// limit and visit marks like any other.
func (e *Engine) exploreAtStepStart(d EventDetails, player PlayerState, ctx ActionContext, guard *EvalGuard) NotifyResult {
	questId, _ := quests.TokenToParts(d.QuestToken)
	result := NotifyResult{}
	for _, t := range e.exploreByQuest[questId] {
		if !exploreReady(t, player) {
			continue
		}
		line := exploredLine(t.def)
		result.merge(e.fire("explore", t, d, player, ctx, guard, func() {
			ctx.Narrate(narration.Variants{Actor: textutil.Pool(line)})
		}))
	}
	return result
}

// exploredLine is what the player is told when an explore step starts with
// its area already mapped. The default line claims every corner only for
// distinct: all; a step needing some of the area says enough was seen.
func exploredLine(t *TriggerDef) string {
	if t.ExploredLine != "" {
		return t.ExploredLine
	}
	name := t.Area
	if a, ok := areas.Get(t.Area); ok && a.Name != "" {
		name = a.Name
	}
	if t.Distinct != quests.DistinctAll {
		return fmt.Sprintf("You have already seen enough of %s.", name)
	}
	return fmt.Sprintf("You already know every corner of %s.", name)
}
