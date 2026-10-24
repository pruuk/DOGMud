package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// walkWorld is Cull the Wash as one clean loop: Garve offers it, a kill moves
// it on, Garve takes the hand-in.
func walkWorld() questloop.World {
	q := &quests.Quest{
		QuestId: 53, Name: "Cull the Wash",
		Steps: []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "firstkill", Hint: "h"}, {Id: "end"}},
		Triggers: []quests.TriggerDef{{
			Event: "mob_death", Mob: 9110,
			Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-firstkill"}},
			Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
		}},
	}
	df := &dialogue.DialogueFile{MobId: 9112, Topics: []dialogue.Topic{
		{Key: "cull_offer", Kind: dialogue.KindOffer, Quest: 53, Label: "Cull", Text: "t",
			Effects: dialogue.Effects{GrantsQuest: "53-start"}},
		{Key: "cull_in", Kind: dialogue.KindHandIn, Quest: 53, Label: "Cull", Text: "t",
			Gates: dialogue.Gates{QuestRequired: []string{"53-firstkill"}}, Effects: dialogue.Effects{GrantsQuest: "53-end"}},
	}}
	return questloop.World{Quests: []*quests.Quest{q}, Dialogue: []*dialogue.DialogueFile{df}}
}

func keys(fs []questloop.Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Key())
	}
	return out
}

func TestWalk_CleanQuestFinishes(t *testing.T) {
	require.Empty(t, Walk(walkWorld(), questloop.Sources{}))
}

func TestWalk_UnreachableStepIsADeadEnd(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Conditions.Has = []string{"53-end"} // can never hold end before firstkill
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 step firstkill")
	require.Contains(t, got, "walk-dead-end 53 quest 53 trigger mob_death mob=9110 grants=53-firstkill",
		"the walk must not hold the quest's own later step to meet the condition")
}

func TestWalk_SkippingAStepIsReported(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Grant: "53-end"}}
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "skill_use", Skill: "brawling",
		Conditions: quests.Conditions{Has: []string{"53-start"}},
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
	})
	fs := Walk(w, questloop.Sources{})
	require.NotEmpty(t, fs)
	require.Equal(t, "walk-dead-end 53 quest 53 trigger mob_death mob=9110 grants=53-end", fs[0].Key())
	require.True(t, strings.Contains(fs[0].Msg, "skips"), fs[0].String())
}

func TestWalk_SkipInsideOneFiringIsReported(t *testing.T) {
	// The kill grants end before firstkill: firstkill is never held.
	w := walkWorld()
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Grant: "53-end"}, {Grant: "53-firstkill"}}
	fs := Walk(w, questloop.Sources{})
	require.NotEmpty(t, fs)
	require.Contains(t, keys(fs), "walk-dead-end 53 quest 53 trigger mob_death mob=9110 grants=53-end,53-firstkill")
	require.True(t, strings.Contains(fs[0].Msg, "skips"), fs[0].String())
}

func TestWalk_BrokenAlternativeIsNamed(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "room_enter", Room: 5,
		Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-start"}}, // impossible
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
	})
	got := keys(Walk(w, questloop.Sources{}))
	require.Len(t, got, 1, "only the broken alternative; the kill still works")
	require.Equal(t, "walk-dead-end 53 quest 53 trigger room_enter room=5 grants=53-firstkill", got[0])
}

func TestWalk_HandInGateTheOfferCannotMeet(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics[1].QuestExcluded = []string{"53-firstkill"} // hand-in hides itself
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 step end")
	require.Contains(t, got, "walk-dead-end 53 mob 9112 topic cull_in")
}

func TestWalk_QuestGrantedChainOffTheOfferFinishes(t *testing.T) {
	// firstkill is granted the moment the offer grants start, through a
	// quest_granted trigger: in the game the hook chains it, so the walk must.
	w := walkWorld()
	w.Quests[0].Triggers = []quests.TriggerDef{{
		Event: "quest_granted", QuestToken: "53-start",
		Actions: []quests.ActionDef{{Grant: "53-firstkill"}},
	}}
	require.Empty(t, Walk(w, questloop.Sources{}))

	// The offer's own firing lands on firstkill: the chain ran inside it.
	// (Walk alone cannot show this, since it also fires the quest_granted
	// Way from start directly and lands in the same place.)
	eng := NewEngine()
	eng.RegisterQuest(w.Quests[0])
	p := newWalkPlayer(map[int][]string{53: {"start", "firstkill", "end"}})
	offer := questloop.WaysInto(w)["53-start"][0]
	trail := fireWay(eng, p, offer, 53)
	require.Equal(t, "firstkill", p.progress[53])
	require.True(t, trail["53-start"] && trail["53-firstkill"], "%v", trail)

	// Without the trigger, firstkill is dead.
	w.Quests[0].Triggers = nil
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 step firstkill")
}

func TestWalk_ChainThatSkipsAStepIsReported(t *testing.T) {
	// Taking the offer chains straight to end: firstkill is never held.
	w := walkWorld()
	w.Quests[0].Triggers = []quests.TriggerDef{{
		Event: "quest_granted", QuestToken: "53-start",
		Actions: []quests.ActionDef{{Grant: "53-end"}},
	}}
	fs := Walk(w, questloop.Sources{})
	require.Len(t, fs, 1, "%v", fs)
	require.Equal(t, "walk-dead-end 53 mob 9112 topic cull_offer", fs[0].Key())
	require.Contains(t, fs[0].Msg, "skips to 53-end, passing 53-firstkill")
}

