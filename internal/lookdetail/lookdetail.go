// Package lookdetail is the engine side of generated closer looks. When a
// player's `look at X` finds nothing (no creature, container, carried or
// room noun, corpse or floor item) while they see the room clearly, and X is
// named in the room's description, a model writes what a closer look shows,
// in the room's voice. The model side is modules/lookdetail, which installs
// a Generator; with none installed, and nothing cached, the look answers as
// it always has.
//
// A detail is cached per room, description and phrase, once it may be shown
// to anyone (moderated, or moderation off), so a second look, by anyone,
// reads the same thing at no cost; a changed description starts afresh.
//
// It only ever runs on the looker's own key, while they leave "Make the
// world livelier" ticked (Generator.Reserve). The request carries authored
// room text and nothing of any player's.
package lookdetail

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// MaxGenerateTime caps one detail's wait for the generator.
const MaxGenerateTime = 45 * time.Second

// CacheEntries is how many details are kept, most recently read first.
const CacheEntries = 4096

// Result is a generated detail, already cleaned (CleanResult).
type Result struct {
	Text string

	// KeyholderOnly is a detail the server could not moderate: only the
	// player whose key wrote it reads it, and it is not cached.
	KeyholderOnly bool
}

// Generator is what modules/lookdetail installs.
type Generator interface {
	// Reserve reports whether userId's own key may write a detail now and,
	// if so, takes their turn: Generate gives it back. Under the mud lock.
	Reserve(userId int) bool
	// Generate writes the detail on userId's key, after Reserve, and gives
	// the turn back however it ends. Off the mud lock, with a context capped
	// at MaxGenerateTime; it must not touch game state.
	Generate(ctx context.Context, userId int, req Request) (Result, error)
}

var generator atomic.Pointer[Generator]

// SetGenerator installs the generator (the module, at load); nil removes it.
func SetGenerator(g Generator) {
	if g == nil {
		generator.Store(nil)
		return
	}
	generator.Store(&g)
}

func installed() Generator {
	if g := generator.Load(); g != nil {
		return *g
	}
	return nil
}

var cache, _ = lru.New[string, string](CacheEntries)

// cacheKey is a room, the description it had, and the phrase.
func cacheKey(roomId int, description string, phrase string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(description))
	return fmt.Sprintf(`%d|%x|%s`, roomId, h.Sum64(), phrase)
}

// ResetCacheForTest forgets every cached detail.
func ResetCacheForTest() { cache.Purge() }

// pending is the players with a detail on its way: one at a time each.
var pending = struct {
	sync.Mutex
	users map[int]bool
}{users: map[int]bool{}}

// Pending is how many details are on their way.
func Pending() int {
	pending.Lock()
	defer pending.Unlock()
	return len(pending.users)
}

func takeUser(userId int) bool {
	pending.Lock()
	defer pending.Unlock()
	if pending.users[userId] {
		return false
	}
	pending.users[userId] = true
	return true
}

func freeUser(userId int) {
	pending.Lock()
	defer pending.Unlock()
	delete(pending.users, userId)
}

// TryLook is called by the look command, under the mud lock, when user's
// look at lookAt found nothing while they see the room clearly. It returns
// true when it answers the look: at once from the cache, or later when a
// detail is written (the look command then says nothing more now). It
// returns false, and the look answers as it always has, when lookAt is not
// a phrase a closer look can be about, the room's description does not
// name it, or nothing is cached and no key may write it.
func TryLook(user *users.UserRecord, room *rooms.Room, lookAt string) bool {
	if user == nil || room == nil {
		return false
	}
	phrase, ok := Phrase(lookAt)
	if !ok {
		return false
	}
	description := room.GetDescription()
	shown, excerpt, ok := Find(description, phrase)
	if !ok {
		return false
	}
	key := cacheKey(room.RoomId, description, shown)
	if text, ok := cache.Get(key); ok {
		show(user, room, shown, text)
		return true
	}
	g := installed()
	if g == nil {
		return false
	}
	if !takeUser(user.UserId) {
		// Their last closer look is still on its way: this one waits on it.
		return true
	}
	if !g.Reserve(user.UserId) {
		freeUser(user.UserId)
		return false
	}
	req := Snapshot(room, shown, excerpt)
	startDelivery(&delivery{gen: g, userId: user.UserId, roomId: room.RoomId, key: key, shown: shown, req: req})
	return true
}

// delivery is one detail on its way.
type delivery struct {
	gen    Generator
	userId int
	roomId int
	key    string
	shown  string
	req    Request
}

// startDelivery runs a delivery on its own goroutine, which takes the mud
// lock to finish.
var startDelivery = func(d *delivery) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				freeUser(d.userId)
				mudlog.Error(`lookdetail`, `action`, `deliver`, `panic`, r)
			}
		}()
		d.run(true)
	}()
}

// InlineForTest runs deliveries in line, for tests in other packages (the
// look command's). It returns the restore.
func InlineForTest() (restore func()) {
	prev := startDelivery
	startDelivery = func(d *delivery) { d.run(false) }
	return func() { startDelivery = prev }
}

// run asks the generator (off the lock), then finishes under the lock when
// lock is set.
func (d *delivery) run(lock bool) {
	ctx, cancel := context.WithTimeout(context.Background(), MaxGenerateTime)
	res, err := d.gen.Generate(ctx, d.userId, d.req)
	cancel()
	if err == nil {
		res, err = CleanResult(res)
	}
	if lock {
		util.LockMud()
		defer util.UnlockMud()
	}
	defer freeUser(d.userId)
	d.finish(res, err)
}

// finish answers the look, under the mud lock, by the look command's own
// light rules judged now: nothing for a player who has left the room; the
// look command's own words for one who can no longer see clearly; else the
// detail, or "nothing special" when none came. A detail anyone may read is
// cached, whoever is still there.
func (d *delivery) finish(res Result, err error) {
	if err == nil && !res.KeyholderOnly {
		cache.Add(d.key, res.Text)
	}
	user := users.GetByUserId(d.userId)
	room := rooms.LoadRoom(d.roomId)
	if user == nil || room == nil || user.Character.RoomId != d.roomId {
		return
	}
	switch messaging.ParticipantSight(user.Character, room) {
	case messaging.SightNone:
		user.SendText(messaging.CategorySystem, `You can't see anything!`)
		return
	case messaging.SightShapes:
		user.SendText(messaging.CategorySystem, `You can only make out shapes here.`)
		return
	}
	if err != nil {
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You see nothing special about the <ansi fg="noun">%s</ansi>.`, d.shown))
		return
	}
	show(user, room, d.shown, res.Text)
}

// show answers a look with a detail, worded as a room noun's answer is in
// the look command, with the same line for onlookers (unless the looker is
// hidden), through the room's sight-gated sender.
func show(user *users.UserRecord, room *rooms.Room, shown string, text string) {
	user.SendText(messaging.CategoryRoomDescription, ``)
	user.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(`You look at the <ansi fg="noun">%s</ansi>:`, shown))
	user.SendText(messaging.CategoryRoomDescription, ``)
	if !user.Character.IsHidden() {
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> is examining the <ansi fg="noun">%s</ansi>.`, user.Character.Name, shown),
			[]string{user.Character.Name},
			user.UserId,
		)
	}
	user.SendText(messaging.CategoryRoomDescription, util.SplitStringNL(text, 80))
	user.SendText(messaging.CategoryRoomDescription, ``)
}
