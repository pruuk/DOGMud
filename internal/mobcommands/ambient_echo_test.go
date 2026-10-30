package mobcommands

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const keenEarsConditionId = 3170

// seedEchoNeighbour: the ambient fixture with user 2 moved next door into
// room 2 (room 1's north neighbour, which leads back south) and given the
// superhearing flag, the only thing that lets a quiet echo reach a player.
func seedEchoNeighbour(t *testing.T, budget int) {
	t.Helper()
	seedAmbientCommand(t, budget)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		keenEarsConditionId: {ConditionId: keenEarsConditionId, Name: "Keen Ears", RoundInterval: 1, TriggerCount: 1000,
			Flags: []conditions.Flag{conditions.SuperHearing}},
	}))
	u2 := users.GetByUserId(2)
	rooms.LoadRoom(1).RemovePlayer(2)
	u2.Character.RoomId = 2
	rooms.LoadRoom(2).AddPlayer(2)
	u2.Character.Conditions.AddCondition(keenEarsConditionId, false)
	if !u2.Character.HasConditionFlag(conditions.SuperHearing) {
		t.Fatal("fixture: user 2 must hold the superhearing flag")
	}
	events.DrainQueuedRoomMessagesForTest(2)
}

func echoTo(t *testing.T, roomId int) events.Message {
	t.Helper()
	for _, m := range events.DrainQueuedRoomMessagesForTest(roomId) {
		if strings.Contains(m.Text, "You hear someone talking.") {
			return m
		}
	}
	t.Fatalf("no echo queued for room %d", roomId)
	return events.Message{}
}

func TestAmbientSayEcho_OverBudgetNeighbourIsLeftOut(t *testing.T) {
	seedEchoNeighbour(t, 5)
	ambient.ChargeForTest(2, 6005)
	TryCommand("ambient", "mob_chatter say Fresh bread!", 100)
	if m := echoTo(t, 2); !slices.Contains(m.ExcludeUserIds, 2) {
		t.Fatalf("user 2 is over budget and must be left out of the echo, excludes %v", m.ExcludeUserIds)
	}
}

func TestAmbientSayEcho_InBudgetNeighbourHearsAndIsCharged(t *testing.T) {
	seedEchoNeighbour(t, 5)
	TryCommand("ambient", "gossip say Heard about the bridge?", 100)
	if m := echoTo(t, 2); slices.Contains(m.ExcludeUserIds, 2) {
		t.Fatalf("user 2 has budget and must get the echo, excludes %v", m.ExcludeUserIds)
	}
	// The echo charged user 2. (A second ambient say would not show it: user
	// 1 in the speaker's room is now over budget too, and a say nobody in its
	// own room hears is dropped whole, echo included.)
	if ambient.AllHaveBudget(rooms.LoadRoom(2)) {
		t.Fatal("the echo must have charged user 2 against the ambient budget")
	}
}

func TestPlainMobSayEcho_IsNeverBudgeted(t *testing.T) {
	seedEchoNeighbour(t, 5)
	ambient.ChargeForTest(2, 6005)
	TryCommand("say", "Halt, who goes there?", 100)
	if m := echoTo(t, 2); slices.Contains(m.ExcludeUserIds, 2) {
		t.Fatalf("a plain say is not ambient; its echo must not leave anyone out, excludes %v", m.ExcludeUserIds)
	}
}
