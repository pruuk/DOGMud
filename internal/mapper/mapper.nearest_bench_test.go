package mapper

import "testing"

// Benchmarks for the quest destination search (#375 phase 4):
//
//	go test ./internal/mapper/ -run x -bench 'NearestIn400|FindPath400' -count=5
//
// Both search a 20x20 grid from one corner toward three rooms in the far
// corner: nearestIn finds the nearest of the three in one breadth-first pass,
// findPath (uncached A*) paths to that same room for comparison.

// buildGridMapperN is buildGridMapper at side n: room ids 1..n*n, row-major.
func buildGridMapperN(n int) *mapper {
	m := NewMapper(1)
	m.crawledRooms = map[int]*mapNode{}
	for id := 1; id <= n*n; id++ {
		x, y := (id-1)%n, (id-1)/n
		exits := map[string]nodeExit{}
		if x > 0 {
			exits["west"] = nodeExit{RoomId: id - 1}
		}
		if x < n-1 {
			exits["east"] = nodeExit{RoomId: id + 1}
		}
		if y > 0 {
			exits["north"] = nodeExit{RoomId: id - n}
		}
		if y < n-1 {
			exits["south"] = nodeExit{RoomId: id + n}
		}
		m.crawledRooms[id] = &mapNode{RoomId: id, Pos: positionDelta{x: x, y: y}, Exits: exits}
	}
	return m
}

// farCorner is the three rooms around the far corner of a 20x20 grid.
var farCorner = inSet(380, 399, 400)

func BenchmarkNearestIn400(b *testing.B) {
	m := buildGridMapperN(20)
	if _, _, _, ok := m.nearestIn(1, farCorner, nil); !ok {
		b.Fatal("no member found")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.nearestIn(1, farCorner, nil)
	}
}

func BenchmarkFindPath400(b *testing.B) {
	m := buildGridMapperN(20)
	goal, _, _, ok := m.nearestIn(1, farCorner, nil)
	if !ok {
		b.Fatal("no member found")
	}
	if _, err := m.findPath(1, goal); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.findPath(1, goal)
	}
}
