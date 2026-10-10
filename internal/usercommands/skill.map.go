package usercommands

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/mattn/go-runewidth"
)

/*
Skill Map
Level 1 - Map a 5x5 area
Level 2 - Map a 9x7 area
Level 3 - Map a 13x9 area
Level 4 - Map a 17x9 area, and enables the "wide" version.
*/
// mobMapSymbol is the map cell for a room holding hostile mobs. The skull has
// no ASCII form (ConvertToAscii drops it, which would erase the cell), so an
// ASCII-mode player gets '!' (#253).
func mobMapSymbol(asciiMode bool) rune {
	if asciiMode {
		return '!'
	}
	return '☠'
}

// occupantMarkerLevel is the map level that draws the NPC, Player and Mob
// markers. They sat under skillLevel > 4, which no Perception tier reaches,
// so no map drew them although `help map` lists them (#253 follow-up; owner
// call 2026-10-10).
const occupantMarkerLevel = 4

// addOccupantMarkers marks, on a map of occupantMarkerLevel or higher, every
// room holding mobs (Mob when one is hostile or fighting, NPC otherwise) and
// every room holding players (Player). Players are marked last, so a room
// holding both reads Player; the party, friend and You markers the caller
// adds afterwards override all three.
func addOccupantMarkers(c *mapper.Config, skillLevel int, asciiMode bool) {
	if skillLevel < occupantMarkerLevel {
		return
	}
	for _, rid := range rooms.GetRoomsWithMobs() {
		if roomInfo := rooms.LoadRoom(rid); roomInfo != nil {
			if len(roomInfo.GetMobs(rooms.FindFighting|rooms.FindHostile)) > 0 {
				c.OverrideSymbol(rid, mobMapSymbol(asciiMode), `Mob`)
			} else {
				c.OverrideSymbol(rid, '☺', `NPC`)
			}
		}
	}
	for _, rid := range rooms.GetRoomsWithPlayers() {
		c.OverrideSymbol(rid, '☺', `Player`)
	}
}

const (
	// wideMapLevel is the map level that enables `map wide`. It sat under
	// skillLevel > 4, which no Perception tier reaches, so `map wide` never
	// changed anything although `help map` promises it (owner call 2026-10-10).
	wideMapLevel = 4

	standardMapWidth  = 65
	standardMapHeight = 21

	mapBorderWidth  = 14
	mapBorderHeight = 6 // Title, map top, map bottom, 2 legend, blank line.
)

// mapSizeFor returns the map's width and height. The standard map is 65x21;
// a wide map of wideMapLevel or higher fills the client's screen less the
// frame, with an even height.
func mapSizeFor(skillLevel int, wide bool, screenWidth, screenHeight int) (int, int) {
	width, height := standardMapWidth, standardMapHeight
	if wide && skillLevel >= wideMapLevel {
		// Wide never shrinks below the standard map: a tiny or unreported
		// screen would otherwise give a negative size.
		width = max(width, screenWidth-mapBorderWidth)
		h := screenHeight - mapBorderHeight
		if h%2 != 0 {
			h--
		}
		height = max(height, h)
	}
	return width, height
}

func Map(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// Map is a free command — no skill gate.
	// Detail level scales with Perception (1–4 equivalent tiers).
	perceptionAdj := user.Character.Stats.Perception.ValueAdj
	skillLevel := 1
	if perceptionAdj >= 175 {
		skillLevel = 4
	} else if perceptionAdj >= 135 {
		skillLevel = 3
	} else if perceptionAdj >= 110 {
		skillLevel = 2
	}

	if rest == "sprawl" {
		user.SendText(messaging.CategorySystem, fmt.Sprintf("The reach of your maps is %d rooms.", user.Character.GetMapSprawlCapacity()))
		return true, nil
	}

	if !user.Character.TryCooldown(`map`, "1 round") {
		user.SendText(messaging.CategorySystem,
			`You can only create 1 map per round.`,
		)
		return true, errors.New(`you're doing that too often`)
	}

	// replace any non alpha/numeric characters in "rest"
	zone := rest
	roomId := 0
	if zone != "" && zone != "wide" {
		zone = rooms.FindZoneName(zone)
		if zone != user.Character.Zone {
			roomId, _ = rooms.GetZoneRoot(zone)
		}
	}

	if zone == "" || roomId == 0 {
		zone = user.Character.Zone
		roomId = user.Character.RoomId
	}

	// First check for a premade map.
	if mapTxt, err := templates.Process("maps/"+rooms.ZoneNameSanitize(zone), zone); err == nil {
		user.SendText(messaging.CategorySystem, mapTxt)
		return true, nil
	}

	var err error

	sw := user.ClientSettings().Display.GetScreenWidth()
	sh := user.ClientSettings().Display.GetScreenHeight()
	mapWidth, mapHeight := mapSizeFor(skillLevel, rest == "wide", sw, sh)

	zMapper := mapper.GetMapper(roomId)
	if zMapper == nil {
		mudlog.Error("Map", "error", "Could not find mapper for zone:"+zone)
		user.SendText(messaging.CategorySystem, `No map found (or an error occured)"`)
		return true, err
	}

	c := mapper.Config{
		ZoomLevel: 5 - skillLevel,
		Width:     mapWidth,
		Height:    mapHeight,
		UserId:    user.UserId,
	}

	addOccupantMarkers(&c, skillLevel, user.AsciiMode)

	if p := parties.Get(user.UserId); p != nil {
		for _, uid := range p.GetMembers() {
			if tmpUser := users.GetByUserId(uid); tmpUser != nil {

				// Add any charmed mobs
				for _, mid := range tmpUser.Character.GetCharmIds() {
					if tmpMob := mobs.GetInstance(mid); tmpMob != nil {
						c.OverrideSymbol(tmpMob.Character.RoomId, '☹', `Friend`)
					}
				}

				c.OverrideSymbol(tmpUser.Character.RoomId, '☺', `Party Member`)
			}
		}
	}

	c.OverrideSymbol(user.Character.RoomId, '@', `You`)

	mapOutput := zMapper.GetLimitedMap(roomId, c)

	legend := mapOutput.GetLegend(keywords.GetAllLegendAliases(room.Zone))

	width := 0

	displayLines := []string{}
	for i, line := range mapOutput.Render {
		displayLines = append(displayLines, string(line))
		if width == 0 {
			width = runewidth.StringWidth(displayLines[0])
		}
		displayLines[i] = mapper.ColorizeLegendLine(displayLines[i], legend)
	}

	mapData := map[string]any{
		"Title":        room.Zone,
		"DisplayLines": displayLines,
		"Height":       len(displayLines),
		"Width":        width,
		"Legend":       legend,
		"LegendWidth":  width,
		"LeftBorder": map[string]any{
			"Top":    ".-=~=-.",
			"Mid":    []string{"( _ __)", "(__  _)"},
			"Bottom": "`-._.-'",
		},
		"MidBorder": map[string]any{
			"Top":    "-._.-=",
			"Bottom": "-._.-=",
		},
		"RightBorder": map[string]any{
			"Top":    ".-=~=-.",
			"Mid":    []string{"( _ __)", "(__  _)"},
			"Bottom": "`-._.-'",
		},
	}

	mapTxt, err := templates.Process("maps/map", mapData, user.UserId)
	if err != nil {
		mudlog.Error("Map", "error", err.Error())
		user.SendText(messaging.CategorySystem, `No map found (or an error occured)"`)
		return true, err
	}

	user.SendText(messaging.CategorySystem, mapTxt)

	return true, nil
}
