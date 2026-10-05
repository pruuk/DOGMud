package roomlife

import (
	"sync/atomic"

	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Place is what a subsystem that owns a room (a rift) says about it for
// generated events, in place of what the world would assume.
type Place struct {
	// Chance replaces the module's Chance for this room (0 to 100), so long
	// as the module's own is above zero: an operator who turns generation
	// off turns it off here too. Negative keeps the module's.
	Chance int
	// Setting tells the model where this room really is when that is not
	// the world the system prompt describes (another dimension, say). It
	// goes into the request as written; authored text only.
	Setting string
	// Timeless rooms carry no time of day: there is no sky to tell it by.
	Timeless bool
}

// PlaceHook reports the Place for room, or false for an ordinary room.
// Called under the mud lock; it must be quick and free of side effects.
type PlaceHook func(room *rooms.Room) (Place, bool)

var placeHook atomic.Pointer[PlaceHook]

// SetPlaceHook installs the hook (a module, at init); nil removes it.
func SetPlaceHook(h PlaceHook) {
	if h == nil {
		placeHook.Store(nil)
		return
	}
	placeHook.Store(&h)
}

// placeFor is the room's Place, if a hook claims it.
func placeFor(room *rooms.Room) (Place, bool) {
	if h := placeHook.Load(); h != nil && room != nil {
		return (*h)(room)
	}
	return Place{}, false
}
