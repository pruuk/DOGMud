package rifts

// memory.go: the lens table, a memory puzzle (puzzle kind `memory`).
//
// A table of 36 lenses in 6 rows (1-6) and 6 columns (a-f), every one face
// down (@). Each lens is tuned to a frequency, shown as a symbol; there are
// 18 pairs, laid out at random each time a room with a table is built, so no
// two tables are alike and no solution carries over to the next run. A
// player turns lenses by typing their place (`1c`, or `c1`; two at once:
// `1c 4e`). Two lenses of one frequency lock face up; two that differ turn
// face down again, and that is a failure. The table bears a number of
// failures (the profile's `memory.fails`, 8 then 4 then 2): when one stage's
// failures run out, every lens turns face down, the locked pairs too, and
// the next stage begins. When the last stage runs out the table goes dark
// for good, and the way it holds shut stays shut: another room, another
// table. Locking all 18 pairs solves the puzzle (solve).
//
// The board's layout does not change between stages: what a player saw
// stays true, which is what makes the later, smaller stages winnable.
//
// Only a reader of the profile's writing (IsReader) can make sense of the
// table: anyone else is told the marks mean nothing and turns nothing.
//
// A room's table is shared by everyone in it: one lens may be waiting for
// its partner at a time, and anyone may turn the next.

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The table's shape.
const (
	MemoryRows  = 6
	MemoryCols  = 6
	MemoryCells = MemoryRows * MemoryCols
	MemoryPairs = MemoryCells / 2

	memoryFaceDown = `@`
)

// MemorySpec is a profile's lens table: what everyone sees, what a reader
// understands, the frequencies (symbols) a board is laid out from, the
// failure stages, and the narration. Each message is a full sentence;
// %s and %d are filled as each one's comment says.
type MemorySpec struct {
	Noun       string `yaml:"noun"`       // `look table`
	Mention    string `yaml:"mention"`    // sentence added to the room's description
	Look       string `yaml:"look"`       // what anyone sees
	Rules      string `yaml:"rules"`      // what a reader understands (before the board)
	Unreadable string `yaml:"unreadable"` // a non-reader looks, or tries a lens
	Symbols    string `yaml:"symbols"`    // the frequencies; 18 are drawn per board
	Fails      []int  `yaml:"fails"`      // failures each stage bears: [8, 4, 2]

	Turned   string `yaml:"turned"`    // to the turner: %s the lens's place, %s its frequency
	Match    string `yaml:"match"`     // to the room: %s and %s the two places
	Miss     string `yaml:"miss"`      // to the turner: %s and %s the two places
	Seen     string `yaml:"seen"`      // to the others: %s who turned a lens
	Reset    string `yaml:"reset"`     // to the room: a stage ran out; %d mistakes undo the next
	Dark     string `yaml:"dark"`      // to the room: the last stage ran out
	DarkLook string `yaml:"dark_look"` // a look at, a try at, or the sealed door of a dark table
	Done     string `yaml:"done"`      // a try at a solved table

	// Abandoned: to a turner, when a lens left up by someone who has gone
	// sinks back first.
	Abandoned string `yaml:"abandoned"`
	// What others who cannot read the marks see instead of Seen, Match,
	// Reset and Dark: the light changing, never the table's workings.
	SeenUnread  string `yaml:"seen_unread"`
	MatchUnread string `yaml:"match_unread"`
	ResetUnread string `yaml:"reset_unread"`
	DarkUnread  string `yaml:"dark_unread"`
}

// MemoryBoard is one table's state, in a RiftRoom.
type MemoryBoard struct {
	Cells     [MemoryCells]string // each lens's frequency
	Up        [MemoryCells]bool   // locked face up
	Open      int                 // the lens waiting for its partner, or -1
	OpenBy    int                 // the user who turned it
	Stage     int                 // index into the profile's Fails
	FailsLeft int                 // failures the current stage still bears
	Dark      bool                // the last stage ran out: the table is spent
}

