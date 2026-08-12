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
| `contest.go` | `Entry`, `Result`, `Run`, `AgainstDifficulty` — the whole package |

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
- `RunWithFloors(atkScore float64, entries []Entry, floorSuccess, floorResist float64) Result`
  — `Run` plus the 5.9 last-resort floors. Reproduces
  `dice.OpposedRollStatWithFloors` exactly. **Transitional — see Gotchas.**
- `AgainstDifficulty(score, difficulty float64) Result` — the same path against a
  fixed number instead of an opponent.

## Gotchas

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
- **`RunWithFloors` is TRANSITIONAL scaffolding, not durable API.** The codebase
  has two floor styles: melee applies floors *after* the contest with no margin
  involved, spell and maneuver apply them *inside* the roll and need the
  sentinel. This reproduces only the second, so U2–U5 can be provable no-ops.
  Roadmap section 8 lists reconciling them as an open question for U6, which may
  delete or reshape this entirely. Ask `Floored` rather than comparing `Margin`
  against the sentinel, so analytics survive that change.

## Dependencies

`internal/dice` only. Keep it that way — see Purpose.

## Consumers

- `internal/combat` — `runBestOfAllDefense` (melee, U1) and `TrySpellDeflection`
  (U2).
- `internal/hooks` — four spell attack sites in `spell_resolution.go` and
  `resolveCharmSpell` in `charm_spell.go` (U2).

Roadmap U3–U4 add ranged, taunt, and the non-harm contests.
