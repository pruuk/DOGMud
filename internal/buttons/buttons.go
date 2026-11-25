// Package buttons is the server side of the touch controls' buttons (#781,
// touch arc spec 2, the first slice of the button protocol). The client never
// builds a command: the server gives every button an opaque id, the client
// sends back only the id, and the server turns it into the line the player
// would have typed. Movement is the first slice: the six directions of the
// inverted T have fixed ids, the same in every room; every other exit gets an
// id issued to one player for one room. Dark marks, the directions a player
// tried in the dark, live here too (darkmarks.go). All state is in memory and
// per session: nothing here is living state.
package buttons

import (
	"maps"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// fixedIds are the T directions' ids (owner ruling 8). They are the same in
// every room and for every player, so they say nothing about where an exit is,
// and they are the only ids the dark offers.
var fixedIds = map[string]string{
	"north": "m1", "south": "m2", "east": "m3", "west": "m4", "up": "m5", "down": "m6",
}

// fixedDirections is fixedIds turned round, id to direction, for Resolve.
var fixedDirections = func() map[string]string {
	out := make(map[string]string, len(fixedIds))
	for dir, id := range fixedIds {
		out[id] = dir
	}
	return out
}()

// FixedIds returns the T directions' ids, direction to id. Room.Info sends
// it in every room, dark or lit.
func FixedIds() map[string]string {
	return maps.Clone(fixedIds)
}

// issued is the per-room ids one player holds: the room they were issued
// for, and the exit name to id map both ways.
type issued struct {
	roomId int
	byName map[string]string
	byId   map[string]string
}

var (
	// mu guards issuedTo and marksOf (darkmarks.go).
	mu sync.Mutex
	// issuedTo is each player's per-room ids, by user id. Guarded by mu.
	issuedTo = map[int]*issued{}
)

// MovementIds returns the id of every movement button for userId standing in
// room, keyed by exit name: a T direction's fixed id, any other known exit's
// per-room id. Per-room ids are issued here, replacing the player's last set:
// an exit that is still known in the same room keeps its id, so a resend does
// not make a tap stale, and the same exits in the same room keep the held set
// as it is (the round watcher calls this every round). In the dark known lists no exits, so no per-room id
// survives (the fixed ids come from FixedIds). A nil room issues nothing and
// leaves the ids held as they were.
func MovementIds(userId int, room *rooms.Room, known rooms.KnownExitList) map[string]string {
	ids := map[string]string{}
	if room == nil {
		return ids
	}
	others := []string{}
	for _, ke := range known.Exits {
		if _, dup := ids[ke.Name]; dup {
			continue
		}
		if id, fixed := fixedIds[ke.Name]; fixed {
			ids[ke.Name] = id
			continue
		}
		ids[ke.Name] = ``
		others = append(others, ke.Name)
	}
	maps.Copy(ids, issue(userId, room.RoomId, others))
	return ids
}

// issue gives each of names a per-room id for userId in roomId.
func issue(userId, roomId int, names []string) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	prev := issuedTo[userId]
	// The round watcher resends Room.Info's ids every round: the same room
	// and the same exits keep the set already held, with nothing rebuilt.
	if sameNames(prev, roomId, names) {
		return maps.Clone(prev.byName)
	}
	next := &issued{roomId: roomId, byName: map[string]string{}, byId: map[string]string{}}
	for _, name := range names {
		id := ``
		if prev != nil && prev.roomId == roomId {
			id = prev.byName[name]
		}
		for id == `` || next.byId[id] != `` {
			id = newId()
		}
		next.byName[name] = id
		next.byId[id] = name
	}
	issuedTo[userId] = next
	return maps.Clone(next.byName)
}

// sameNames reports whether prev was issued for roomId to exactly names
// (MovementIds passes each name once).
func sameNames(prev *issued, roomId int, names []string) bool {
	if prev == nil || prev.roomId != roomId || len(prev.byName) != len(names) {
		return false
	}
	for _, name := range names {
		if _, ok := prev.byName[name]; !ok {
			return false
		}
	}
	return true
}

// newId is an opaque per-room id: "x" and eight random hex digits from
// crypto/rand. It encodes no room, destination or exit.
func newId() string {
	return `x` + util.RandomHex(4)
}

// Resolve turns a pressed id into the line the player would have typed: a
// fixed id is its bare direction ("north"), in any room; a per-room id is
// "go <exit>", but only while the player stands in the room it was issued
// for. ok is false for an id never issued to this player, or issued for
// another room: the press is stale (owner ruling 7).
func Resolve(userId, roomId int, id string) (line string, ok bool) {
	if dir, fixed := fixedDirections[id]; fixed {
		return dir, true
	}
	mu.Lock()
	defer mu.Unlock()
	is := issuedTo[userId]
	if is == nil || is.roomId != roomId {
		return ``, false
	}
	name, ok := is.byId[id]
	if !ok {
		return ``, false
	}
	return `go ` + name, true
}

// Forget drops every id and dark mark held for userId (they left the game).
func Forget(userId int) {
	mu.Lock()
	defer mu.Unlock()
	delete(issuedTo, userId)
	delete(marksOf, userId)
}
