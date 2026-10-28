package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// A backlog line for one refs error must not hide a second, different refs
// error on the same quest: each refs error has its own key.
func TestKeys_RefsErrorsDoNotCollide(t *testing.T) {
	v := quests.QuestValidators{MobExists: func(id int) bool { return id < 424242 }}
	fillValidators(&v)

	known := goodWorld()
	known.Refs = &v
	known.Quests[0].Triggers[0].Mob = 424242
	b := Backlog{}
	for _, f := range Validate(known) {
		b[f.Key()] = true
	}
	require.Len(t, b, 1, "one known refs finding")

	w := goodWorld()
	w.Refs = &v
	w.Quests[0].Triggers[0].Mob = 424242
	w.Quests[0].Rewards.ItemId = 0
	w.Quests[0].Steps[0].Destination = quests.StepDestination{Mob: 424243, Room: 5}
	fresh, known2, _ := Ratchet(Validate(w), b)
	require.Len(t, known2, 1)
	require.Len(t, fresh, 1, "the second refs error is fresh, got %v", fresh)
	require.Equal(t, "refs", fresh[0].Rule)
}

// One trigger that breaks one-action two ways gives two findings.
func TestKeys_OneActionReasonsDoNotMerge(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Conditions = quests.Conditions{HasItem: 7}
	got := rules(Validate(w))
	require.Contains(t, got, "one-action quest 53 trigger mob_death mob=9110 grants=53-firstkill:prev")
	require.Contains(t, got, "one-action quest 53 trigger mob_death mob=9110 grants=53-firstkill:cond")
}

// The three repeat sub-rules have their own keys.
func TestKeys_RepeatSubRules(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Repeatable = true
	w.Quests[0].RepeatLimit = -1
	got := rules(Validate(w))
	require.Contains(t, got, "repeat quest 53:cooldown")
	require.Contains(t, got, "repeat quest 53:negative")
	w = goodWorld()
	w.Quests[0].RepeatLimit = 2
	require.Contains(t, rules(Validate(w)), "repeat quest 53:notrepeatable")
}

// Inserting a step or a trigger in front of an existing one leaves the
// existing finding's key unchanged.
func TestKeys_StableAcrossInsertion(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Conditions.Has = nil // one-action :prev on the kill trigger
	w.Quests[0].Steps[1].Hint = ""               // hint on firstkill
	before := map[string]bool{}
	for _, f := range Validate(w) {
		before[f.Key()] = true
	}
	require.True(t, before["one-action 53 quest 53 trigger mob_death mob=9110 grants=53-firstkill:prev"], "%v", before)
	require.True(t, before["hint 53 step firstkill"], "%v", before)

	w.Quests[0].Triggers = append([]quests.TriggerDef{{Event: "room_enter", Room: 5,
		Conditions: quests.Conditions{Has: []string{"53-start"}}}}, w.Quests[0].Triggers...)
	w.Quests[0].Steps = append([]quests.QuestStep{w.Quests[0].Steps[0], {Id: "scout", Description: "d", Hint: "h"}}, w.Quests[0].Steps[1:]...)
	after := map[string]bool{}
	for _, f := range Validate(w) {
		after[f.Key()] = true
	}
	require.True(t, after["one-action 53 quest 53 trigger mob_death mob=9110 grants=53-firstkill:prev"], "%v", after)
	require.True(t, after["hint 53 step firstkill"], "%v", after)
}

// Two triggers with one signature are told apart by file order.
func TestKeys_SameSignatureGetsOrdinal(t *testing.T) {
	q := &quests.Quest{QuestId: 53, Triggers: []quests.TriggerDef{
		{Event: "mob_death", Mob: 9110, Actions: []quests.ActionDef{{Grant: "53-firstkill"}}},
		{Event: "mob_death", Mob: 9110, Actions: []quests.ActionDef{{Grant: "53-firstkill"}}},
		{Event: "room_enter", Room: 5},
	}}
	require.Equal(t, []string{
		"quest 53 trigger mob_death mob=9110 grants=53-firstkill",
		"quest 53 trigger mob_death mob=9110 grants=53-firstkill #2",
		"quest 53 trigger room_enter room=5",
	}, triggerWheres(q))
}

// A refs refusal's positional prefix is named by signature or step id.
func TestKeys_RefsTextIsStable(t *testing.T) {
	q := &quests.Quest{QuestId: 53, Triggers: []quests.TriggerDef{{Event: "room_enter", Room: 5}}}
	require.Equal(t, "trigger room_enter room=5 mob filter: mob 9 does not exist",
		refsText(q, "trigger 0 mob filter: mob 9 does not exist\n"))
	require.Equal(t, "step start destination room: room 9 does not exist",
		refsText(q, " step 0 (start) destination room: room 9 does not exist"))
	require.Equal(t, "rewards itemid: item 9 does not exist", refsText(q, "rewards itemid: item 9 does not exist"))
}
