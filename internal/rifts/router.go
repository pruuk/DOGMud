package rifts

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// router.go: the exit router and entry guard rifts register with the room
// layer (rooms.AddExitRouter, rooms.AddEntryGuard). Look, picklock, unlock
// and flee ask the router too, and a guard is asked for every placement, so
// neither spends anything: spending a key happens after the move, in the
// RoomChange handler. Two effects are deliberate: asking the router about a
// portal makes the site's waiting run (a chunk and the portal exit) if it
// has none, and the entry guard records a claim on the run being entered
// (see instance.go).

// Router decides one traveller taking one exit of a rift room. It claims
// every door of a rift room and the exit room's way out, and nothing else.
func Router(userId int, fromRoomId int, exitName string) (rooms.ExitRoute, bool) {
	if site := SiteAt(fromRoomId); site != nil {
		return routePortal(site, userId, exitName)
	}
	run, rr := RoomInfo(fromRoomId)
	if run == nil || rr == nil {
		return rooms.ExitRoute{}, false
	}
	p := run.Profile
	refuse := func(why string) (rooms.ExitRoute, bool) {
		return rooms.ExitRoute{Refusal: why, PickRefusal: p.Msg(`pick_refusal`, exitName)}, true
	}

	// A hunter standing over its quarry lets them go nowhere: no door, no
	// flee, not even the breach. (Only rift exits are claimed here, so this
	// covers every way out of a rift room.)
	if userId != 0 && (rr.Doors[exitName] != nil || (rr.Pool == PoolExit && exitName == p.ExitName)) &&
		run.hunterHolds(userId, fromRoomId) {
		return refuse(p.Msg(`hunter_holds`))
	}

	if rr.Pool == PoolExit && exitName == p.ExitName {
		if userId == 0 {
			return refuse(p.Msg(`sealed_hostile`)) // the way out is for players
		}
		return rooms.ExitRoute{RoomId: run.exitRoomId(), PickRefusal: p.Msg(`pick_refusal`, exitName)}, true
	}

	door := rr.Doors[exitName]
	if door == nil {
		return rooms.ExitRoute{}, false
	}
	if userId == 0 {
		// Mobs never pass a rift door: a boss that fled would unseal its room
		// without being beaten, and a mob loose in the next room is a trap
		// nobody authored.
		return refuse(p.Msg(`sealed_hostile`))
	}
	room := rooms.LoadRoom(fromRoomId)
	if room == nil {
		return refuse(p.Msg(`unsettled`))
	}
	// Hostiles hold a monster or boss room shut against anyone they know is
	// there. A flee still gets out (at a price: see hunter.go); walking does
	// not; a player who stays hidden slips through.
	if (rr.Pool == PoolMonster || rr.Pool == PoolBoss) && hostilesIn(p, room, 0) && !rooms.FleeRouting() &&
		!unseen(userId) {
		return refuse(p.Msg(`sealed_hostile`))
	}
	if door.Sealed && !rr.PuzzleSolved {
		if rr.Memory != nil && rr.Memory.Dark && p.Memory != nil && IsReader(users.GetByUserId(userId), p) {
			return refuse(wrap(p.Memory.DarkLook))
		}
		return refuse(p.Msg(`sealed_puzzle`))
	}
	if door.Locked && !door.Unlocked && !hasKey(users.GetByUserId(userId), p) {
		return refuse(p.Msg(`locked`))
	}
	if door.DestRoomId == 0 || rooms.LoadRoom(door.DestRoomId) == nil {
		return refuse(p.Msg(`unsettled`))
	}
	return rooms.ExitRoute{RoomId: door.DestRoomId, PickRefusal: p.Msg(`pick_refusal`, exitName)}, true
}

// routePortal decides the portal exit of a site room: into the site's
// joinable run, or nowhere yet.
func routePortal(site *Site, userId int, exitName string) (rooms.ExitRoute, bool) {
	p := GetProfile(site.ProfileId)
	if p == nil || exitName != p.PortalExit {
		return rooms.ExitRoute{}, false
	}
	pick := p.Msg(`pick_refusal`, exitName)
	if userId == 0 || rooms.FleeRouting() {
		// Mobs never pass, and nobody flees into a rift.
		return rooms.ExitRoute{Refusal: p.Msg(`portal_dormant`), PickRefusal: pick}, true
	}
	// The run of this player's party, if it is still open to them; else the
	// site's waiting run, which nobody has claimed yet.
	run := partyRunAt(site, userId)
	if run == nil {
		run = site.waitingRun()
	}
	if run == nil {
		return rooms.ExitRoute{Refusal: p.Msg(`portal_dormant`), PickRefusal: pick}, true
	}
	// Once the party has gone on from the entry room, someone following
	// within the join window lands beside them rather than in an entry room
	// whose doors may already lead nowhere.
	if run.Entered {
		if id := run.frontierRoomId(); id > 0 {
			return rooms.ExitRoute{RoomId: id, PickRefusal: pick}, true
		}
	}
	if rooms.LoadRoom(run.EntryRoomId) == nil {
		return rooms.ExitRoute{Refusal: p.Msg(`portal_dormant`), PickRefusal: pick}, true
	}
	return rooms.ExitRoute{RoomId: run.EntryRoomId, PickRefusal: pick}, true
}

// exitRoomId is a loadable room for the way out: the portal's room, else the
// configured start room. (A route must name a real room; the start-room alias
// is only understood by MoveToRoom.)
func (run *Run) exitRoomId() int {
	if run.OriginRoomId > 0 && rooms.LoadRoom(run.OriginRoomId) != nil {
		return run.OriginRoomId
	}
	if id := int(configs.GetSpecialRoomsConfig().StartRoom); id > 0 && rooms.LoadRoom(id) != nil {
		return id
	}
	return 1
}

// EntryGuard keeps everybody out of a run's rooms but its members, with one
// door in: the portal, while it is open (into the entry room, or beside the
// party once it has moved on). It covers walking,
// fold-recall to an anchor set inside a rift, summons and login placement. A
// refused login (the saved room is now somebody else's rift, or a rift that is
// gone) is redirected to the run's portal room.
func EntryGuard(userId int, toRoomId int) (bool, int, string) {
	run := RunForRoom(toRoomId)
	if run == nil || userId == 0 {
		return true, 0, ``
	}
	if run.Members[userId] {
		return true, 0, ``
	}
	// The one way in: through the portal, from the site, into a run that is
	// fresh or belongs to this player's party. The move commits them to it.
	if u := users.GetByUserId(userId); u != nil && u.Character.RoomId == run.OriginRoomId &&
		(toRoomId == run.EntryRoomId || toRoomId == run.frontierRoomId()) && run.mayJoin(userId) {
		run.claim(userId)
		return true, 0, ``
	}
	// A refused placement goes back to where this player last entered a
	// rift, if that is known; else to this run's portal room.
	redirect := run.exitRoomId()
	if u := users.GetByUserId(userId); u != nil {
		if o := originOf(u); o > 0 && !IsRiftRoom(o) && rooms.LoadRoom(o) != nil {
			redirect = o
		}
	}
	return false, redirect, run.Profile.Msg(`closed`)
}

// unseen reports whether userId is hidden (sneaking undetected).
func unseen(userId int) bool {
	u := users.GetByUserId(userId)
	return u != nil && u.Character.IsHidden()
}
