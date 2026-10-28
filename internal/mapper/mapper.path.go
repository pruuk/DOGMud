package mapper

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	lru "github.com/hashicorp/golang-lru/v2"
)

var (
	ErrPathNotFound  = errors.New(`path not found`)
	ErrPathDestMatch = errors.New(`path destination is same as source`)
	pathCache, _     = lru.New[pathCacheKey, pathCacheValue](128)
)

type pathCacheKey struct {
	startRoomId int
	endRoomId   int
}

type pathCacheValue struct {
	steps []pathStep
	err   error
}

// pathStep is one move: take ExitName to arrive in RoomID.
type pathStep struct {
	exitName string
	roomId   int
	waypoint bool
}

func (p pathStep) ExitName() string {
	return p.exitName
}

func (p pathStep) RoomId() int {
	return p.roomId
}

func (p pathStep) Waypoint() bool {
	return p.waypoint
}

// internal struct to record how we reached each node
type prevInfo struct {
	prevRoom int    // the room we came from
	viaExit  string // the exit name we used
}

// A nodeRecord is what we store in the priority queue.
type nodeRecord struct {
	roomId int
	fScore float64
	index  int
}

type priorityQueue []*nodeRecord

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].fScore < pq[j].fScore }
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *priorityQueue) Push(x interface{}) {
	n := x.(*nodeRecord)
	n.index = len(*pq)
	*pq = append(*pq, n)
}
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := old[len(old)-1]
	old[len(old)-1] = nil // avoid mem leak
	n.index = -1
	*pq = old[:len(old)-1]
	return n
}
func (pq *priorityQueue) update(n *nodeRecord, newF float64) {
	n.fScore = newF
	heap.Fix(pq, n.index)
}

// heuristic is the 3D Manhattan distance between two nodes' coordinates (the
// authored ones, unless the zone fell back to the exit-delta crawl).
// This should be redone - rooms can be one space away exit-wise, but 2+ spacex in grid space.
// Example: Frostfang city walls
func heuristic(a, b *mapNode) float64 {
	return math.Abs(float64(a.Pos.x-b.Pos.x)) +
		math.Abs(float64(a.Pos.y-b.Pos.y)) +
		math.Abs(float64(a.Pos.z-b.Pos.z))
}

// nodeFor returns a room's node from its own zone's map: this map when it
// holds the room, else the map that owns it, built on first use the way
// GetPath has always built a start room's map. nil when no map holds the
// room (it does not exist). A zone's map stops at its border, so a path
// that crosses zones reads each room from its own zone's map.
func (r *mapper) nodeFor(roomId int) *mapNode {
	if n, ok := r.crawledRooms[roomId]; ok {
		return n
	}
	m := GetMapperIfExists(roomId)
	if m == nil {
		m = GetMapper(roomId)
	}
	if m == nil {
		return nil
	}
	return m.crawledRooms[roomId]
}

// FindPath returns the sequence of ExitName/RoomID steps from startRoom to goalRoom.
//
// The search starts in this map and follows exits into other zones' maps
// (nodeFor), so a route crosses zone borders exactly as it did when one map
// crawled the whole connected world. Scores start empty and grow as rooms are
// reached: the cost is the rooms the search touches, not the size of a map.
func (r *mapper) findPath(startRoom, goalRoom int) ([]pathStep, error) {

	if startRoom == goalRoom {
		return nil, ErrPathDestMatch
	}

	// sanity check
	startNode := r.nodeFor(startRoom)
	if startNode == nil {
		return nil, ErrRoomNotFound
	}
	goalNode := r.nodeFor(goalRoom)
	if goalNode == nil {
		return nil, ErrRoomNotFound
	}

	// cameFrom holds, for each room, how we got there.
	cameFrom := map[int]prevInfo{}

	// gScore: cost from start to each room reached so far. A room absent
	// from it has not been reached (infinite cost).
	gScore := map[int]float64{startRoom: 0}

	// open set as a priority queue
	openSet := make(priorityQueue, 0, 64)
	heap.Init(&openSet)
	heap.Push(&openSet, &nodeRecord{roomId: startRoom, fScore: heuristic(startNode, goalNode)})
	inOpen := map[int]*nodeRecord{startRoom: openSet[0]}

	for openSet.Len() > 0 {
		current := heap.Pop(&openSet).(*nodeRecord)
		delete(inOpen, current.roomId)

		// reached goal!
		if current.roomId == goalRoom {
			// reconstruct path
			var path []pathStep
			cur := goalRoom
			for cur != startRoom {
				info := cameFrom[cur]

				// record the exit name and the room we arrived in
				path = append(path, pathStep{exitName: info.viaExit, roomId: cur})
				cur = info.prevRoom
			}

			pathLen := len(path)

			if pathLen > 0 {
				// reverse
				for i := 0; i < pathLen/2; i++ {
					j := len(path) - 1 - i
					path[i], path[j] = path[j], path[i]
				}

				// Mark the final room as the waypoint
				path[pathLen-1].waypoint = true
			}
			return path, nil
		}

		// expand neighbors
		node := r.nodeFor(current.roomId)
		if node == nil {
			continue
		}
		for exitName, exitInfo := range node.Exits {
			neighbor := exitInfo.RoomId
			tentativeG := gScore[current.roomId] + 1 // uniform cost

			if g, reached := gScore[neighbor]; reached && tentativeG >= g {
				continue
			}
			// An exit to a room no map holds is not taken (it does not exist).
			neighborNode := r.nodeFor(neighbor)
			if neighborNode == nil {
				continue
			}

			// this is a better path to neighbor
			cameFrom[neighbor] = prevInfo{
				prevRoom: current.roomId,
				viaExit:  exitName,
			}
			gScore[neighbor] = tentativeG
			f := tentativeG + heuristic(neighborNode, goalNode)

			if nr, ok := inOpen[neighbor]; ok {
				openSet.update(nr, f)
			} else {
				nr := &nodeRecord{roomId: neighbor, fScore: f}
				heap.Push(&openSet, nr)
				inOpen[neighbor] = nr
			}
		}
	}

	return nil, ErrPathNotFound
}

