package scavenger

import (
	"fmt"
	"time"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

// Walk is a scavenger's walk in progress. It lives in the mob's TempData, so
// a restart simply starts a fresh walk from wherever the scavenger respawns.
type Walk struct {
	Target     int       // the room it is walking to; 0 picks a new one
	NextMoveAt time.Time // no step before this: the linger in each room
	LastFrom   int       // the room its last step was issued from
	Expect     int       // the room that step should have reached; 0 when none is pending
	Fails      int       // steps in a row that went nowhere (a locked door, say)
}

// MaxFailedSteps is how many steps in a row may go nowhere before the
// scavenger gives up on its target and picks another.
const MaxFailedSteps = 2

// RetryAfterNoPath is how soon a scavenger tries again when no path to its
// target could be found; short, since it simply picks another target.
const RetryAfterNoPath = 10 * time.Second

// StepDelay is how long to linger in a room before the next step: a uniform
// pick in [minSecs, maxSecs] real seconds. randn(n) returns 0..n-1.
func StepDelay(minSecs, maxSecs int, randn func(int) int) time.Duration {
	if minSecs < 0 {
		minSecs = 0
	}
	if maxSecs < minSecs {
		maxSecs = minSecs
	}
	secs := minSecs + randn(maxSecs-minSecs+1)
	return time.Duration(secs) * time.Second
}

// PickTarget picks a room from pool other than current, or 0 when the pool
// offers nowhere else.
func PickTarget(pool []int, current int, randn func(int) int) int {
	others := make([]int, 0, len(pool))
	for _, id := range pool {
		if id != current {
			others = append(others, id)
		}
	}
	if len(others) == 0 {
		return 0
	}
	return others[randn(len(others))]
}

// NoteArrival settles the step issued last time, now that the scavenger is in
// current: a step that left it where it was counts as failed, and too many in
// a row drop the target. Call before deciding the next step.
func (w *Walk) NoteArrival(current int) {
	if w.Expect == 0 {
		return
	}
	if current == w.LastFrom {
		w.Fails++
	} else {
		w.Fails = 0
	}
	w.Expect = 0
	if w.Fails >= MaxFailedSteps {
		w.Target = 0
		w.Fails = 0
	}
}

// Line renders one of a scavenger's authored lines, picked with randn, with
// {actor}, {item} and {gold} filled in and marked up for the room, through
// the one token engine (narration.Substitute).
func Line(lines []string, randn func(int) int, name, item string, gold int) string {
	if len(lines) == 0 {
		return ``
	}
	return narration.Substitute(lines[randn(len(lines))], map[string]string{
		narration.TokenActor: `<ansi fg="mobname">` + name + `</ansi>`,
		`{item}`:             `<ansi fg="itemname">` + item + `</ansi>`,
		`{gold}`:             fmt.Sprintf(`<ansi fg="gold">%d gold</ansi>`, gold),
	})
}
