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

	// The offer's own firing lands on firstkill: the chain ran inside it
	// (the walk never fires the quest_granted Way directly).
	eng := NewEngine()
	eng.RegisterQuest(w.Quests[0])
	p := newWalkPlayer(map[int][]string{53: {"start", "firstkill", "end"}})
	offer := questloop.WaysInto(w)["53-start"][0]
	trail := fireWay(&walkEnv{eng: eng}, p, offer, 53)
	require.Equal(t, "firstkill", p.progress[53])
	require.True(t, trail["53-start"] && trail["53-firstkill"], "%v", trail)

	// Without the trigger, firstkill is dead.
	w.Quests[0].Triggers = nil
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 step firstkill")
}

// A quest_granted Way of the walked quest fires only inside the chain of the
// grant that lands on its token, as in the game, never directly from a later
// rest on that step: here it needs item 900 at the moment 53-start is
// granted, and nothing gives 900 by then.
func TestWalk_OwnQuestGrantedFiresOnlyAtTheGrant(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers = []quests.TriggerDef{{
		Event: "quest_granted", QuestToken: "53-start",
		Conditions: quests.Conditions{HasItem: 900},
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
	}}
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 step firstkill")

	// The offer hands item 900 over with the grant: the chain then fires.
	w.Dialogue[0].Topics[0].GivesItem = 900
	require.Empty(t, Walk(w, questloop.Sources{}))
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
	return questloop.Sources{ItemAvailable: func(id, questId int, _ func(string) bool) bool {
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

// The six shipped skips' shape (quest 63 tallyhandoff and its kin): the kill
// that reaches firstkill hands over item 900, a later step (report) follows,
// and the hand-in is gated only on 53-start plus item 900. The item is the
// quest's own, so it cannot be held at start, but from firstkill on the
// player holds it, and the hand-in jumps over report.
func TestWalk_ItemFromAnEarlierStepOpensASkipFromThatStep(t *testing.T) {
	w := handInNeedsKillItem()
	q := w.Quests[0]
	q.Steps = []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "firstkill", Hint: "h"}, {Id: "report", Hint: "h"}, {Id: "end"}}
	q.Triggers = append(q.Triggers, quests.TriggerDef{
		Event: "skill_use", Skill: "brawling",
		Conditions: quests.Conditions{Has: []string{"53-firstkill"}, Missing: []string{"53-report"}},
		Actions:    []quests.ActionDef{{Grant: "53-report"}},
	})
	got := Walk(w, outside())
	require.Contains(t, keys(got), "walk-dead-end 53 mob 9112 topic cull_in", "%v", got)
	for _, f := range got {
		if f.Where == "mob 9112 topic cull_in" {
			require.Equal(t, "skips to 53-end from 53-firstkill, passing 53-report", f.Msg)
		}
	}
}

// An item whose only source opens at 53-start (a topic of another quest
// gated on 53-start) is available to the probe from start: the hand-in that
// needs it skips firstkill.
func TestWalk_ItemOpenFromStartIsASkip(t *testing.T) {
	src := questloop.Sources{ItemAvailable: func(id, questId int, has func(string) bool) bool {
		return id == 900 && has("53-start")
	}}
	got := keys(Walk(handInNeedsKillItem(), src))
	require.Contains(t, got, "walk-dead-end 53 mob 9112 topic cull_in")
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

// The walk sorts quests by id, so the trigger engine sees them in one order
// whatever order the caller lists them in. Here quest 7's kill trigger also
// grants 53-end: run before quest 53's own kill trigger it hides firstkill
// (a skip); run after it, the firing passes through firstkill.
func TestWalk_QuestOrderDoesNotChangeFindings(t *testing.T) {
	build := func(sevenFirst bool) questloop.World {
		w := walkWorld()
		seven := &quests.Quest{QuestId: 7, Steps: []quests.QuestStep{{Id: "end"}},
			Triggers: []quests.TriggerDef{{Event: "mob_death", Mob: 9110,
				Conditions: quests.Conditions{Has: []string{"53-start"}},
				Actions:    []quests.ActionDef{{Grant: "53-end"}}}}}
		if sevenFirst {
			w.Quests = append([]*quests.Quest{seven}, w.Quests...)
		} else {
			w.Quests = append(w.Quests, seven)
		}
		return w
	}
	a, b := Walk(build(true), questloop.Sources{}), Walk(build(false), questloop.Sources{})
	require.Equal(t, a, b)
}

// A step only a behaviour tree grants (Hadwen's 30-rite) is reachable: the
// walk cannot run the tree, so it takes the grant from the step before,
// once a topic the tree answers is open there.
func TestWalk_StepGrantedByATreeIsNotADeadEnd(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers = nil
	w.Dialogue[0].Topics = append(w.Dialogue[0].Topics, dialogue.Topic{Key: "hunt", Kind: dialogue.KindInfo,
		Label: "Hunt", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"53-start"}}})
	tree := questloop.ExternalGrant{Where: "mob 9112 tree grant_quest 53-firstkill", Token: "53-firstkill",
		MobId: 9112, Topics: []string{"hunt"}}
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 step firstkill", "no tree: dead")
	w.External = []questloop.ExternalGrant{tree}
	require.Empty(t, Walk(w, questloop.Sources{}))

	// The tree answers a topic that never opens: the tree Way is dead.
	w.Dialogue[0].Topics[2].QuestRequired = []string{"53-end"}
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 mob 9112 tree grant_quest 53-firstkill")
	require.Contains(t, got, "walk-dead-end 53 step firstkill")

	// A branch that keys on no topic (a room_enter tree) is taken as is.
	w.External = []questloop.ExternalGrant{{Where: "room 407 tree grant_quest 53-firstkill", Token: "53-firstkill"}}
	require.Empty(t, Walk(w, questloop.Sources{}))
}

