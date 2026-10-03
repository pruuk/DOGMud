package rifts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Every board holds 18 frequencies, each on exactly two lenses, none of
// them a letter, a digit or the face-down @; and boards differ.
func TestMemoryBoard_Layout(t *testing.T) {
	p := shippedObelisk(t)
	require.NotNil(t, p.Memory)
	layouts := map[string]bool{}
	for i := 0; i < 20; i++ {
		b := newMemoryBoard(p.Memory)
		count := map[string]int{}
		for _, c := range b.Cells {
			count[c]++
			require.Len(t, c, 1)
			assert.True(t, validMemorySymbol(rune(c[0])), c)
		}
		assert.Len(t, count, MemoryPairs, `18 frequencies`)
		for sym, n := range count {
			assert.Equal(t, 2, n, sym)
		}
		assert.Equal(t, -1, b.Open)
		assert.Equal(t, 8, b.FailsLeft)
		layouts[strings.Join(b.Cells[:], ``)] = true
	}
	assert.Greater(t, len(layouts), 18, `layouts are random`)
}

func TestMemory_Places(t *testing.T) {
	for in, want := range map[string]int{`1a`: 0, `1c`: 2, `c1`: 2, `1C`: 2, `C1`: 2, `6f`: 35, `f6`: 35, `2a`: 6} {
		got, ok := parseCoord(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
		assert.Equal(t, strings.ToLower(in), map[bool]string{true: coordName(got), false: strings.ToLower(in)}[in[0] >= '1' && in[0] <= '9'], in)
	}
	for _, bad := range []string{`7a`, `1g`, `a7`, `11`, `aa`, `1`, ``, `1cc`} {
		_, ok := parseCoord(bad)
		assert.False(t, ok, bad)
	}
	assert.True(t, IsLensPlace(`7a`), `a place, though off the table`)
	assert.False(t, IsLensPlace(`look`))
}

// memoryRoom builds a lens table room of the shipped template id, with u a
// reader standing in it.
func memoryRoom(t *testing.T, u *users.UserRecord, id string) (*Run, *RiftRoom, *rooms.Room) {
	t.Helper()
	run, err := OpenPortal(`obelisk`, testOrigin)
	require.NoError(t, err)
	walk(t, u, run.EntryRoomId)
	cPool := run.Profile.templates[PoolPuzzle]
	for _, tp := range cPool {
		if tp.Id == id {
			run.Profile.templates[PoolPuzzle] = []*Template{tp}
		}
	}
	t.Cleanup(func() { run.Profile.templates[PoolPuzzle] = cPool })
	rr, err := run.buildRoom(PoolPuzzle, 1, run.Rooms[run.EntryRoomId], `test`)
	require.NoError(t, err)
	require.NotNil(t, rr.Memory, `a lens table`)
	room := rooms.LoadRoom(rr.RoomId)
	assert.Contains(t, room.Nouns, `table`)
	assert.Contains(t, room.Description, `thirty-six small lenses`)
	MakeReader(u, run.Profile)
	rooms.LoadRoom(u.Character.RoomId).RemovePlayer(u.UserId)
	room.AddPlayer(u.UserId)
	u.Character.RoomId = room.RoomId
	return run, rr, room
}

// pairOf finds two face-down lenses that match (or differ).
func pairOf(b *MemoryBoard, match bool) (int, int) {
	for i := 0; i < MemoryCells; i++ {
		for j := i + 1; j < MemoryCells; j++ {
			if !b.Up[i] && !b.Up[j] && (b.Cells[i] == b.Cells[j]) == match {
				return i, j
			}
		}
	}
	return -1, -1
}

func turn(u *users.UserRecord, room *rooms.Room, cells ...int) bool {
	words := make([]string, len(cells))
	for i, c := range cells {
		words[i] = coordName(c)
	}
	return TurnLenses(u, room, words[0], strings.Join(words[1:], ` `))
}

// A match locks; a miss turns both back and costs a mistake; a stage's
// mistakes running out turns everything face down, locked pairs too, with
// the next stage bearing fewer (8, 4, 2); the last turns the table dark
// for good, and its door stays shut.
func TestMemory_StagesAndDark(t *testing.T) {
	u := setupRuntime(t)
	run, rr, room := memoryRoom(t, u, `c-the-room-of-one-hand`)
	b := rr.Memory

	// Not a lens place, or not a table: not handled.
	assert.False(t, TurnLenses(u, room, `look`, ``))

	i, j := pairOf(b, true)
	require.True(t, turn(u, room, i))
	assert.Equal(t, i, b.Open, `waiting for its partner`)
	assert.True(t, turn(u, room, i), `turning it again is refused...`)
	assert.Equal(t, i, b.Open)
	require.True(t, turn(u, room, j))
	assert.True(t, b.Up[i] && b.Up[j], `a match locks`)
	assert.Equal(t, 8, b.FailsLeft, `...and costs nothing`)
	assert.True(t, turn(u, room, i))
	assert.Equal(t, -1, b.Open, `a locked lens cannot be turned`)

	fail := func() {
		t.Helper()
		x, y := pairOf(b, false)
		require.True(t, turn(u, room, x, y), `two places in one line`)
		assert.False(t, b.Up[x] || b.Up[y])
		assert.Equal(t, -1, b.Open)
	}
	for k := 7; k >= 1; k-- {
		fail()
		assert.Equal(t, k, b.FailsLeft)
		assert.Equal(t, 0, b.Stage)
	}
	assert.True(t, b.Up[i], `still locked before the stage runs out`)
	fail()
	assert.Equal(t, 1, b.Stage)
	assert.Equal(t, 4, b.FailsLeft)
	assert.Equal(t, 0, b.aligned(), `everything face down, the locked pair too`)
	layout := b.Cells
	for k := 0; k < 4; k++ {
		fail()
	}
	assert.Equal(t, 2, b.Stage)
	assert.Equal(t, 2, b.FailsLeft)
	assert.Equal(t, layout, b.Cells, `the layout never changes between stages`)
	fail()
	assert.False(t, b.Dark)
	fail()
	assert.True(t, b.Dark, `the last stage ran out`)
	assert.False(t, rr.PuzzleSolved)

	x, _ := pairOf(b, true)
	assert.True(t, turn(u, room, x))
	assert.Equal(t, -1, b.Open, `a dark table turns nothing`)

	sealed := rr.Template.Puzzle.SealedDoor
	route, handled := Router(u.UserId, room.RoomId, sealed)
	assert.True(t, handled)
	assert.Contains(t, route.Refusal, `dark`, `the way stays shut`)
	_ = run
}

// Locking all 18 pairs solves the table: its door opens and a key-rewarding
// table pays its key. A non-reader turns nothing.
func TestMemory_SolveAndReaders(t *testing.T) {
	u := setupRuntime(t)
	run, rr, room := memoryRoom(t, u, `c-the-lensmakers-proving`)
	b := rr.Memory

	ResetLore(u, run.Profile)
	i, _ := pairOf(b, true)
	assert.True(t, turn(u, room, i), `handled...`)
	assert.Equal(t, -1, b.Open, `...but a non-reader turns nothing`)
	assert.Contains(t, b.render(run.Profile.Memory), `Pairs locked: 0 of 18`)

	MakeReader(u, run.Profile)
	for n := 0; n < MemoryPairs; n++ {
		x, y := pairOf(b, true)
		require.True(t, turn(u, room, x, y))
	}
	assert.True(t, rr.PuzzleSolved)
	assert.True(t, rr.KeyAwarded)
	for _, d := range rr.Doors {
		assert.False(t, d.Sealed)
	}
	assert.True(t, turn(u, room, 0), `a solved table turns nothing more`)
}

// The board as a reader sees it: rows 1-6 down the left, columns a-f along
// the bottom, face down until turned.
func TestMemory_Render(t *testing.T) {
	p := shippedObelisk(t)
	b := newMemoryBoard(p.Memory)
	out := b.render(p.Memory)
	assert.Equal(t, MemoryCells, strings.Count(out, `>@<`), `all face down`)
	assert.Contains(t, out, `>6</ansi>`)
	assert.Contains(t, out, `>f</ansi>`)
	assert.Contains(t, out, `forgets everything at the next 8 mistakes. Then it allows 4, then 2`)
	b.Open = 7
	out = b.render(p.Memory)
	assert.Equal(t, MemoryCells-1, strings.Count(out, `>@<`))
	assert.Contains(t, out, `yellow-bold">`+b.Cells[7])
	assert.Contains(t, b.render(p.Memory, 3), `red-bold">`+b.Cells[3])
}

// A puzzle room keeps a way out that needs no key besides its sealed door,
// since a lens table can be lost for good.
func TestKeepAWayOpen(t *testing.T) {
	doors := []DoorSpec{{Exit: `seal`}, {Exit: `sill`}}
	assert.Equal(t, []Pool{PoolExit, PoolPassage}, keepAWayOpen([]Pool{PoolExit, PoolExit}, doors, `seal`))
	assert.Equal(t, []Pool{PoolPassage, PoolPassage}, keepAWayOpen([]Pool{PoolPassage, PoolExit}, doors, `seal`))
	assert.Equal(t, []Pool{PoolExit, PoolFeature}, keepAWayOpen([]Pool{PoolExit, PoolFeature}, doors, `seal`))
}

// A line is checked whole before any lens turns: a bad second place or a
// third place turns nothing. A lens left up by someone who has gone sinks
// back without charging the next turner. A non-reader at a dark table's door
// is told only that it is sealed.
func TestMemory_LinesAbandonedAndDarkDoor(t *testing.T) {
	u := setupRuntime(t)
	run, rr, room := memoryRoom(t, u, `c-the-room-of-one-hand`)
	b := rr.Memory

	assert.True(t, TurnLenses(u, room, `1a`, `zz`))
	assert.Equal(t, -1, b.Open, `a bad second place turns nothing`)
	assert.True(t, TurnLenses(u, room, `1a`, `1b 1c`))
	assert.Equal(t, -1, b.Open, `three places turn nothing`)
	assert.True(t, TurnLenses(u, room, `1a`, `1a`))
	assert.Equal(t, -1, b.Open, `one lens twice turns nothing`)

	// Two places while one lens waits would pair the waiting one with the
	// first: refused, nothing turns.
	b.Open, b.OpenBy = 0, u.UserId
	assert.True(t, TurnLenses(u, room, `2a`, `2b`))
	assert.Equal(t, 0, b.Open, `still waiting`)
	assert.Equal(t, 8, b.FailsLeft)
	assert.False(t, b.Up[6] || b.Up[7])
	b.Open, b.OpenBy = -1, 0

	x, y := pairOf(b, false)
	b.Open, b.OpenBy = x, 999 // someone else, gone
	require.True(t, turn(u, room, y))
	assert.Equal(t, y, b.Open, `the abandoned lens sank back; this one waits`)
	assert.Equal(t, 8, b.FailsLeft, `nobody paid for it`)

	b.Dark = true
	sealed := rr.Template.Puzzle.SealedDoor
	route, _ := Router(u.UserId, room.RoomId, sealed)
	assert.Contains(t, route.Refusal, `dark`)
	ResetLore(u, run.Profile)
	route, _ = Router(u.UserId, room.RoomId, sealed)
	assert.NotContains(t, route.Refusal, `dark`, `a non-reader learns nothing of the table`)
	assert.Contains(t, route.Refusal, `sealed`)
}

// After a miss the board shows the count that is left.
func TestMemory_CountAfterMiss(t *testing.T) {
	p := shippedObelisk(t)
	b := newMemoryBoard(p.Memory)
	b.FailsLeft--
	assert.Contains(t, b.render(p.Memory, 0, 1), `at the next 7 mistakes`)
	b.Stage, b.FailsLeft = 2, 1
	assert.Contains(t, b.render(p.Memory), `goes dark for good at the next mistake.`)
}
