package rooms

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Owned ephemeral chunks.
//
// CreateEphemeralRoomIds and GenerateOasisCube build a whole chunk of rooms at
// once and hand its lifetime to TryEphemeralCleanup, which unloads the chunk
// when nobody is in any of its rooms. That does not fit a subsystem that builds
// rooms one at a time while players are inside and tears each one down on its
// own schedule (internal/rifts). An OwnedChunk is a chunk reserved by such an
// owner: the owner adds and removes single rooms, the generic cleanup paths
// leave it alone, and the owner releases it when it is done.
//
// Every function here runs on the game loop, like the rest of the ephemeral
// bookkeeping (see the NOTE on OriginalRoomId).

var (
	ownedChunks = [ephemeralChunksLimit]*OwnedChunk{}

	errOwnedChunkFull     = errors.New(`owned chunk has no free room slots`)
	errOwnedChunkReleased = errors.New(`owned chunk was released`)
)

// OwnedChunk is one ephemeral chunk reserved by a single owner.
type OwnedChunk struct {
	id       int
	plane    int
	zone     string
	slots    [ephemeralChunkSize]bool
	released bool
}

// chunkFree reports whether chunk i can be handed to a new user: it holds no
// rooms and no owner has reserved it.
func chunkFree(i int) bool {
	return len(ephemeralRoomChunks[i]) == 0 && ownedChunks[i] == nil
}

// isOwnedChunk reports whether chunk i is reserved by an owner.
func isOwnedChunk(i int) bool {
	return i >= 0 && i < ephemeralChunksLimit && ownedChunks[i] != nil
}

// ReserveOwnedChunk reserves a free ephemeral chunk for rooms of zone. The
// chunk gets its own coordinate plane, marked non-Euclidean so the mapper and
// the consistency checks never try to lay it out.
func ReserveOwnedChunk(zone string) (*OwnedChunk, error) {
	for i := 0; i < ephemeralChunksLimit; i++ {
		if !chunkFree(i) {
			continue
		}
		c := &OwnedChunk{id: i, plane: nextInstancePlane(), zone: zone}
		ownedChunks[i] = c
		GetPlaneRegistry().Mark(c.plane, true, zone)
		return c, nil
	}
	return nil, errEphemeralChunkLimit
}

// Id is the chunk index.
func (c *OwnedChunk) Id() int { return c.id }

// Plane is the coordinate plane every room in the chunk is placed on.
func (c *OwnedChunk) Plane() int { return c.plane }

// RoomCount is how many rooms the chunk currently holds.
func (c *OwnedChunk) RoomCount() int {
	n := 0
	for _, used := range c.slots {
		if used {
			n++
		}
	}
	return n
}

// AddRoom gives r the next free room id in the chunk, places it on the
// chunk's plane and loads it into memory. r.Zone is left as the caller set it.
func (c *OwnedChunk) AddRoom(r *Room) (int, error) {
	if c.released {
		return 0, errOwnedChunkReleased
	}
	slot := -1
	for i, used := range c.slots {
		if !used {
			slot = i
			break
		}
	}
	if slot < 0 {
		return 0, errOwnedChunkFull
	}

	r.RoomId = ephemeralRoomIdMinimum + (c.id * ephemeralChunkSize) + slot
	r.Plane = c.plane
	if r.Exits == nil {
		r.Exits = map[string]exit.RoomExit{}
	}
	if err := addRoomToMemory(r); err != nil {
		return 0, fmt.Errorf(`OwnedChunk.AddRoom: %w`, err)
	}
	c.slots[slot] = true
	ephemeralRoomChunks[c.id] = append(ephemeralRoomChunks[c.id], r.RoomId)
	return r.RoomId, nil
}

