package behaviortree

// actions_scavenger.go: scavenger_step, the whole idle life of a city
// scavenger (internal/scavenger), one tick at a time:
//
//  1. At each real-world day boundary (UTC midnight, the same day the floor
//     decay counts in) its haul is taken away: every item it picked up and
//     any gold above its authored purse. Until then the haul sits in its
//     inventory, where a pickpocket can take it (actions.Steal draws a random
//     carried item and most of its gold).
//  2. Litter on the floor where it stands is picked up, one item a tick,
//     then any gold, each with a line in its own voice. "Litter" is the
//     floor decay's rule (rooms.Room.FloorItemIsLitter): never what the world
//     itself put there, a quest item, or a found bauble still lying untaken.
//     Every pickup goes through actions.TakeFloorItem, so the dark, a full
//     pack and a household's bauble refuse it exactly as they refuse a player.
//  3. Otherwise, once it has lingered long enough (ScavengerStepMinSeconds to
//     ScavengerStepMaxSeconds, picked afresh in each room), it takes one step
//     toward a target room picked at random from its city's pool. The path
//     comes from mapper.GetPath, the pathfinder `pathto` uses; `pathto`
//     itself is not used, because the path walker takes a step every round
//     (about 4 seconds) and holds the mob's idle ticks until it arrives, so a
//     scavenger on `pathto` would race across town and never stop to pick
//     anything up.
//  4. While lingering it does what its kind does: its authored idle commands,
//     at its activity level.
//
// It always returns Success for a scavenger (it owns the idle tick, so the
// legacy wander and goal planner never pull it off its rounds) and Failure
// for any other mob.

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/npcidle"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/scavenger"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func init() {
	actionRegistry["scavenger_step"] = actScavengerStep
}

const (
	// scavengerWalkKey holds the *scavenger.Walk in the mob's TempData.
	scavengerWalkKey = `scavenger_walk`
	// scavengerDayKey holds, in MiscData, the day (rooms.DecayDay) the
	// scavenger's haul was last cleared, so the reset happens once a day.
	scavengerDayKey = `scavenger_day`
)

// Seams for tests.
var (
	scavengerNow   = time.Now
	scavengerRandn = util.Rand
	// scavengerNextStep is the first step of a path from one room to
	// another: the exit to take and the room it leads to.
	scavengerNextStep = func(from, to int) (exitName string, nextRoom int, ok bool) {
		path, err := mapper.GetPath(from, to)
		if err != nil || len(path) == 0 {
			return ``, 0, false
		}
		return path[0].ExitName(), path[0].RoomId(), true
	}
)

func actScavengerStep(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	prof := scavenger.ProfileFor(int(mob.MobId))
	if prof == nil {
		return Failure
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return Failure
	}
	now := scavengerNow()

	scavengerDailyReset(mob, room, prof, now)

	if scavengerPickUp(mob, room, prof) {
		return Success
	}

	w := scavengerWalk(mob)
	if now.Before(w.NextMoveAt) {
		scavengerIdle(mob)
		return Success
	}
	scavengerStep(mob, prof, w, now)
	return Success
}

// scavengerWalk is the mob's walk state, made on first use.
func scavengerWalk(mob *mobs.Mob) *scavenger.Walk {
	if w, ok := mob.GetTempData(scavengerWalkKey).(*scavenger.Walk); ok && w != nil {
		return w
	}
	w := &scavenger.Walk{}
	mob.SetTempData(scavengerWalkKey, w)
	return w
}