// newMemoryBoard lays out a fresh board: 18 frequencies drawn from the
// profile's symbols, each placed twice, at random.
func newMemoryBoard(m *MemorySpec) *MemoryBoard {
	pool := memorySymbols(m.Symbols)
	shuffle(pool)
	b := &MemoryBoard{Open: -1, FailsLeft: m.Fails[0]}
	for i := 0; i < MemoryPairs; i++ {
		b.Cells[2*i], b.Cells[2*i+1] = pool[i], pool[i]
	}
	cells := b.Cells[:]
	shuffle(cells)
	return b
}

// memorySymbols is the distinct symbols of s.
func memorySymbols(s string) []string {
	seen := map[rune]bool{}
	var out []string
	for _, r := range s {
		if !seen[r] {
			seen[r] = true
			out = append(out, string(r))
		}
	}
	return out
}

// validMemorySymbol is a symbol a board may show: printable ASCII, not a
// letter, digit, space or the face-down @, and nothing the text layer reads
// as markup.
func validMemorySymbol(r rune) bool {
	if r <= ' ' || r > '~' || r == '@' || r == '<' || r == '>' || r == '\\' || r == '`' {
		return false
	}
	return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
}

func (m *MemorySpec) validate() error {
	if !exitNameRE.MatchString(m.Noun) {
		return fmt.Errorf(`noun must be one lowercase word`)
	}
	for name, s := range map[string]string{`mention`: m.Mention, `look`: m.Look, `rules`: m.Rules, `unreadable`: m.Unreadable,
		`turned`: m.Turned, `match`: m.Match, `miss`: m.Miss, `seen`: m.Seen, `reset`: m.Reset, `dark`: m.Dark,
		`dark_look`: m.DarkLook, `done`: m.Done, `abandoned`: m.Abandoned, `seen_unread`: m.SeenUnread,
		`match_unread`: m.MatchUnread, `reset_unread`: m.ResetUnread, `dark_unread`: m.DarkUnread} {
		if strings.TrimSpace(s) == `` {
			return fmt.Errorf(`%s is required`, name)
		}
	}
	for name, s := range map[string]string{`turned`: m.Turned, `match`: m.Match, `miss`: m.Miss, `seen`: m.Seen, `seen_unread`: m.SeenUnread} {
		if strings.Count(s, `%s`) != map[string]int{`turned`: 2, `match`: 2, `miss`: 2, `seen`: 1, `seen_unread`: 1}[name] {
			return fmt.Errorf(`%s has the wrong number of %%s`, name)
		}
	}
	if strings.Count(m.Reset, `%d`) != 1 {
		return fmt.Errorf(`reset needs one %%d`)
	}
	for _, r := range m.Symbols {
		if !validMemorySymbol(r) {
			return fmt.Errorf(`symbol %q may not be used (no letters, digits, spaces, @, < > \ or backquote)`, r)
		}
	}
	if n := len(memorySymbols(m.Symbols)); n < MemoryPairs {
		return fmt.Errorf(`symbols: %d distinct, need at least %d`, n, MemoryPairs)
	}
	if len(m.Fails) == 0 {
		return fmt.Errorf(`fails: at least one stage`)
	}
	for _, f := range m.Fails {
		if f < 1 {
			return fmt.Errorf(`fails: every stage bears at least one failure`)
		}
	}
	return nil
}

// ---- turning lenses ----

var (
	coordRE  = regexp.MustCompile(`^([1-9])([a-z])$`)
	coordRE2 = regexp.MustCompile(`^([a-z])([1-9])$`)
)

// parseCoord reads a lens's place: `1c` or `c1`, any case. ok is false for
// anything else, or a place off the table.
func parseCoord(s string) (cell int, ok bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	var row, col byte
	if m := coordRE.FindStringSubmatch(s); m != nil {
		row, col = m[1][0], m[2][0]
	} else if m := coordRE2.FindStringSubmatch(s); m != nil {
		row, col = m[2][0], m[1][0]
	} else {
		return 0, false
	}
	r, c := int(row-'1'), int(col-'a')
	if r < 0 || r >= MemoryRows || c < 0 || c >= MemoryCols {
		return 0, false
	}
	return r*MemoryCols + c, true
}

