// Package roomlife is the engine side of generated ambient events: now and
// then, when a room is about to show one of its set ambient lines (the
// room's or its zone's IdleMessages, from the player round tick), a model
// writes a fresh event instead, seen or heard, from the room's text, the
// time of day, and who and what is there. The model side is
// modules/roomlife, which installs a Generator; with none installed nothing
// here does anything and every ambient line is the set one.
//
// The round tick calls TryReplace with the set line it was about to show.
// When it returns true the event is taken: the set line is not shown now,
// and a delivery goroutine asks the generator (off the mud lock) and then,
// under the lock, shows the new event, or the set line after all if none
// came.
//
// It only ever runs on a player's own key, for a player in the room who
// left "Make the world livelier" ticked (Generator.Reserve). The request
// carries authored room text and nothing of any player's (players are only
// counted, never named).
package roomlife

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// MaxGenerateTime caps one event's wait for the generator, whatever it is
// configured to allow. An event that takes longer is stale.
const MaxGenerateTime = 45 * time.Second

// Result is a generated event, already cleaned (CleanResult).
type Result struct {
	Kind string // KindSeen or KindHeard
	Text string

	// KeyholderOnly is an event the server could not moderate: only the
	// player whose key wrote it reads it; everyone else in the room gets
	// nothing (not the set line either).
	KeyholderOnly bool
}

// Generator is what modules/roomlife installs.
type Generator interface {
	// Chance is the percent (0 to 100) of the room's ambient lines, once
	// one is due, that are tried as generated events. Read on the game loop.
	Chance() int
	// Reserve reports whether userId's own key may write an event now and,
	// if so, takes their turn: Generate gives it back. Under the mud lock.
	Reserve(userId int) bool
	// Generate writes the event on userId's key, after Reserve, and gives
	// the turn back however it ends. Off the mud lock, with a context
	// capped at MaxGenerateTime; it must not touch game state.
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

// pending is the rooms with an event on its way: one at a time each.
var pending = struct {
	sync.Mutex
	rooms map[int]bool
}{rooms: map[int]bool{}}

// Pending is how many events are on their way.
func Pending() int {
	pending.Lock()
	defer pending.Unlock()
	return len(pending.rooms)
}

func takeRoom(roomId int) bool {
	pending.Lock()
	defer pending.Unlock()
	if pending.rooms[roomId] {
		return false
	}
	pending.rooms[roomId] = true
	return true
}

func freeRoom(roomId int) {
	pending.Lock()
	defer pending.Unlock()
	delete(pending.rooms, roomId)
}

// Test seams: the chance roll (n in [0, 100)), the keyholder order, and
// the sight judgement.
var (
	roll    = func() int { return util.Rand(100) }
	shuffle = func(ids []int) {
		for i := len(ids) - 1; i > 0; i-- {
			j := util.Rand(i + 1)
			ids[i], ids[j] = ids[j], ids[i]
		}
	}
	canSee = func(userId int, room *rooms.Room) bool { return seesClearly(userId, room) }
)

// TryReplace is called by the round tick, under the mud lock, with the set
// ambient line (setMsg, already wrapped) it was about to show in room, and
// the pool that line came from (its voice, for the model). It returns true
// when it takes the event: the caller then does NOT show setMsg, which is
// shown later instead if no event comes.
//
// It takes the event only when a generator is installed, the chance roll
// hits, the room has no event already on its way, and an awake player in
// the room has a key the generator may use now (Reserve). The light rules
// decide what may be written: for a keyholder who sees the room clearly the
// event may be seen or heard; for one who does not, only heard
// (Request.CanSee false). Every reader then gets it through the room's own
// senders: a seen event sight-gated, a heard one on the audio channel.
func TryReplace(room *rooms.Room, setMsg string, pool []string) bool {
	g := installed()
	if g == nil || room == nil || strings.TrimSpace(setMsg) == `` {
		return false
	}
	chance := g.Chance()
	place, claimed := placeFor(room)
	if chance > 0 && claimed && place.Chance >= 0 {
		chance = place.Chance // a rift asks more often than the world does
	}
	if chance <= 0 || roll() >= chance {
		return false
	}
	players := room.GetPlayers()
	if len(players) == 0 || !takeRoom(room.RoomId) {
		return false
	}
	candidates := append([]int(nil), players...)
	shuffle(candidates)
	keyholder, sees := 0, false
	for _, uid := range candidates {
		if !awake(uid) {
			continue
		}
		see := canSee(uid, room)
		if g.Reserve(uid) {
			keyholder, sees = uid, see
			break
		}
	}
	if keyholder == 0 {
		freeRoom(room.RoomId)
		return false
	}
	req := Snapshot(room, pool)
	req.CanSee = sees
	startDelivery(&delivery{
		gen:        g,
		userId:     keyholder,
		roomId:     room.RoomId,
		fallback:   setMsg,
		req:        req,
		maxGenWait: MaxGenerateTime,
	})
	return true
}

// awake reports whether a player is there and not asleep: a sleeper's key
// writes nothing (they would not notice it).
func awake(userId int) bool {
	u := users.GetByUserId(userId)
	return u != nil && !u.Character.HasConditionFlag(conditions.Sleeping)
}

// seesClearly reports whether a player sees the room clearly now, by the
// engine's own predicate (light, blindness, sleep).
func seesClearly(userId int, light messaging.RoomVisibility) bool {
	u := users.GetByUserId(userId)
	return u != nil && light != nil && messaging.CanSeeClearly(u.Character, light)
}

// delivery is one event on its way.
type delivery struct {
	gen        Generator
	userId     int
	roomId     int
	fallback   string
	req        Request
	maxGenWait time.Duration
}

// startDelivery runs a delivery on its own goroutine, which takes the mud
// lock to finish. Tests replace it to run in line.
var startDelivery = func(d *delivery) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				freeRoom(d.roomId)
				mudlog.Error(`roomlife`, `action`, `deliver`, `panic`, r)
			}
		}()
		d.run(true)
	}()
}

