// Package itemlight holds the light and darkness that fixtures give their
// rooms (lighting 5e, item behaviour slice 1, spec X6).
//
// A fixture is an item fixed to a room's floor (ItemSpec `fixture: light` or
// `fixture: darkness`). Its behaviour tree writes its output here through
// set_light and pulse_light, and internal/rooms reads every lit fixture of a
// room as one term each when it composes the room's light. The package is a
// leaf so that both can reach it: behaviortree imports rooms, so rooms cannot
// read tree state, and this sits below both.
//
// Outputs are in memory only, keyed by room and item UUID. An item's UUID is
// minted afresh on every load, so a restart or a room reload starts empty and
// the item tick writes the outputs again (the room's first visit evaluates
// its fixtures at once, internal/hooks).
package itemlight

import (
	"bytes"
	"math"
	"sort"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Kind is which combine a fixture's output joins.
type Kind uint8

const (
	// Light joins the room's light combine.
	Light Kind = iota
	// Darkness joins the room's darkness combine.
	Darkness
)

type output struct {
	kind  Kind
	value float64 // math.Inf(-1) when unlit
}

var (
	mu    sync.RWMutex
	rooms = map[int]map[uuid.UUID]output{}
)

// Set records a fixture's output in its room and reports whether anything
// changed. A non-finite or negative value records the fixture as unlit
// (it stays known, so a look can say "unlit").
func Set(roomId int, id uuid.UUID, kind Kind, value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		value = math.Inf(-1)
	}
	mu.Lock()
	defer mu.Unlock()
	byId, ok := rooms[roomId]
	if !ok {
		byId = map[uuid.UUID]output{}
		rooms[roomId] = byId
	}
	next := output{kind: kind, value: value}
	if prev, ok := byId[id]; ok && prev == next {
		return false
	}
	byId[id] = next
	return true
}

// Get returns a fixture's recorded output and whether one is recorded. An
// unlit fixture returns math.Inf(-1) and true.
func Get(roomId int, id uuid.UUID) (float64, bool) {
	mu.RLock()
	defer mu.RUnlock()
	o, ok := rooms[roomId][id]
	return o.value, ok
}

// Lit reports whether a fixture is recorded and gives light (or darkness).
func Lit(roomId int, id uuid.UUID) bool {
	v, ok := Get(roomId, id)
	return ok && !math.IsInf(v, -1)
}

// Clear drops one fixture's output: it left the floor.
func Clear(roomId int, id uuid.UUID) {
	mu.Lock()
	defer mu.Unlock()
	if byId, ok := rooms[roomId]; ok {
		delete(byId, id)
		if len(byId) == 0 {
			delete(rooms, roomId)
		}
	}
}

// ClearRoom drops every output in a room: it left memory.
func ClearRoom(roomId int) {
	mu.Lock()
	defer mu.Unlock()
	delete(rooms, roomId)
}

// Retain drops every output in a room whose UUID is not in keep, so an item
// that left the floor by a path that never called Clear stops lighting it.
func Retain(roomId int, keep map[uuid.UUID]bool) {
	mu.Lock()
	defer mu.Unlock()
	byId, ok := rooms[roomId]
	if !ok {
		return
	}
	for id := range byId {
		if !keep[id] {
			delete(byId, id)
		}
	}
	if len(byId) == 0 {
		delete(rooms, roomId)
	}
}

// Terms returns a room's lit light outputs and lit darkness outputs, each in
// a stable order (by UUID), ready for the room's two combines. Unlit
// fixtures add no term. O(fixtures in the room).
func Terms(roomId int) (light, dark []float64) {
	mu.RLock()
	defer mu.RUnlock()
	byId := rooms[roomId]
	if len(byId) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(byId))
	for id := range byId {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	for _, id := range ids {
		o := byId[id]
		if math.IsInf(o.value, -1) {
			continue
		}
		if o.kind == Darkness {
			dark = append(dark, o.value)
		} else {
			light = append(light, o.value)
		}
	}
	return light, dark
}

// ResetForTest empties every room and returns a restore func.
func ResetForTest() func() {
	mu.Lock()
	orig := rooms
	rooms = map[int]map[uuid.UUID]output{}
	mu.Unlock()
	return func() {
		mu.Lock()
		rooms = orig
		mu.Unlock()
	}
}