// IsLensPlace reports whether word reads as a place on a lens table (`1c`,
// `c1`), whether or not that place is on the board.
func IsLensPlace(word string) bool {
	w := strings.ToLower(strings.TrimSpace(word))
	return coordRE.MatchString(w) || coordRE2.MatchString(w)
}

func coordName(cell int) string {
	return fmt.Sprintf(`%d%c`, cell/MemoryCols+1, 'a'+cell%MemoryCols)
}

// TurnLenses handles a player typing one or two lens places (`1c`, or
// `1c 4e`) in room. handled is false when the room holds no lens table, so
// the caller can treat the words as it otherwise would.
func TurnLenses(u *users.UserRecord, room *rooms.Room, cmd string, rest string) (handled bool) {
	if !IsLensPlace(cmd) {
		return false
	}
	run, rr := RoomInfo(room.RoomId)
	if run == nil || rr == nil || rr.Memory == nil || run.Profile.Memory == nil {
		return false
	}
	m, b := run.Profile.Memory, rr.Memory
	if !IsReader(u, run.Profile) {
		u.SendText(messaging.CategorySystem, wrap(m.Unreadable))
		return true
	}
	if u.Character.IsInCombat() {
		u.SendText(messaging.CategorySystem, `Not while you are fighting.`)
		return true
	}
	if b.Dark {
		u.SendText(messaging.CategorySystem, wrap(m.DarkLook))
		return true
	}
	if rr.PuzzleSolved {
		u.SendText(messaging.CategorySystem, wrap(m.Done))
		return true
	}
	places := append([]string{cmd}, strings.Fields(rest)...)
	if len(places) > 2 {
		u.SendText(messaging.CategorySystem, `Two lenses at a time: <ansi fg="command">1c 4e</ansi>.`)
		return true
	}
	cells := make([]int, 0, 2)
	for _, place := range places {
		cell, ok := parseCoord(place)
		if !ok {
			u.SendText(messaging.CategorySystem, fmt.Sprintf(`There is no lens at %q. Rows run 1 to %d, columns a to %c: <ansi fg="command">1c</ansi>.`,
				place, MemoryRows, 'a'+MemoryCols-1))
			return true
		}
		cells = append(cells, cell)
	}
	if len(cells) == 2 && cells[0] == cells[1] {
		u.SendText(messaging.CategorySystem, `Two different lenses.`)
		return true
	}

	// A lens left turned up by someone no longer here sinks back unpaired:
	// nobody pays for another's half-finished turn.
	if b.Open >= 0 && !playerIn(room, b.OpenBy) {
		b.Open, b.OpenBy = -1, 0
		u.SendText(messaging.CategorySystem, wrap(m.Abandoned))
	}

	// Two places while a lens already waits would pair the first with the
	// waiting one, not with each other: refuse, and say which is up.
	if len(cells) == 2 && b.Open >= 0 {
		u.SendText(messaging.CategorySystem, fmt.Sprintf(`The lens at %s is already turned up, waiting for its partner. Turn one lens to pair with it.`, coordName(b.Open)))
		u.SendText(messaging.CategorySystem, b.render(m))
		return true
	}

	tellOthers(run, room, u, fmt.Sprintf(m.Seen, turnerName(u)), fmt.Sprintf(m.SeenUnread, turnerName(u)))
	for i, cell := range cells {
		if !turnLens(u, run, rr, room, cell, i == len(cells)-1) {
			break
		}
		if b.Dark || rr.PuzzleSolved {
			break
		}
	}
	return true
}

// turnerName is who others see turning a lens: "Someone" for a hidden
// player.
func turnerName(u *users.UserRecord) string {
	if u.Character.IsHidden() {
		return `Someone`
	}
	return u.Character.Name
}

