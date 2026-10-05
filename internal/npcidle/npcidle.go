// Package npcidle is the engine side of generated idle moments: now and
// then, when an NPC is about to run one of its set idle lines (an emote or a
// say), a model writes it a fresh one instead, from the NPC's own text, the
// room's, and who and what is there. The model side is modules/npcidle,
// which installs a Generator; with none installed nothing here does anything
// and every idle line is the set one.
//
// The idle paths call TryReplace with the set line they were about to run.
// When it returns true the idle event is taken: the set line is not run now,
// and a delivery goroutine asks the generator (off the mud lock) and then,
// under the lock, has the NPC act the new moment, or the set line after all
// if no moment came.
//
// It only ever runs on a player's own key: the generator says which players
// in the room have one it may use (Generator.Reserve). The request carries
// authored NPC and room text and nothing of any player's (players are only
// counted, never named), so a moment is the same whoever's key paid for it.
package npcidle

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// MaxGenerateTime caps one moment's wait for the generator, whatever it is
// configured to allow. A moment that takes longer is stale: the NPC has
// moved on, and the set line is run instead.
const MaxGenerateTime = 45 * time.Second

// Result is a generated moment, already cleaned (CleanReply).
type Result struct {
	Kind string // KindEmote or KindSay
	Text string

	// KeyholderOnly is a moment the server could not moderate: only the
	// player whose key wrote it reads it; everyone else in the room sees
	// nothing (not the set line either, which would read as a double).
	KeyholderOnly bool
}

