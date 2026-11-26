package mobcommands

import (
	"errors"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// pathtoRouteWorld is two zones.
//
// PF, a ring: 9701 (0,0) east to 9702 (1,0) is secret both ways, so the short
// way from 9701 to 9702 is that one step and the open way is south, east,
// north through 9703 (0,1) and 9704 (1,1).
//
// PG: 9711 (10,10) east to 9712 is secret both ways and the only way there;
// 9711 south to 9713 is open.
func pathtoRouteWorld(t *testing.T) {
	t.Helper()
	room := func(id int, zone string, x, y int, exits map[string]exit.RoomExit) *rooms.Room {
		return &rooms.Room{RoomId: id, Zone: zone, Title: "Room", X: x, Y: y, Exits: exits}
	}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		9701: room(9701, "PF", 0, 0, map[string]exit.RoomExit{"east": {RoomId: 9702, Secret: true}, "south": {RoomId: 9703}}),
		9702: room(9702, "PF", 1, 0, map[string]exit.RoomExit{"west": {RoomId: 9701, Secret: true}, "south": {RoomId: 9704}}),
		9703: room(9703, "PF", 0, 1, map[string]exit.RoomExit{"north": {RoomId: 9701}, "east": {RoomId: 9704}}),
		9704: room(9704, "PF", 1, 1, map[string]exit.RoomExit{"west": {RoomId: 9703}, "north": {RoomId: 9702}}),
		9711: room(9711, "PG", 10, 10, map[string]exit.RoomExit{"east": {RoomId: 9712, Secret: true}, "south": {RoomId: 9713}}),
		9712: room(9712, "PG", 11, 10, map[string]exit.RoomExit{"west": {RoomId: 9711, Secret: true}}),
		9713: room(9713, "PG", 10, 11, map[string]exit.RoomExit{"north": {RoomId: 9711}}),
	}, map[string]*rooms.ZoneConfig{
		"PF": {Name: "PF", RoomId: 9701, RoomIds: map[int]struct{}{9701: {}, 9702: {}, 9703: {}, 9704: {}}},
		"PG": {Name: "PG", RoomId: 9711, RoomIds: map[int]struct{}{9711: {}, 9712: {}, 9713: {}}},
	}))
	// Fresh maps and an empty path cache, as a forced rebuild leaves them.
	mapper.GetMapper(9701, true)
	mapper.GetMapper(9711, true)
	// A forced rebuild also purges the shared path cache, so no route of this
	// fixture outlives the test.
	t.Cleanup(func() {
		mapper.GetMapper(9701, true)
		mapper.GetMapper(9711, true)
	})
	t.Cleanup(util.ResetRoundCountForTest)
}

func pathtoMob(roomId int) *mobs.Mob {
	m := &mobs.Mob{InstanceId: 9790}
	m.Character = *characters.New()
	m.Character.RoomId = roomId
	return m
}

// #781, owner ruling 13: `pathto found` goes the open way round an unfound
// secret exit and marks the path found-only; once the mob learns the exit it
// takes the short way. A scripted pathto takes the secret exit.
func TestPathto_FoundOnlyRouteSkipsAnUnfoundSecretExit(t *testing.T) {
	pathtoRouteWorld(t)
	room := rooms.LoadRoom(9701)

	m := pathtoMob(9701)
	if _, err := Pathto(mobs.PathtoFoundOnly+" 9702", m, room); err != nil {
		t.Fatalf("found route: %v", err)
	}
	if !m.Path.FoundOnly() {
		t.Error("a `pathto found` path must be marked found-only")
	}
	if step := m.Path.Peek(); m.Path.Len() != 3 || step == nil || step.ExitName() != "south" {
		t.Fatalf("found route: %d steps, first %v; want 3 steps, south first (the open way)", m.Path.Len(), step)
	}

	m.Character.LearnSecretExit(9701, "east")
	if _, err := Pathto(mobs.PathtoFoundOnly+" 9702", m, room); err != nil {
		t.Fatalf("found route after the find: %v", err)
	}
	if step := m.Path.Peek(); m.Path.Len() != 1 || step == nil || step.ExitName() != "east" {
		t.Fatalf("after the find: %d steps, first %v; want 1 step east", m.Path.Len(), step)
	}

	scripted := pathtoMob(9701)
	if _, err := Pathto("9702", scripted, room); err != nil {
		t.Fatalf("scripted route: %v", err)
	}
	if scripted.Path.FoundOnly() {
		t.Error("a scripted path must not be marked found-only")
	}
	if step := scripted.Path.Peek(); scripted.Path.Len() != 1 || step == nil || step.ExitName() != "east" {
		t.Fatalf("scripted route: %d steps, first %v; want 1 step east", scripted.Path.Len(), step)
	}
}

// #781: a found-only route that found no way is not searched again for
// pathtoRefusedRounds rounds, but learning the exit, another target or the
// end of the wait searches again.
func TestPathto_RefusedFoundOnlyRouteWaits(t *testing.T) {
	pathtoRouteWorld(t)
	room := rooms.LoadRoom(9711)
	util.SetRoundCountForTest(util.RoundCountMinimum + 100)

	m := pathtoMob(9711)
	cmd := mobs.PathtoFoundOnly + " 9712"
	if _, err := Pathto(cmd, m, room); err == nil || errors.Is(err, errPathtoRefusedRecently) {
		t.Fatalf("first try: %v, want a search that finds no way", err)
	}
	if _, err := Pathto(cmd, m, room); !errors.Is(err, errPathtoRefusedRecently) {
		t.Fatalf("second try, same round: %v, want no search", err)
	}

	// Another target is searched.
	if _, err := Pathto(mobs.PathtoFoundOnly+" 9713", m, room); err != nil {
		t.Fatalf("another target: %v, want a route", err)
	}

	// The wait ends.
	m.Path.Clear()
	util.SetRoundCountForTest(util.RoundCountMinimum + 100 + pathtoRefusedRounds)
	if _, err := Pathto(cmd, m, room); err == nil || errors.Is(err, errPathtoRefusedRecently) {
		t.Fatalf("after the wait: %v, want a fresh search that finds no way", err)
	}

	// Learning the exit lifts it at once.
	m.Character.LearnSecretExit(9711, "east")
	if _, err := Pathto(cmd, m, room); err != nil {
		t.Fatalf("after the find: %v, want a route", err)
	}
	if step := m.Path.Peek(); step == nil || step.ExitName() != "east" {
		t.Fatalf("after the find: first step %v, want east", step)
	}
}