// tellOthers shows the room's other players what happened at the table, by
// sight: a reader the readable line, anyone else the unreadable one (the
// table's workings are told only to those who can read them).
func tellOthers(run *Run, room *rooms.Room, actor *users.UserRecord, readable string, unreadable string) {
	for _, uid := range room.GetPlayers() {
		if actor != nil && uid == actor.UserId {
			continue
		}
		other := users.GetByUserId(uid)
		if other == nil {
			continue
		}
		line := unreadable
		if IsReader(other, run.Profile) {
			line = readable
		}
		room.SendTextVisualToUser(other, messaging.CategoryRoomDescription, wrap(line))
	}
}

// turnLens turns one lens. It returns false when the turn was refused (the
// lens is already face up), so a second place in the same line is not
// tried. last is whether it is the last lens of the player's line: the
// board is drawn once, after it.
func turnLens(u *users.UserRecord, run *Run, rr *RiftRoom, room *rooms.Room, cell int, last bool) bool {
	m, b := run.Profile.Memory, rr.Memory
	if b.Up[cell] || b.Open == cell {
		u.SendText(messaging.CategorySystem, fmt.Sprintf(`The lens at %s is already turned up.`, coordName(cell)))
		if last {
			u.SendText(messaging.CategorySystem, b.render(m))
		}
		return false
	}
	turned := wrap(fmt.Sprintf(m.Turned, coordName(cell), `<ansi fg="yellow-bold">`+b.Cells[cell]+`</ansi>`))

	if b.Open < 0 {
		b.Open, b.OpenBy = cell, u.UserId
		u.SendText(messaging.CategorySystem, turned)
		if last {
			u.SendText(messaging.CategorySystem, b.render(m))
		}
		return true
	}

	first := b.Open
	b.Open, b.OpenBy = -1, 0
	if b.Cells[first] == b.Cells[cell] {
		b.Up[first], b.Up[cell] = true, true
		match := wrap(fmt.Sprintf(m.Match, coordName(first), coordName(cell)))
		u.SendText(messaging.CategorySystem, turned)
		u.SendText(messaging.CategorySystem, match)
		tellOthers(run, room, u, match, m.MatchUnread)
		u.SendText(messaging.CategorySystem, b.render(m))
		if b.aligned() == MemoryPairs {
			solve(u, run, rr, room)
		}
		return true
	}

	// A failure: counted first, then both are shown so they can be
	// remembered, then they turn face down again.
	b.FailsLeft--
	u.SendText(messaging.CategorySystem, turned)
	u.SendText(messaging.CategorySystem, b.render(m, first, cell))
	u.SendText(messaging.CategorySystem, wrap(fmt.Sprintf(m.Miss, coordName(first), coordName(cell))))
	if b.FailsLeft > 0 {
		return true
	}
	// The stage is spent: every lens turns face down.
	b.Up = [MemoryCells]bool{}
	b.Stage++
	if b.Stage >= len(m.Fails) {
		b.Dark = true
		u.SendText(messaging.CategorySystem, wrap(m.Dark))
		tellOthers(run, room, u, m.Dark, m.DarkUnread)
		return true
	}
	b.FailsLeft = m.Fails[b.Stage]
	reset := wrap(fmt.Sprintf(m.Reset, b.FailsLeft))
	u.SendText(messaging.CategorySystem, reset)
	tellOthers(run, room, u, reset, m.ResetUnread)
	return true
}

// aligned is how many pairs are locked face up.
func (b *MemoryBoard) aligned() int {
	n := 0
	for _, up := range b.Up {
		if up {
			n++
		}
	}
	return n / 2
}

