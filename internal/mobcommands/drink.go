package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Drink is eat under its other name for a mob, as for a player
// (usercommands.Drink): it takes food and cooked drinks. A mob drinks a
// potion with quaff (quaff.go).
func Drink(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	return Eat(rest, mob, room)
}
