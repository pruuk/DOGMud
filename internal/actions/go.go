package actions

import (
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// FindExitResult holds the outcome of an exit lookup.
type FindExitResult struct {
	ExitName string
	RoomId   int
	Found    bool
}

// IsCompassDirection reports whether name is one of the ten compass words a
// mover can try even where the room has no such exit ("bumping into walls").
// A player's go and a mob's go both read it to tell a movement attempt from
// any other word (#512).
func IsCompassDirection(name string) bool {
	switch name {
	case "north", "south", "east", "west", "up", "down",
		"northwest", "northeast", "southwest", "southeast":
		return true
	}
	return false
}

// FindExit looks up an exit on the given room by name (supports aliases and
// fuzzy matching via Room.FindExitByName). Returns Found=false when no
// matching exit exists.
func FindExit(room *rooms.Room, exitName string) FindExitResult {
	name, roomId := room.FindExitByName(exitName)
	if name == `` {
		return FindExitResult{Found: false}
	}
	return FindExitResult{
		ExitName: name,
		RoomId:   roomId,
		Found:    true,
	}
}
