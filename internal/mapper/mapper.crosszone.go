package mapper

import (
	"sort"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Cross-zone routing.
//
// History: until #375 phase 4, GetPath resolved its mapper off the START room
// and rejected a target that map did not hold, so it could not route to a room
// in another zone. That left every cross-zone quest marker inert: no
// destination dot (the room is not in the current zone's snapshot) and no
// next-step arrow. Six quests were affected, worst being the New Plymouth
// questline, whose guidance went dark exactly when a player was hunting for
// the right district. GetPath now searches across zone maps (findPath reads
// each room from its own zone's map), so this graph serves the quest
// destination search (NearestOf) and NextStep's fallback when GetPath fails.
//
// The destination search is deliberately NOT a global room-level search.
// Zones are joined by ordinary exits, so a coarse zone-level graph is enough:
// work out which neighbouring zone to head for, then search inside the zone
// for the nearest border room toward it (hopToward). The player gets a correct
// arrow the whole way, one zone at a time, and each search is bounded by the
// player's zone.
//
// ---------------------------------------------------------------------------
// YES, THIS IS THE SECOND ZONE-ADJACENCY CRAWL IN THE CODEBASE. ON PURPOSE.
// ---------------------------------------------------------------------------
//
// modules/weather/crawler/build.go (buildEdges) already walks every room
// looking for cross-zone exits, to build the graph weather fronts drift along.
// This file does the same walk again. That duplication was reviewed on
// 2026-07-30, unification was designed out loud, and it was REJECTED. The
// reasons are recorded here so nobody has to rediscover them — and so nobody
// "tidies up" the duplication without knowing what it would cost.
//
// 1. THE WEATHER GRAPH CANNOT ANSWER THE ROUTING QUESTION.
//    sim.Edge is {A, B string, Weight int} — undirected, canonicalised so
//    A <= B, and weighted by how many exits cross the border. It records THAT
//    two zones touch. Routing needs to know WHICH WAY TO WALK: the border room
//    in the current zone, the exit name to take, and the room it lands in.
//    buildEdges has all three in hand (`room`, `exitName`, `ex.ToRoom`) and
//    deliberately throws them away, because a weather front does not walk.
//    Reusing it would mean re-deriving the discarded fields — i.e. this crawl.
//
// 2. THE IMPORT DIRECTION FORBIDS IT.
//    internal/ imports modules/ in exactly zero places, and cross-zone routing
//    has to live in internal/mapper because it feeds NextStep, which feeds the
//    GMCP quest payload. A router in internal/ cannot read a graph in modules/.
//
// 3. THE WEATHER PACKAGES ARE DELIBERATELY ENGINE-FREE, AND A TEST ENFORCES IT.
//    modules/weather is vendored from a standalone weather-module repo and is
//    periodically re-synced (see the 2026-06-19 v0.2.0 sync). Both sim/ and
//    crawler/ carry arch tests — TestCrawlerPackageStaysPure parses every file
//    in the package and FAILS on any internal/* import. The WorldReader
//    interface exists precisely so the crawl can run outside a GoMud checkout
//    against an in-memory fake. Merging the crawls would break that invariant,
//    fail that test, and turn every future upstream sync into a conflict
//    against code we had rewritten locally.
//
//    (The "share only the traversal helper" middle ground dies on the same
//    rock: a shared traversal in internal/ is exactly the import crawler/ is
//    forbidden to make.)
//
// WHY THE USUAL COST OF DUPLICATION IS LOW HERE:
//   - Drift risk is small. crawler/ is vendored, so it changes only on a
//     deliberate sync, not when someone edits DOGMud.
//   - Boot cost is small. Weather's crawl sits behind a versioned on-disk
//     cache; ours runs inside PreCacheMaps, where every room is already loaded.
//   - The one cost that was real — a reader finding one crawl and assuming it
//     is the only one — is what this comment block exists to pay off.
//
// KNOWN BEHAVIOURAL DIFFERENCES (both intentional):
//   - Secret exits: weather can exclude them (Options.IncludeSecretExits);
//     this graph records every link and marks a secret one (zoneLink.secret).
//     Mob routing (GetPath, NextStep's fallback) crosses every link, as it
//     takes secret exits in a zone. Quest guidance (NearestOf) crosses a
//     secret link only when the player knows that exit (SecretExitKnown), the
//     same rule its in-zone search (nearestIn) applies, so the two never
//     disagree. joinedMaps reads the graph for which maps sit side by side,
//     where secrecy does not matter.
//   - Zone exclusion: weather skips instance_*/ephemeral_* zones; this graph
//     does not. Currently moot — those zones have zero authored cross-zone
//     exits (they are entered by teleport), so they contribute no links either
//     way. If an instanced zone ever gains a real exit, revisit.
//
// WHAT WOULD CHANGE THIS DECISION:
//   - crawler/context.md anticipates "directional edge metadata for prevailing
//     wind". If weather ever needs direction and border rooms, the two graphs
//     genuinely converge and unification becomes worth re-costing.
//   - If internal/ ever gains a sanctioned way to consume module data.
//   - If the weather module stops being vendored and becomes DOGMud-native,
//     the arch-test constraint goes away.
//
// Cheap anti-divergence option if it is ever wanted, which needs NO change to
// the vendored module: a test asserting both graphs agree on the set of
// adjacent zone pairs.

// zoneLink is one ordinary exit that leaves a zone.
//
// exitRoom is in the FROM zone and destRoom is in toZone — crossing means
// taking exitName out of exitRoom.
type zoneLink struct {
	toZone   string
	exitRoom int
	exitName string
	destRoom int
	secret   bool // the crossing exit is secret
}

// usable reports whether a journey may cross l: always for an open exit, and
// for a secret one only when known lets it through.
func (l zoneLink) usable(known SecretExitKnown) bool {
	return !l.secret || known.allows(l.exitRoom, l.exitName, l.destRoom)
}

var (
	zoneGraphMu    sync.RWMutex
	zoneGraph      map[string][]zoneLink
	zoneGraphBuilt bool
)

// InvalidateZoneGraph drops the cached graph so the next cross-zone query
// rebuilds it. Call after anything that moves rooms between zones (zone
// rename/delete, room re-zone).
func InvalidateZoneGraph() {
	zoneGraphMu.Lock()
	zoneGraph, zoneGraphBuilt = nil, false
	zoneGraphMu.Unlock()
}

// buildZoneGraph walks every room once and records each exit whose destination
// lies in a different zone. Built lazily on first use and cached; the world is
// ~1400 rooms, so this is a one-off scan, not a per-move cost.
//
// Every border link of a zone pair is kept (#375 p4 Task 17), so the in-zone
// leg can head for the nearest of them (hopToward) rather than whichever one
// the scan met first.
func buildZoneGraph() {
	g := map[string][]zoneLink{}

	for _, roomId := range rooms.GetAllRoomIds() {
		room := rooms.LoadRoom(roomId)
		if room == nil || room.Zone == "" {
			continue
		}
		for exitName, ex := range room.Exits {
			dest := rooms.LoadRoom(ex.RoomId)
			if dest == nil || dest.Zone == "" || dest.Zone == room.Zone {
				continue
			}
			g[room.Zone] = append(g[room.Zone], zoneLink{
				toZone:   dest.Zone,
				exitRoom: roomId,
				exitName: exitName,
				destRoom: ex.RoomId,
				secret:   ex.Secret,
			})
		}
	}

	// Room ids and exits come out of maps in no fixed order; sort each zone's
	// links so the zone hop picks the same next zone on every boot.
	links := 0
	for _, v := range g {
		sort.Slice(v, func(i, j int) bool {
			if v[i].toZone != v[j].toZone {
				return v[i].toZone < v[j].toZone
			}
			if v[i].exitRoom != v[j].exitRoom {
				return v[i].exitRoom < v[j].exitRoom
			}
			return v[i].exitName < v[j].exitName
		})
		links += len(v)
	}
	mudlog.Info("mapper.buildZoneGraph", "zones", len(g), "links", links)

	zoneGraph, zoneGraphBuilt = g, true
}

func ensureZoneGraph() {
	zoneGraphMu.RLock()
	built := zoneGraphBuilt
	zoneGraphMu.RUnlock()
	if built {
		return
	}
	zoneGraphMu.Lock()
	defer zoneGraphMu.Unlock()
	if zoneGraphBuilt { // another goroutine won the race
		return
	}
	buildZoneGraph()
}

// nextZoneHop returns the link to take FIRST when travelling from fromZone to
// toZone — i.e. the border out of the caller's current zone, even when the
// destination is several zones away. Breadth-first, so it is the fewest-zones
// route. It crosses secret borders, as GetPath takes secret exits.
func nextZoneHop(fromZone, toZone string) (zoneLink, bool) {
	if toZone == "" {
		return zoneLink{}, false
	}
	return nearestZoneHop(fromZone, []string{toZone}, anySecretExit)
}

// nearestZoneHop is nextZoneHop toward whichever of toZones is the fewest
// zone crossings away (#375 phase 4: an area or an NPC's rooms can span
// zones). fromZone itself and "" are never targets. A link through a secret
// exit counts only when known lets it through (nil: never).
func nearestZoneHop(fromZone string, toZones []string, known SecretExitKnown) (zoneLink, bool) {
	want := map[string]bool{}
	for _, z := range toZones {
		if z != "" && z != fromZone {
			want[z] = true
		}
	}
	if fromZone == "" || len(want) == 0 {
		return zoneLink{}, false
	}
	ensureZoneGraph()

	zoneGraphMu.RLock()
	defer zoneGraphMu.RUnlock()

	// BFS over zones, remembering which first-leg link each frontier came from.
	type node struct {
		zone  string
		first zoneLink
	}
	queue := make([]node, 0, len(zoneGraph[fromZone]))
	seen := map[string]bool{fromZone: true}
	for _, l := range zoneGraph[fromZone] {
		if !l.usable(known) {
			continue
		}
		if want[l.toZone] {
			return l, true // direct neighbour
		}
		if !seen[l.toZone] {
			seen[l.toZone] = true
			queue = append(queue, node{zone: l.toZone, first: l})
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, l := range zoneGraph[n.zone] {
			if !l.usable(known) {
				continue
			}
			if want[l.toZone] {
				return n.first, true // keep the FIRST leg, not this one
			}
			if !seen[l.toZone] {
				seen[l.toZone] = true
				queue = append(queue, node{zone: l.toZone, first: n.first})
			}
		}
	}
	return zoneLink{}, false
}

// crossZoneHop returns the next room + exit for a journey that leaves the
// caller's zone. Standing on a border room toward the next zone, that is the
// crossing exit itself; otherwise it is the first in-zone step toward the
// nearest such border room (hopToward).
func crossZoneHop(fromRoomId int, fromZone, toZone string) (nextRoomId int, exitName string, found bool) {
	link, ok := nextZoneHop(fromZone, toZone)
	if !ok {
		return 0, "", false
	}
	return hopToward(fromRoomId, fromZone, link.toZone, anySecretExit)
}

// hopToward is the first step from fromRoomId (in fromZone) toward nextZone,
// a neighbouring zone (#375 p4 Task 17). Standing on a border room of a link
// to nextZone, it is that link's crossing. Otherwise it is the first step of
// the in-zone search (nearestIn) to the nearest border room among every link
// from fromZone to nextZone, so the cost is bounded by the zone and a border
// the player cannot reach (behind a locked door, say) leaves the others. A
// secret exit, in the zone or across the border, is taken only when known
// lets it through (nil: never).
func hopToward(fromRoomId int, fromZone, nextZone string, known SecretExitKnown) (nextRoomId int, exitName string, found bool) {
	zoneGraphMu.RLock()
	var links []zoneLink
	for _, l := range zoneGraph[fromZone] {
		if l.toZone == nextZone && l.usable(known) {
			links = append(links, l)
		}
	}
	zoneGraphMu.RUnlock()

	borders := make(map[int]bool, len(links))
	for _, l := range links { // sorted by exitRoom then exitName: a fixed pick
		if l.exitRoom == fromRoomId {
			return l.destRoom, l.exitName, true
		}
		borders[l.exitRoom] = true
	}
	if len(borders) == 0 {
		return 0, "", false
	}
	m := GetMapper(fromRoomId)
	if m == nil {
		return 0, "", false
	}
	_, next, exit, ok := m.nearestIn(fromRoomId, func(roomId int) bool { return borders[roomId] }, known)
	return next, exit, ok
}
