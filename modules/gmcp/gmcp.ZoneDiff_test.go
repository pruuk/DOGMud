package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/stretchr/testify/require"
)

func snap(ids ...int) []mapper.SnapshotRoom {
	out := []mapper.SnapshotRoom{}
	for _, id := range ids {
		out = append(out, mapper.SnapshotRoom{RoomId: id, X: id, Symbol: "o", Name: "r"})
	}
	return out
}

func TestZoneDiff_FirstSendIsFull(t *testing.T) {
	d := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2, 3))
	require.True(t, d.send)
	require.True(t, d.full)
	require.Len(t, d.rooms, 3)
}

func TestZoneDiff_RepeatStepSendsNothing(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2, 3)).next
	d := zoneDiff(st, 7, "Thornwall", 0, nil, snap(1, 2, 3))
	require.False(t, d.send)
}

func TestZoneDiff_NewRoomSendsOnlyIt(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2)).next
	d := zoneDiff(st, 7, "Thornwall", 0, nil, snap(1, 2, 3))
	require.True(t, d.send)
	require.False(t, d.full)
	require.Len(t, d.rooms, 1)
	require.Equal(t, 3, d.rooms[0].RoomId)
}

func TestZoneDiff_ChangedRoomIsResent(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2)).next
	s := snap(1, 2)
	s[1].Tags = []string{"shop"}
	d := zoneDiff(st, 7, "Thornwall", 0, nil, s)
	require.True(t, d.send)
	require.False(t, d.full)
	require.Len(t, d.rooms, 1)
	require.Equal(t, 2, d.rooms[0].RoomId)
}

func TestZoneDiff_ExitFlagChangeIsResent(t *testing.T) {
	s := snap(1)
	s[0].Exits = []mapper.SnapshotExit{{ToRoomId: 2, DX: 1}}
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, s).next
	s2 := snap(1)
	s2[0].Exits = []mapper.SnapshotExit{{ToRoomId: 2, DX: 1, Locked: true}}
	d := zoneDiff(st, 7, "Thornwall", 0, nil, s2)
	require.True(t, d.send)
	require.Len(t, d.rooms, 1)
}

func TestZoneDiff_ZoneFloorOrConnectionChangeIsFull(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, nil, snap(1, 2)).next
	require.True(t, zoneDiff(st, 7, "Stillwater", 0, nil, snap(1, 2)).full, "zone change")
	require.True(t, zoneDiff(st, 7, "Thornwall", 1, nil, snap(1, 2)).full, "floor change")
	require.True(t, zoneDiff(st, 8, "Thornwall", 0, nil, snap(1, 2)).full, "new connection")
}

func TestZoneDiff_PartyChangeAloneSends(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, []int{5}, snap(1)).next
	d := zoneDiff(st, 7, "Thornwall", 0, []int{6}, snap(1))
	require.True(t, d.send)
	require.False(t, d.full)
	require.Empty(t, d.rooms)
	require.Equal(t, []int{6}, d.party)
	require.False(t, zoneDiff(d.next, 7, "Thornwall", 0, []int{6}, snap(1)).send)
}

func TestZoneDiff_PartyOrderDoesNotMatter(t *testing.T) {
	st := zoneDiff(nil, 7, "Thornwall", 0, []int{5, 6}, snap(1)).next
	require.False(t, zoneDiff(st, 7, "Thornwall", 0, []int{6, 5}, snap(1)).send)
}

// mapper.Snapshot builds a room's exits by ranging over mapNode.Exits, a Go
// map, so the same room can arrive with its exits in a different order on
// every call. That must not count as a change.
func TestZoneDiff_ExitOrderDoesNotMatter(t *testing.T) {
	a := mapper.SnapshotExit{ToRoomId: 2, DX: 1}
	b := mapper.SnapshotExit{ToRoomId: 3, DY: 1, Locked: true}
	s1 := snap(1)
	s1[0].Exits = []mapper.SnapshotExit{a, b}
	s2 := snap(1)
	s2[0].Exits = []mapper.SnapshotExit{b, a}
	require.Equal(t, roomFingerprint(s1[0]), roomFingerprint(s2[0]))
	require.Equal(t, []mapper.SnapshotExit{b, a}, s2[0].Exits, "fingerprint must not reorder the caller's exits")

	st := zoneDiff(nil, 7, "Thornwall", 0, nil, s1).next
	require.False(t, zoneDiff(st, 7, "Thornwall", 0, nil, s2).send)
}
