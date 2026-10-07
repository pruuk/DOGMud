package actions

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ScanOptions parameterizes a one-step adjacent-room sweep.
type ScanOptions struct {
	// HostileOnly: when true, mob-actor btree wrappers may use this to
	// gate SoftTarget population on hostile-ness. The action itself does
	// not filter Sightings by hostility — that decision is up to the caller.
	HostileOnly bool
}

// ScanEntity describes one occupant in a sighted room.
type ScanEntity struct {
	Id    int
	Name  string
	IsMob bool
}

// ScanSighting describes one adjacent room's occupants.
type ScanSighting struct {
	ExitName  string
	RoomId    int
	RoomTitle string
	// Locked: the exit is locked (#427). A locked door blocks scan as it
	// blocks look, so Mobs and Players stay empty for it.
	Locked  bool
	Mobs    []ScanEntity
	Players []ScanEntity
}

// ScanResult is the structured outcome of a Scan call.
type ScanResult struct {
	Sightings []ScanSighting
}

// Scan walks each visible (non-secret) exit from the actor's room, loads
// the adjacent room, and lists non-hidden mobs and players in each.
// No skill check or cooldown is applied. UserActor receives a rendered
// text list via SendText; MobActor.SendText is a no-op (silent). The
// structured result is returned in both cases.
func Scan(actor Actor, opts ScanOptions) ScanResult {
	result := ScanResult{Sightings: []ScanSighting{}}

	room := actor.GetRoom()
	if room == nil {
		return result
	}

	for exitName, exitInfo := range room.Exits {
		if exitInfo.Secret {
			continue
		}

		adjRoom := rooms.LoadRoom(exitInfo.RoomId)
		if adjRoom == nil {
			continue
		}

		sighting := ScanSighting{
			ExitName:  exitName,
			RoomId:    adjRoom.RoomId,
			RoomTitle: adjRoom.Title,
		}

		// A locked door blocks scan as it blocks look (#427, owner ruling
		// 2026-10-07): the direction stays, marked locked, with nobody.
		if info, _ := room.GetExitInfo(exitName); info.Lock.IsLocked() {
			sighting.Locked = true
			result.Sightings = append(result.Sightings, sighting)
			continue
		}

		for _, mobInstId := range adjRoom.GetMobs(rooms.FindAll) {
			m := mobs.GetInstance(mobInstId)
			if m == nil || m.Character.IsHidden() {
				continue
			}
			sighting.Mobs = append(sighting.Mobs, ScanEntity{
				Id:    mobInstId,
				Name:  m.Character.Name,
				IsMob: true,
			})
		}

		for _, pId := range adjRoom.GetPlayers(rooms.FindAll) {
			if pId == actor.GetUserId() {
				continue
			}
			u := users.GetByUserId(pId)
			if u == nil {
				continue
			}
			sighting.Players = append(sighting.Players, ScanEntity{
				Id:    pId,
				Name:  u.Character.Name,
				IsMob: false,
			})
		}

		result.Sightings = append(result.Sightings, sighting)
	}

	// UserActor text rendering. MobActor.SendText is a no-op.
	if actor.IsPlayer() {
		actor.SendText(messaging.CategorySystem, `You scan the surrounding area...`)
		actor.SendText(messaging.CategorySystem, ``)
		if len(result.Sightings) == 0 {
			actor.SendText(messaging.CategorySystem,
				`  There are no visible exits to scan.`)
		}
		viewer := actor.GetCharacter()
		seesOut := messaging.SeesThroughExit(viewer, room)
		for _, s := range result.Sightings {
			// The player's list follows the player's sight (lighting plan
			// 5c): out through the exit from here, then into the next room.
			// With faces, names; with shapes, the anonymous figure the room
			// roster uses, one per creature and uncolored so a mob and a
			// player read alike; with neither, nobody. When the light here
			// refuses, heat may still show the next room's occupants as
			// shapes (lighting plan 6, owner ruling O6); it never upgrades a
			// view the light grants. Whom it lists is the roster's rule
			// (listedOccupants), the same one look's heat uses. The
			// structured result is left whole for the mob callers.
			//
			// The next room's title shows only when the light here sees out
			// (#428). When it refuses, by heat-only figures or nothing at
			// all, the line names the direction alone, as look <exit> names
			// no room under heat.
			//
			// A locked exit (#427) reads as look reads it: locked whenever
			// light or heat would reach through it, with nobody listed; too
			// dark when neither would.
			sight := messaging.SightNone
			reaches := false
			adjRoom := rooms.LoadRoom(s.RoomId)
			if adjRoom != nil {
				switch {
				case seesOut:
					sight = messaging.ParticipantSight(viewer, adjRoom)
					reaches = true
				case messaging.SensesHeatThroughExit(viewer, room, adjRoom):
					sight = messaging.SightShapes
					reaches = true
				}
			}
			dirLabel := fmt.Sprintf(`<ansi fg="exit">%s</ansi>`, s.ExitName)
			if s.Locked && reaches {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s: the exit is locked`, dirLabel))
				continue
			}
			parts := []string{}
			listedMobs, listedPlayers := listedOccupants(viewer, adjRoom, actor.GetUserId())
			for _, m := range listedMobs {
				parts = append(parts,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, m.Character.Name))
			}
			for _, u := range listedPlayers {
				parts = append(parts,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>`, u.Character.Name))
			}
			switch sight {
			case messaging.SightShapes:
				for i := range parts {
					parts[i] = messaging.UnseenFigure(messaging.SightShapes)
				}
			case messaging.SightNone:
				parts = nil
			}
			// The next room's title belongs to a view the light grants
			// (#428): heat or nothing names a direction, never a room.
			label := dirLabel
			if seesOut {
				label = fmt.Sprintf(`%s (<ansi fg="room-title">%s</ansi>)`,
					dirLabel, s.RoomTitle)
			}
			if sight == messaging.SightNone {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s: too dark to make anything out`, label))
			} else if len(parts) > 0 {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s: %s`, label, strings.Join(parts, `, `)))
			} else {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s: nothing of interest`, label))
			}
		}
		actor.SendText(messaging.CategorySystem, ``)
	}

	return result
}

// listedOccupants is every creature in room that the viewer's room roster
// would list there (rooms.GetDetails): each mob actually in the room (a stale
// listing, a mob whose RoomId says it has left, never shows) that the viewer
// Perceives, and each player other than selfUserId that the viewer Perceives.
// Perceives is the one hidden rule: a sneaking or camouflaged creature is
// left out unless the viewer has see-hidden. A nil viewer perceives whoever is
// not hidden.
//
// It is the one occupant filter behind what a player sees of the next room:
// scan's list, by name or as shapes, and the figures heat shows through an
// exit for scan and for look (FiguresSensedIn).
func listedOccupants(viewer *characters.Character, room *rooms.Room, selfUserId int) ([]*mobs.Mob, []*users.UserRecord) {
	if room == nil {
		return nil, nil
	}
	perceives := func(c *characters.Character) bool {
		if viewer == nil {
			return c != nil && !c.IsHidden()
		}
		return viewer.Perceives(c)
	}
	var ms []*mobs.Mob
	for _, id := range room.GetMobs(rooms.FindAll) {
		m := mobs.GetInstance(id)
		if m == nil || m.Character.RoomId != room.RoomId || !perceives(&m.Character) {
			continue
		}
		ms = append(ms, m)
	}
	var us []*users.UserRecord
	for _, id := range room.GetPlayers(rooms.FindAll) {
		if id == selfUserId {
			continue
		}
		u := users.GetByUserId(id)
		if u == nil || u.Character == nil || !perceives(u.Character) {
			continue
		}
		us = append(us, u)
	}
	return ms, us
}

// FiguresSensedIn is one anonymous figure (messaging.UnseenFigure at
// SightShapes, the room roster's shapes vocabulary) per creature in room that
// the viewer's roster would list there (listedOccupants), leaving out
// selfUserId. It is what heat shows through an exit (lighting plan 6): a
// count of bodies, never a name.
func FiguresSensedIn(viewer *characters.Character, room *rooms.Room, selfUserId int) []string {
	ms, us := listedOccupants(viewer, room, selfUserId)
	n := len(ms) + len(us)
	out := make([]string, 0, n)
	for range n {
		out = append(out, messaging.UnseenFigure(messaging.SightShapes))
	}
	return out
}
