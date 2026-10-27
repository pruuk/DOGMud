package configs

import "testing"

// QuestGuardLimits reads the same two knobs GetBalanceConfig carries.
func TestQuestGuardLimits_MatchTheBalanceConfig(t *testing.T) {
	bal := GetBalanceConfig()
	depth, warn := QuestGuardLimits()
	if depth != int(bal.QuestChainDepthLimit) || warn != int64(bal.QuestPerformanceWarnMs) {
		t.Fatalf("QuestGuardLimits() = %d, %d; balance has %d, %d",
			depth, warn, bal.QuestChainDepthLimit, bal.QuestPerformanceWarnMs)
	}
	if depth < 1 || warn < 1 {
		t.Fatalf("limits must be positive: %d, %d", depth, warn)
	}
}
