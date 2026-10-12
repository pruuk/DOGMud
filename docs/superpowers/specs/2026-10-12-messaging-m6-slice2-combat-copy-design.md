# Messaging M6 slice 2: combat copy and the world dash sweep

**Date:** 2026-10-12
**Issue:** #370 (M6), ledger rows 36 to 59 of
`docs/superpowers/audits/messaging-m6-content-ledger.md`
**Status:** design approved by the owner in session, 2026-10-12

## Facts verified against source (master `61da8280b`)

| Fact | Where read |
|---|---|
| The twelve dark-room literals were deleted with `replaceDarknessMessages`; its last body is at `6370977fd^:internal/hooks/NewRound_DoCombat_helpers.go`. It bucketed per `combat.SwingEvent`: skip `DoubleFumble`; `Fumble`; `Crit`; deflected (`DefenseUsed != "" && !DefenseCrit && Damage > 0`); defended (`DefenseCrit \|\| DefenseUsed != ""`); `Hit`; else miss. Categories: defended is `CategoryDodge`, every other bucket `CategoryHitMelee` | `git show 6370977fd^:...` |
| `AttackResult.DefenseUsed` is stamped on a defensive win and never cleared, so it is sticky across a round; the old bucketing narrated a later clean hit as "defended". Implementation added `SwingEvent.Defended` (per swing, `hitResolution.defenceWon()`) | `internal/combat/combat_helpers.go` |
| Today a blind reader's personal lines are the composed store line with the other party hidden: `hideIdentitiesInPersonalLines` (`internal/combat/combat.go`) runs `messaging.HideNames` on `MessagesToSource` at `ctx.sourceSight` and `HideNames` + `HideWeapons` on `MessagesToTarget` at `ctx.targetSight` | `internal/combat/combat.go:712-724` |
| `ctx.sourceSight` / `ctx.targetSight` are `messaging.ReaderSight`, which CARRIES the sleep gate (a sleeper in a lit room is `SightNone`) | `internal/combat/combat_helpers.go:75-76`, `internal/messaging/predicates.go:216-233` |
| `messaging.ParticipantSight` is the optics verdict with no sleep gate; `CanSeeSightImpairedOnly` is built on it | `internal/messaging/predicates.go:56`, `:192` |
| `messaging.ReaderSight` returns `SightShapes` for infravision in the dark (`CanSeeShapes`), so shapes readers read "a figure" and, as defender, "weapon" for a manufactured weapon (`HideWeapons`, ruling R8) | `predicates.go:225`, `internal/messaging/hideweapons.go:13,37` |
| The per-round blind notice is `blindCombatNoticeText` in `internal/hooks/combat_verbosity.go`, marked by `markBlindCombatant` | `combat_verbosity.go:421` |
| Composed core-combat lines claim sight at higher tiers, e.g. `slashing.yaml` normal/together/actor/expert "Reading their stance, you strike with practiced precision!" | `_datafiles/world/dogmud/combat-messages/slashing.yaml` |
| Narration stores follow a domain-package pattern: `internal/movenarration/store.go` (`LoadFrom`, `LoadMoveNarrationFiles`, `Validate` via `narration.ValidateVariants`) and `internal/lightnotice/store.go` | both files |
| `narration.ValidateVariants` already fails boot when sibling roles differ in length; it cannot check that index N describes one moment | `internal/narration/render.go:213` |
| 14 special-move files in `_datafiles/world/dogmud/narration/special-moves/`; `player_*` event counts: bash 4, charge 0, drain 5, gore 4, grapple 4, hamstring 0, kick 10, maul 3, pounce 4, rake 3, shoot 13, throttle 4, throw 4, trip 8 | `grep -c '^  player_'` |
| Both twins render through `renderMoveEvent(verb, event, ids, extra)`: `internal/usercommands/move_narration.go:72`, `internal/mobcommands/move_narration.go:33` | both files |
| Row 58's doubled apostrophe is `narration/special-moves/drain.yaml:37` (`'{actor}''''s touch drains you, ...'`) and is frozen in `internal/narration/testdata/stores/special_moves.golden:74` | both files |
| Row 54: `UnseenNoun`, `UnseenFigure`, `SpeakerNoun` live in `internal/messaging/hidenames.go`, `WeaponWord` in `hideweapons.go`. The stealth-shot literal `` `Someone` `` remains at `internal/mobcommands/shoot.go:97` and `internal/usercommands/shoot.go:441` | those files |
| Idiom counts, special moves / core combat (`combat-messages` + `defense-messages`): "clip" 19/18, "goes wide" 5/12, "slip most" 10/0, "sliver" 4/0, "keep ... footing" 9/0, any "footing" 9/33, "overrun\|carries straight past" 3/0. "sliver" and "clip" also appear in `bauble-corpus.yaml` item names, out of scope | `grep -rniE` |
| Em/en dash lines (non-comment): rooms 3,840 in 771 files (48 zone dirs), mobs 317 in 145 files, messaging 109 (99 in `grapple_outcomes.yaml`), special-move narration 9, combat-messages 0. Spaced hyphen used as a dash: combat-messages 171, narration 2, rooms 1. Spaced double hyphen ` -- ` (added to scope by the owner at plan review): rooms 1,015, mobs 496, narration 1, Go player strings 12. With it, rooms total 4,857 lines in 1,174 files and mobs 813 lines in 461 files | `grep -rnP '\x{2014}\|\x{2013}'`, `grep -rnE '[A-Za-z,.!'"'"'] - [A-Za-z]'` |
| Go string literals with a dash in combat code (`internal/combat`, `internal/mobcommands`, special-move usercommands, `internal/movenarration`): 0. Player-facing Go literals with a dash exist elsewhere, e.g. `internal/hooks/spell_foldrecall.go:66`, `companion_summon.go:102`, `NewRound_AutoHeal.go:118`, `pinnacle_tick.go:359` | `grep -rnP` over `*.go`, non-test, non-comment |

## Scope

Rows 36 to 59, plus a widened row 55 (the owner chose the scope 2026-10-12:
combat and narration text, rooms and mobs). Quests and dialogue keep their
dashes until the quest and dialogue rework arc (#375).

Two PRs:

- **PR 1, combat copy:** rows 36 to 54, 56 to 59, and row 55 for combat and
  narration text and player-facing Go literals.
- **PR 2, world dash sweep:** row 55 for rooms and mobs, plus the dash guard.

## PR 1

### Rows 36 to 47: a dark pool for readers with no sight

Owner ruling (2026-10-12): a reader with NO sight reads purpose-written dark
prose. A shapes reader (infravision) keeps today's composed line with "a
figure" and "weapon", because they can make out the fight.

- New store directory `_datafiles/world/dogmud/narration/dark-combat/`, one
  file per viewpoint (`attacker.yaml`, `defender.yaml`, because the loader is
  keyed by file), loaded by a new package `internal/darkcombat` built on the
  `movenarration` / `lightnotice` pattern (`LoadFrom(dir) error`, a boot
  loader, `narration.ValidateVariants` at load). Each file holds six outcome
  lists: `fumble`, `crit`, `deflected`, `defended`, `hit`, `miss`. The two lists are picked independently (a blind attacker
  and a blind defender are separate readers), so the store validates each
  list on its own with a minimum of 4 variants, rather than as index-paired
  roles.
- Variant 0 of each list is the retired literal from rows 36 to 47 with its
  original colour wrapper (`fumble-text` `!!!` banners on the attacker fumble,
  `crit-text` `***` banners on both crits). Three or more new variants each,
  written from touch and sound, never sight. No `{tokens}`: the lines name no
  one, so nothing can leak.
- Hook: in `hideIdentitiesInPersonalLines`, when the reader's OPTICS verdict
  (`messaging.ParticipantSight(reader, room) == SightNone`) is none, that
  side's PER-SWING narration lines (those `buildAttackMessages` and
  `sendDefenseMessages` record through `sendSwingToSource/Target`) are
  replaced by one dark line per `SwingEvent`, bucketed and categorised as
  `replaceDarknessMessages` did, except that "defended" reads the per-swing
  `SwingEvent.Defended`. Every other personal line (pet damage, defence
  shortage, entangled, double fumble) stays, names hidden. The
  optics verdict, not `ctx.*Sight`, because `ReaderSight` counts a sleeper as
  blind and a sleeper hit in a lit room must not read "in the dark" (the
  2026-08-31 defect). A sleeper keeps today's composed line with
  "something". `combatContext` gains `sourceOptics` / `targetOptics` set in
  `newCombatContext`.
- If an attack produced no `SwingEvent` (nothing to bucket), the composed
  lines stay, as the old function did (`len(newMsgs) > 0`).
- Unchanged: room lines, crit-effect lines (`sendCritEffectTrio`), counter
  lines, wait-round lines, the per-round blind notice.

### Rows 48 to 53: plain words for idioms

Owner-approved replacements, applied in special moves AND core combat
(`combat-messages`, `defense-messages`):

| Now | Becomes |
|---|---|
| clips you / gets clipped / still clips you | barely hits you / is barely hit / still barely hits you |
| shot goes wide / blow goes wide | shot misses / flies past {actee}; blow misses |
| you slip most of it | you dodge most of it |
| still catches a sliver of you | still hurts you a little |
| keep your footing / keep their footing | stay on your feet / stay on their feet |
| lose your footing / lose their footing | lose your balance / lose their balance |
| overruns them / carries straight past | runs over them / runs right past them |

A line that reads badly after the swap is rewritten around the plain word.
"Footing" in non-combat text (dialogue, biomes, quest names) is out of scope.
Every changed line keeps its index, so pairing is preserved.

### Rows 56, 57, 59: one pool per special-move event

Owner rulings (2026-10-12): mobs and players draw from one pool (row 56
option A), and every room line states the same outcome as its personal lines
at the same index (row 59).

- For each of the 12 verbs with `player_*` keys, `player_X` merges into `X`:
  the merged event lists the mob entries then the player entries, index by
  index. A `player_X` with no mob `X` is renamed `X`. Mob entries have no
  actor line, so one is authored for each (the ledger's estimate is about
  24 lines), so the actor role matches its siblings' length.
- The `player_*` event keys go away. The usercommands callers switch to the
  plain keys. Mob callers already use the plain keys and draw from the wider
  pool with no code change.
- Pairing audit over all 14 files: every index of every event is read across
  its roles, recorded in the plan as a table, and fixed where the moments
  differ or a room line states no outcome. Known cases: row 57 (`drain`
  observer index 2 against actor index 3) and row 59 (`kick`: reorder two
  observer lines, author the missing "boot connects" observer line, fix the
  actee drift at index 4).
- `special_moves.golden` and `pre_migration_literals.json` consumers are
  re-recorded or retired as the plan finds them.

### Row 58

`drain.yaml:37` becomes `'{actor}''s touch drains you, ...'` (one rendered
apostrophe). It disappears into the row 56 merge either way, but is called out
so the golden diff shows it.

### Row 54: the anonymity words stay in Go

Decision (owner: either way is fine): engine vocabulary is NOT authorable
content. It already lives in one package. The two `` `Someone` `` literals
become one exported helper in `internal/messaging` (name fixed in the plan,
for example `StealthActorNoun`) used by both shoot sites, and the row closes
recording the decision.

### Row 55, PR 1 part: dashes in combat, narration and Go player text

- Em and en dashes in `narration/` (9 lines) and `messaging/` (109 lines,
  99 in `grapple_outcomes.yaml`).
- Spaced hyphens used as dashes in `combat-messages` (171 lines) and
  `narration` (2 lines). Real hyphens ("life-force") stay.
- Player-facing Go string literals with a dash (text sent to a player, help,
  prompts), outside the quest, dialogue and conversation packages, which
  wait for #375. Log messages, error values and validator output are not
  player prose and stay. The plan enumerates every site before the task
  starts. The text stays in Go (owner, plan review 2026-10-12; M7 #371
  decides any store question), so a root guard parses Go source and fails on
  an em dash, en dash or ` -- ` in a string literal outside log, error and
  exempt builder or admin code.
- Each sentence is rewritten by hand: period, comma, colon, or "and". No
  mechanical replace.

## PR 2: world dash sweep

- Em dashes, en dashes and spaced double hyphens in every room file (1,174
  files across 48 zone directories) and mob file (461 files), plus the one
  spaced-hyphen room line.
- Done zone by zone by subagents with a controller review of each zone. One
  commit per zone, so the owner can read a zone at a time.
- Same hand-rewrite rule. Room descriptions keep their 80-column wrap.
- New guard test: fails on any em dash, en dash, or spaced hyphen or double hyphen used as a dash in a
  non-comment line under `combat-messages`, `defense-messages`, `narration`,
  `messaging`, `rooms` and `mobs`. Quests and dialogue are exempt by name
  with a pointer to #375. The guard lands in PR 2 because only then are all
  its paths clean; PR 1's paths are checked by a narrower form of the same
  test in PR 1.

## Testing

- `internal/darkcombat`: load and validate the shipped store; a missing
  outcome key or a list under 4 variants fails load.
- Combat: unit tests per bucket for a `SightNone` attacker and a `SightNone`
  defender; a shapes reader still gets the composed line; a sleeper in a lit
  room gets the composed line with "something", not a dark line; a double
  fumble produces no dark line; categories match the old function.
- Special moves: the store loads with no `player_*` keys; every usercommand
  caller resolves its event (a test that renders every event key the Go code
  names); goldens re-recorded and the diff reviewed.
- Idioms: a guard that fails on the retired idioms in the combat stores.
- Dashes: the guard above.
- Gate: the full `dogmud-shipping` pre-push gate, a boot check, then the M6
  playtest: a dark fight (blind attacker and blind defender), an infravision
  fight, special moves from both twins in a lit room, and a sample of swept
  rooms.

## Ledger

Rows 36 to 59 are marked `done (PR #___)` in the PR that closes each. Row 55
names both PRs and records that quests and dialogue moved to #375.

## Out of scope

- Dashes in quests, dialogue and conversations (#375).
- Ledger rows 3 to 35 (triaged after this slice).
