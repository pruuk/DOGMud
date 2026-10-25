package questloop

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// way is one thing in the world that grants a quest token.
type way struct {
	where   string             // "quest 53 trigger mob_death mob=9110 grants=53-firstkill", "quest 53 rewards questid", "mob 9112 topic cull_in"
	trigger *quests.TriggerDef // set for a trigger
	owner   int                // the quest whose file holds the trigger
	nested  bool               // granted inside a sequence's on_complete
	topic   *dialogue.Topic    // set for a topic
	mobId   int                // the topic's NPC
	reward  bool               // rewards.questid
	// external is set for a behaviour-tree grant or an item questtoken. It
	// is a way in like any other, so a step only it grants is not a
	// one-action :none finding. The other rules treat it as follows:
	//   - start and end: it is neither an offer nor a hand-in, so a grant of
	//     a quest's start or end token is a start or end finding, and it
	//     never stands in for the missing offer or hand-in.
	//   - one-action :prev, :cond, :seq, :double: not checked. It carries no
	//     trigger conditions to read; its gate is the topic that fires the
	//     tree (or the room or item event behind it), or getting the item.
	//     The walkthrough checks those gates (questengine.Walk).
	external *ExternalGrant
}

// grants maps a token ("53-firstkill") to every way that grants it.
type grants map[string][]way

func indexGrants(w World) grants {
	g := grants{}
	for _, q := range w.Quests {
		if q == nil {
			continue
		}
		wheres := triggerWheres(q)
		for i := range q.Triggers {
			t := &q.Triggers[i]
			where := wheres[i]
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
	for i := range w.External {
		x := &w.External[i]
		if x.Token == "" || strings.HasPrefix(x.Token, "-") {
			continue
		}
		g[x.Token] = append(g[x.Token], way{where: x.Where, mobId: x.MobId, external: x})
	}
	return g
}

// triggerWheres names each of q's triggers by a signature that does not
// depend on its position: "quest 53 trigger mob_death mob=9110
// grants=53-firstkill", the event, its non-zero filters in a fixed order and
// the tokens it grants (sorted, sequences included). Triggers that share a
// signature take " #2", " #3" in file order.
func triggerWheres(q *quests.Quest) []string {
	out := make([]string, len(q.Triggers))
	seen := map[string]int{}
	for i := range q.Triggers {
		sig := triggerSignature(&q.Triggers[i])
		seen[sig]++
		where := fmt.Sprintf("quest %d trigger %s", q.QuestId, sig)
		if n := seen[sig]; n > 1 {
			where += fmt.Sprintf(" #%d", n)
		}
		out[i] = where
	}
	return out
}

func triggerSignature(t *quests.TriggerDef) string {
	parts := []string{t.Event}
	addInt := func(name string, v int) {
		if v != 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", name, v))
		}
	}
	addStr := func(name, v string) {
		if v != "" {
			parts = append(parts, name+"="+v)
		}
	}
	addInt("room", t.Room)
	addInt("mob", t.Mob)
	addInt("item", t.Item)
	addStr("skill", t.Skill)
	addStr("command", t.Command)
	addStr("quest_token", t.QuestToken)
	addStr("noun", t.Noun)
	addStr("verb", t.Verb)
	var toks []string
	walkGrants(t.Actions, false, func(tok string, _ bool) {
		if !slices.Contains(toks, tok) {
			toks = append(toks, tok)
		}
	})
	if len(toks) > 0 {
		sort.Strings(toks)
		parts = append(parts, "grants="+strings.Join(toks, ","))
	}
	return strings.Join(parts, " ")
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
