package rifts

import (
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// events.go: what rifts do when the world moves. modules/rifts registers a
// listener for each event and calls these. None of them removes a room:
// teardown is Sweep's, on NewRound (see the RemoveRoom note in rooms'
// context.md).

// OnRoomChange handles a player moving. Into a rift room: they join the run,
// pay a key if the door behind them wanted one, the room's doors are built,
// and a trap may spring. Out of a run into anywhere else: they leave the run
// and lose their keys. unseen is whether they moved hidden (the event's own
// reading, taken at the move: entry detection in the new room may have
// revealed them since).
func OnRoomChange(userId, fromRoomId, toRoomId int, unseen bool) {
	if userId == 0 || fromRoomId == toRoomId {
		return
	}
	u := users.GetByUserId(userId)
	if u == nil {
		return
	}
	fromRun, fromRR := RoomInfo(fromRoomId)
	toRun, toRR := RoomInfo(toRoomId)

	if fromRR != nil {
		delete(fromRR.present, userId)
	}
	if fromRun == nil && toRun == nil {
		// The room left was torn down before this event (a run ended
		// under them): leaving is all there is to do.
		if r := runOfMember(userId); r != nil {
			fromRun = r
		}
	}

	// Fled a fight inside the rift: something will come looking (hunter.go).
	if fromRun != nil && fromRun == toRun && fromRun.fledFight(userId, unseen, fromRR, rooms.LoadRoom(fromRoomId)) {
		fromRun.startHunt(u)
	}

	if fromRun != nil && fromRun != toRun {
		wasMember := fromRun.Members[userId]
		delete(fromRun.Members, userId)
		fromRun.endHunt(userId)
		PurgeKeys(u)
		clearOrigin(u)
		if wasMember && fromRR != nil && fromRR.Pool == PoolExit && toRoomId == fromRun.exitRoomId() {
			u.SendText(messaging.CategoryRoomDescription, fromRun.Profile.Msg(`leave`))
		}
	}

	if toRun != nil && toRR != nil {
		joined := !toRun.Members[userId]
		toRun.Members[userId] = true
		delete(toRun.claims, userId)
		if toRR.present == nil {
			toRR.present = map[int]bool{}
		}
		toRR.present[userId] = true
		if joined && fromRun != toRun {
			setOrigin(u, toRun)
		}
		if !toRun.Entered {
			toRun.startJoinWindow()
		}
		toRun.Entered = true
		toRR.Entered = true
		if joined && fromRun != toRun {
			u.SendText(messaging.CategoryRoomDescription, toRun.Profile.Msg(`enter`))
		}
		if fromRun == toRun && fromRR != nil && toRR.ParentRoomId == fromRoomId {
			spendKeyOnDoor(u, toRun, fromRR, toRR.ViaDoor)
		}
		toRun.expand(toRR)
		springTrap(u, toRun, toRR)
	}

	// Arriving at a portal site: make sure its way in is ready before the
	// player's next command.
	if site := SiteAt(toRoomId); site != nil {
		site.ensureRun()
	}
}

// spendKeyOnDoor takes a key from u for a locked door they just came through.
// The first one through pays and the door stays open for the rest of the
// party. (The router already checked they had a key.)
func spendKeyOnDoor(u *users.UserRecord, run *Run, from *RiftRoom, exitName string) {
	door := from.Doors[exitName]
	if door == nil || !door.Locked || door.Unlocked {
		return
	}
	if !takeKey(u, run.Profile) {
		return
	}
	door.Unlocked = true
	fromRoom := rooms.LoadRoom(from.RoomId)
	if fromRoom == nil {
		u.SendText(messaging.CategorySystem, run.Profile.Msg(`key_used`, exitName))
		return
	}
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategorySystem, run.Profile.Msg(`key_used`, exitName)),
		Actee:    messaging.NoLine, // a door has no viewpoint
		Observer: messaging.Say(messaging.CategoryMobEmote, run.Profile.Msg(`key_used_room`, u.Character.Name, exitName)),
	}, messaging.Audience{
		Actor:     u,
		ActorId:   u.UserId,
		ActorName: u.Character.Name,
		ActeeName: messaging.NoName,
		Room:      fromRoom,
	})
}

