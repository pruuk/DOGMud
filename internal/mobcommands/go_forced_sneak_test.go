package mobcommands

import (
	"strconv"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// mobGoForcedMoveScene seeds two rooms with no exit between them, a watcher
// in each, and a mob in fromRoom, then forces the mob to toRoom the way
// callforhelp does (`go <roomId>` with no matching exit). It returns the
// lines each watcher received, drained. Ruling D1: a sneaking mob's step is
// not announced, on this path exactly as it is on the normal exit path
// (actions.RelocateMob).
func mobGoForcedMoveScene(t *testing.T, sneaking bool) (fromMsgs, toMsgs []string) {
	t.Helper()
	const fromRoom, toRoom, instId = 9600, 9601, 98600
	const fromWatcher, toWatcher = 9610, 9611

	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		fromRoom: {RoomId: fromRoom, Zone: "test"},
		toRoom:   {RoomId: toRoom, Zone: "test"},
	}, map[string]*rooms.ZoneConfig{}))

	fw := users.NewTestUser(fromWatcher, "fromwatch", "Fromwatch", 0)
	fw.Character.RoomId = fromRoom
	tw := users.NewTestUser(toWatcher, "towatch", "Towatch", 0)
	tw.Character.RoomId = toRoom
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{fromWatcher: fw, toWatcher: tw}))

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Caller"
	m.Character.RoomId = fromRoom
	if sneaking {
		m.Character.SetMiscData("sneaking", true)
	}
	mobs.SetInstanceForTest(instId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	fromRoomObj, toRoomObj := rooms.LoadRoom(fromRoom), rooms.LoadRoom(toRoom)
	fromRoomObj.AddPlayer(fromWatcher)
	toRoomObj.AddPlayer(toWatcher)
	fromRoomObj.AddMob(instId)

	events.DrainQueuedMessagesForTest(fromWatcher)
	events.DrainQueuedMessagesForTest(toWatcher)

	handled, err := Go(strconv.Itoa(toRoom), m, fromRoomObj)
	if err != nil {
		t.Fatalf("Go returned error: %v", err)
	}
	if !handled {
		t.Fatalf("Go did not report handled")
	}

	fromMsgs = events.DrainQueuedMessagesForTest(fromWatcher)
	toMsgs = events.DrainQueuedMessagesForTest(toWatcher)
	return fromMsgs, toMsgs
}

// A sneaking mob forced to a non-adjacent room (callforhelp's `go <roomId>`)
// must send neither "runs off suddenly" nor "enters from nearby", exactly as
// a sneaking mob's ordinary step sends no line via actions.RelocateMob. A
// visible mob (the control) must send both.
func TestMobGo_ForcedMove_SneakGatesTheAnnounceLines(t *testing.T) {
	fromMsgs, toMsgs := mobGoForcedMoveScene(t, false)
	if len(fromMsgs) == 0 || len(toMsgs) == 0 {
		t.Fatalf("control: a visible mob's forced move must be announced, got from=%q to=%q", fromMsgs, toMsgs)
	}

	fromMsgs, toMsgs = mobGoForcedMoveScene(t, true)
	if len(fromMsgs) != 0 || len(toMsgs) != 0 {
		t.Errorf("a sneaking mob's forced move was announced: from=%q to=%q", fromMsgs, toMsgs)
	}
}
