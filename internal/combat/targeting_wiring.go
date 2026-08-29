package combat

import "github.com/GoMudEngine/GoMud/internal/targeting"

// The weakest-mob selection strategy in internal/targeting needs PowerScore,
// but targeting MUST NOT import this package: internal/combat is itself a
// targeting.Commit call site (combat.go:409, migrated in U12b), so the import
// would be a cycle. TestTargetingDoesNotImportCombat enforces that direction.
//
// So the PROVIDER registers itself. Wiring it here rather than in main.go is
// deliberate: targeting.Select fails closed without a scorer, which is correct
// (picking arbitrarily would silently change which mob gets eaten) but means a
// missing registration is invisible until a predator quietly stops hunting.
// main.go is not linked into any test binary, so a main.go-only registration
// left every test that exercises predation silently failing closed. Binding it
// to the package that owns the function makes that impossible.
//
// Tests may still override or disable it with targeting.SetPowerScoreFn.
func init() {
	targeting.SetPowerScoreFn(PowerScore)
}
