package mapper

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// ExitKind classifies a spatial exit by comparing its nominal compass delta to
// the actual placed coordinate delta between the two crawled rooms.
type ExitKind string

const (
	ExitNormal   ExitKind = "normal"   // unit cardinal/diagonal, placed exactly as nominal
	ExitLong     ExitKind = "long"     // multi-cell (-x2/-x3), placed exactly as nominal
	ExitVertical ExitKind = "vertical" // up/down (dz != 0)
	ExitWrap     ExitKind = "wrap"     // actual delta != nominal (toroidal/torn edge)
)

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// samePos compares only x/y/z (NOT arrow — positionDelta.arrow differs by direction).
func samePos(a, b positionDelta) bool {
	return a.x == b.x && a.y == b.y && a.z == b.z
}

// isSpatialExit reports whether a direction carries a real coordinate delta
// (compass/vertical). A zero delta means a portal/named exit (non-spatial),
// which may legitimately connect rooms across planes.
func isSpatialExit(d positionDelta) bool {
	return d.x != 0 || d.y != 0 || d.z != 0
}

// classifyKind decides the render/consistency kind from nominal vs actual delta.
func classifyKind(nominal, actual positionDelta) ExitKind {
	if !samePos(nominal, actual) {
		return ExitWrap
	}
	if nominal.z != 0 {
		return ExitVertical
	}
	if absInt(nominal.x) > 1 || absInt(nominal.y) > 1 {
		return ExitLong
	}
	return ExitNormal
}

// findCollisions returns groups of >=2 roomIds that share the same
// (plane,x,y,z) — coordinates only collide within the same plane.
func findCollisions(nodes map[int]*mapNode) [][]int {
	byCell := map[[4]int][]int{}
	for id, n := range nodes {
		key := [4]int{n.Plane, n.Pos.x, n.Pos.y, n.Pos.z}
		byCell[key] = append(byCell[key], id)
	}
	groups := [][]int{}
	for _, ids := range byCell {
		if len(ids) >= 2 {
			sort.Ints(ids)
			groups = append(groups, ids)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i][0] < groups[j][0] })
	return groups
}

// Finding is one consistency problem.
type Finding struct {
	Severity string // "error" (collision/reciprocity/delta) | "warn" (long-crossing)
	Kind     string // "collision" | "noreciprocal" | "deltamismatch" | "longcrossing"
	Zone     string
	RoomId   int
	ExitName string
	Detail   string
}

func (f Finding) String() string {
	loc := fmt.Sprintf("zone=%s room=%d", f.Zone, f.RoomId)
	if f.ExitName != "" {
		loc += " exit=" + f.ExitName
	}
	return fmt.Sprintf("[%s] %s: %s (%s)", f.Severity, f.Kind, f.Detail, loc)
}

// hasReturnExit reports whether dst has any spatial exit leading back to srcId.
func hasReturnExit(dst *mapNode, srcId int) bool {
	for _, e := range dst.Exits {
		if e.RoomId == srcId {
			return true
		}
	}
	return false
}

// roomCrawlable skips ephemeral/instance rooms (they don't exist at boot;
// guards the live cartcheck command).
func roomCrawlable(roomId int) bool {
	return !rooms.IsEphemeralRoomId(roomId)
}

// worldCells indexes the rooms of a set of zone maps at authored coordinates
// by (plane, x, y, z), each room from its own zone's map. Maps at authored
// coordinates share one frame per plane, so two rooms of different zones in
// one cell genuinely overlap; a map at crawl positions has a frame of its own
// and is left out.
type worldCells map[[4]int][]int

// joinedCells is worldCells over the maps of r's WebMap: r's map and the map
// of every zone its border exits reach, the rooms one map crawled before zone
// maps stopped at their border. Not every cached map: a room no exit reaches
// (an instance template such as the jail cell 5107, unplaced at the origin)
// was never checked against the overworld and would now collide with
// Dustwalk Road 400.
func (r *mapper) joinedCells() worldCells {
	if !r.authored {
		return nil
	}
	return buildWorldCells(r.WebMap().maps)
}

func buildWorldCells(maps []*mapper) worldCells {
	w := worldCells{}
	for _, m := range maps {
		if !m.authored || rooms.IsEphemeralRoomId(m.rootRoomId) {
			continue
		}
		for id, n := range m.crawledRooms {
			if rooms.IsEphemeralRoomId(id) || GetMapperIfExists(id) != m {
				continue // a stale map's copy: the room counts once, from its owner
			}
			key := [4]int{n.Plane, n.Pos.x, n.Pos.y, n.Pos.z}
			w[key] = append(w[key], id)
		}
	}
	for _, ids := range w {
		sort.Ints(ids)
	}
	return w
}

// farNode is the node of an exit's far room that is not in this map: its own
// zone's map's node, and whether the two positions share a frame (both maps
// at authored coordinates). nil when the room is in no cached map.
func (r *mapper) farNode(roomId int) (*mapNode, bool) {
	if rooms.IsEphemeralRoomId(roomId) {
		return nil, false
	}
	m := GetMapperIfExists(roomId)
	if m == nil || m == r {
		return nil, false
	}
	n, ok := m.crawledRooms[roomId]
	if !ok {
		return nil, false
	}
	return n, r.authored && m.authored
}

