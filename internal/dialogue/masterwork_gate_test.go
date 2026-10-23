package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEligible_MasterworkGate exercises the masterworkRequired gate on a
// topic (Veyra's "show me an item you crafted at skill 50+" intro gate).
// Ported from the old TreeAdvance test (#375 Task 21).
func TestEligible_MasterworkGate(t *testing.T) {
	df := &DialogueFile{MobId: 6000, Topics: []Topic{{
		Key: "commission", Kind: KindInfo, Label: "I want a commission.",
		Text:  "Ah, a fellow artisan of skill. I'll take your commission.",
		Gates: Gates{MasterworkRequired: 50},
	}}}
	tp := df.FindTopic("commission")

	// hasMasterwork toggled by the test to flip HasOwnMasterwork's answer.
	hasMasterwork := false
	ps := &PlayerState{
		HasQuest:  func(token string) bool { return false },
		GiveQuest: func(token string) {},
		HasOwnMasterwork: func(skillMin int) bool {
			return hasMasterwork && skillMin <= 50
		},
	}

	assert.False(t, df.Eligible(tp, ps), "gate should hide the topic when HasOwnMasterwork is false")

	hasMasterwork = true
	assert.True(t, df.Eligible(tp, ps), "gate should reveal the topic once HasOwnMasterwork returns true")

	// Backward compat: a PlayerState with a nil HasOwnMasterwork callback must
	// not block the gated topic (the nil-guard skips the check entirely).
	legacyPs := &PlayerState{
		HasQuest:  func(token string) bool { return false },
		GiveQuest: func(token string) {},
	}
	assert.True(t, df.Eligible(tp, legacyPs),
		"nil HasOwnMasterwork callback should skip the gate, not fail it (backward compat)")
}
