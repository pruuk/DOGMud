package mapper

import (
	"slices"
	"sort"
	"time"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// nearestIn is a breadth-first search from fromRoomId over this map's rooms
// that stops at the first room in accepts (#375 phase 4). Every exit costs 1,
// as in findPath, so the first room found is a nearest one; exits are taken
// in name order so a tie always goes the same way. It returns that room, the
// first room on the way and the exit to it. fromRoomId itself is never the
// answer, and an exit to a room this map lacks (another zone) is not taken.
func (r *mapper) nearestIn(fromRoomId int, in func(roomId int) bool) (pin, nextRoomId int, exitName string, found bool) {
	if _, ok := r.crawledRooms[fromRoomId]; !ok {
		return 0, 0, "", false
	}
	type hop struct {
		room, first int
		exit        string
	}
	seen := map[int]bool{fromRoomId: true}
	var queue []hop
	expand := func(at hop) {
		node := r.crawledRooms[at.room]
		names := make([]string, 0, len(node.Exits))
		for name := range node.Exits {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			next := node.Exits[name].RoomId
			if seen[next] {
				continue
			}
			if _, ok := r.crawledRooms[next]; !ok {
				continue
			}
			seen[next] = true
			h := hop{room: next, first: at.first, exit: at.exit}
			if at.room == fromRoomId {
				h.first, h.exit = next, name
			}
			queue = append(queue, h)
		}
	}
	expand(hop{room: fromRoomId})
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if in(h.room) {
			return h.room, h.first, h.exit, true
		}
		expand(h)
	}
	return 0, 0, "", false
}

// searchesHere reports whether NearestOf crawls the start zone's map. zones
// lists every zone the set's rooms touch, so a start zone outside it holds no
// member and the crawl (a whole zone, per move) is skipped for the zone hop.
func searchesHere(fromZone string, zones []string) bool {
	return slices.Contains(zones, fromZone)
}

// NearestOf is where to head from fromRoomId toward the nearest room in
// accepts (#375 phase 4: quest destinations). Inside the start room's zone it
// searches the map and returns that room as pin with the first hop. When no
// room of that zone qualifies, it heads for the zone in zones with the fewest
// zone crossings, the way crossZoneHop does, and pin is 0: the room is not on
// the player's drawn map. found is false when neither works.
func NearestOf(fromRoomId int, in func(roomId int) bool, zones []string) (pin, nextRoomId int, exitName string, found bool) {
	start := time.Now()
	defer func() { util.TrackTime(`mapper.NearestOf()`, time.Since(start).Seconds()) }()

	fromRoom := rooms.LoadRoom(fromRoomId)
	if fromRoom == nil {
		return 0, 0, "", false
	}
	if searchesHere(fromRoom.Zone, zones) {
		if m := GetMapper(fromRoomId); m != nil {
			if pin, next, exit, ok := m.nearestIn(fromRoomId, in); ok {
				return pin, next, exit, true
			}
		}
	}
	link, ok := nearestZoneHop(fromRoom.Zone, zones)
	if !ok {
		return 0, 0, "", false
	}
	next, exit, ok := hopToward(fromRoomId, link)
	return 0, next, exit, ok
}
