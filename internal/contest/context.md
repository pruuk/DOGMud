# internal/contest

## Purpose

The shared resolution seam for opposed rolls: roll one attack against N
defences, report which defence did best. Introduced by roadmap U1 to collapse 33
scattered opposed-roll sites (plus melee, which used none of them) onto one path.

It deliberately does **not** compute scores, spend resources, apply mitigation,
decide crits, or know what a Character is. Callers build fully-modified scores
and hand them over. That keeps it a leaf importing only `internal/dice`, so both
heavy packages (`internal/combat`) and light ones (`internal/forager`) can call
it without a cycle.

## Files

| File | Purpose |
|------|---------|
| `contest.go` | `Entry`, `Result`, `Run`, `RunWithFloors`, `AgainstDifficulty`, and the unexported `clampFloor`. The whole package. |

## Core types

```go
type Entry struct {
	Name  string  // identifies the winner; "" is legal for a static difficulty
	Score float64 // FULLY modified by the caller; this package does no score maths
}

type Result struct {
	AttackRoll  dice.RollResult // always populated, even when uncontested
	DefenseRoll dice.RollResult // zero when Contested is false
	Margin      float64         // ATTACK-POSITIVE; zero when uncontested, never infinite
	Winner      string          // Name of the best defence; "" when uncontested
	Contested   bool
	Success     bool            // the ATTACKER won; read this, not Margin's sign, after floors
	Floored     bool            // a contest floor CHANGED this outcome
}
```

## Public API

- `Run(atkScore float64, entries []Entry) Result` — one attack roll contested by
  every entry, keeping the widest defensive margin.
- `RunWithFloors(atkScore float64, entries []Entry, floor float64) Result`
  — `Run` plus a single SYMMETRIC last-resort floor: the hopeless attacker is
  rescued at rate `floor`, the overwhelming attacker is stopped at rate
  `floor`. At most one flip per call. **See Gotchas.**
- `AgainstDifficulty(score, difficulty float64) Result` — the same path against a
  fixed number instead of an opponent.

## Gotchas

- **`Run` and `AgainstDifficulty` are UNFLOORED.** Migrating a floored caller
  onto either silently deletes its floor. `contest_floor_guard_test.go` fails any
  new production caller outside `internal/combat/combat_helpers.go`.
- **`AgainstDifficulty` has zero production callers** as of U4. The static-check
  sites it was built for (`actions/search.go` x6, `actions/track.go`,
  `forager/forage_core.go`) are still off the core and unassigned; see the
  roadmap's Category B. It is therefore guarded-and-unused: Task 9 added it to
  the floor guard's watch list, so it is protected against misuse but nothing
  calls it yet.
- **`Run` leaves both `RollResult`s' `.Success` and `.Margin` at zero.** It uses
  `dice.Roll`, not `dice.OpposedRoll`. Read `Result.Margin`, never
  `Result.DefenseRoll.Margin`.
- **`Margin` is ATTACK-POSITIVE.** Positive means the attacker won. This is the
  opposite of `internal/combat`'s `bestDefenseResult.margin`, which is
  defence-positive. Mixing the two compiles cleanly and silently puts crit on the
  losing side. `runBestOfAllDefense` performs the one legitimate conversion, at
  its boundary, and says so in a comment.
- **Ask `Contested`, never `Winner`, to find out whether a contest happened.**
  `Winner` is also empty for a legitimately unnamed static-difficulty entry.
- **`Margin` is never an infinity.** `bestDefenseResult` initialises its margin
  to `math.Inf(-1)` and only overwrites it inside its loop, so a defender with no
  usable defence leaves it there — negated, that reads as an infinitely decisive
  attack and crits every swing. This package returns a neutral zero instead.
- **Every defence is rolled with the ATTACKER's stdDev.** Downstream crit maths
  divides the margin by `stdDev * sqrt(2)` on the strength of that. Rolling a
  defence with its own spread would silently shift crit rates everywhere.
- **The attack is rolled ONCE.** All defences contest the same swing, so three
  defences are three chances to beat one roll — not three swings to survive.
