package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// seedAmbientCommand: the package fixture (users 1 and 2 with mob 100 in
// room 1), the budget pinned, room 1 lit, the round at 6000.
func seedAmbientCommand(t *testing.T, budget int) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(6000)
	room := rooms.LoadRoom(1)
	room.SkyLight = rooms.SkyLightPtr(0)
	room.Lamp = rooms.LampPtr(60)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
}

func heard(userId int, needle string) int {
	n := 0
	for _, m := range events.DrainQueuedMessagesForTest(userId) {
		if strings.Contains(m, needle) {
			n++
		}
	}
	return n
}

func TestAmbientSay_BudgetSkipsOnlyTheListenerOverIt(t *testing.T) {
	seedAmbientCommand(t, 5)
	ambient.ChargeForTest(1, 6005)
	if handled, err := TryCommand("ambient", "mob_chatter say Fresh bread, warm from the oven!", 100); !handled || err != nil {
		t.Fatalf("ambient say: handled %v err %v", handled, err)
	}
	if n := heard(1, "Fresh bread"); n != 0 {
		t.Errorf("user 1 is over budget and must skip the line, heard it %d times", n)
	}
	if n := heard(2, "Fresh bread"); n != 1 {
		t.Errorf("user 2 has budget and must hear the line once, heard it %d times", n)
	}
}

func TestAmbientSay_TwoLinesInAWindowReachEachListenerOnce(t *testing.T) {
	seedAmbientCommand(t, 5)
	TryCommand("ambient", "mob_chatter say First line.", 100)
	TryCommand("ambient", "gossip say Second line.", 100)
	for _, uid := range []int{1, 2} {
		msgs := strings.Join(events.DrainQueuedMessagesForTest(uid), "")
		if !strings.Contains(msgs, "First line") || strings.Contains(msgs, "Second line") {
			t.Errorf("user %d read %q, want the first line only", uid, msgs)
		}
	}
}

func TestPlainSay_IsNeverBudgeted(t *testing.T) {
	seedAmbientCommand(t, 5)
	ambient.ChargeForTest(1, 6005)
	ambient.ChargeForTest(2, 6005)
	TryCommand("say", "This is a quest line.", 100)
	for _, uid := range []int{1, 2} {
		if n := heard(uid, "quest line"); n != 1 {
			t.Errorf("a plain say is not ambient; user %d heard it %d times, want 1", uid, n)
		}
	}
}

func TestAmbientEmote_BudgetSkipsTheListenerOverIt(t *testing.T) {
	seedAmbientCommand(t, 5)
	ambient.ChargeForTest(2, 6005)
	TryCommand("ambient", "mob_chatter emote rattles its bones; dust falls.", 100)
	if n := heard(1, "rattles its bones; dust falls"); n != 1 {
		t.Errorf("user 1 must see the whole emote once, saw it %d times", n)
	}
	if n := heard(2, "rattles its bones"); n != 0 {
		t.Errorf("user 2 is over budget, saw the emote %d times", n)
	}
}

func TestAmbient_RefusesAnUnknownSourceOrVerb(t *testing.T) {
	seedAmbientCommand(t, 0)
	if _, err := TryCommand("ambient", "tips say hello", 100); err == nil {
		t.Error("an unknown source must be an error, so a typo shows in the log")
	}
	if _, err := TryCommand("ambient", "mob_chatter wander", 100); err == nil {
		t.Error("only emote and say are ambient lines")
	}
}
