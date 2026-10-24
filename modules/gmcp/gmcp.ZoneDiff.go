package gmcp

import (
	"bytes"
	"encoding/json"
	"hash/fnv"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/mapper"
)

// zoneSentState is what the server last sent one player's web client as a
// Zone.Map (#650): the connection it was built for, the zone, the floor, the
// party positions and a fingerprint of every room.
type zoneSentState struct {
	conn  uint64
	zone  string
	cz    int
	party []int
	rooms map[int]uint64 // room id -> fingerprint
}

// zoneDiffResult says what to send: nothing (send false), the whole map
// (full), or only rooms that are new or changed. next replaces the state.
type zoneDiffResult struct {
	send  bool
	full  bool
	rooms []mapper.SnapshotRoom
	party []int
	next  *zoneSentState
}

// zoneDiff compares a fresh snapshot with what was last sent. A new
// connection, a zone change or a floor change sends the full map; otherwise
// only new or changed rooms, and a send at all only when a room or the party
// changed. Rooms are never removed: the visited set only grows.
func zoneDiff(prev *zoneSentState, conn uint64, zone string, cz int, party []int, snap []mapper.SnapshotRoom) zoneDiffResult {
	sortedParty := slices.Clone(party)
	slices.Sort(sortedParty)
	next := &zoneSentState{conn: conn, zone: zone, cz: cz, party: sortedParty, rooms: make(map[int]uint64, len(snap))}
	for _, r := range snap {
		next.rooms[r.RoomId] = roomFingerprint(r)
	}
	if prev == nil || prev.conn != conn || prev.zone != zone || prev.cz != cz {
		return zoneDiffResult{send: true, full: true, rooms: snap, party: sortedParty, next: next}
	}
	var changed []mapper.SnapshotRoom
	for _, r := range snap {
		if old, ok := prev.rooms[r.RoomId]; !ok || old != next.rooms[r.RoomId] {
			changed = append(changed, r)
		}
	}
	partyChanged := !slices.Equal(prev.party, sortedParty)
	if len(changed) == 0 && !partyChanged {
		return zoneDiffResult{next: next}
	}
	return zoneDiffResult{send: true, rooms: changed, party: sortedParty, next: next}
}

// roomFingerprint hashes everything the client draws for a room, so any
// visible change re-sends it.
//
// mapper.Snapshot fills Exits by ranging over mapNode.Exits, a Go map, so the
// order differs from call to call. The exits are hashed in a canonical order
// (each exit's own JSON, sorted) so an unchanged room always hashes the same.
// Tags are appended in a fixed order (bank, storage, shop) and need no sort.
func roomFingerprint(r mapper.SnapshotRoom) uint64 {
	exits := r.Exits
	r.Exits = nil
	h := fnv.New64a()
	b, _ := json.Marshal(r)
	h.Write(b)

	encoded := make([][]byte, 0, len(exits))
	for _, e := range exits {
		eb, _ := json.Marshal(e)
		encoded = append(encoded, eb)
	}
	slices.SortFunc(encoded, bytes.Compare)
	for _, eb := range encoded {
		h.Write([]byte{0})
		h.Write(eb)
	}
	return h.Sum64()
}
