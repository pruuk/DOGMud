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
}
```

## Public API

- `Run(atkScore float64, entries []Entry) Result` — one attack roll contested by
  every entry, keeping the widest defensive margin.
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

## Dependencies

`internal/dice` only. Keep it that way — see Purpose.

## Consumers

`internal/combat` (`runBestOfAllDefense`). Roadmap U2–U4 add the spell, ranged,
taunt and non-harm sites.