// RemoveRoom unloads one room of the chunk. It refuses, returning false, while
// a player is in the room. Mobs in the room are destroyed without being saved,
// and any instance file a periodic save wrote for them is deleted, so a later
// room that reuses this id never inherits their progression. The room itself
// is not saved: ephemeral rooms have no instance file.
func (c *OwnedChunk) RemoveRoom(roomId int) bool {
	slot := roomId - ephemeralRoomIdMinimum - (c.id * ephemeralChunkSize)
	if slot < 0 || slot >= ephemeralChunkSize || !c.slots[slot] {
		return false
	}

	if r := roomManager.rooms[roomId]; r != nil {
		if len(r.players) > 0 {
			return false
		}
		destroyTransientMobs(r)
		if err := ClearRoomCache(roomId); err != nil {
			mudlog.Error(`OwnedChunk.RemoveRoom`, `roomId`, roomId, `error`, err)
		}
	}

	c.slots[slot] = false
	kept := ephemeralRoomChunks[c.id][:0]
	for _, id := range ephemeralRoomChunks[c.id] {
		if id != roomId {
			kept = append(kept, id)
		}
	}
	ephemeralRoomChunks[c.id] = kept
	return true
}

// Release unloads every room still in the chunk and frees it for reuse. Rooms
// with players in them cannot be unloaded; Release reports how many were left
// behind and keeps the chunk reserved in that case so nothing reuses their ids.
func (c *OwnedChunk) Release() (stranded int) {
	if c.released {
		return 0
	}
	for _, roomId := range append([]int(nil), ephemeralRoomChunks[c.id]...) {
		if !c.RemoveRoom(roomId) {
			stranded++
		}
	}
	if stranded > 0 {
		return stranded
	}
	ephemeralRoomChunks[c.id] = []int{}
	ownedChunks[c.id] = nil
	c.released = true
	return 0
}

// destroyTransientMobs removes every mob standing in or spawned for r without
// saving it, and deletes any instance file already written for it.
func destroyTransientMobs(r *Room) {
	ids := append([]int(nil), r.mobs...)
	for _, s := range r.SpawnInfo {
		if s.InstanceId > 0 {
			ids = append(ids, s.InstanceId)
		}
	}
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		m := mobs.GetInstance(id)
		if m == nil {
			continue
		}
		// A mob that has left the room is no longer this room's to destroy.
		if m.Character.RoomId != r.RoomId {
			continue
		}
		// A companion left behind (its owner died, fled or logged out) goes
		// to its owner, wherever they are now: the room's id will be reused,
		// and nothing may be left pointing at it.
		if m.Character.IsCharmed() && rehomeCompanion(m, r) {
			continue
		}
		if m.Character.IsCharmed() {
			if owner := users.GetByUserId(m.Character.Charmed.UserId); owner != nil {
				owner.Character.TrackCharmed(id, false)
			}
		}
		mobs.DeleteMobInstance(m.MobId, m.Zone, m.Character.Name, m.HomeRoomId)
		mobs.DestroyInstance(id)
	}
	r.mobs = nil
}

// rehomeCompanion moves a charmed mob standing in r to its owner's room. It
// reports false when the owner is not online or not somewhere it can go.
func rehomeCompanion(m *mobs.Mob, r *Room) bool {
	owner := users.GetByUserId(m.Character.Charmed.UserId)
	if owner == nil || owner.Character.RoomId == r.RoomId {
		return false
	}
	dest := LoadRoom(owner.Character.RoomId)
	if dest == nil {
		return false
	}
	dest.AddMob(m.InstanceId)
	return true
}

// OwnedChunkOf returns the id of the owned chunk roomId belongs to, or -1 when
// it is not in one.
func OwnedChunkOf(roomId int) int {
	if roomId < ephemeralRoomIdMinimum {
		return -1
	}
	i := (roomId - ephemeralRoomIdMinimum) / ephemeralChunkSize
	if !isOwnedChunk(i) {
		return -1
	}
	return i
}

// MobMayMove reports whether a mob may be moved between two rooms by a
// forced relocation that does not follow an exit (callforhelp's
// `go <roomId>`): never into, out of or between owned chunks, whose owner
// decides on its own who passes its doors.
func MobMayMove(fromRoomId, toRoomId int) bool {
	return OwnedChunkOf(fromRoomId) < 0 && OwnedChunkOf(toRoomId) < 0
}
