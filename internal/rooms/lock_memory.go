package rooms

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
)

// Lock state across an unload (#781, owner ruling 2026-11-25: every relock
// gets a new combination).
//
// A lock's combination comes from its RotationSeed and UnlockedRound
// (gamelock.Lock.Rotation). Neither survives an unload on its own: Room.Exits
// is instance:"skip", so a reloaded room's exit locks come back from the
// template at RotationSeed 0, and UnlockedRound is never saved (yaml:"-"), so
// an open exit or container comes back locked at the rotation it was opened
// at. Either way a combination retired by a relock would open the lock again.
//
// This is session state, not living state: a lock solution is good only until
// the server restarts (owner ruling 2026-11-18, util.LockSequence reseeds at
// every boot), so the lock state behind it needs to outlive an unload within
// one boot and nothing more. It lives in memory only and is never written to
// disk. A room's record is written whenever the room leaves memory
// (rememberLocks) and taken back when it loads (restoreLocks); ephemeral rooms
// are skipped, since their ids are reused by unrelated rooms.

type roomLocks struct {
	exits      map[string]gamelock.Lock
	containers map[string]gamelock.Lock
}

var (
	lockMemoryMu sync.Mutex
	lockMemory   = map[int]roomLocks{}
)

// rememberLocks records the state of every lock on r, replacing any older
// record for the room.
func rememberLocks(r *Room) {
	if r == nil {
		return
	}
	lockMemoryMu.Lock()
	defer lockMemoryMu.Unlock()
	if IsEphemeralRoomId(r.RoomId) {
		delete(lockMemory, r.RoomId)
		return
	}
	rec := roomLocks{}
	for name, ex := range r.Exits {
		if ex.HasLock() {
			if rec.exits == nil {
				rec.exits = map[string]gamelock.Lock{}
			}
			rec.exits[name] = ex.Lock
		}
	}
	for name, c := range r.Containers {
		if c.HasLock() {
			if rec.containers == nil {
				rec.containers = map[string]gamelock.Lock{}
			}
			rec.containers[name] = c.Lock
		}
	}
	if rec.exits == nil && rec.containers == nil {
		delete(lockMemory, r.RoomId)
		return
	}
	lockMemory[r.RoomId] = rec
}

// restoreLocks takes the room's record, if any, and puts each remembered
// lock's rotation and open state back on the lock of the same name. The rest
// of the lock (difficulty, traps, relock interval) stays as loaded, so a
// template edit still takes effect.
func restoreLocks(r *Room) {
	if r == nil {
		return
	}
	lockMemoryMu.Lock()
	rec, ok := lockMemory[r.RoomId]
	delete(lockMemory, r.RoomId)
	lockMemoryMu.Unlock()
	if !ok || IsEphemeralRoomId(r.RoomId) {
		return
	}
	for name, l := range rec.exits {
		if ex, ok := r.Exits[name]; ok && ex.HasLock() {
			ex.Lock.RotationSeed, ex.Lock.UnlockedRound = l.RotationSeed, l.UnlockedRound
			r.Exits[name] = ex
		}
	}
	for name, l := range rec.containers {
		if c, ok := r.Containers[name]; ok && c.HasLock() {
			c.Lock.RotationSeed, c.Lock.UnlockedRound = l.RotationSeed, l.UnlockedRound
			r.Containers[name] = c
		}
	}
}

// ExitLockAt returns the lock on a room's exit as it stands now, and never
// touches disk (the mapper calls it on the game loop): the live room's lock
// when the room is in memory, else the state remembered when it unloaded,
// else a lock of the given difficulty (the caller's crawl-time copy) at
// rotation 0. A room with no record has not been loaded and unloaded this
// boot, so its exit locks are as the template has them, and no template sets
// rotationseed on an exit (guarded in internal/guards,
// exit_rotationseed_guard_test.go).
func ExitLockAt(roomId int, exitName string, difficulty uint8) gamelock.Lock {
	if r := getRoomFromMemory(roomId); r != nil {
		if ex, ok := r.Exits[exitName]; ok {
			return ex.Lock
		}
	}
	lockMemoryMu.Lock()
	l, remembered := lockMemory[roomId].exits[exitName]
	lockMemoryMu.Unlock()
	if remembered {
		return l
	}
	return gamelock.Lock{Difficulty: difficulty}
}
