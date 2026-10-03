package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Taking from a merchant's chest (internal/merchantchests) is watched once
// per lock cycle: the first take after the chest is opened runs the theft
// contest (actions.WatchMerchantChest), and a thief who got away with it may
// empty the rest without rolling again until the chest is locked afresh
// (restocked, or slammed shut by its merchant), which rotates the lock's
// RotationSeed. A thief caught is refused for the rest of that round, so the
// per-item Get calls of one `get all <chest>` raise one alarm, not several.

func merchantChestClearKey(room *rooms.Room, containerName string) string {
	return fmt.Sprintf(`merchantchest-clear:%d:%s`, room.RoomId, containerName)
}

func merchantChestCaughtKey(room *rooms.Room, containerName string) string {
	return fmt.Sprintf(`merchantchest-caught:%d:%s`, room.RoomId, containerName)
}

// merchantChestCaughtThisRound reports whether user was caught at this
// chest earlier in the current round.
func merchantChestCaughtThisRound(user *users.UserRecord, room *rooms.Room, containerName string) bool {
	at, ok := user.GetTempData(merchantChestCaughtKey(room, containerName)).(uint64)
	return ok && at == util.GetRoundCount()
}

// merchantChestTakeWatched runs the take contest for a merchant's chest and
// reports whether the take may go ahead. Any other container always may.
func merchantChestTakeWatched(user *users.UserRecord, room *rooms.Room, containerName string) bool {
	if !actions.IsMerchantChest(room, containerName) {
		return true
	}
	seed := room.Containers[containerName].Lock.RotationSeed
	clearKey := merchantChestClearKey(room, containerName)
	if cleared, ok := user.GetTempData(clearKey).(uint64); ok && cleared == seed {
		return true
	}
	if actions.WatchMerchantChest(actions.NewUserActorInRoom(user, room), containerName, actions.MerchantChestTake) {
		user.SetTempData(merchantChestCaughtKey(room, containerName), util.GetRoundCount())
		return false
	}
	user.SetTempData(clearKey, seed)
	return true
}
