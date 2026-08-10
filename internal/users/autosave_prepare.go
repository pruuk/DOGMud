package users

import (
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// User saving, split into a phase that reads live state and a phase that only
// touches bytes (roadmap chunk 3.6b-1, review finding 36).
//
// Users were originally OUT of scope for this work. 3.6a reported usersMs=0,
// but with zero players connected, so user saves were unmeasured rather than
// cheap. Benchmarked on a realistic 48KB established character:
//
//	old hand-rolled .new+rename, no fsync    0.696ms    70ms at 100 players
//	through util.Save (what chunk 2.8 ships) 3.873ms   387ms at 100 players
//
// Users were cheap because they were not durable. Chunk 2.8 fixed that, which
// made user saves the LARGER half of the autosave pause at the 100-player
// target -- ahead of the ~236ms room prepare. Deferring them would have meant
// shipping the sub-chunk that exists to bound the pause while the bigger half
// of it grew.

// autosaveQueue is shared with rooms via the hook, which owns the prepare/drain
// cycle. Users keep a reference rather than a second queue: guard G1 requires
// rooms and users to land in ONE pending set prepared in ONE lock hold, because
// the two halves of an item pickup live in different files.
var autosaveQueue = savequeue.New()

// SetAutosaveQueue points this package at the shared pending set. Called once
// at startup by the autosave hook.
func SetAutosaveQueue(q *savequeue.Queue) {
	if q != nil {
		autosaveQueue = q
	}
}

// userFilePathFor returns where a user's save file lives.
func userFilePathFor(u *UserRecord) string {
	return util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/`, `users`, `/`,
		strconv.Itoa(u.UserId)+`.yaml`)
}

// PrepareUserWrite computes what persisting this user would write, without
// writing it. The caller must hold the world lock.
//
// Unlike rooms there is no delete case: autosave never removes a user file.
// Removing one is account deletion, which is not an autosave concern, so Data
// is always non-nil on success.
func PrepareUserWrite(u *UserRecord) (savequeue.PendingWrite, error) {

	data, err := yaml.Marshal(u)
	if err != nil {
		return savequeue.PendingWrite{}, err
	}

	return savequeue.PendingWrite{
		Kind:    "user",
		Id:      u.UserId,
		Path:    userFilePathFor(u),
		Data:    data,
		Careful: bool(configs.GetFilePathsConfig().CarefulSaveFiles),
	}, nil
}

// CancelPendingUserWrite drops any queued write for this user.
//
// Guard G2. Every synchronous save path calls it, because a pending entry holds
// an older snapshot by definition and committing it afterwards would roll the
// character back to whenever autosave last looked at it.
//
// The stakes are higher here than for rooms: a stale user write can resurrect
// spent gold or a consumed item.
func CancelPendingUserWrite(u *UserRecord) {
	if u == nil {
		return
	}
	autosaveQueue.Cancel(userFilePathFor(u))
}

// PrepareAllUserWrites prepares every active user in one pass.
//
// Must be called under the world lock, as part of the SAME lock hold that
// prepares rooms. See the hook for why that is not merely tidy.
func PrepareAllUserWrites() ([]savequeue.PendingWrite, error) {
	userManager.mu.RLock()
	snapshot := make([]*UserRecord, 0, len(userManager.Users))
	for _, u := range userManager.Users {
		snapshot = append(snapshot, u)
	}
	userManager.mu.RUnlock()

	writes := make([]savequeue.PendingWrite, 0, len(snapshot))
	var firstErr error
	for _, u := range snapshot {
		p, err := PrepareUserWrite(u)
		if err != nil {
			// A marshal failure is this user's problem, not the cycle's. Record
			// it and keep going: one bad character must not stop every other
			// player's progress being persisted.
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		writes = append(writes, p)
	}

	return writes, firstErr
}
