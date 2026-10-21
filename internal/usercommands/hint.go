package usercommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// hintNoQuestText is the one line every "nothing to hint at" path sends (owner
// ruling 2026-10-21, #375): no current quest, so say how to find one. [!] is
// menu.MarkerQuest, the mark the room list prints beside a quest giver.
const hintNoQuestText = `You don't have any current quests. Look for someone with [!] next to their name. That mark means they have a quest for you. Type <ansi fg="command">talk &lt;name&gt;</ansi> to speak with them.`

func Hint(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	allProgress := user.Character.GetQuestProgress()

	// Secret quests are hidden markers that `quests` never lists, so hint must
	// not name one by any route (no argument, a number, or a partial name).
	for questId := range allProgress {
		if characters.IsSecretQuest(questId) {
			delete(allProgress, questId)
		}
	}

	if len(allProgress) == 0 {
		user.SendText(messaging.CategoryTip, hintNoQuestText)
		return true, nil
	}

	rest = strings.TrimSpace(rest)

	// Resolve target quest ID from argument or last-progressed fallback.
	targetQuestId := 0

	if rest == "" {
		// Default: the focused quest — LastQuestId when it's still in progress,
		// else a deterministic fallback to an active quest (shared with the web
		// Quests panel + minimap marker so all three agree on one focus).
		targetQuestId = user.Character.GetFocusQuestId()
		if _, ok := allProgress[targetQuestId]; !ok {
			user.SendText(messaging.CategoryTip, hintNoQuestText)
			return true, nil
		}
	} else if numId, err := strconv.Atoi(rest); err == nil {
		// Numeric ID provided.
		if _, ok := allProgress[numId]; !ok {
			user.SendText(messaging.CategoryTip, `You don't have an active quest by that name.`)
			return true, nil
		}
		targetQuestId = numId
	} else {
		// Partial name match — case-insensitive.
		engine := questengine.GetEngine()
		searchLower := strings.ToLower(rest)
		for qid := range allProgress {
			qDef := engine.GetQuest(qid)
			if qDef == nil {
				continue
			}
			if strings.Contains(strings.ToLower(qDef.Name), searchLower) {
				targetQuestId = qid
				break
			}
		}
		if targetQuestId == 0 {
			user.SendText(messaging.CategoryTip, `You don't have an active quest by that name.`)
			return true, nil
		}
	}

	engine := questengine.GetEngine()
	qDef := engine.GetQuest(targetQuestId)
	if qDef == nil {
		user.SendText(messaging.CategoryTip, hintNoQuestText)
		return true, nil
	}

	currentStep := allProgress[targetQuestId]

	// Completed quests have step "end".
	if currentStep == "end" {
		user.SendText(messaging.CategoryTip, fmt.Sprintf(
			`<ansi fg="yellow">Quest Hint</ansi> <ansi fg="yellow-bold">(%s)</ansi>: <ansi fg="white-bold">This quest is complete!</ansi>`,
			qDef.Name,
		))
		return true, nil
	}

	// Find the hint for the current step (what the player should do now).
	hintText := ""
	if currentStep == "" {
		if len(qDef.Steps) > 0 {
			hintText = qDef.Steps[0].Hint
		}
	} else {
		for _, step := range qDef.Steps {
			if step.Id == currentStep {
				hintText = step.Hint
				break
			}
		}
	}

	if hintText == "" {
		hintText = "No hint available for this step."
	}

	user.SendText(messaging.CategoryTip, fmt.Sprintf(
		`<ansi fg="yellow">Quest Hint</ansi> <ansi fg="yellow-bold">(%s)</ansi>: <ansi fg="white-bold">%s</ansi>`,
		qDef.Name,
		hintText,
	))

	return true, nil
}