// render draws the table: rows 1-6 down the left, columns a-f along the
// bottom. Locked pairs show their frequency, the lens waiting for its
// partner and any shown cells (a failed pair, for this one look) too;
// every other lens is face down.
func (b *MemoryBoard) render(m *MemorySpec, show ...int) string {
	shown := map[int]bool{}
	for _, c := range show {
		if c >= 0 {
			shown[c] = true
		}
	}
	var sb strings.Builder
	sb.WriteString("\n")
	for r := 0; r < MemoryRows; r++ {
		sb.WriteString(fmt.Sprintf(`   <ansi fg="yellow">%d</ansi>  `, r+1))
		for c := 0; c < MemoryCols; c++ {
			cell := r*MemoryCols + c
			switch {
			case b.Up[cell]:
				sb.WriteString(`<ansi fg="green-bold">` + b.Cells[cell] + `</ansi>`)
			case cell == b.Open:
				sb.WriteString(`<ansi fg="yellow-bold">` + b.Cells[cell] + `</ansi>`)
			case shown[cell]:
				sb.WriteString(`<ansi fg="red-bold">` + b.Cells[cell] + `</ansi>`)
			default:
				sb.WriteString(`<ansi fg="black-bold">` + memoryFaceDown + `</ansi>`)
			}
			if c < MemoryCols-1 {
				sb.WriteString(`  `)
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString(`      `)
	for c := 0; c < MemoryCols; c++ {
		sb.WriteString(fmt.Sprintf(`<ansi fg="yellow">%c</ansi>`, 'a'+c))
		if c < MemoryCols-1 {
			sb.WriteString(`  `)
		}
	}
	sb.WriteString("\n\n")
	// The count is the mistake that ends the stage: with 1 left, the next
	// mistake is the last.
	if b.Stage+1 < len(m.Fails) {
		rest := m.Fails[b.Stage+1:]
		parts := make([]string, len(rest))
		for i, f := range rest {
			parts[i] = fmt.Sprint(f)
		}
		sb.WriteString(fmt.Sprintf(`   Pairs locked: %d of %d.  The table forgets everything at the next %s.`, b.aligned(), MemoryPairs, mistakes(b.FailsLeft)))
		sb.WriteString(fmt.Sprintf(` Then it allows %s, and goes dark at the last.`, strings.Join(parts, `, then `)))
	} else {
		sb.WriteString(fmt.Sprintf(`   Pairs locked: %d of %d.  The table goes dark for good at the next %s.`, b.aligned(), MemoryPairs, mistakes(b.FailsLeft)))
	}
	sb.WriteString("\n")
	return sb.String()
}

// mistakes is "mistake" or "N mistakes": the next one, or the Nth from now.
func mistakes(n int) string {
	if n == 1 {
		return `mistake`
	}
	return fmt.Sprintf(`%d mistakes`, n)
}

// lookTable is what a look at the table adds to its noun's description.
func lookTable(u *users.UserRecord, run *Run, rr *RiftRoom) {
	m, b := run.Profile.Memory, rr.Memory
	switch {
	case !IsReader(u, run.Profile):
		u.SendText(messaging.CategorySystem, wrap(m.Unreadable))
	case b.Dark:
		u.SendText(messaging.CategorySystem, wrap(m.DarkLook))
	default:
		u.SendText(messaging.CategorySystem, wrap(m.Rules))
		u.SendText(messaging.CategorySystem, b.render(m))
		if rr.PuzzleSolved {
			u.SendText(messaging.CategorySystem, wrap(m.Done))
		}
	}
}

// MemorySolution is the board laid bare, for admins (`rift info`).
func (b *MemoryBoard) MemorySolution() string {
	var sb strings.Builder
	for r := 0; r < MemoryRows; r++ {
		sb.WriteString(fmt.Sprintf(`   %d  `, r+1))
		for c := 0; c < MemoryCols; c++ {
			sb.WriteString(b.Cells[r*MemoryCols+c] + `  `)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("      a  b  c  d  e  f\n")
	return sb.String()
}

func playerIn(room *rooms.Room, userId int) bool {
	for _, uid := range room.GetPlayers() {
		if uid == userId {
			return true
		}
	}
	return false
}