func GetPath(startRoomId int, endRoomId ...int) ([]pathStep, error) {

	start := time.Now()
	defer func() {
		util.TrackTime(`mapper.GetPath()`, time.Since(start).Seconds())
	}()

	if len(endRoomId) == 0 {
		return []pathStep{}, fmt.Errorf("%d => %d (endRoom not found): %w", startRoomId, endRoomId, ErrPathNotFound)
	}

	startRoom := rooms.LoadRoom(startRoomId)
	if startRoom == nil {
		return []pathStep{}, fmt.Errorf("%d => %d (startRoom  not found): %w", startRoomId, endRoomId, ErrPathNotFound)
	}

	m := GetMapper(startRoom.RoomId)
	if m == nil {
		return []pathStep{}, fmt.Errorf("%d => %d (mapper not fond): %w", startRoomId, endRoomId, ErrPathNotFound)
	}

	cacheKey := pathCacheKey{}
	rNow := startRoomId
	finalPath := []pathStep{}
	for _, roomId := range endRoomId {

		if rNow == roomId { // Avoid repeating id's
			continue
		}

		cacheKey.startRoomId = rNow
		cacheKey.endRoomId = roomId

		if pCache, ok := pathCache.Get(cacheKey); ok {

			if pCache.err != nil {
				return pCache.steps, fmt.Errorf("%d => %d: %w", rNow, roomId, pCache.err)
			}

			finalPath = append(finalPath, pCache.steps...)
			rNow = roomId
			continue
		}

		// No start-map HasRoom check: a waypoint in another zone is found in
		// its own zone's map (findPath crosses maps). A waypoint no map holds
		// fails in findPath with ErrRoomNotFound and is cached like any miss.
		p, err := m.findPath(rNow, roomId)
		if err != nil {
			pathCache.Add(cacheKey, pathCacheValue{steps: p, err: err})
			return []pathStep{}, fmt.Errorf("%d => %d: %w", rNow, roomId, ErrPathNotFound)
		}

		// Add to LRU cache
		pathCache.Add(cacheKey, pathCacheValue{steps: p, err: nil})

		finalPath = append(finalPath, p...)
		rNow = roomId
	}

	// If final path is empty, they mapped to the same room they are in.
	// This can occur if endRoomId differs from startRoomId, but endRoomId was actually
	// an alias equal to startRoomId
	if len(finalPath) == 0 {
		return finalPath, ErrPathDestMatch
	}

	return finalPath, nil
}

// firstHop extracts the next room + exit direction from a computed path.
func firstHop(path []pathStep) (nextRoomId int, exitName string, found bool) {
	if len(path) == 0 {
		return 0, "", false
	}
	return path[0].roomId, path[0].exitName, true
}

// NextStep returns the next room to head toward on the way from fromRoomId to
// toRoomId, and the compass exit name to take. found is false when from == to
// or the target is genuinely unreachable.
//
// It takes GetPath's first step (as `pathto` does); GetPath searches across
// zone maps, so that covers cross-zone journeys too. When GetPath fails and
// the rooms are in different zones, it falls back to zone-level routing: head
// for the border of the neighbouring zone that leads toward the target. See
// mapper.crosszone.go.
func NextStep(fromRoomId, toRoomId int) (nextRoomId int, exitName string, found bool) {
	if fromRoomId == toRoomId {
		return 0, "", false
	}
	if path, err := GetPath(fromRoomId, toRoomId); err == nil {
		return firstHop(path)
	}

	fromRoom := rooms.LoadRoom(fromRoomId)
	toRoom := rooms.LoadRoom(toRoomId)
	if fromRoom == nil || toRoom == nil || fromRoom.Zone == toRoom.Zone {
		return 0, "", false // same zone and still unreachable: nothing to add
	}
	return crossZoneHop(fromRoomId, fromRoom.Zone, toRoom.Zone)
}
