package rifts

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// hunter.go: the price of running.
//
// A player who leaves a rift room while something hostile there still knows
// they are there and is still alive (they fled: a monster or boss room is
// sealed to walkers while its hostiles stand, and a flee is the only way
// through, unless they slip out unseen) is hunted. One hunter per player at
// a time, and always the same one: the profile's hunter (Mobs.Hunter) is
// made once, and from then on it is the one creature, carried between rooms
// with whatever damage it has taken.
//
// It comes for its quarry room by room. Each time the quarry is in a new
// room, the hunter appears there HunterDelaySeconds (10 s to 1 min) later if
// they are still in it; moving on before then starts the count again, so a
// player who keeps moving stays ahead of it.
//
// It plays by the ordinary stealth rules. Appearing beside a hidden quarry,
// it does not attack: it watches, rolling its detection against their sneak
// every few rounds (actions.SpotHiddenPlayer, the same contest as entry
// detection), and the moment it spots them it is on them. A hidden quarry
// may slip away, and the hunter follows (withdrawing into the walls and
// coming again in their next room). A quarry it can see cannot leave while it
// stands with them: every door refuses them, a flee finds no way out, and the
// breach does not open (Router). Party members come and go as they like.
//
// The hunt ends when the hunter is destroyed, or when its quarry leaves the
// rift (out through the breach, dead, logged out). Fleeing another fight
// after that starts a new hunt with a new hunter.

// watchEvery is how many rounds a hunter standing over a hidden quarry waits
// between looks for them.
const watchEvery = 2

// Hunt is one player's hunter.
type Hunt struct {
	UserId int
	DueAt  time.Time // when it appears in RoomId, while it is not out
	RoomId int       // the quarry's room the delay is counting in (0: none yet)

	hunter *mobs.Mob // nil until it first appears; then always this one
	out    bool      // standing in a room (else between rooms, out of the world)
	watch  int       // rounds since it last looked for a hidden quarry
}

// Hunted reports whether userId is hunted in run, and the hunter's instance
// id while it is out in a room (admin, tests).
func (run *Run) Hunted(userId int) (hunted bool, hunterInstanceId int) {
	h := run.hunts[userId]
	if h == nil {
		return false, 0
	}
	if h.out && h.hunter != nil {
		return true, h.hunter.InstanceId
	}
	return true, 0
}

// HunterOf is userId's hunter, in a room or between rooms, or nil if it has
// not appeared yet (tests).
func (run *Run) HunterOf(userId int) *mobs.Mob {
	if h := run.hunts[userId]; h != nil {
		return h.hunter
	}
	return nil
}

func (run *Run) huntDelay() time.Duration {
	return time.Duration(RollRange(run.Profile.HunterDelaySeconds, rng)) * time.Second
}

// startHunt sets a hunter on u, unless the profile has none or u is hunted
// already.
func (run *Run) startHunt(u *users.UserRecord) {
	p := run.Profile
	if u == nil || p.Mobs.Hunter == 0 || run.hunts[u.UserId] != nil {
		return
	}
	run.hunts[u.UserId] = &Hunt{UserId: u.UserId}
	u.SendText(messaging.CategoryRoomDescription, p.Msg(`hunted`))
	mudlog.Info(`rifts`, `run`, run.Id, `hunt`, `started`, `userId`, u.UserId)
}

// StartHunt sets run's hunter on u as if they had fled a fight (admin,
// testing). False when the profile has no hunter, u is not in the run, or u
// is hunted already.
func (run *Run) StartHunt(u *users.UserRecord) bool {
	if u == nil || !run.Members[u.UserId] || run.Profile.Mobs.Hunter == 0 || run.hunts[u.UserId] != nil {
		return false
	}
	run.startHunt(u)
	return true
}

