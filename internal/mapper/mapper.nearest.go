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
// answer. The search stays in fromRoomId's zone (#375 p4 Task 17): a
// neighbour this map lacks, or one it holds from another zone, is never
// entered, so the cost is bounded by the zone, not the world. A secret exit
// is taken only when known lets it through (nil: never).
func (r *mapper) nearestIn(fromRoomId int, in func(roomId int) bool, known SecretExitKnown) (pin, nextRoomId int, exitName string, found bool) {
	start, ok := r.crawledRooms[fromRoomId]
	if !ok {
		return 0, 0, "", false
	}
	zone := start.Zone
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
			ex := node.Exits[name]
			next := ex.RoomId
			if seen[next] {
				continue
			}
			if ex.Secret && !known.allows(at.room, name, next) {
				continue
			}
			if n, ok := r.crawledRooms[next]; !ok || n.Zone != zone {
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

// SecretExitKnown reports whether quest guidance may send its traveller
// through the secret exit exitName of roomId, which leads to toRoomId. Only
// guidance asks: a mob's path (GetPath, `pathto`) takes every secret exit.
// A nil SecretExitKnown lets none through.
type SecretExitKnown func(roomId int, exitName string, toRoomId int) bool

func (k SecretExitKnown) allows(roomId int, exitName string, toRoomId int) bool {
	return k != nil && k(roomId, exitName, toRoomId)
}

// anySecretExit lets every secret exit through, as GetPath does.
func anySecretExit(int, string, int) bool { return true }

// NearestOf is where to head from fromRoomId toward the nearest room in
// accepts (#375 phase 4: quest destinations). Inside the start room's zone it
// searches the map and returns that room as pin with the first hop. When no
// room of that zone qualifies, it heads for the zone in zones with the fewest
// zone crossings, the way crossZoneHop does, by the nearest border room
// toward the next zone (hopToward), and pin is 0: the room is not on the
// player's drawn map. found is false when neither works.
//
// It is guidance, so it never sends the player through a secret exit known
// does not let through, in the zone or across a border: "Head west." through
// an exit the player has not found would be a direction they cannot follow
// and a spoiler of the secret.
func NearestOf(fromRoomId int, in func(roomId int) bool, zones []string, known SecretExitKnown) (pin, nextRoomId int, exitName string, found bool) {
	start := time.Now()
	defer func() { util.TrackTime(`mapper.NearestOf()`, time.Since(start).Seconds()) }()

	fromRoom := rooms.LoadRoom(fromRoomId)
	if fromRoom == nil {
		return 0, 0, "", false
	}
	if searchesHere(fromRoom.Zone, zones) {
		if m := GetMapper(fromRoomId); m != nil {
			if pin, next, exit, ok := m.nearestIn(fromRoomId, in, known); ok {
				return pin, next, exit, true
			}
		}
	}
	link, ok := nearestZoneHop(fromRoom.Zone, zones, known)
	if !ok {
		return 0, 0, "", false
	}
	next, exit, ok := hopToward(fromRoomId, fromRoom.Zone, link.toZone, known)
	return 0, next, exit, ok
}