// An item questtoken (item 40035 grants 14-explore) is a Way in once the
// player can hold the item; the hook's quest_granted chain runs after it.
func TestWalk_ItemQuestTokenIsAWayIn(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers = nil
	w.External = []questloop.ExternalGrant{{Where: "item 900 questtoken", Token: "53-firstkill", ItemId: 900}}
	ok := questloop.Sources{ItemAvailable: func(id, _ int, _ func(string) bool) bool { return id == 900 }}
	require.Empty(t, Walk(w, ok))

	never := ok
	never.ItemAvailable = func(int, int, func(string) bool) bool { return false }
	got := keys(Walk(w, never))
	require.Contains(t, got, "walk-dead-end 53 item 900 questtoken")
	require.Contains(t, got, "walk-dead-end 53 step firstkill")
}

// A flag of the walked quest is never faked: a hand-in gated on one that
// nothing sets is a dead end. Another quest's flag is a prerequisite and is
// held, like its tokens.
func TestWalk_OwnFlagNothingSetsIsADeadEnd(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics[1].QuestFlagRequired = map[string]string{"53-cleared": "yes"}
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 mob 9112 topic cull_in")
	require.Contains(t, got, "walk-dead-end 53 step end")

	// The kill sets it: the hand-in opens.
	w.Quests[0].Triggers[0].Actions = append(w.Quests[0].Triggers[0].Actions,
		quests.ActionDef{SetFlag: &quests.QuestFlagAction{Key: "53-cleared", Value: "yes"}})
	require.Empty(t, Walk(w, questloop.Sources{}))

	// Another quest's flag is held as a prerequisite.
	w = walkWorld()
	w.Dialogue[0].Topics[1].QuestFlagRequired = map[string]string{"7-sided": "garve"}
	w.Quests[0].Triggers[0].Conditions.HasFlag = map[string]string{"8-done": "yes"}
	require.Empty(t, Walk(w, questloop.Sources{}))
}

// A flag of the walked quest set by a trigger or topic that grants nothing
// (quest 72's room_interact set_flag, quest 18's bedroll before the offer)
// is set by firing that setter, so the gate on it opens. A setter that
// cannot fire leaves the gate shut.
func TestWalk_OwnFlagSetByAGrantlessSetterOpensTheGate(t *testing.T) {
	w := walkWorld()
	q := w.Quests[0]
	q.Triggers = append(q.Triggers, quests.TriggerDef{
		Event: "room_interact", Room: 6078, Noun: "channel",
		Conditions: quests.Conditions{Has: []string{"53-start"}},
		Actions:    []quests.ActionDef{{SetFlag: &quests.QuestFlagAction{Key: "53-channel", Value: "found"}}},
	})
	// Before the offer: a bedroll sets 53-barn, which an info topic needs,
	// which the offer needs heard.
	q.Triggers = append(q.Triggers, quests.TriggerDef{
		Event: "room_interact", Room: 4050, Noun: "bedroll",
		Conditions: quests.Conditions{Missing: []string{"53-start"}},
		Actions:    []quests.ActionDef{{SetFlag: &quests.QuestFlagAction{Key: "53-barn", Value: "seen"}}},
	})
	w.Dialogue[0].Topics = append(w.Dialogue[0].Topics, dialogue.Topic{Key: "barn_trace", Kind: dialogue.KindInfo,
		Label: "Barn", Text: "t", Gates: dialogue.Gates{QuestFlagRequired: map[string]string{"53-barn": "seen"}}})
	w.Dialogue[0].Topics[0].HeardRequired = []string{"barn_trace"}
	w.Dialogue[0].Topics[1].QuestFlagRequired = map[string]string{"53-channel": "found"}
	require.Empty(t, Walk(w, questloop.Sources{}))

	// The channel can be found only after end: the hand-in stays shut.
	q.Triggers[1].Conditions.Has = []string{"53-end"}
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 step end")
}