// Generator is what modules/npcidle installs.
type Generator interface {
	// Chance is the percent (0 to 100) of an NPC's idle emotes and says
	// that are tried as generated moments, read on the game loop.
	Chance() int
	// Reserve reports whether userId's own key may write a moment now
	// (their relay is up and allows it, they are not over their allowance
	// or still waiting on their last moment or its spacing), and if so
	// takes their turn: Generate gives it back. Called under the mud lock.
	Reserve(userId int) bool
	// Generate writes the moment on userId's key, after Reserve took their
	// turn, and gives the turn back however it ends. Called off the mud
	// lock with a context capped at MaxGenerateTime; it must not touch game
	// state.
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

// pending is the NPCs with a moment on its way: one at a time each.
var pending = struct {
	sync.Mutex
	mobs map[int]bool
}{mobs: map[int]bool{}}

// Pending is how many moments are on their way.
func Pending() int {
	pending.Lock()
	defer pending.Unlock()
	return len(pending.mobs)
}

func takeMob(instanceId int) bool {
	pending.Lock()
	defer pending.Unlock()
	if pending.mobs[instanceId] {
		return false
	}
	pending.mobs[instanceId] = true
	return true
}

func freeMob(instanceId int) {
	pending.Lock()
	defer pending.Unlock()
	delete(pending.mobs, instanceId)
}

// roll is the chance roll, a variable so tests can fix it: n in [0, 100).
var roll = func() int { return util.Rand(100) }

// canSee is seesClearly, a variable so tests need no lighting fixtures.
var canSee = func(userId int, room *rooms.Room) bool { return seesClearly(userId, room) }

// shuffle orders the candidate keyholders, a variable so tests can fix it.
var shuffle = func(ids []int) {
	for i := len(ids) - 1; i > 0; i-- {
		j := util.Rand(i + 1)
		ids[i], ids[j] = ids[j], ids[i]
	}
}

// IsFlavor reports whether a set idle command is a line of flavor a
// generated moment may stand in for: a single emote or say with something
// to it. Anything else (wandering, casting, an empty slot, a compound
// command) is left alone.
func IsFlavor(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == `` || strings.Contains(cmd, `;`) {
		return false
	}
	verb, rest, ok := strings.Cut(cmd, ` `)
	if !ok || strings.TrimSpace(rest) == `` {
		return false
	}
	switch strings.ToLower(verb) {
	case KindEmote, KindSay:
		return true
	}
	return false
}

// Excluded reports whether a mob never has generated moments: an AI
// companion (bonded, waiting in the Waystone Hollow, or any instance of a
// companion's template, whose words are the aicompanion module's), and a
// charmed mob, which belongs to whoever charmed it.
func Excluded(mob *mobs.Mob) bool {
	if mob == nil {
		return true
	}
	if companionai.IsBondedCompanion(mob.InstanceId) || companionai.DrivesBonded(int(mob.MobId)) {
		return true
	}
	for _, g := range mob.Groups {
		if g == mobs.HollowGroup {
			return true
		}
	}
	return mob.Character.IsCharmed()
}

// dormant reports whether a mob should not act an idle moment now.
func dormant(mob *mobs.Mob) bool {
	return mob.Character.IsDead() || mob.Character.IsInCombat() ||
		mob.Character.HasConditionFlag(conditions.Sleeping)
}

// TryReplace is called by an idle path with the set line (cmd) the mob was
// about to run, under the mud lock. It returns true when it takes the idle
// event: the caller then does NOT run cmd, which is run later instead if no
// moment comes. It takes the event only when a generator is installed, cmd
// is flavor (IsFlavor), the mob is not excluded or dormant and has no
// moment already on its way, the chance roll hits, and a player in the room
// who sees it clearly (the light rules every idle line follows) has a key
// the generator may use now. The moment itself is then acted through the
// mob's own emote or say, so every reader sees or hears it exactly as they
// would a set line.
func TryReplace(mob *mobs.Mob, cmd string) bool {
	g := installed()
	if g == nil || mob == nil || !IsFlavor(cmd) {
		return false
	}
	chance := g.Chance()
	if chance <= 0 || roll() >= chance {
		return false
	}
	if Excluded(mob) || dormant(mob) {
		return false
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return false
	}
	players := room.GetPlayers()
	if len(players) == 0 {
		return false
	}
	if !takeMob(mob.InstanceId) {
		return false
	}
	candidates := append([]int(nil), players...)
	shuffle(candidates)
	keyholder := 0
	for _, uid := range candidates {
		if canSee(uid, room) && g.Reserve(uid) {
			keyholder = uid
			break
		}
	}
	if keyholder == 0 {
		freeMob(mob.InstanceId)
		return false
	}
	startDelivery(&delivery{
		gen:        g,
		userId:     keyholder,
		mobId:      mob.InstanceId,
		roomId:     room.RoomId,
		fallback:   strings.TrimSpace(cmd),
		req:        Snapshot(mob, room),
		maxGenWait: MaxGenerateTime,
	})
	return true
}

// delivery is one moment on its way.
type delivery struct {
	gen        Generator
	userId     int
	mobId      int
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
				freeMob(d.mobId)
				mudlog.Error(`npcidle`, `action`, `deliver`, `panic`, r)
			}
		}()
		d.run(true)
	}()
}

// run asks the generator (off the lock), then finishes under the lock when
// lock is set (tests that already run in line pass false).
func (d *delivery) run(lock bool) {
	ctx, cancel := context.WithTimeout(context.Background(), d.maxGenWait)
	res, err := d.gen.Generate(ctx, d.userId, d.req)
	cancel()
	if err == nil {
		res, err = CleanResult(res, d.req.NPC.Name)
	}
	if lock {
		util.LockMud()
		defer util.UnlockMud()
	}
	defer freeMob(d.mobId)
	d.finish(res, err)
}

// finish acts the moment, or the set line when none came, unless the mob
// has gone, left the room, or fallen asleep, into a fight or dead since.
// Runs under the mud lock.
func (d *delivery) finish(res Result, err error) {
	mob := mobs.GetInstance(d.mobId)
	if mob == nil || mob.Character.RoomId != d.roomId || dormant(mob) {
		return
	}
	if err != nil {
		mob.Command(d.fallback)
		return
	}
	if res.KeyholderOnly {
		sendToKeyholder(d.userId, rooms.LoadRoom(d.roomId), mob.Character.Name, res)
		return
	}
	mob.Command(res.Kind + ` ` + res.Text)
}
