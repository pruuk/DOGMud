package targeting

import "github.com/GoMudEngine/GoMud/internal/characters"

// powerScoreFn is injected at boot rather than imported, because
// internal/combat is itself a Commit call site (combat.go:409) and importing
// it here would make U12b's migration an import cycle. This mirrors
// characters.SetUserUntargetableCheck (main.go:307), which exists for the
// same reason, and follows the same Set* naming as rooms.SetCompanionTransport
// and rooms.SetBTreeStateEvictor.
//
// Note the VALUE receiver: combat.PowerScore takes characters.Character, not
// a pointer.
var powerScoreFn func(c characters.Character) float64

// SetPowerScoreFn wires the scoring function. Call it once at boot.
// Passing nil unregisters, which is what the tests use.
func SetPowerScoreFn(fn func(c characters.Character) float64) {
	powerScoreFn = fn
}
