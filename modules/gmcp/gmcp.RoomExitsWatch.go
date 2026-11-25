package gmcp

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Room.Info kept current (#781, spec "Kept current").
//
// Room.Info goes out on a move and on a light change. A door unlocked or
// relocking on its timer (gamelock relocks lazily, by a time compare, and a
// dozen call sites change locks with no event), a gangplank or portal coming
// and going, a secret exit found, and a light going out all change what the
// touch buttons must show without either. Rather than touch every one of
// those writers, the module checks once a round, as the Char.Vitals watcher
// does (gmcp.CharVitalsWatch.go): it builds the exit half each GMCP client
// would be sent (buildRoomExitsInfo) and, when its signature differs from the
// last one actually sent to that user, re-sends Room.Info. A round with no
// change sends nothing. A dark mark does not wait for the round:
// events.RoomExitsChanged sends it at once.

var (
	lastSentRoomExits   = map[int]string{}
	lastSentRoomExitsMu sync.Mutex
)

// roomExitsSignature is everything the touch buttons show, as one comparable
// string: the room, the dark flag, each exit's name, kind, id, destination and
// details, and the dark marks. Map order is sorted away. The exits map and
// the fixed ids (moves) are left out: exits repeats exitsv2's names and
// destinations, and moves is the same in every room.
func roomExitsSignature(roomId int, dark bool, exitsV2 map[string]GMCPRoomModule_Payload_Contents_ExitInfo, marks map[string]string) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(roomId))
	if dark {
		b.WriteString(`|dark`)
	}
	for _, name := range slices.Sorted(maps.Keys(exitsV2)) {
		v := exitsV2[name]
		b.WriteString("\n")
		b.WriteString(name)
		b.WriteString(`|`)
		b.WriteString(v.Kind)
		b.WriteString(`|`)
		b.WriteString(v.Id)
		b.WriteString(`|`)
		b.WriteString(strconv.Itoa(v.RoomId))
		b.WriteString(`|`)
		b.WriteString(strings.Join(v.Details, `,`))
	}
	for _, dir := range slices.Sorted(maps.Keys(marks)) {
		b.WriteString("\nmark|")
		b.WriteString(dir)
		b.WriteString(`|`)
		b.WriteString(marks[dir])
	}
	return b.String()
}

// recordSentRoomExits notes the exit half of the Room.Info just sent to a user.
func recordSentRoomExits(userId int, signature string) {
	lastSentRoomExitsMu.Lock()
	lastSentRoomExits[userId] = signature
	lastSentRoomExitsMu.Unlock()
}

// forgetSentRoomExits drops a user's record, so a later login starts clean.
func forgetSentRoomExits(userId int) {
	lastSentRoomExitsMu.Lock()
	delete(lastSentRoomExits, userId)
	lastSentRoomExitsMu.Unlock()
}

// roomExitsDue reports whether what u's buttons should show differs from the
// last Room.Info sent to them. Never sent is due. It asks ParticipantSight
// once and hands the verdict to buildRoomExitsInfo, as GetRoomNode does.
func roomExitsDue(u *users.UserRecord) bool {
	room := rooms.LoadRoom(u.Character.RoomId)
	if room == nil {
		return false
	}
	now := buildRoomExitsInfo(u, room, messaging.ParticipantSight(u.Character, room))
	sig := roomExitsSignature(room.RoomId, now.Dark, now.ExitsV2, now.DarkMarks)
	lastSentRoomExitsMu.Lock()
	last, ok := lastSentRoomExits[u.UserId]
	lastSentRoomExitsMu.Unlock()
	return !ok || last != sig
}

// roomExitsWatchHandler runs each round and re-sends Room.Info to every GMCP
// client whose exits, locks, dark flag or dark marks moved since the last
// one. Telnet users without GMCP cost one map lookup.
func (g *GMCPRoomModule) roomExitsWatchHandler(e events.Event) events.ListenerReturn {
	if _, ok := e.(events.NewRound); !ok {
		return events.Continue
	}
	for _, u := range users.GetAllActiveUsers() {
		if u.UserId == 0 || u.Character == nil || !isGMCPEnabled(u.ConnectionId()) {
			continue
		}
		if roomExitsDue(u) {
			events.AddToQueue(GMCPRoomUpdate{UserId: u.UserId, Identifier: `Room.Info`})
		}
	}
	return events.Continue
}

// roomExitsChangedHandler sends Room.Info now: a dark mark was recorded.
func (g *GMCPRoomModule) roomExitsChangedHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomExitsChanged)
	if !ok || evt.UserId < 1 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || !isGMCPEnabled(user.ConnectionId()) {
		return events.Continue
	}
	events.AddToQueue(GMCPRoomUpdate{UserId: evt.UserId, Identifier: `Room.Info`})
	return events.Continue
}
