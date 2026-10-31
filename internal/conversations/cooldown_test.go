package conversations

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// ConversationCooldownRounds 0 means no cooldown (#317): a pair that just
// finished may start again the same round.
func TestCooldownUntil_ZeroIsNoCooldown(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.ConversationCooldownRounds = 0
	configs.SetConfigForTest(t, cfg)
	if got := cooldownUntil(4000); got != 4000 {
		t.Fatalf("cooldown 0: until = %d, want 4000", got)
	}
	cfg.Balance.ConversationCooldownRounds = 50
	configs.SetConfigForTest(t, cfg)
	if got := cooldownUntil(4000); got != 4050 {
		t.Fatalf("cooldown 50: until = %d, want 4050", got)
	}
}
