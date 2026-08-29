// Package targeting is the single seam for choosing and committing combat
// targets, for players and mobs alike.
//
// Two verbs, deliberately separate, because the codebase discovered the
// distinction three times independently before it had a name for it:
//
//   - Select answers "who should I fight?" and has NO combat consequence.
//     A thief archetype selects a victim without starting a fight
//     (behaviortree's SoftTarget), and StageMeleeTarget resolves a target
//     before the action has been paid for.
//   - Commit enters combat with a selected target. It is the only door
//     external packages use.
//
// LAYERING (see docs/superpowers/plans/2026-08-29-u12a-targeting-seam.md):
//
//   - This package MUST NOT import internal/combat. internal/combat is
//     itself a Commit call site, so importing it creates a cycle. The
//     weakest-mob score arrives through SetPowerScoreFn instead, and
//     TestTargetingDoesNotImportCombat fails if this is ever violated.
//   - internal/characters can never import this package, because this
//     package imports it. That is a constraint on where targeting LOGIC may
//     live, not a licence for characters to keep committing: ForceTauntAggro
//     moved here as CommitTaunt, and characters kept only the lock state.
//     There are no exemptions from this seam.
//
// In U12a, Commit and Release delegate to characters.SetAggro and
// characters.EndAggro, so the Aggro/CombatPhase dual-write and every guard
// are untouched and behaviour is unchanged. U12b migrates the remaining
// write sites; U12c collapses the stores and deletes Aggro.
package targeting