- **Ties go to the earliest entry.** Selection uses a strict comparison, so when
  two defences roll identical margins the one earlier in the slice wins. Callers
  that care about tie preference control it through entry order.
- **A `NaN` in `Entry.Score` is caller-responsibility and unguarded.** `NaN < x`
  is always false, so a NaN entry would make the first entry stick as the winner
  regardless of what follows. Scores arrive fully-modified by contract; this
  package does not validate them.
- **No config reads, deliberately.** A Go test binary never loads
  `_datafiles/config.yaml`, so a core that read balance config would be tested
  against Go defaults and any knob defaulting to zero would make its assertions
  vacuously true. Tunables are parameters.
- **After a floor fires, `Success` and `Margin` disagree by design.** A
  floor-granted success carries margin `+1` and a floor-granted resist carries
  `-1` — sentinels, not real margins. Read `Success` for the outcome; never
  re-derive it from the sign of `Margin`.
- **The sentinel is what stops a floor-granted hit from also critting.**
  `ContestCrit` normalises `±1` to a near-zero z. If a future change leaked the
  real margin through a floored outcome, a hopelessly outclassed attacker
  rescued by the floor would crit.
- **`RollResult.Margin` and `.Success` are NOT populated on the rolls this
  package returns.** `dice.OpposedRoll` sets them; `dice.Roll`, which this
  package uses, does not. Read `Result.Margin` and `Result.Success` instead.
  `TrySpellDeflection` read `defRoll.Margin` before U2 — translated naively it
  would have silently passed zero and no spell deflection would ever have
  critted again, with nothing failing and no test breaking.
- **`RunWithFloors` takes ONE symmetric floor, not a success/resist pair.**
  U6 collapsed the two-floor style into a single `ContestFloor` knob (Balance
  config); this function still takes the floor as a plain parameter and never
  reads config itself — `internal/combat.RunContest` is the one place that
  reads `ContestFloor`. Ask `Floored` rather than comparing `Margin` against the
  sentinel, so analytics survive future changes.

## Dependencies

`internal/dice` only. Keep it that way — see Purpose.

## Consumers

**`internal/combat`, and nothing else.** Verified with `grep -rn
"internal/contest" internal/` after U3: the only non-comment import lines are
`internal/combat/combat_helpers.go` and `internal/combat/contest_floors.go`.

- `runBestOfAllDefense` (melee, U1) calls `Run` directly, and is the one place
  that converts this package's attack-positive margin into `internal/combat`'s
  defence-positive one.
- Everything else goes through the three wrappers in
  `internal/combat/contest_floors.go`, `RunWithManeuverFloors`,
  `RunWithSpellFloors` and `RunWithGlobalFloors`, which are where the maneuver,
  spell and global floor pairs are read.

`internal/hooks`, `internal/actions` and `internal/usercommands` all resolve
contests, but they reach this core **through `internal/combat`** and must NOT
import this package directly. U3 removed the last direct `internal/hooks`
import, along with the private `spellHitFloor` / `spellResistFloor` /
`maneuverHitFloor` / `maneuverResistFloor` duplicates that made those sites
invisible to a grep for the exported accessors. Keeping the funnel narrow is
what makes "the floors are read in one place" a checkable claim rather than a
hope.

Sites migrated so far: melee (U1), the spell attack sites and `resolveCharmSpell`
(U2), ranged, taunt, the special-move family, grapple and the submission roll
(U3), and the 19 non-harm contests of U4 (17 on the global pair, plus both
`internal/combat/flee.go` rolls on the maneuver pair). No floored opposed roll is
left off the core.

Still off the core, and unassigned: the static-difficulty checks in
`actions/search.go` (x6), `actions/track.go` and `forager/forage_core.go`. They
use a flat `dice.RollStat` threshold rather than a contest, so converting them is
a behaviour change that U4, contracted as a no-op, could not make. Each site is
breadcrumbed in code. Two of `search.go`'s answer the SAME question as
`usercommands/go.go`'s opposed hidden-detection contest but ignore the hider's
score entirely; whichever chunk claims them must reconcile the two
implementations, not just move one.
