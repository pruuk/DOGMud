package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func pinConversationChance(t *testing.T, pct float64) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ConversationBaseChancePct = configs.ConfigFloat(pct)
	configs.SetConfigForTest(t, cfg)
}

// ConversationBaseChancePct 0 never opens a conversation (#317), even on
// the luckiest draw; 1.0 does on that draw.
func TestConversationStartRoll_ZeroNeverPasses(t *testing.T) {
	luckiest := func(int) int { return 0 }
	pinConversationChance(t, 0)
	if conversationStartRoll(luckiest) {
		t.Fatal("ConversationBaseChancePct 0 must never start a conversation")
	}
	pinConversationChance(t, 1.0)
	if !conversationStartRoll(luckiest) {
		t.Fatal("ConversationBaseChancePct 1.0 must pass on a draw of 0")
	}
}
