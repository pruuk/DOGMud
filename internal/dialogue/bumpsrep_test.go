package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestApply_TopicBumpsRep verifies a topic applies faction-reputation changes,
// so a hand-in topic can carry what a give-path bump_rep action did (Tier C
// delivery quests: Q67/Q68/Q70/Q74). The change goes through the
// PlayerState.BumpRep callback, as grantsQuest/requiresItem route through
// PlayerState. Ported from the old TreeAdvance test (#375 Task 21).
func TestApply_TopicBumpsRep(t *testing.T) {
	df := &DialogueFile{MobId: 6001, Topics: []Topic{{
		Key: "deliver", Kind: KindInfo, Label: "I brought the goods.", Text: "done",
		Gates:   Gates{QuestRequired: []string{"x-mid"}},
		Effects: Effects{GrantsQuest: "x-end", BumpsRep: []RepBump{{Faction: "alpha", Delta: 20}, {Faction: "beta", Delta: -15}}},
	}}}

	bumps := map[string]int{}
	granted := ""
	ps := &PlayerState{
		HasQuest:  func(tok string) bool { return tok == "x-mid" },
		GiveQuest: func(tok string) { granted = tok },
		BumpRep:   func(faction string, delta int) { bumps[faction] += delta },
	}

	tp := df.FindTopic("deliver")
	assert.True(t, df.Eligible(tp, ps), "the hand-off topic must be open")
	assert.True(t, df.Apply(tp, ps))
	assert.Equal(t, "x-end", granted, "the topic must grant its token")
	assert.Equal(t, 20, bumps["alpha"], "alpha rep must be bumped +20")
	assert.Equal(t, -15, bumps["beta"], "beta rep must be bumped -15")
}

// TestApply_TopicGivesGold verifies a topic awards gold, so a hand-in topic
// can carry what a give-path give_gold action did (e.g. Q19/Q68/Q74).
func TestApply_TopicGivesGold(t *testing.T) {
	df := &DialogueFile{MobId: 6002, Topics: []Topic{{
		Key: "deliver", Kind: KindInfo, Label: "I brought the goods.", Text: "done",
		Gates:   Gates{QuestRequired: []string{"x-mid"}},
		Effects: Effects{GrantsQuest: "x-end", GivesGold: 350},
	}}}

	gold := 0
	ps := &PlayerState{
		HasQuest:  func(tok string) bool { return tok == "x-mid" },
		GiveQuest: func(string) {},
		GiveGold:  func(amount int) { gold += amount },
	}

	assert.True(t, df.Apply(df.FindTopic("deliver"), ps))
	assert.Equal(t, 350, gold, "the topic must award 350 gold")
}