// CheckConsistency walks the crawled rooms of this mapper and returns findings.
// nonCartesian=true (zone flag) suppresses collision/reciprocity/deltamismatch
// (the zone is intentionally non-Euclidean); the long-crossing warning still runs.
//
// A zone's map stops at its border (#375 phase 4), so a border exit is checked
// against the far room's node in its own zone's map, and a room is checked for
// a room of another zone in its cell (worldCells): the cross-zone checks one
// map of the connected world used to make.
func (r *mapper) CheckConsistency(zone string, nonCartesian bool) []Finding {
	return r.checkConsistency(zone, nonCartesian, r.joinedCells())
}

func (r *mapper) checkConsistency(zone string, nonCartesian bool, world worldCells) []Finding {
	findings := []Finding{}
	reg := rooms.GetPlaneRegistry()

	// Rooms of other zones in this map's cells, reported once per cell by the
	// map holding the cell's lowest room id, as one shared map reported a
	// collision group under the zone of its lowest room.
	if r.authored && world != nil {
		cells := map[[4]int]bool{}
		for id, n := range r.crawledRooms {
			key := [4]int{n.Plane, n.Pos.x, n.Pos.y, n.Pos.z}
			if cells[key] {
				continue
			}
			cells[key] = true
			group := []int{id}
			for _, other := range world[key] {
				if _, mine := r.crawledRooms[other]; !mine {
					group = append(group, other)
				}
			}
			if len(group) < 2 {
				continue
			}
			sort.Ints(group)
			if _, mine := r.crawledRooms[group[0]]; !mine {
				continue // the other zone's check reports it
			}
			if nonCartesian || reg.IsNonEuclidean(n.Plane) {
				continue
			}
			findings = append(findings, Finding{
				Severity: "error", Kind: "collision", Zone: zone, RoomId: group[0],
				Detail: fmt.Sprintf("rooms %v occupy the same coordinate on plane %d", group, n.Plane),
			})
		}
	}

	for _, group := range findCollisions(r.crawledRooms) {
		n := r.crawledRooms[group[0]]
		if nonCartesian || (n != nil && reg.IsNonEuclidean(n.Plane)) {
			continue // non-Euclidean plane/zone: overlap is allowed by design
		}
		plane := 0
		if n != nil {
			plane = n.Plane
		}
		findings = append(findings, Finding{
			Severity: "error", Kind: "collision", Zone: zone, RoomId: group[0],
			Detail: fmt.Sprintf("rooms %v occupy the same coordinate on plane %d", group, plane),
		})
	}

	for srcId, src := range r.crawledRooms {
		if !roomCrawlable(srcId) {
			continue
		}
		for exitName, e := range src.Exits {
			dst, ok := r.crawledRooms[e.RoomId]
			sameFrame := true
			if !ok {
				// A border exit: the far room is in its own zone's map.
				if dst, sameFrame = r.farNode(e.RoomId); dst == nil {
					continue // in no map (missing, or an instance room)
				}
			}
			actual := positionDelta{x: dst.Pos.x - src.Pos.x, y: dst.Pos.y - src.Pos.y, z: dst.Pos.z - src.Pos.z}

			srcNonEuclid := nonCartesian || reg.IsNonEuclidean(src.Plane)
			if !srcNonEuclid {
				crossPlane := dst.Plane != src.Plane
				if isSpatialExit(e.Direction) && crossPlane && !reg.IsNonEuclidean(dst.Plane) {
					// Crossing to a DIFFERENT Euclidean plane is a bug — use a
					// portal. (Crossing into a non-Euclidean plane — a maze/warped
					// area — is a legitimate boundary and allowed.)
					findings = append(findings, Finding{
						Severity: "error", Kind: "deltamismatch", Zone: zone, RoomId: srcId, ExitName: exitName,
						Detail: fmt.Sprintf("spatial exit crosses Euclidean planes (%d -> %d); use a portal/door exit", src.Plane, dst.Plane),
					})
				} else if sameFrame && !crossPlane && !samePos(e.Direction, actual) {
					// (Positions in two frames, a map at crawl positions on one
					// side, cannot be compared.)
					findings = append(findings, Finding{
						Severity: "error", Kind: "deltamismatch", Zone: zone, RoomId: srcId, ExitName: exitName,
						Detail: fmt.Sprintf("nominal delta (%d,%d,%d) != actual (%d,%d,%d) — wrap exit in a Cartesian plane; fix the geometry or move the zone to a non-Euclidean plane",
							e.Direction.x, e.Direction.y, e.Direction.z, actual.x, actual.y, actual.z),
					})
				}
				if !e.OneWay && !hasReturnExit(dst, srcId) {
					findings = append(findings, Finding{
						Severity: "error", Kind: "noreciprocal", Zone: zone, RoomId: srcId, ExitName: exitName,
						Detail: fmt.Sprintf("exit to room %d has no return exit (use oneway: true if intentional)", e.RoomId),
					})
				}
			}

			// Soft, always-on: long exit whose straight span crosses an occupied cell.
			if sameFrame && samePos(e.Direction, actual) && (absInt(actual.x) > 1 || absInt(actual.y) > 1) {
				if crossed := r.longSpanCrossesRoom(src.Pos, actual, srcId, e.RoomId, world); crossed != 0 {
					findings = append(findings, Finding{
						Severity: "warn", Kind: "longcrossing", Zone: zone, RoomId: srcId, ExitName: exitName,
						Detail: fmt.Sprintf("long exit connector passes over room %d", crossed),
					})
				}
			}
		}
	}
	return findings
}

