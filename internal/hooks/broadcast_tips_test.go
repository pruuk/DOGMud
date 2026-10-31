package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/tips"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func TestBroadcastTips(t *testing.T) {
	drainEventQueueOnCleanup(t)
	on := users.NewTestUser(9301, "tipson", "Tipson", 19301)
	off := users.NewTestUser(9302, "tipsoff", "Tipsoff", 19302)
	off.SetConfigOption("tips", false)
	deaf := users.NewTestUser(9303, "tipsdeaf", "Tipsdeaf", 19303)
	deaf.Deafened = true
	defer users.SeedUsersForTest(map[int]*users.UserRecord{9301: on, 9302: off, 9303: deaf})()
	defer tips.SeedForTest([]string{"First tip.", "Second tip."})()
	for _, id := range []int{9301, 9302, 9303} {
		events.DrainQueuedMessagesForTest(id)
	}

	tipIntervalRounds := uint64(configs.GetBalanceConfig().TipIntervalRounds)
	BroadcastTips(events.NewRound{RoundNumber: tipIntervalRounds - 1})
	if got := events.DrainQueuedMessagesForTest(9301); len(got) != 0 {
		t.Fatalf("off-interval round sent %q", got)
	}

	BroadcastTips(events.NewRound{RoundNumber: tipIntervalRounds})
	got := events.DrainQueuedMessagesForTest(9301)
	if len(got) != 1 || !strings.Contains(got[0], "[Tip]") || !strings.Contains(got[0], "First tip.") {
		t.Fatalf("player with tips on got %q, want one [Tip] First tip. line", got)
	}
	if got := events.DrainQueuedMessagesForTest(9302); len(got) != 0 {
		t.Errorf("player with tips off got %q", got)
	}
	if got := events.DrainQueuedMessagesForTest(9303); len(got) != 0 {
		t.Errorf("deafened player got %q", got)
	}

	BroadcastTips(events.NewRound{RoundNumber: 2 * tipIntervalRounds})
	if got := events.DrainQueuedMessagesForTest(9301); len(got) != 1 || !strings.Contains(got[0], "Second tip.") {
		t.Errorf("second broadcast = %q, want Second tip.", got)
	}
}

// TipIntervalRounds 0 turns tips off (#317): no round sends one.
func TestBroadcastTips_ZeroIntervalIsOff(t *testing.T) {
	drainEventQueueOnCleanup(t)
	on := users.NewTestUser(9311, "tipzero", "Tipzero", 19311)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{9311: on})()
	defer tips.SeedForTest([]string{"A tip."})()
	cfg := configs.GetConfig()
	cfg.Balance.TipIntervalRounds = 0
	configs.SetConfigForTest(t, cfg)
	events.DrainQueuedMessagesForTest(9311)

	for _, round := range []uint64{0, 1, 75, 150} {
		BroadcastTips(events.NewRound{RoundNumber: round})
	}
	if got := events.DrainQueuedMessagesForTest(9311); len(got) != 0 {
		t.Fatalf("TipIntervalRounds 0 must send no tip, got %q", got)
	}
}
