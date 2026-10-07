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
	Mobs      []ScanEntity
	Players   []ScanEntity
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
			// view the light grants. The structured result is left whole
			// for the mob callers.
			sight := messaging.SightNone
			if adjRoom := rooms.LoadRoom(s.RoomId); adjRoom != nil {
				switch {
				case seesOut:
					sight = messaging.ParticipantSight(viewer, adjRoom)
				case messaging.SensesHeatThroughExit(viewer, room, adjRoom):
					sight = messaging.SightShapes
				}
			}
			parts := []string{}
			for _, m := range s.Mobs {
				parts = append(parts,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, m.Name))
			}
			for _, p := range s.Players {
				if u := users.GetByUserId(p.Id); viewer != nil && u != nil && !viewer.Perceives(u.Character) {
					continue
				}
				parts = append(parts,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>`, p.Name))
			}
			switch sight {
			case messaging.SightShapes:
				for i := range parts {
					parts[i] = messaging.UnseenFigure(messaging.SightShapes)
				}
			case messaging.SightNone:
				parts = nil
			}
			dirLabel := fmt.Sprintf(`<ansi fg="exit">%s</ansi>`, s.ExitName)
			titleLabel := fmt.Sprintf(`<ansi fg="room-title">%s</ansi>`,
				s.RoomTitle)
			if sight == messaging.SightNone {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s (%s): too dark to make anything out`,
						dirLabel, titleLabel))
			} else if len(parts) > 0 {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s (%s): %s`,
						dirLabel, titleLabel, strings.Join(parts, `, `)))
			} else {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`  %s (%s): nothing of interest`,
						dirLabel, titleLabel))
			}
		}
		actor.SendText(messaging.CategorySystem, ``)
	}

	return result
}

// FiguresSensedIn is one anonymous figure (messaging.UnseenFigure at
// SightShapes, the room roster's shapes vocabulary) per creature in room that
// a viewer would list there: every mob that is not hidden and every player the
// viewer perceives, leaving out selfUserId. It is what heat shows through an
// exit (lighting plan 6): a count of bodies, never a name.
func FiguresSensedIn(viewer *characters.Character, room *rooms.Room, selfUserId int) []string {
	if room == nil {
		return nil
	}
	n := 0
	for _, id := range room.GetMobs(rooms.FindAll) {
		if m := mobs.GetInstance(id); m != nil && !m.Character.IsHidden() {
			n++
		}
	}
	for _, id := range room.GetPlayers(rooms.FindAll) {
		if id == selfUserId {
			continue
		}
		u := users.GetByUserId(id)
		if u == nil || (viewer != nil && !viewer.Perceives(u.Character)) {
			continue
		}
		n++
	}
	out := make([]string, 0, n)
	for range n {
		out = append(out, messaging.UnseenFigure(messaging.SightShapes))
	}
	return out
}