// scavengerDailyReset clears the day's haul once per real-world day. The
// first tick a scavenger ever takes only records the day: it has nothing to
// clear yet.
func scavengerDailyReset(mob *mobs.Mob, room *rooms.Room, prof *scavenger.Profile, now time.Time) {
	today := rooms.DecayDay(now)
	last, known := miscDay(mob.Character.GetMiscData(scavengerDayKey))
	if known && last >= today {
		return
	}
	mob.Character.SetMiscData(scavengerDayKey, int(today))
	if !known {
		return
	}

	purse := 0
	if spec := mobs.GetMobSpec(mob.MobId); spec != nil {
		purse = spec.Character.Gold
	}
	if len(mob.Character.Items) == 0 && mob.Character.Gold <= purse {
		return
	}

	haul := mob.Character.Items
	mob.Character.Items = nil
	for _, itm := range haul {
		if itm.IsBauble() {
			baubles.MarkVanished(itm.Bauble, now)
		}
		events.AddToQueue(events.ItemOwnership{MobInstanceId: mob.InstanceId, Item: itm, Gained: false})
	}
	if mob.Character.Gold > purse {
		mob.Character.Gold = purse
	}
	if line := scavenger.Line(prof.ResetLines, scavengerRandn, mob.Character.Name, ``, 0); line != `` {
		room.SendTextVisual(messaging.CategoryMobIdle, line)
	}
}

// miscDay reads the stored reset day, whatever number type it came back as
// (an int in memory, possibly another width after a save round trip).
func miscDay(v any) (int64, bool) {
	switch d := v.(type) {
	case int:
		return int64(d), true
	case int64:
		return d, true
	case float64:
		return int64(d), true
	case uint64:
		return int64(d), true
	}
	return 0, false
}

// scavengerPickUp picks up one litter item from the floor, or failing that
// the gold, and says so. It reports whether it picked anything up.
func scavengerPickUp(mob *mobs.Mob, room *rooms.Room, prof *scavenger.Profile) bool {
	actor := &actions.MobActor{Mob: mob, Room: room}
	if actions.TooDarkToGet(actor) {
		return false
	}

	// A copy: a successful take changes room.Items.
	floor := append(room.Items[:0:0], room.Items...)
	for _, itm := range floor {
		if !room.FloorItemIsLitter(itm) {
			continue
		}
		if err := actions.TakeFloorItem(actor, itm, false); err != nil {
			continue // too heavy for what it carries, about to explode, ...
		}
		mob.Character.CancelConditionsWithFlag(conditions.Hidden)
		room.SendTextVisual(messaging.CategoryLoot,
			scavenger.Line(prof.PickupLines, scavengerRandn, mob.Character.Name, itm.DisplayName(), 0))
		return true
	}

	if gold := room.Gold; gold > 0 {
		if err := actions.GetGoldFromFloor(actor, gold); err == nil {
			mob.Character.CancelConditionsWithFlag(conditions.Hidden)
			room.SendTextVisual(messaging.CategoryLoot,
				scavenger.Line(prof.GoldLines, scavengerRandn, mob.Character.Name, ``, gold))
			return true
		}
	}
	return false
}

// scavengerIdle does what its kind does while it lingers: one of its
// authored idle commands, at its activity level, the way any idle mob does.
func scavengerIdle(mob *mobs.Mob) {
	if scavengerRandn(100) >= mob.ActivityLevel {
		return
	}
	if cmd := mob.GetIdleCommand(); cmd != `` {
		// Now and then written fresh instead (internal/npcidle).
		if npcidle.TryReplace(mob, cmd) {
			return
		}
		mob.Command(cmd)
	}
}

// scavengerStep takes one step toward the walk's target, picking a new
// target when it has none, has arrived, or cannot get there.
func scavengerStep(mob *mobs.Mob, prof *scavenger.Profile, w *scavenger.Walk, now time.Time) {
	current := mob.Character.RoomId
	w.NoteArrival(current)

	pool := prof.Pool()
	for attempt := 0; attempt < 3; attempt++ {
		if w.Target == 0 || w.Target == current {
			w.Target = scavenger.PickTarget(pool, current, scavengerRandn)
			if w.Target == 0 {
				break
			}
		}
		exitName, next, ok := scavengerNextStep(current, w.Target)
		if !ok {
			w.Target = 0
			continue
		}
		mob.Command(exitName)
		w.LastFrom, w.Expect = current, next
		cfg := configs.GetBalanceConfig()
		w.NextMoveAt = now.Add(scavenger.StepDelay(
			int(cfg.ScavengerStepMinSeconds), int(cfg.ScavengerStepMaxSeconds), scavengerRandn))
		return
	}
	// Nowhere to go from here right now: try again shortly.
	w.NextMoveAt = now.Add(scavenger.RetryAfterNoPath)
}