// heardRequired holds only when the info topic can be heard in the walk's
// state: it opens there, or the walk chose it.
func TestWalk_HeardTopicThatNeverShowsIsADeadEnd(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics = append(w.Dialogue[0].Topics, dialogue.Topic{Key: "lore", Kind: dialogue.KindInfo,
		Label: "Lore", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"53-end"}}}) // never open before end
	w.Dialogue[0].Topics[1].HeardRequired = []string{"lore"}
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 mob 9112 topic cull_in")

	// Open from start: the player can hear it before handing in.
	w.Dialogue[0].Topics[2].QuestRequired = []string{"53-start"}
	require.Empty(t, Walk(w, questloop.Sources{}))
}

// branchWorld has two ways into 53-path, each setting its own 53-route, and
// a hand-in that wants route b.
func branchWorld() questloop.World {
	w := walkWorld()
	q := w.Quests[0]
	q.Steps = []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "path", Hint: "h"}, {Id: "end"}}
	setRoute := func(v string) quests.ActionDef {
		return quests.ActionDef{SetFlag: &quests.QuestFlagAction{Key: "53-route", Value: v}}
	}
	q.Triggers = []quests.TriggerDef{
		{Event: "mob_death", Mob: 9110, Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-path"}},
			Actions: []quests.ActionDef{setRoute("a"), {Grant: "53-path"}}},
		{Event: "skill_use", Skill: "brawling", Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-path"}},
			Actions: []quests.ActionDef{setRoute("b"), {Grant: "53-path"}}},
	}
	w.Dialogue[0].Topics[1].QuestRequired = []string{"53-path"}
	w.Dialogue[0].Topics[1].QuestFlagRequired = map[string]string{"53-route": "b"}
	return w
}

// Alternatives are judged per branch: the hand-in needs the flag only the
// second way into path sets, so the walk carries both branches forward.
func TestWalk_LaterWayReachedThroughTheSecondBranch(t *testing.T) {
	require.Empty(t, Walk(branchWorld(), questloop.Sources{}))
}

// Negative gates read the walk's real state: a hand-in that refuses route a
// is still reached through route b, and one that refuses both is dead.
func TestWalk_NegativeGatesReadTheRealState(t *testing.T) {
	w := branchWorld()
	w.Dialogue[0].Topics[1].QuestFlagRequired = nil
	w.Dialogue[0].Topics[1].QuestFlagExcluded = map[string]string{"53-route": "a"}
	require.Empty(t, Walk(w, questloop.Sources{}))

	w.Quests[0].Triggers[1].Actions[0].SetFlag.Value = "a"
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 step end")
}

// A dead step does not stop the source checks: the hand-in after it still
// reports the item nothing provides.
func TestWalk_StepsAfterADeadStepAreSourceChecked(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers = nil // firstkill is dead
	w.Dialogue[0].Topics[1].RequiresItem = 4242
	got := keys(Walk(w, srcWith(nil, map[int]bool{9110: true, 9112: true})))
	require.Contains(t, got, "walk-dead-end 53 step firstkill")
	require.Contains(t, got, "walk-no-source 53 mob 9112 topic cull_in")
}

// A step granted inside a sequence's on_complete is on the trail: the walk
// runs on_complete through the trail's context (trailCtx.QueueSequence), so
// the firing passes through the step rather than skipping it.
func TestWalk_StepGrantedInASequenceIsOnTheTrail(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Sequence: &quests.SequenceDef{
		OnComplete: []quests.ActionDef{{Grant: "53-firstkill"}}}}}
	require.Empty(t, Walk(w, questloop.Sources{}))
}

func TestWalk_GuardCanFail(t *testing.T) {
	// The clean world must stay clean, and every mutation above must be
	// caught: this is the proof the walk is capable of failing.
	require.Empty(t, Walk(walkWorld(), questloop.Sources{}))
	w := walkWorld()
	w.Quests[0].Triggers = nil
	require.NotEmpty(t, Walk(w, questloop.Sources{}))
}
