// Package questloop holds the quest arc's loop validators (#375 phase 2):
// every quest starts at an NPC offer, ends at an NPC hand-in, hints each
// step, moves one action at a time, chains at its hand-in NPC, and declares
// repeats and reward overrides honestly. It is pure: it reads a proposed
// world and reports findings, so boot, CI and the quest editor run the same
// rules. internal/questworld builds the world from the live registries.
package questloop

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// World is everything the loop rules read. Refs and Names may be zero in
// tests: a nil callback inside them skips that check.
type World struct {
	Quests   []*quests.Quest
	Dialogue []*dialogue.DialogueFile
	Refs     *quests.QuestValidators // nil skips the refs rule
	Names    quests.RewardNames
}

// Finding is one rule failure. Key (rule, quest, where) is its backlog
// line; Msg explains it and may be reworded without touching the backlog.
type Finding struct {
	Rule    string
	QuestId int
	Where   string
	Msg     string
}

// Key is the finding's backlog line: "rule questId where".
func (f Finding) Key() string { return fmt.Sprintf("%s %d %s", f.Rule, f.QuestId, f.Where) }

func (f Finding) String() string { return f.Key() + ": " + f.Msg }

// Validate runs every rule over w, sorted by key.
func Validate(w World) []Finding {
	g := indexGrants(w)
	var out []Finding
	for _, q := range w.Quests {
		if q == nil || len(q.Steps) == 0 {
			continue // Quest.Validate refuses these at load
		}
		out = append(out, ruleStart(q, g)...)
		out = append(out, ruleEnd(q, g)...)
		out = append(out, ruleHint(q)...)
		out = append(out, ruleOneAction(q, g)...)
		out = append(out, ruleRepeat(q)...)
		out = append(out, ruleReward(q, w.Names)...)
		out = append(out, ruleRefs(q, w.Refs)...)
	}
	out = append(out, ruleChain(w)...)
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

func token(q *quests.Quest, i int) string { return fmt.Sprintf("%d-%s", q.QuestId, q.Steps[i].Id) }

func stepWhere(q *quests.Quest, i int) string { return fmt.Sprintf("step %d (%s)", i, q.Steps[i].Id) }
