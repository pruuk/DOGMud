package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// wanderSecretRoom is room 1 with a secret north to 100 and an open south to
// 200; both far rooms hold gold, so `wander loot` wants both.
func wanderSecretRoom(t *testing.T) *rooms.Room {
	t.Helper()
	here := &rooms.Room{RoomId: 1, Exits: map[string]exit.RoomExit{
		"north": {RoomId: 100, Secret: true},
		"south": {RoomId: 200},
	}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1:   here,
		100: {RoomId: 100, Gold: 5},
		200: {RoomId: 200, Gold: 5},
	}, map[string]*rooms.ZoneConfig{}))
	return here
}

// #781, owner ruling 13: a wandering NPC takes a secret exit only once it
// found it.
func TestWanderCandidates_AnUnfoundSecretExitIsNoCandidate(t *testing.T) {
	room := wanderSecretRoom(t)
	mover := &characters.Character{}
	wantsAll := func(*rooms.Room) bool { return true }

	got := wanderCandidates(room, mover, wantsAll)
	if len(got) != 1 || got[0] != "south" {
		t.Fatalf("candidates %v, want only south", got)
	}
	mover.LearnSecretExit(1, "north")
	if got := wanderCandidates(room, mover, wantsAll); len(got) != 2 {
		t.Fatalf("after the find: candidates %v, want north and south", got)
	}
}

func TestPickWanderExit_FallbackTakesOnlyAFoundSecretExit(t *testing.T) {
	room := &rooms.Room{RoomId: 1, Exits: map[string]exit.RoomExit{"north": {RoomId: 100, Secret: true}}}
	mover := &characters.Character{}
	for range 50 {
		if name, _ := pickWanderExit(nil, room, mover); name != "" {
			t.Fatalf("picked %q: the only exit is an unfound secret", name)
		}
	}
	mover.LearnSecretExit(1, "north")
	if name, _ := pickWanderExit(nil, room, mover); name != "north" {
		t.Fatalf("picked %q, want the found north", name)
	}
}
