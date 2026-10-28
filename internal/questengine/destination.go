package questengine

import (
	"slices"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// DestKind is what a quest step's destination resolved to (#375 phase 4).
type DestKind string

const (
	DestNone     DestKind = ""         // the final step, or no such quest or step
	DestAnywhere DestKind = "anywhere" // the step can be done anywhere
	DestRoom     DestKind = "room"
	DestMob      DestKind = "mob" // live copies of an NPC the player perceives
	DestArea     DestKind = "area"
	DestZone     DestKind = "zone"
	DestUnknown  DestKind = "unknown" // nothing resolved
)

// Destination is where a quest step points for one player right now.
type Destination struct {
	Kind      DestKind
	Label     string // area or zone name; "" otherwise
	Npc       string // the NPC's name for a mob destination, also when it fell back
	Fallback  bool   // a mob destination showing its fallback set (no copy perceived)
	ZoneRooms []int  // the set's template rooms in the player's zone, sorted, for the map wash; may be shared with the areas index: do not change it
	Pin       int    // nearest set room inside the player's zone; 0 if none
	Inside    bool   // the player's room is in the set
	NextRoom  int
	NextDir   string
	Inferred  bool // read from the step's triggers, not a destination block
}

// liveCopy is one live instance of a mob template.
type liveCopy struct {
	roomId int // the room it stands in (an instance room id when instanced)
	char   *characters.Character
}

// The world seams ResolveDestination reads; package vars so tests stub them.
var (
	liveCopies = func(mobId int) []liveCopy {
		var out []liveCopy
		for _, instId := range mobs.GetAllMobInstanceIds() {
			m := mobs.GetInstance(instId)
			if m == nil || int(m.MobId) != mobId || m.Character.RoomId < 1 {
				continue
			}
			out = append(out, liveCopy{roomId: m.Character.RoomId, char: &m.Character})
		}
		return out
	}
	// sightIn is what viewer would make out standing in roomId.
	sightIn = func(viewer *characters.Character, roomId int) messaging.SightDecision {
		r := rooms.LoadRoom(roomId)
		if r == nil {
			return messaging.SightNone
		}
		return messaging.ParticipantSight(viewer, r)
	}
	templateRoom = func(roomId int) int { id, _ := rooms.OriginalRoomId(roomId); return id }
	roomZone     = rooms.TemplateRoomZone
	spawnRoomsOf = rooms.SpawnRooms
	npcName      = func(mobId int) string {
		if m := mobs.GetMobSpec(mobs.MobId(mobId)); m != nil {
			return m.Character.Name
		}
		return ""
	}
	nearestOf = mapper.NearestOf
)

// destSet is a destination's rooms: a membership test on template room ids
// and the zones they touch, never a full room list per call.
type destSet struct {
	kind      DestKind
	label     string
	npc       string
	fallback  bool
	in        func(templateRoomId int) bool
	zones     []string
	zoneRooms func(zone string) []int // nil: no wash (a zone, a single room)
}

func roomSet(id int) destSet {
	return destSet{kind: DestRoom, in: func(r int) bool { return r == id }, zones: []string{roomZone(id)}}
}

func areaSet(key string) destSet {
	a, _ := areas.Get(key)
	return destSet{kind: DestArea, label: a.Name, in: func(r int) bool { return areas.Contains(key, r) },
		zones: areas.ZonesOf(key), zoneRooms: func(z string) []int { return areas.RoomsIn(key, z) }}
}

func zoneSet(zone string) destSet {
	return destSet{kind: DestZone, label: zone, in: func(r int) bool { return roomZone(r) == zone }, zones: []string{zone}}
}

// roomsSet is a set of listed rooms: an NPC's rooms, spawn rooms, an inferred
// union. ids is cloned before sorting: rooms.SpawnRooms hands out a shared slice.
func roomsSet(kind DestKind, ids []int) destSet {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	member := make(map[int]bool, len(ids))
	var zones []string
	for _, id := range ids {
		member[id] = true
		if z := roomZone(id); z != "" && !slices.Contains(zones, z) {
			zones = append(zones, z)
		}
	}
	sort.Strings(zones)
	return destSet{kind: kind, in: func(r int) bool { return member[r] }, zones: zones,
		zoneRooms: func(z string) []int {
			var out []int
			for _, id := range ids {
				if roomZone(id) == z {
					out = append(out, id)
				}
			}
			return out
		}}
}

// perceivedRooms lists the template rooms of the live copies of any of
// mobIds the viewer perceives at full sight, judged as if standing in each
// copy's room: Character.Perceives (hidden) and ParticipantSight == SightFull
// (owner rulings 4, 5 and 16). The sight is judged once per room per call.
func perceivedRooms(viewer *characters.Character, mobIds []int) []int {
	sight := map[int]bool{}
	seen := map[int]bool{}
	var out []int
	for _, mobId := range mobIds {
		for _, c := range liveCopies(mobId) {
			if !viewer.Perceives(c.char) {
				continue
			}
			ok, judged := sight[c.roomId]
			if !judged {
				ok = sightIn(viewer, c.roomId) == messaging.SightFull
				sight[c.roomId] = ok
			}
			if !ok {
				continue
			}
			if tpl := templateRoom(c.roomId); !seen[tpl] {
				seen[tpl] = true
				out = append(out, tpl)
			}
		}
	}
	sort.Ints(out)
	return out
}

// npcNames is the destination's NPC name: one name, or several joined with
// " or " in list order (owner ruling 16).
func npcNames(mobIds []int) string {
	var names []string
	for _, m := range mobIds {
		if n := npcName(m); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, " or ")
}

// mobSet is a mob destination naming one NPC or several, any of which will
// do: the perceived copies of any of them, else the fallback the step
// declares (room, area or zone), else the union of their spawn rooms (owner
// rulings 6 and 16).
func mobSet(viewer *characters.Character, mobIds []int, fallback quests.StepDestination) (destSet, bool) {
	npc := npcNames(mobIds)
	if ids := perceivedRooms(viewer, mobIds); len(ids) > 0 {
		s := roomsSet(DestMob, ids)
		s.npc = npc
		return s, true
	}
	var s destSet
	switch {
	case fallback.Room > 0:
		s = roomSet(fallback.Room)
	case fallback.Area != "":
		s = areaSet(fallback.Area)
	case fallback.Zone != "":
		s = zoneSet(fallback.Zone)
	default:
		var ids []int // appended, never aliased: rooms.SpawnRooms slices are shared
		for _, m := range mobIds {
			ids = append(ids, spawnRoomsOf(m)...)
		}
		if len(ids) == 0 {
			return destSet{}, false
		}
		slices.Sort(ids)
		s = roomsSet(DestArea, slices.Compact(ids))
	}
	s.npc, s.fallback = npc, true
	return s, true
}

func declaredSet(viewer *characters.Character, d quests.StepDestination) (destSet, bool) {
	switch {
	case len(d.Mobs) > 0:
		return mobSet(viewer, d.Mobs, d)
	case d.Room > 0:
		return roomSet(d.Room), true
	case d.Area != "":
		return areaSet(d.Area), true
	case d.Zone != "":
		return zoneSet(d.Zone), true
	}
	return destSet{}, false
}

// stepPlaces is what the triggers gated on a step name as places, the
// inference input (owner ruling 7).
type stepPlaces struct {
	rooms []int
	mobs  []int
	areas []string
	zones []string
}

func (p stepPlaces) empty() bool {
	return len(p.rooms)+len(p.mobs)+len(p.areas)+len(p.zones) == 0
}

// A trigger belongs to the step Quest.CountStep names (the latest step of the
// quest its has lists), so a trigger gated on two steps counts for the later
// one only: it cannot fire before the player holds both tokens.
func placesOf(q *quests.Quest, step string) stepPlaces {
	var p stepPlaces
	for i := range q.Triggers {
		t := &q.Triggers[i]
		if q.CountStep(*t) != step {
			continue
		}
		for _, r := range []int{t.Room, t.Conditions.InRoom} {
			if r > 0 && !slices.Contains(p.rooms, r) {
				p.rooms = append(p.rooms, r)
			}
		}
		if t.Mob > 0 && !slices.Contains(p.mobs, t.Mob) {
			p.mobs = append(p.mobs, t.Mob)
		}
		if t.Area != "" && !slices.Contains(p.areas, t.Area) {
			p.areas = append(p.areas, t.Area)
		}
		if t.Zone != "" && !slices.Contains(p.zones, t.Zone) {
			p.zones = append(p.zones, t.Zone)
		}
	}
	return p
}

// inferredSet joins every place the step's triggers name. One kind keeps its
// kind (and its label when all parts share it); mixed kinds give a room set.
func inferredSet(viewer *characters.Character, p stepPlaces) (destSet, bool) {
	var parts []destSet
	if len(p.rooms) > 0 {
		parts = append(parts, roomsSet(DestRoom, p.rooms))
	}
	for _, m := range p.mobs {
		if s, ok := mobSet(viewer, []int{m}, quests.StepDestination{}); ok {
			parts = append(parts, s)
		}
	}
	for _, a := range p.areas {
		parts = append(parts, areaSet(a))
	}
	for _, z := range p.zones {
		parts = append(parts, zoneSet(z))
	}
	switch len(parts) {
	case 0:
		return destSet{}, false
	case 1:
		return parts[0], true
	}
	u := destSet{kind: parts[0].kind, label: parts[0].label, npc: parts[0].npc, fallback: parts[0].fallback}
	for _, s := range parts[1:] {
		if s.kind != u.kind {
			u.kind = DestRoom
		}
		if s.label != u.label {
			u.label = ""
		}
		if s.npc != u.npc {
			u.npc = ""
		}
		u.fallback = u.fallback && s.fallback
	}
	if u.kind == DestRoom {
		u.label, u.npc, u.fallback = "", "", false
	}
	u.in = func(r int) bool {
		for _, s := range parts {
			if s.in(r) {
				return true
			}
		}
		return false
	}
	u.zoneRooms = func(z string) []int {
		var out []int
		for _, s := range parts {
			if s.zoneRooms != nil { // a zone part has none: the banner carries it
				out = append(out, s.zoneRooms(z)...)
			}
		}
		slices.Sort(out)
		return slices.Compact(out)
	}
	for _, s := range parts {
		for _, z := range s.zones {
			if !slices.Contains(u.zones, z) {
				u.zones = append(u.zones, z)
			}
		}
	}
	sort.Strings(u.zones)
	return u, true
}

// ResolveDestination is where questId's step points user right now (#375
// phase 4): the declared destination block, else what the step's triggers
// name, else unknown. Spec section 2.
func (e *Engine) ResolveDestination(user *users.UserRecord, questId int, step string) Destination {
	q := e.quests[questId]
	if q == nil || user == nil || user.Character == nil {
		return Destination{}
	}
	idx := slices.IndexFunc(q.Steps, func(s quests.QuestStep) bool { return s.Id == step })
	if idx < 0 || idx == len(q.Steps)-1 {
		return Destination{}
	}
	s := q.Steps[idx]
	if s.Destination.Anywhere {
		return Destination{Kind: DestAnywhere}
	}
	var set destSet
	var ok bool
	inferred := s.Destination.IsZero()
	if inferred {
		set, ok = inferredSet(user.Character, placesOf(q, s.Id))
	} else {
		set, ok = declaredSet(user.Character, s.Destination)
	}
	if !ok {
		return Destination{Kind: DestUnknown, Inferred: inferred}
	}
	return finishDestination(set, user.Character.RoomId, inferred)
}

func finishDestination(set destSet, playerRoom int, inferred bool) Destination {
	from := templateRoom(playerRoom)
	d := Destination{Kind: set.kind, Label: set.label, Npc: set.npc, Fallback: set.fallback, Inferred: inferred}
	if set.zoneRooms != nil {
		if zr := set.zoneRooms(roomZone(from)); len(zr) > 1 || (len(zr) == 1 && set.kind == DestArea) {
			d.ZoneRooms = zr
		}
	}
	if set.in(from) {
		d.Inside = true
		return d
	}
	in := func(r int) bool { return set.in(templateRoom(r)) }
	if pin, next, exit, ok := nearestOf(playerRoom, in, set.zones); ok {
		d.Pin, d.NextRoom, d.NextDir = templateRoom(pin), templateRoom(next), exit
	}
	return d
}