// run asks the generator (off the lock), then finishes under the lock when
// lock is set.
func (d *delivery) run(lock bool) {
	ctx, cancel := context.WithTimeout(context.Background(), d.maxGenWait)
	res, err := d.gen.Generate(ctx, d.userId, d.req)
	cancel()
	if err == nil {
		res, err = CleanResult(res, d.req.CanSee)
	}
	if lock {
		util.LockMud()
		defer util.UnlockMud()
	}
	defer freeRoom(d.roomId)
	d.finish(res, err)
}

// finish shows the event, or the set line when none came, to whoever is in
// the room now, through the room's own senders, the same ones every ambient
// line uses: a seen event or the set line through the sight-gated visual
// sender (nothing in the dark, a shapes render at shapes), a heard event on
// the audio channel (heard whatever the light). A KeyholderOnly event goes
// the same way with every other player excluded, and only while the
// keyholder is still there. Runs under the mud lock.
func (d *delivery) finish(res Result, err error) {
	room := rooms.LoadRoom(d.roomId)
	if room == nil || room.PlayerCt() < 1 {
		return
	}
	if err != nil {
		room.SendTextVisual(messaging.CategoryRoomDescription, d.fallback)
		return
	}
	var exclude []int
	if res.KeyholderOnly {
		here := false
		for _, uid := range room.GetPlayers() {
			if uid == d.userId {
				here = true
			} else {
				exclude = append(exclude, uid)
			}
		}
		if !here {
			return
		}
	}
	text := util.SplitStringNL(res.Text, 80)
	if res.Kind == KindHeard {
		room.SendText(messaging.CategoryRoomDescription, text, exclude...)
		return
	}
	room.SendTextVisual(messaging.CategoryRoomDescription, text, exclude...)
}