// OnMobDeath handles a mob dying in a rift room: a boss leaves a key behind,
// and the last hostile to fall opens the room's doors.
func OnMobDeath(roomId, mobId, mobInstanceId int) {
	run, rr := RoomInfo(roomId)
	if run == nil || rr == nil {
		return
	}
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return
	}
	p := run.Profile
	run.onHunterDeath(mobInstanceId, room)
	if rr.Pool == PoolBoss && mobId == rr.BossMobId && !rr.KeyAwarded {
		rr.KeyAwarded = true
		room.AddItem(items.New(p.KeyItemId), false)
		room.SendTextVisual(messaging.CategoryLoot, p.Msg(`boss_key`))
	}
	if (rr.Pool == PoolMonster || rr.Pool == PoolBoss) && !rr.Cleared && !hostilesIn(p, room, mobInstanceId) {
		rr.Cleared = true
		room.SendTextVisual(messaging.CategoryRoomDescription, p.Msg(`doors_open`))
	}
}

// OnPlayerDeath takes a dying player's keys. They stay a member until the
// respawn moves them out (OnRoomChange), so their body does not end the run
// for a party still inside.
func OnPlayerDeath(userId, roomId int) {
	run := RunForRoom(roomId)
	if run == nil {
		return
	}
	u := users.GetByUserId(userId)
	PurgeKeys(u)
	// The rift keeps what the dead carried (lost.go). A player whose
	// connection dropped and who was killed while waiting to come back pays
	// only what leaving would have cost.
	if linkdead(u) {
		forfeitOnLogout(u, run.Profile)
		return
	}
	forfeitOnDeath(u, run.Profile)
}

// OnPlayerDespawn handles a player logging out inside a rift: they leave the
// run (the entry guard will not let them back in) and lose their keys. The
// room they stood in is emptied by the despawn hook after this, and the
// sweep tears down what is left.
func OnPlayerDespawn(userId, roomId int) {
	if run := runOfMember(userId); run != nil {
		if u := users.GetByUserId(userId); u != nil {
			PurgeKeys(u)
			// Leaving the game inside a rift costs what is carried loose
			// (lost.go); told at the next login.
			forfeitOnLogout(u, run.Profile)
		}
	}
	for _, run := range runs {
		delete(run.Members, userId)
		run.endHunt(userId)
		for _, rr := range run.Rooms {
			delete(rr.present, userId)
		}
	}
	if u := users.GetByUserId(userId); u != nil {
		PurgeKeys(u)
	}
}

// OnPlayerSpawn handles a player arriving in the world outside a rift run
// they belong to: rift things in their pack (a crash or copyover can leave
// some) are purged, and a player who logged out inside a rift is put back
// at the portal they came in by, wherever login placement sent them.
func OnPlayerSpawn(userId int) {
	u := users.GetByUserId(userId)
	if u == nil {
		return
	}
	if run := RunForRoom(u.Character.RoomId); run != nil && run.Members[userId] {
		return
	}
	PurgeKeys(u)
	tellLogoutLosses(u)
	origin := originOf(u)
	clearOrigin(u)
	if origin > 0 && origin != u.Character.RoomId && !IsRiftRoom(origin) && rooms.LoadRoom(origin) != nil {
		if err := rooms.MoveToRoom(userId, origin); err != nil {
			mudlog.Warn(`rifts.OnPlayerSpawn`, `userId`, userId, `origin`, origin, `error`, err)
		}
	}
}

// The portal room a player entered their current rift by is kept with the
// character, so logging out (or a reboot) inside a rift still brings them
// back out where they went in.
func originKey() string { return `rift-origin` }

func setOrigin(u *users.UserRecord, run *Run) {
	if run.OriginRoomId > 0 {
		u.Character.SetMiscData(originKey(), run.OriginRoomId)
	}
}

func clearOrigin(u *users.UserRecord) { u.Character.SetMiscData(originKey(), nil) }

func originOf(u *users.UserRecord) int {
	switch v := u.Character.GetMiscData(originKey()).(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}
