package conversation

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// cardNames are the reward names for c's offer card: the live registry
// names, plus SkillMet from c's skill levels, read the way the completion
// payout reads them (Quest_HandleQuestUpdate), so the card never promises
// training a repeat run would not pay.
func cardNames(c *characters.Character) quests.RewardNames {
	n := questworld.Names()
	if c != nil {
		n.SkillMet = func(skill string, level int) bool {
			return c.GetSkillLevel(skills.SkillTag(skill)) >= level
		}
	}
	return n
}

// The repeat lines on an offer card (#375 P6). No round count: the
// player-copy rule.
const (
	cardRepeatLine = "You can take this on again after a while."
	cardLastLine   = "This is the last time this work will be offered to you."
)

// cardBody is the offer card's body (spec 3, 4.3): the quest's first-step
// description as the objective, the reward line, and for a repeatable the
// repeat line. lastRun picks the last-time line.
func cardBody(q *quests.Quest, names quests.RewardNames, lastRun bool) []string {
	if q == nil {
		return nil
	}
	var body []string
	if len(q.Steps) > 0 && q.Steps[0].Description != "" {
		body = append(body, q.Steps[0].Description)
	}
	if r := quests.RewardSummary(q, names); r != "" {
		if last := r[len(r)-1]; last != '.' && last != '!' && last != '?' {
			r += "."
		}
		body = append(body, "Reward: "+r)
	}
	if q.Repeatable {
		if lastRun {
			body = append(body, cardLastLine)
		} else {
			body = append(body, cardRepeatLine)
		}
	}
	return body
}
