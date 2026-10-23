package dialogue

import "testing"

// A topic carrying BOTH givesItem and grantsQuest must not grant the quest
// when the item cannot be delivered (GiveItem returns false). Otherwise the
// player holds the quest token, lacks the item, and the granting topic is
// then hidden by questExcluded: an unrecoverable soft-lock. Apply must also
// leave requiresItem in the player's pack, skip flags/rep/gold, and leave the
// topic unheard so it plays again once the player makes room. (Mirror of the
// questengine bridge fix from Tier 2.) Ported from the old TreeAdvance, Match
// and Greet tests (#375 Task 21); opener variants carry no effects, so the
// old Greet case has no counterpart.

// failingGiveState returns a PlayerState whose GiveItem reports delivered,
// plus pointers to the observation flags.
func failingGiveState(delivered bool) (*PlayerState, *bool, *bool, *bool, *bool, map[string]bool) {
	granted := false
	removed := false
	flagSet := false
	goldGiven := false
	heard := map[string]bool{}
	ps := &PlayerState{
		HasQuest:   func(string) bool { return false },
		HasItem:    func(int) bool { return true },
		RemoveItem: func(int) bool { removed = true; return true },
		GiveQuest:  func(string) { granted = true },
		GiveItem:   func(int) bool { return delivered },
		GetQuestFlag: func(string) string {
			return ""
		},
		SetQuestFlag: func(string, string) { flagSet = true },
		GiveGold:     func(int) { goldGiven = true },
		HasHeard:     func(k string) bool { return heard[k] },
		MarkHeard:    func(k string) { heard[k] = true },
	}
	return ps, &granted, &removed, &flagSet, &goldGiven, heard
}

func failedDeliveryFile() *DialogueFile {
	return &DialogueFile{MobId: 9999, Topics: []Topic{{
		Key: "reward", Kind: KindInfo, Label: "About that reward.", Text: "Take this, and my thanks.",
		Gates: Gates{RequiresItem: 1111},
		Effects: Effects{
			GrantsQuest:   "77-start",
			GivesItem:     4242,
			SetsQuestFlag: &QuestFlagSet{Key: "77-k", Value: "v"},
			GivesGold:     10,
		},
	}}}
}

func TestApply_GiveItemFails_NoQuestNoConsumption(t *testing.T) {
	df := failedDeliveryFile()
	tp := df.FindTopic("reward")
	ps, granted, removed, flagSet, goldGiven, heard := failingGiveState(false)

	if df.Apply(tp, ps) {
		t.Fatal("Apply must report false when the item cannot be delivered")
	}
	if *granted {
		t.Error("quest granted despite failed item delivery")
	}
	if *removed {
		t.Error("requiresItem removed despite failed item delivery")
	}
	if *flagSet {
		t.Error("quest flag set despite failed item delivery")
	}
	if *goldGiven {
		t.Error("gold given despite failed item delivery")
	}
	if heard[HeardKey(9999, "reward")] {
		t.Error("topic recorded as heard despite failed item delivery")
	}
	if !df.Eligible(tp, ps) {
		t.Error("the topic must stay open so it can play again")
	}

	// After the player makes room, the same topic must deliver everything.
	ps2, granted2, removed2, flagSet2, goldGiven2, heard2 := failingGiveState(true)
	if !df.Apply(tp, ps2) {
		t.Fatal("expected the topic to apply after making room")
	}
	if !*granted2 || !*removed2 || !*flagSet2 || !*goldGiven2 || !heard2[HeardKey(9999, "reward")] {
		t.Errorf("expected all effects on retry: granted=%v removed=%v flag=%v gold=%v heard=%v",
			*granted2, *removed2, *flagSet2, *goldGiven2, heard2)
	}
}

// Nil GiveItem callback with givesItem set keeps the skip-checks contract:
// effects still apply (partial PlayerStates are test territory; the
// production conversation.PlayerState always wires GiveItem).
func TestApply_NilGiveItemCallback_StillGrants(t *testing.T) {
	df := failedDeliveryFile()
	granted := false
	ps := &PlayerState{
		HasQuest:  func(string) bool { return false },
		HasItem:   func(int) bool { return true },
		GiveQuest: func(string) { granted = true },
	}
	if !df.Apply(df.FindTopic("reward"), ps) {
		t.Fatal("expected the topic to apply")
	}
	if !granted {
		t.Error("nil GiveItem callback must not block the grant (skip-checks contract)")
	}
}
