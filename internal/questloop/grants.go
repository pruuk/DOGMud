package questloop

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// way is one thing in the world that grants a quest token.
type way struct {
	where   string             // "quest 53 trigger 1", "quest 53 rewards questid", "mob 9112 topic cull_in"
	trigger *quests.TriggerDef // set for a trigger
	owner   int                // the quest whose file holds the trigger
	nested  bool               // granted inside a sequence's on_complete
	topic   *dialogue.Topic    // set for a topic
	mobId   int                // the topic's NPC
	reward  bool               // rewards.questid
}

// grants maps a token ("53-firstkill") to every way that grants it.
type grants map[string][]way

func indexGrants(w World) grants {
	g := grants{}
	for _, q := range w.Quests {
		if q == nil {
			continue
		}
		for i := range q.Triggers {
			t := &q.Triggers[i]
			where := fmt.Sprintf("quest %d trigger %d", q.QuestId, i)
			walkGrants(t.Actions, false, func(tok string, nested bool) {
				g[tok] = append(g[tok], way{where: where, trigger: t, owner: q.QuestId, nested: nested})
			})
		}
		if tok := q.Rewards.QuestId; tok != "" && !strings.HasPrefix(tok, "-") {
			g[tok] = append(g[tok], way{where: fmt.Sprintf("quest %d rewards questid", q.QuestId), owner: q.QuestId, reward: true})
		}
	}
	for _, df := range w.Dialogue {
		if df == nil {
			continue
		}
		for i := range df.Topics {
			t := &df.Topics[i]
			if t.GrantsQuest == "" || strings.HasPrefix(t.GrantsQuest, "-") {
				continue
			}
			g[t.GrantsQuest] = append(g[t.GrantsQuest], way{
				where: fmt.Sprintf("mob %d topic %s", df.MobId, t.Key), topic: t, mobId: df.MobId})
		}
	}
	return g
}

// walkGrants visits every grant action, sequences included at any depth.
// A "-" grant removes a token and is not a grant.
func walkGrants(actions []quests.ActionDef, nested bool, fn func(tok string, nested bool)) {
	for _, a := range actions {
		if a.Grant != "" && !strings.HasPrefix(a.Grant, "-") {
			fn(a.Grant, nested)
		}
		if a.Sequence != nil {
			walkGrants(a.Sequence.OnComplete, true, fn)
		}
	}
}

// ownGrants counts the grants a trigger makes of its own quest's tokens.
func ownGrants(t *quests.TriggerDef, questId int) int {
	n := 0
	prefix := fmt.Sprintf("%d-", questId)
	walkGrants(t.Actions, false, func(tok string, _ bool) {
		if strings.HasPrefix(tok, prefix) {
			n++
		}
	})
	return n
}
