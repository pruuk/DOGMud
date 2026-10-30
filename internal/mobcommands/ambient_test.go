package mobcommands

import (
	"slices"
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

// A malformed line is an error, and it is still handled, so world.go logs
// it once as an error rather than again as an unknown command.
func TestAmbient_RefusesAnUnknownSourceOrVerb(t *testing.T) {
	seedAmbientCommand(t, 0)
	if handled, err := TryCommand("ambient", "tips say hello", 100); err == nil || !handled {
		t.Errorf("an unknown source must be a handled error, so a typo shows once in the log: handled %v err %v", handled, err)
	}
	if handled, err := TryCommand("ambient", "mob_chatter wander", 100); err == nil || !handled {
		t.Errorf("only emote and say are ambient lines: handled %v err %v", handled, err)
	}
}

// say_echo is charged only by an ambient say's echo next door, never issued
// as a command: typed as a source it is a handled error that sends nothing
// and charges nobody, in any case.
func TestAmbient_RefusesSayEchoAsATypedSource(t *testing.T) {
	seedAmbientCommand(t, 5)
	for _, line := range []string{"say_echo say Typed echo.", "SAY_ECHO emote Typed echo."} {
		if handled, err := TryCommand("ambient", line, 100); err == nil || !handled {
			t.Errorf("%q: say_echo is not a typed source and must be a handled error: handled %v err %v", line, handled, err)
		}
	}
	for _, uid := range []int{1, 2} {
		if n := heard(uid, "Typed echo"); n != 0 {
			t.Errorf("user %d heard a refused say_echo line %d times, want 0", uid, n)
		}
	}
	TryCommand("ambient", "mob_chatter say After the echo ones.", 100)
	for _, uid := range []int{1, 2} {
		if n := heard(uid, "After the echo ones"); n != 1 {
			t.Errorf("user %d was charged by a refused say_echo line: heard the next line %d times, want 1", uid, n)
		}
	}
}

// An ambient line with no text is refused before the budget is asked, so
// nobody is charged for an empty say.
func TestAmbient_RefusesEmptyTextAndChargesNobody(t *testing.T) {
	seedAmbientCommand(t, 5)
	for _, line := range []string{"mob_chatter say", "mob_chatter say   ", "mob_chatter emote"} {
		if _, err := TryCommand("ambient", line, 100); err == nil {
			t.Errorf("%q: an ambient line with no text must be an error", line)
		}
	}
	TryCommand("ambient", "mob_chatter say After the empty ones.", 100)
	for _, uid := range []int{1, 2} {
		if n := heard(uid, "After the empty ones"); n != 1 {
			t.Errorf("user %d was charged by an empty line: heard the next line %d times, want 1", uid, n)
		}
	}
}

// The source word reads case-blind, as the verb does.
func TestAmbient_SourceAndVerbAreCaseBlind(t *testing.T) {
	seedAmbientCommand(t, 0)
	if handled, err := TryCommand("ambient", "Mob_Chatter SAY Loud and clear.", 100); !handled || err != nil {
		t.Fatalf("handled %v err %v", handled, err)
	}
	if n := heard(1, "Loud and clear"); n != 1 {
		t.Errorf("user 1 heard the line %d times, want 1", n)
	}
}

// sayEvents drains the queued Communication events.
func sayEvents() []events.Communication {
	var out []events.Communication
	for _, e := range events.DrainQueuedByTypeForTest(`Communication`) {
		if c, ok := e.(events.Communication); ok {
			out = append(out, c)
		}
	}
	return out
}

// The Communication event of an ambient say carries the over-budget
// listeners, so the web client's Say tab and the AI companion skip them
// too; a plain say carries none.
func TestAmbientSay_CommunicationCarriesTheSkip(t *testing.T) {
	seedAmbientCommand(t, 5)
	events.DrainQueuedByTypeForTest(`Communication`)
	ambient.ChargeForTest(1, 6005)
	TryCommand("ambient", "mob_chatter say Budgeted words.", 100)
	got := sayEvents()
	if len(got) != 1 || !slices.Equal(got[0].ExcludeUserIds, []int{1}) {
		t.Errorf("ambient say: want one Communication excluding [1], got %+v", got)
	}

	TryCommand("say", "Plain words.", 100)
	got = sayEvents()
	if len(got) != 1 || len(got[0].ExcludeUserIds) != 0 {
		t.Errorf("plain say: want one Communication with no excludes, got %+v", got)
	}
}
