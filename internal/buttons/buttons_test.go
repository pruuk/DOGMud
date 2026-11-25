package buttons

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
)

func list(names ...string) rooms.KnownExitList {
	l := rooms.KnownExitList{}
	for _, n := range names {
		kind := rooms.ExitKindOf(n, n == "portal")
		l.Exits = append(l.Exits, rooms.KnownExit{Name: n, Kind: kind})
	}
	return l
}

var perRoomId = regexp.MustCompile(`^x[0-9a-f]{8}$`)

// Ruling 8: the six T directions have fixed ids, the same in every room and
// for every player; a fast double tap works like typing.
func TestMovementIds_TheTDirectionsHaveFixedIds(t *testing.T) {
	t.Cleanup(func() { Forget(1); Forget(2) })
	a := MovementIds(1, &rooms.Room{RoomId: 5}, list("north", "up"))
	b := MovementIds(2, &rooms.Room{RoomId: 6}, list("north", "up"))
	if a["north"] != "m1" || a["up"] != "m5" || b["north"] != "m1" || b["up"] != "m5" {
		t.Fatalf("fixed ids differ: %v %v", a, b)
	}
	for dir, id := range FixedIds() {
		line, ok := Resolve(1, 999, id)
		if !ok || line != dir {
			t.Errorf("fixed id %q resolves to %q %v in any room, want %q", id, line, ok, dir)
		}
	}
}

// Other exits get per-room ids: opaque, unguessable, stable while the player
// stays in the room, stale once they leave.
func TestMovementIds_PerRoomIdsAreStableThenStale(t *testing.T) {
	t.Cleanup(func() { Forget(1) })
	room5 := &rooms.Room{RoomId: 5}
	first := MovementIds(1, room5, list("north", "northeast", "out", "portal"))
	for _, name := range []string{"northeast", "out", "portal"} {
		if !perRoomId.MatchString(first[name]) {
			t.Fatalf("%s id %q is not an opaque per-room id", name, first[name])
		}
		if strings.Contains(first[name], name) {
			t.Fatalf("%s id %q names its exit", name, first[name])
		}
	}
	again := MovementIds(1, room5, list("north", "northeast", "out"))
	if again["northeast"] != first["northeast"] || again["out"] != first["out"] {
		t.Fatalf("a resend in the same room changed the ids: %v then %v", first, again)
	}
	if line, ok := Resolve(1, 5, first["northeast"]); !ok || line != "go northeast" {
		t.Fatalf("resolve %q = %q %v, want go northeast", first["northeast"], line, ok)
	}
	if _, ok := Resolve(1, 5, first["portal"]); ok {
		t.Error("the portal is gone, its id must be stale")
	}
	inSix := MovementIds(1, &rooms.Room{RoomId: 6}, list("south", "out"))
	if _, ok := Resolve(1, 5, first["northeast"]); ok {
		t.Error("after moving, an old per-room id must be stale")
	}
	if _, ok := Resolve(1, 6, first["northeast"]); ok {
		t.Error("an old per-room id must not resolve in the new room")
	}
	// A press that lands after a move but before the next Room.Info: the ids
	// held were issued for the room just left, so they are stale here.
	if _, ok := Resolve(1, 5, inSix["out"]); ok {
		t.Error("an id issued for room 6 must not resolve while the player stands in room 5")
	}
}

// Abuse rules: an id never issued, or issued to someone else, is refused.
func TestResolve_RefusesIdsNeverIssuedToThisPlayer(t *testing.T) {
	t.Cleanup(func() { Forget(1); Forget(2) })
	theirs := MovementIds(2, &rooms.Room{RoomId: 5}, list("northeast"))
	MovementIds(1, &rooms.Room{RoomId: 5}, list("northeast"))
	if _, ok := Resolve(1, 5, "xdeadbeef"); ok {
		t.Error("an id never issued must be refused")
	}
	if _, ok := Resolve(1, 5, theirs["northeast"]); ok {
		t.Error("another player's id must be refused")
	}
	if _, ok := Resolve(1, 5, "north"); ok {
		t.Error("an exit name is not an id")
	}
}

// Ids reveal nothing: two players in the same room get different per-room
// ids, so an id says nothing about the room or the exit.
func TestMovementIds_TwoPlayersGetDifferentIds(t *testing.T) {
	t.Cleanup(func() { Forget(1); Forget(2) })
	a := MovementIds(1, &rooms.Room{RoomId: 5}, list("northeast"))
	b := MovementIds(2, &rooms.Room{RoomId: 5}, list("northeast"))
	if a["northeast"] == b["northeast"] {
		t.Fatalf("two players got the same per-room id %q", a["northeast"])
	}
}

// In the dark (no known exits) no per-room id survives.
func TestMovementIds_TheDarkIssuesNoPerRoomIds(t *testing.T) {
	t.Cleanup(func() { Forget(1) })
	lit := MovementIds(1, &rooms.Room{RoomId: 5}, list("northeast"))
	dark := MovementIds(1, &rooms.Room{RoomId: 5}, rooms.KnownExitList{Dark: true})
	if len(dark) != 0 {
		t.Fatalf("the dark issued %v", dark)
	}
	if _, ok := Resolve(1, 5, lit["northeast"]); ok {
		t.Error("a lit room's per-room id must be stale once the room is dark")
	}
}
