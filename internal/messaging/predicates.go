package messaging

import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)

// RoomVisibility is the minimal interface CanSeeClearly / CanSeeShapes
// need from a room. *rooms.Room satisfies this implicitly. Decoupled
// so messaging/ does not import rooms/ — rooms/ imports messaging/,
// and an interface here keeps the dependency arrow one-way.
type RoomVisibility interface {
	GetVisibility() int
}

// CanSeeClearly returns true if the observer can read normal-text
// visual broadcasts in this room. Composes Perception state, room
// lighting, and the NightVision buff flag.
//
// Blinded observers (any source) return false unconditionally.
// A nil observer defaults to true (defensive — pre-init characters
// during boot must not be silently dropped).
func CanSeeClearly(observer *characters.Character, room RoomVisibility) bool {
	if observer == nil {
		return true
	}
	if observer.Perception != nil && observer.Perception.State() == perception.Blinded {
		return false
	}
	// Sleep is a perception state, even though it is carried as a buff flag
	// rather than by the Perception machine. This pipeline had no concept of it
	// at all until 2026-08-31, so a sleeping player kept receiving every visual
	// broadcast in the room: NPC dialogue, ambient flavour, arrivals.
	//
	// AUDIO IS DELIBERATELY UNAFFECTED. Room.SendText bypasses this gate, so a
	// shout still reaches a sleeper and still wakes them (shout.go owns that
	// wake trigger). Gating audio here would make sleep unwakeable by sound.
	if observer.HasBuffFlag(buffs.Sleeping) {
		return false
	}
	if room == nil || roomIsLit(room) {
		return true
	}
	return observer.HasFlagFromAnySource(buffs.NightVision)
}

// CanSeeShapes returns true if the observer can detect SOMETHING is
// happening — either full clarity (subsumes CanSeeClearly) OR
// infrared in the dark. Blindness gates this too — broken eyes don't
// see infrared. So does sleep: closed eyes see no shapes.
//
// A nil observer defaults to true (matches CanSeeClearly's defensive
// behavior).
func CanSeeShapes(observer *characters.Character, room RoomVisibility) bool {
	if CanSeeClearly(observer, room) {
		return true
	}
	if observer == nil {
		return true
	}
	if observer.Perception != nil && observer.Perception.State() == perception.Blinded {
		return false
	}
	// Must be repeated here, not inherited. CanSeeClearly returning false is
	// the NORMAL path into this function (that is what "in the dark" means), so
	// a sleeper reaching the infrared branch would see shapes while asleep.
	if observer.HasBuffFlag(buffs.Sleeping) {
		return false
	}
	return observer.HasFlagFromAnySource(buffs.InfraredVision)
}

// roomIsLit returns true if the room is bright enough to read
// (visibility >= 1). Helper so callers don't need to know the
// threshold value.
func roomIsLit(room RoomVisibility) bool {
	if room == nil {
		return true
	}
	// Reflection-free nil-interface guard: a typed-nil *rooms.Room
	// would panic on GetVisibility; callers must pass nil interface,
	// not a typed-nil. The room/Room.SendText path always has a real
	// receiver, so this is safe in practice.
	return room.GetVisibility() >= 1
}
