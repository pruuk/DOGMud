package gmcp

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// queuedInputs drains userId's queued input events.
func queuedInputs(userId int) []events.Input {
	var out []events.Input
	for _, e := range events.DrainQueuedByTypeForTest(`Input`) {
		if in, ok := e.(events.Input); ok && in.UserId == userId && in.MobInstanceId == 0 {
			out = append(out, in)
		}
	}
	return out
}

func pressFixture(t *testing.T) (*users.UserRecord, map[string]string) {
	t.Helper()
	viewer, room := exitsFixture(t, 60)
	ids := buttons.MovementIds(viewer.UserId, room, rooms.KnownExits(room, viewer.Character))
	queuedInputs(viewer.UserId)
	events.DrainQueuedMessagesForTest(viewer.UserId)
	events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)
	events.DrainQueuedRoomExitsChangedForTest(0)
	return viewer, ids
}

// Ruling 7: a press goes through the normal input queue as the typed line,
// at the current turn (the one-command-per-turn limit applies; a ReadyTurn at
// or below 0 would bypass it, so the turn count is moved off 0 first; there
// is no setter, and one more turn harms no other test), flagged typed so a
// menu or the pager treats it as typing.
func TestPressButton_QueuesTheTypedLine(t *testing.T) {
	if util.GetTurnCount() == 0 {
		util.IncrementTurnCount()
	}
	viewer, ids := pressFixture(t)

	pressButton(viewer, "m1")
	pressButton(viewer, ids["northeast"])
	got := queuedInputs(viewer.UserId)
	require.Len(t, got, 2)
	require.Equal(t, "north", got[0].InputText)
	require.Equal(t, "go northeast", got[1].InputText)
	for _, in := range got {
		require.True(t, in.Flags.Has(events.CmdTyped), "a press is typed input")
		require.Equal(t, util.GetTurnCount(), in.ReadyTurn, "a press waits its turn like typing")
		require.NotZero(t, in.ReadyTurn, "ReadyTurn 0 would bypass the per-turn limit")
	}
}

// Abuse rules: an id never issued, another player's id, an exit name, an
// empty id, an overlong id and an old per-room id after moving are all
// refused with the same one line, which never repeats the id, so a refusal
// says nothing about whether the id ever existed; and Room.Info is sent again.
func TestPressButton_RefusesStaleIds(t *testing.T) {
	viewer, ids := pressFixture(t)
	kesh := users.GetByUserId(9723)
	keshIds := buttons.MovementIds(kesh.UserId, rooms.LoadRoom(9720), rooms.KnownExits(rooms.LoadRoom(9720), kesh.Character))
	t.Cleanup(func() { buttons.Forget(kesh.UserId) })

	refusalOf := func(name string) string {
		t.Helper()
		require.Empty(t, queuedInputs(viewer.UserId), name)
		msgs := events.DrainQueuedMessagesForTest(viewer.UserId)
		require.Len(t, msgs, 1, "%s: one line", name)
		require.Contains(t, msgs[0], refusal.StaleButtonLine, name)
		require.Len(t, events.DrainQueuedRoomExitsChangedForTest(viewer.UserId), 1, "%s: Room.Info is sent again", name)
		return msgs[0]
	}

	lines := map[string]string{}
	for name, id := range map[string]string{
		"never issued":        "xdeadbeef",
		"another player's id": keshIds["northeast"],
		"an exit name":        "northeast",
		"an empty id":         "",
		"an overlong id":      strings.Repeat("x", 70000),
	} {
		pressButton(viewer, id)
		lines[name] = refusalOf(name)
	}

	viewer.Character.RoomId = 9721 // moved, no Room.Info built for the new room yet
	pressButton(viewer, ids["northeast"])
	lines["an old per-room id after moving"] = refusalOf("an old per-room id after moving")

	for name, line := range lines {
		require.Equal(t, lines["never issued"], line, "%s reads like an id that never existed", name)
	}
}

// A press flood: each stale press is refused, but the Room.Info re-send is
// queued once per player while one is pending.
func TestPressButton_StaleFloodSendsRoomInfoOnce(t *testing.T) {
	viewer, _ := pressFixture(t)
	for range 50 {
		pressButton(viewer, "xdeadbeef")
	}
	require.Empty(t, queuedInputs(viewer.UserId))
	require.Len(t, events.DrainQueuedMessagesForTest(viewer.UserId), 50)
	require.Len(t, events.DrainQueuedRoomExitsChangedForTest(viewer.UserId), 1)
}

// A press never answers a prompt.
func TestPressButton_DoesNothingWhileAPromptIsOpen(t *testing.T) {
	viewer, _ := pressFixture(t)
	_, ok := viewer.StartPrompt("mail", "Bob")
	require.True(t, ok)
	t.Cleanup(viewer.ClearPrompt)
	pressButton(viewer, "m1")
	require.Empty(t, queuedInputs(viewer.UserId))
	require.Empty(t, events.DrainQueuedMessagesForTest(viewer.UserId))
}

// The wire: Char.Button.Press reaches pressButton on MainWorker. A payload
// that is not {"id": "<string>"}, or a connection with no player, does
// nothing.
func TestHandleCharOp_ButtonPress(t *testing.T) {
	viewer, _ := pressFixture(t)
	gmcpModule.handleCharOp(GMCPCharOp{
		ConnectionId: viewer.ConnectionId(),
		Command:      `Char.Button.Press`,
		Payload:      []byte(`{"id":"m2"}`),
	})
	got := queuedInputs(viewer.UserId)
	require.Len(t, got, 1)
	require.Equal(t, "south", got[0].InputText)

	for _, op := range []GMCPCharOp{
		{ConnectionId: viewer.ConnectionId(), Command: `Char.Button.Press`, Payload: []byte(`{"id":2}`)},
		{ConnectionId: viewer.ConnectionId(), Command: `Char.Button.Press`, Payload: []byte(`north`)},
		{ConnectionId: 1, Command: `Char.Button.Press`, Payload: []byte(`{"id":"m2"}`)},
	} {
		gmcpModule.handleCharOp(op)
		require.Empty(t, queuedInputs(viewer.UserId), "%s", op.Payload)
		require.Empty(t, events.DrainQueuedMessagesForTest(viewer.UserId), "%s", op.Payload)
	}
}
