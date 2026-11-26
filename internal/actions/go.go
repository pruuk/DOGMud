package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
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

// FindKnownExit is FindExit for a mover who may use only the exits they know
// (Room.FindKnownExitByName): a secret exit they have not found is not found
// (#781, owner ruling 1). Every player path that names an exit uses it; mob
// moves keep FindExit, since a scripted route takes its secret exits.
func FindKnownExit(room *rooms.Room, char *characters.Character, exitName string) FindExitResult {
	name, roomId := room.FindKnownExitByName(char, exitName)
	if name == `` {
		return FindExitResult{Found: false}
	}
	return FindExitResult{
		ExitName: name,
		RoomId:   roomId,
		Found:    true,
	}
}
