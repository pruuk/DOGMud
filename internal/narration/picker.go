// Package narration owns the seam every narration store uses to choose a line
// from a pool, so snapshot runs can be deterministic without touching global
// randomness.
package narration

import "github.com/GoMudEngine/GoMud/internal/util"

// Picker chooses an index in [0,n). Production supplies the engine's
// randomness; the snapshot harness supplies a fixed sequence.
type Picker func(n int) int

// DefaultPicker is what production uses. It routes through util.Rand, which is
// the engine's single randomness seam, rather than stdlib rand.
func DefaultPicker(n int) int {
	if n < 1 {
		return 0
	}
	return util.Rand(n)
}

// SequencePicker returns a picker that walks indices in order, wrapping at n.
//
// IT IS A CLOSURE, NOT A SEED, AND THAT IS THE WHOLE POINT. Seeding global
// randomness would be process-wide, and this repo runs every test in ONE
// binary, so a seeded snapshot's output would depend on which tests ran before
// it. Relative state that passes or fails by order is a trap this codebase has
// already been bitten by. Each caller gets its own counter instead.
func SequencePicker() Picker {
	i := 0
	return func(n int) int {
		if n < 1 {
			return 0
		}
		v := i % n
		i++
		return v
	}
}

// FirstPicker always returns 0 and never touches util.Rand. It is the picker
// for a single-variant store (buffs, spells, quests), where there is nothing
// to choose.
//
// It exists because Render always calls pick(n), DefaultPicker always calls
// util.Rand(n) for n >= 1, and util.Rand(1) still calls rand.Intn. Rendering a
// one-variant pool through DefaultPicker would therefore consume one GLOBAL
// random draw per narrated buff, spell or quest phase and shift every later
// combat roll. Render cannot special-case a pool of one instead: itemvoices
// never validates its pool sizes, so a one-line voice pool is legitimate and
// its draw count must not change either.
func FirstPicker(n int) int { return 0 }
