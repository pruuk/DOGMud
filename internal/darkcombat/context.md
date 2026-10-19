# internal/darkcombat

The combat lines a player reads when they cannot see at all. Added by
messaging M6 slice 2 (#370 ledger rows 36 to 47), replacing the twelve Go
literals M4d PR 2 retired with `replaceDarknessMessages`.

Go decides WHEN: `internal/combat.hideIdentitiesInPersonalLines` swaps a
reader's per-swing narration lines (only those; pet, shortage, entangled and
double-fumble lines are kept, names hidden) for one dark line per
`combat.SwingEvent` when that reader's OPTICS verdict (`messaging.ParticipantSight`) is `SightNone`. Not
`ReaderSight`: that one counts a sleeper as blind, and a sleeper hit in a lit
room must not read "in the dark". Shapes readers (infravision) keep the
composed line with "a figure".

Go holds no wording: `_datafiles/world/dogmud/narration/dark-combat/`
`attacker.yaml` and `defender.yaml`, one list per `Outcome`. The one
addition is the wound words `internal/combat.darkLines` appends to a
defender line for a swing that dealt damage (#572), as every lit incoming hit
carries; the pool lines themselves stay token-free.

| Symbol | Use |
|---|---|
| `Viewpoint` (`Attacker`, `Defender`) | whose personal line is replaced |
| `Outcome` (`Fumble`, `Crit`, `Deflected`, `Defended`, `Hit`, `Miss`) | the swing's bucket |
| `Viewpoints()`, `Outcomes()` | fixed-order vocabularies |
| `ViewpointGroup` | one file; `Validate` fails boot on a missing outcome, fewer than `MinVariants` (4) lines, a token, a number, a dash (em, en, spaced hyphen or spaced double hyphen), or over 80 visible columns |
| `LoadFrom(dir) error`, `LoadDarkCombatFiles()` | test and boot loaders |
| `Pool(v, o)`, `Line(v, o, pick)` | read; both are empty while unloaded, so callers keep their composed lines |
| `UnloadForTest()` | test reset |

The bucketing (which `Outcome` a swing is) lives in `internal/combat`
(`darkOutcome`), because this package cannot import `combat`.