// endHunt ends u's hunt, taking the hunter away if it is out.
func (run *Run) endHunt(userId int) {
	h := run.hunts[userId]
	if h == nil {
		return
	}
	delete(run.hunts, userId)
	if h.hunter == nil {
		return
	}
	if h.out {
		m := h.hunter
		if m.Character.Health <= 0 {
			return // dying: the death pipeline has it
		}
		// It may be in the middle of a fight with the quarry's party: say
		// where it went.
		if room := rooms.LoadRoom(m.Character.RoomId); room != nil && room.PlayerCt() > 0 {
			room.SendText(messaging.CategoryRoomDescription, run.Profile.Msg(`hunter_withdraws`))
		}
		removeMob(m.InstanceId)
		return
	}
	mobs.DeleteMobInstance(h.hunter.MobId, h.hunter.Zone, h.hunter.Character.Name, h.hunter.HomeRoomId)
}

// hunterHolds reports whether userId's hunter is out in roomId and can see
// them there: they may not leave it. A hidden quarry may.
func (run *Run) hunterHolds(userId int, roomId int) bool {
	h := run.hunts[userId]
	if h == nil || !h.out || h.hunter == nil {
		return false
	}
	m := h.hunter
	if mobs.GetInstance(m.InstanceId) == nil || m.Character.Health <= 0 || m.Character.RoomId != roomId {
		return false
	}
	u := users.GetByUserId(userId)
	return u != nil && !u.Character.IsHidden()
}

// onHunterDeath ends the hunt whose hunter was mobInstanceId. True when it
// was one.
func (run *Run) onHunterDeath(mobInstanceId int, room *rooms.Room) bool {
	for uid, h := range run.hunts {
		if h.hunter != nil && h.hunter.InstanceId == mobInstanceId {
			delete(run.hunts, uid)
			if room != nil {
				room.SendText(messaging.CategoryRoomDescription, run.Profile.Msg(`hunter_destroyed`))
			}
			return true
		}
	}
	return false
}

// sweepHunts runs every round: a hunter out with its quarry keeps after them
// (or keeps watching for them, hidden); one whose quarry has gone withdraws
// to follow; one between rooms appears once its quarry has been in one room
// long enough.
func (run *Run) sweepHunts() {
	for uid, h := range run.hunts {
		u := users.GetByUserId(uid)
		if u == nil || !run.Members[uid] {
			run.endHunt(uid)
			continue
		}
		if h.out {
			if !run.huntOut(h, u) {
				continue
			}
		}
		run.huntApproach(h, u)
	}
}

// huntOut handles a hunter standing in a room. It returns true when the
// hunter has just withdrawn (its quarry is elsewhere) and the approach to
// their new room should begin.
func (run *Run) huntOut(h *Hunt, u *users.UserRecord) bool {
	m := h.hunter
	if mobs.GetInstance(m.InstanceId) == nil {
		delete(run.hunts, h.UserId) // gone without its death being seen
		return false
	}
	if m.Character.Health <= 0 {
		return false // dying: OnMobDeath ends the hunt and says so
	}
	room := rooms.LoadRoom(m.Character.RoomId)
	if room != nil && m.Character.RoomId == u.Character.RoomId {
		if u.Character.Health <= 0 {
			return false
		}
		if u.Character.IsHidden() {
			// Vigilant: it knows its quarry is near, and keeps looking.
			if h.watch++; h.watch >= watchEvery {
				h.watch = 0
				actions.SpotHiddenPlayer(actions.NewMobActorInRoom(m, room), room, h.UserId)
			}
			return false
		}
		// On its quarry: out of a fight, or fighting someone no longer here.
		tgt := m.Character.CurrentCombatTarget()
		stale := false
		if tgt.UserId > 0 && tgt.UserId != h.UserId {
			if other := users.GetByUserId(tgt.UserId); other == nil || other.Character.RoomId != room.RoomId {
				stale = true
			}
		}
		if !m.Character.IsInCombat() || stale {
			targeting.Commit(&m.Character, state.ActorRef{UserId: h.UserId}, targeting.ReasonAttack)
		}
		return false
	}
	// Its quarry is elsewhere: into the walls, to follow.
	if room != nil {
		room.SendText(messaging.CategoryRoomDescription, run.Profile.Msg(`hunter_withdraws`))
	}
	parkMob(m)
	h.out, h.RoomId, h.watch = false, 0, 0
	return true
}

