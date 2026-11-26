package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #781 owner ruling 17: `track` follows a trail through a secret exit the
// tracker has found, and loses it at one the tracker has not; an ordinary
// exit is unchanged. Both readers of findExitedTrack (the trail scan's exit
// column and the named track's direction) take the tracker, and an NPC
// tracker answers to the same rule by what it found.
func TestTrack_FollowsATrailThroughAFoundSecretExitOnly(t *testing.T) {
	pinConfigForTest(t)
	const here, behindCrack, behindNorth, quill, wren = 99611, 99612, 99613, 98611, 98613
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		here: {RoomId: here, Zone: "test", Exits: map[string]exit.RoomExit{
			"crack": {RoomId: behindCrack, Secret: true},
			"north": {RoomId: behindNorth},
		}},
		behindCrack: {RoomId: behindCrack, Zone: "test"},
		behindNorth: {RoomId: behindNorth, Zone: "test"},
	}, map[string]*rooms.ZoneConfig{}))
	room := rooms.LoadRoom(here)
	// Quill left through the secret crack; Wren left north.
	for id, via := range map[int]int{quill: behindCrack, wren: behindNorth} {
		name := map[int]string{quill: "Quill", wren: "Wren"}[id]
		m := newTrackTestMob(id, name, via)
		mobs.SetInstanceForTest(id, m)
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		room.MarkVisited(id, rooms.VisitorMob)
		rooms.LoadRoom(via).MarkVisited(id, rooms.VisitorMob)
	}

	// readsExit is what each reader names for tracker following quarry: the
	// trail scan's exit column and the named track's direction.
	readsExit := func(tracker *characters.Character, quarry string) (scan, named string) {
		found := false
		for _, v := range readRoomTrail(room, tracker, trailDetail{SeesAnything: true, SeesAll: true, SeesExits: true}, 0, 0) {
			if v.Name == quarry {
				scan, found = v.ExitName, true
			}
		}
		require.True(t, found, "premise: %s's trail is in the room", quarry)
		applied, _, _, dir, _, _, _ := lookupAdjacentTrail(room, tracker, strings.ToLower(quarry), 0)
		require.True(t, applied, "premise: the named trail is found")
		return scan, dir
	}

	for name, tracker := range map[string]*characters.Character{
		"player": {Name: "Tracker"},
		"npc":    &newTrackTestMob(98612, "Hound", here).Character,
	} {
		t.Run(name, func(t *testing.T) {
			scan, named := readsExit(tracker, "Quill")
			require.Equal(t, "", scan, "an unfound secret exit loses the trail")
			require.Equal(t, "", named, "an unfound secret exit loses the trail")

			scan, named = readsExit(tracker, "Wren")
			require.Equal(t, "north", scan, "an ordinary exit is unchanged")
			require.Equal(t, "north", named, "an ordinary exit is unchanged")

			tracker.LearnSecretExit(here, "crack")
			scan, named = readsExit(tracker, "Quill")
			require.Equal(t, "crack", scan, "a found secret exit carries the trail")
			require.Equal(t, "crack", named, "a found secret exit carries the trail")
		})
	}
}