func TestWalk_PrerequisiteOfAnotherQuestIsHeld(t *testing.T) {
	// The offer needs quest 7 finished and the kill needs quest 8 started:
	// both are prerequisites the walk holds, so the quest finishes.
	w := walkWorld()
	w.Quests = append(w.Quests, &quests.Quest{QuestId: 7, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}},
		&quests.Quest{QuestId: 8, Steps: []quests.QuestStep{{Id: "start"}, {Id: "end"}}})
	w.Dialogue[0].Topics[0].QuestRequired = []string{"7-end"}
	w.Quests[0].Triggers[0].Conditions.Has = []string{"53-start", "8-start"}
	require.Empty(t, Walk(w, questloop.Sources{}))
}

func TestWalk_OfferNeedingItsOwnStepIsADeadEnd(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics[0].QuestRequired = []string{"53-firstkill"}
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 mob 9112 topic cull_offer")
}

func srcWith(items map[int][]string, spawned map[int]bool) questloop.Sources {
	return questloop.Sources{
		ItemSources: func(id int) []string { return items[id] },
		MobSpawned:  func(id int) bool { return spawned[id] },
	}
}

func TestWalk_ItemWithNoSource(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Conditions.HasItem = 777
	got := keys(Walk(w, srcWith(nil, map[int]bool{9110: true, 9112: true})))
	require.Contains(t, got, "walk-no-source 53 quest 53 trigger mob_death mob=9110 grants=53-firstkill")
}

func TestWalk_ItemWithASourcePasses(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Conditions.HasItem = 777
	require.Empty(t, Walk(w, srcWith(map[int][]string{777: {"shop 9123"}}, map[int]bool{9110: true, 9112: true})))
}

func TestWalk_UnspawnedMobAndNPC(t *testing.T) {
	got := keys(Walk(walkWorld(), srcWith(nil, map[int]bool{})))
	require.Contains(t, got, "walk-unspawned 53 quest 53 trigger mob_death mob=9110 grants=53-firstkill")
	require.Contains(t, got, "walk-unspawned 53 mob 9112 topic cull_offer")
}

func TestWalk_HandInItemNeedsASource(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics[1].RequiresItem = 4242
	got := keys(Walk(w, srcWith(nil, map[int]bool{9110: true, 9112: true})))
	require.Contains(t, got, "walk-no-source 53 mob 9112 topic cull_in")
}

// handInNeedsKillItem is walkWorld with the kill handing over item 900 and
// the hand-in gated on 53-start plus that item, not on 53-firstkill.
func handInNeedsKillItem() questloop.World {
	w := walkWorld()
	w.Quests[0].Triggers[0].Actions = append(w.Quests[0].Triggers[0].Actions, quests.ActionDef{GiveItem: 900})
	w.Dialogue[0].Topics[1].QuestRequired = []string{"53-start"}
	w.Dialogue[0].Topics[1].RequiresItem = 900
	return w
}

func outside(ids ...int) questloop.Sources {
	return questloop.Sources{ItemOutside: func(id, questId int) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}}
}

func TestWalk_HandInItemOnlyTheStepGivesIsNotASkip(t *testing.T) {
	// Item 900 comes only from the kill, which grants 53-firstkill, so no
	// player can hold it at 53-start: the hand-in cannot fire early.
	require.Empty(t, Walk(handInNeedsKillItem(), outside()))
}

func TestWalk_HandInItemObtainableOutsideIsASkip(t *testing.T) {
	// Item 900 also sits on a room floor: a player who picks it up first can
	// hand in at 53-start and pass 53-firstkill.
	got := Walk(handInNeedsKillItem(), outside(900))
	require.Contains(t, keys(got), "walk-dead-end 53 mob 9112 topic cull_in")
	require.Contains(t, got[0].Msg, "passing 53-firstkill")
}

func TestWalk_OrderBugIsStillASkip(t *testing.T) {
	// Quest 19's shape: both kills gate on start, so the second kill jumps
	// over the first one's step. No item is involved.
	w := walkWorld()
	q := w.Quests[0]
	q.Steps = []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "shrimp", Hint: "h"}, {Id: "hunter", Hint: "h"}, {Id: "end"}}
	q.Triggers = []quests.TriggerDef{
		{Event: "mob_death", Mob: 330, Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-shrimp"}},
			Actions: []quests.ActionDef{{Grant: "53-shrimp"}}},
		{Event: "mob_death", Mob: 331, Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-hunter"}},
			Actions: []quests.ActionDef{{Grant: "53-hunter"}}},
	}
	w.Dialogue[0].Topics[1].QuestRequired = []string{"53-hunter"}
	got := keys(Walk(w, outside()))
	require.Contains(t, got, "walk-dead-end 53 quest 53 trigger mob_death mob=331 grants=53-hunter")
}

func TestWalk_GuardCanFail(t *testing.T) {
	// The clean world must stay clean, and every mutation above must be
	// caught: this is the proof the walk is capable of failing.
	require.Empty(t, Walk(walkWorld(), questloop.Sources{}))
	w := walkWorld()
	w.Quests[0].Triggers = nil
	require.NotEmpty(t, Walk(w, questloop.Sources{}))
}
