package dialogue

import (
	"github.com/GoMudEngine/GoMud/internal/util"
)

// PlayerMemory is the in-process state one mob instance keeps about one
// player: whether it has greeted them, and when, so SweepMemories can age the
// entry out. What a player has heard is saved on the character (HeardTopics),
// never here.
type PlayerMemory struct {
	LastVisitRound uint64
	Greeted        bool // this mob instance has greeted this player (5a; in-process, so once per boot)
}

// memoryCache keys: (mobInstanceId << 32 | uint32(userId)) → *PlayerMemory
var memoryCache = map[uint64]*PlayerMemory{}

// memorySweepIdleRounds is how long a (mob instance, player) pair may go
// untouched before its memory is dropped by SweepMemories.
//
// The cache grows by one entry per pair that has ever met and would
// otherwise grow monotonically with uptime and player count, holding entries
// for mob instances that despawned days earlier. The state is already
// per-instance and lost on restart, so dropping an idle entry costs nothing a
// respawn would not.
const memorySweepIdleRounds uint64 = 20000

func memKey(mobInstanceId, userId int) uint64 {
	return uint64(mobInstanceId)<<32 | uint64(uint32(userId))
}

// SweepMemories drops memories untouched for longer than
// memorySweepIdleRounds and returns how many were removed. Safe to call on a
// timer; MarkGreeted refreshes an entry's LastVisitRound.
func SweepMemories() int {
	now := util.GetRoundCount()
	removed := 0

	for key, mem := range memoryCache {
		// LastVisitRound == 0 means the entry was created by GetMemory but
		// never marked. Leave it; it is one round old at most.
		if mem.LastVisitRound == 0 {
			continue
		}
		if now-mem.LastVisitRound > memorySweepIdleRounds {
			delete(memoryCache, key)
			removed++
		}
	}

	return removed
}

// ForgetMobInstance drops every player's memory of one mob instance. Call it
// when a mob despawns: the instance id may be reused by a later spawn, and a
// new mob should not inherit a stranger's history.
func ForgetMobInstance(mobInstanceId int) int {
	removed := 0
	prefix := uint64(mobInstanceId) << 32

	for key := range memoryCache {
		if key&0xFFFFFFFF00000000 == prefix {
			delete(memoryCache, key)
			removed++
		}
	}

	return removed
}

// GetMemory returns the player's memory for this mob instance, creating it if absent.
func GetMemory(mobInstanceId, userId int) *PlayerMemory {
	key := memKey(mobInstanceId, userId)
	if mem, ok := memoryCache[key]; ok {
		return mem
	}
	mem := &PlayerMemory{}
	memoryCache[key] = mem
	return mem
}