// huntApproach runs the delay for a hunter between rooms, and brings it out
// when its quarry has stood in one rift room for it.
func (run *Run) huntApproach(h *Hunt, u *users.UserRecord) {
	p := run.Profile
	rr := run.Rooms[u.Character.RoomId]
	if rr == nil || u.Character.Health <= 0 {
		h.RoomId = 0 // not standing in the rift just now
		return
	}
	if h.RoomId != rr.RoomId {
		h.RoomId, h.DueAt = rr.RoomId, now().Add(run.huntDelay())
		return
	}
	if now().Before(h.DueAt) {
		return
	}
	room := rooms.LoadRoom(rr.RoomId)
	if room == nil {
		return
	}
	if h.hunter == nil {
		m := spawnMob(room, p.Mobs.Hunter, StatPool(p, `hunter`, rr.Depth))
		if m == nil {
			h.DueAt = now().Add(run.huntDelay())
			return
		}
		h.hunter = m
	} else {
		mobs.RestoreInstance(h.hunter)
		room.AddMob(h.hunter.InstanceId)
	}
	h.out, h.watch = true, 0
	room.SendText(messaging.CategoryRoomDescription, p.Msg(`hunter_arrives`))
	mudlog.Info(`rifts`, `run`, run.Id, `hunt`, `hunter out`, `userId`, h.UserId, `room`, room.RoomId)

	m := h.hunter
	if u.Character.IsHidden() {
		// Arriving, it looks around once, as anyone walking in would.
		if !actions.SpotHiddenPlayer(actions.NewMobActorInRoom(m, room), room, h.UserId) {
			return
		}
	}
	targeting.Commit(&m.Character, state.ActorRef{UserId: h.UserId}, targeting.ReasonAttack)
}

// parkMob takes a mob out of its room and out of the world registry without
// a death or a save, so it can be brought back as it was (mobs.
// RestoreInstance): everyone fighting it lets go, and it lets go of them.
func parkMob(m *mobs.Mob) {
	if room := rooms.LoadRoom(m.Character.RoomId); room != nil {
		room.RemoveMob(m.InstanceId)
		actions.ClearRoomAggroOnDeparture(room, m.InstanceId)
	}
	targeting.Release(&m.Character, targeting.ReasonDisengage)
	mobs.DestroyInstance(m.InstanceId)
}

// fledFight reports whether u, leaving from, left a fight behind: a monster
// or boss room whose hostiles still stand (sealed to walkers, so they fled),
// unless they went unseen, or any room where something alive is still
// fighting them.
func (run *Run) fledFight(userId int, unseen bool, from *RiftRoom, fromRoom *rooms.Room) bool {
	if from == nil || fromRoom == nil {
		return false
	}
	// Slipping out of a sealed room unseen is not fleeing (Router lets a
	// hidden player past the seal).
	if !unseen && (from.Pool == PoolMonster || from.Pool == PoolBoss) && hostilesIn(run.Profile, fromRoom, 0) {
		return true
	}
	for _, id := range fromRoom.GetMobs() {
		m := mobs.GetInstance(id)
		if m == nil || m.Character.Health <= 0 || m.Character.IsCharmed() {
			continue
		}
		// Walking away from someone else's hunter is not fleeing: it holds
		// only its quarry, and the others may come and go.
		if run.Profile.Mobs.Hunter != 0 && int(m.MobId) == run.Profile.Mobs.Hunter {
			continue
		}
		if m.Character.CurrentCombatTarget().UserId == userId {
			return true
		}
	}
	return false
}

// removeMob takes a rift mob out of the world without a death: out of its
// room (everyone fighting it lets go), and its instance gone.
func removeMob(instanceId int) {
	m := mobs.GetInstance(instanceId)
	if m == nil {
		return
	}
	if room := rooms.LoadRoom(m.Character.RoomId); room != nil {
		room.RemoveMob(instanceId)
		actions.ClearRoomAggroOnDeparture(room, instanceId)
	}
	mobs.DeleteMobInstance(m.MobId, m.Zone, m.Character.Name, m.HomeRoomId)
	mobs.DestroyInstance(instanceId)
}