// longSpanCrossesRoom returns the roomId of an intervening occupied cell on the
// straight line from start by delta (exclusive of endpoints), or 0 if none.
// Only handles axis-aligned and pure-diagonal spans (the only shapes posDeltas produce).
// world (nil: none) adds the rooms of other zones' maps in the span's cells.
func (r *mapper) longSpanCrossesRoom(start, delta positionDelta, srcId, dstId int, world worldCells) int {
	steps := max(absInt(delta.x), absInt(delta.y))
	if steps <= 1 {
		return 0
	}
	sx, sy := sign(delta.x), sign(delta.y)
	planeOf := 0
	if n := r.crawledRooms[srcId]; n != nil {
		planeOf = n.Plane
	}
	byCell := map[[4]int]int{}
	for id, n := range r.crawledRooms {
		byCell[[4]int{n.Plane, n.Pos.x, n.Pos.y, n.Pos.z}] = id
	}
	for i := 1; i < steps; i++ {
		cell := [4]int{planeOf, start.x + sx*i, start.y + sy*i, start.z}
		if id, ok := byCell[cell]; ok && id != srcId && id != dstId {
			return id
		}
		for _, id := range world[cell] {
			if id != srcId && id != dstId {
				return id
			}
		}
	}
	return 0
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// FilterFindingsToZone drops findings whose room belongs to a different zone.
// Since #375 phase 4 a zone's crawl stops at its border, so a zone map's
// CheckConsistency findings name only its own rooms and this filter keeps
// them all; it stays as a guard for the callers (internal/guards) that scope
// findings to the zone they report on.
func FilterFindingsToZone(findings []Finding, zone string) []Finding {
	out := findings[:0:0] // new backing array, preserve nil-vs-empty sanity
	for _, f := range findings {
		if room := rooms.LoadRoom(f.RoomId); room != nil && room.Zone == zone {
			out = append(out, f)
		}
	}
	return out
}

// zoneMaps is every cached map of zone that still owns its root room (a stale
// map a forced rebuild left behind does not), ordered by root room id. A zone
// can have more than one: a room its root cannot reach inside the zone gets a
// map of its own (PreCacheMaps).
func zoneMaps(zone string) []*mapper {
	var out []*mapper
	for _, m := range mapperZoneCache {
		if rooms.IsEphemeralRoomId(m.rootRoomId) || GetMapperIfExists(m.rootRoomId) != m {
			continue
		}
		if m.rootZone() == zone {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rootRoomId < out[j].rootRoomId })
	return out
}

// CheckZone is the consistency findings for zone's rooms: CheckConsistency over
// every map of the zone, scoped to the zone's rooms. Before zone maps stopped
// at their border one map of the connected world held every room a compass
// exit reached, so a room its zone's root reaches only through another zone
// was checked; it now has its own map, checked here.
func CheckZone(zone string) []Finding {
	nonCartesian := rooms.IsZoneNonCartesian(zone)
	var out []Finding
	for _, m := range zoneMaps(zone) {
		out = append(out, FilterFindingsToZone(m.CheckConsistency(zone, nonCartesian), zone)...)
	}
	return out
}

// ValidateZoneConsistency runs CheckZone for every loaded zone and either
// warns or panics per the MapConsistencyEnforce config knob. Called from PreCacheMaps
// AFTER all zone mappers are built. non_cartesian zones are passed their flag.
func ValidateZoneConsistency() {
	// MapConsistencyEnforce is normalized to off|warn|panic by GamePlay.Validate(); empty never reaches here.
	mode := string(configs.GetGamePlayConfig().MapConsistencyEnforce)
	if mode == "off" {
		return
	}

	errorCount, warnCount := 0, 0
	var firstError string

	zoneNames := rooms.GetAllZoneNames()
	sort.Strings(zoneNames) // the first error named is the same on every boot
	for _, zoneName := range zoneNames {
		for _, f := range CheckZone(zoneName) {
			if f.Severity == "error" {
				errorCount++
				if firstError == "" {
					firstError = f.String()
				}
				mudlog.Error("mapper.ValidateZoneConsistency", "finding", f.String())
			} else {
				warnCount++
				mudlog.Warn("mapper.ValidateZoneConsistency", "finding", f.String())
			}
		}
	}

	mudlog.Info("mapper.ValidateZoneConsistency", "errors", errorCount, "warnings", warnCount, "mode", mode)

	if mode == "panic" && errorCount > 0 {
		panic(fmt.Sprintf("mapper.ValidateZoneConsistency: %d consistency error(s); first: %s", errorCount, firstError))
	}
}
