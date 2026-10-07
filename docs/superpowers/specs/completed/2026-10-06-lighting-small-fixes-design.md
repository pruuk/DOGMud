# Lighting small fixes (#220, #221, #319, #364)

Status: owner-approved design, 2026-10-06. Part of the lighting arc epic #372;
cleared before plan 6's balance work, at the owner's choice. #332 (the owed
night playtest of the #207 merchant lights) moves into plan 6's closing
playtest instead (owner, 2026-10-06).

## Facts verified against source (master `59851f0a4`)

| # | Fact | Where |
|---|---|---|
| F1 | `look` answers a `LookDark` resolution with "You can't see anything!"; `ResolveLook` returns `LookDark` whenever `ParticipantSight` is `SightNone`, which covers a blinded looker and a room too dark alike | `internal/usercommands/look.go:49-51`; `internal/actions/look.go:50-56` |
| F2 | `who` refuses at `SightNone` with the same line | `internal/usercommands/who.go:14-17` |
| F3 | Two refusals already name darkness: "It's too dark to see anything in that direction." (look at an exit) and "It is too dark to aim. You need light, or eyes that do not need it." (shoot) | `look.go:149`; `shoot.go:131-133` |
| F4 | The blind fight notice: `markBlindCombatant` records a player combatant who is not `SightFull` and whose `SightMult` is below 1.0; `flushBlindCombatNotices` sends `blindCombatNoticeText` once per round, category `CategoryCombatBlindWarning` (suppressible at Light verbosity only) | `internal/hooks/combat_verbosity.go:421,435-475,495-525`; called at `NewRound_DoCombat_unified.go:577-578`; `internal/messaging/verbosity.go:70-73` |
| F5 | A dazzled fighter is `SightFull` (faces), so the blind notice never fires for them, yet `SightMult` is below 1.0; `messaging.ComfortDistance(observer, room)` returns the dark and bright fractions, and a bright fraction above 0 means glare | `combat_verbosity.go:466`; `internal/messaging/comfort.go:20`; `sight_mult.go:18-32` |
| F6 | `Hood` sets `Hooded` on the light records, then sends the room line through `SendTextVisualAsLit` | `internal/usercommands/hood.go:51-62` |
| F7 | A light or darkness record expires inside `Conditions.Trigger` on the round tick (`TriggersLeft` reaches 0), after which `LightNow` stops counting it; its end line goes out at the next turn's prune through `sendConditionEndRoomText`, which judges a light source "as lit" (`SendTextVisualAsLitHidingNames`) and everything else, darkness included, by the room as it is after the change | `internal/conditions/conditions.go:470-516`; `conditions/light.go:43-48`; `hooks/NewRound_UserRoundTick.go:242`; `NewRound_MobRoundTick.go:219`; `hooks/NewTurn_PruneConditions.go:53,112,128-146` |
| F8 | `Room.VisualSnapshot()` and `Room.SendTextVisualToSnapshot(snap, cat, txt, names, exclude...)` implement the owner rule of 2026-10-05 (a line announcing a change is judged against the state before it resolves); 5d's four darkness sites use them | `internal/rooms/rooms.go:414-470` |
| F9 | `SendTextVisualAsLit` and `SendTextVisualAsLitHidingNames` have exactly two production callers, F6 and F7; tests and guards name them | `rooms.go:364-380`; `hiding_senders_test.go:116-118`; `hooks/condition_room_text_test.go:331-341`; `messaging_surface_guard_test.go:880,893`; `bauble_finder_view_guard_test.go:99` |
| F10 | `Character.DarknessTerms` has no production caller; one test calls it (corrected in the plan's dry run: `rooms/darkness_compose_test.go:49` only has it in a test name) | `internal/characters/light.go:17-30`; `usercommands/darkness_test.go:72` |
| F11 | `carriedTerms` looks up a spec once and calls `rec.LightNow(spec)` before `spec.IsDarknessSource()`; `LightNow` with a nil spec returns false (`LightMax(nil)` is 0), so the nil dereference in #221 cannot happen | `internal/rooms/lighting.go:214-230`; `conditions/light.go:24-27,43-52` |

## Design

**1. #364: `look` and `who` name darkness.** When the looker sees nothing
because the room is too dark (not blinded), both answer:

> It is too dark to see. You need light, or eyes that do not need it.

A blinded looker still reads "You can't see anything!". The cause is told
apart by the blind check the sight code already uses; `ResolveLook` gains
the distinction (a `LookTooDark` kind beside `LookDark`, or a cause on the
resolution), so the mob look and every caller share it.

**2. #319: a dazzle cue, once per fight.** A player who fights while glare
costs them (`SightFull`, a bright fraction above 0, `SightMult` below 1.0)
reads, once, the first round it applies in a fight:

> The glare is too bright, so your attacks and defense are weaker.

It does not repeat while that fight goes on; it can fire again in a later
fight. A fight ends when the player leaves combat. Same category and
verbosity rule as the blind notice (`CategoryCombatBlindWarning`). The blind
notice itself is unchanged.

**3. #220: announcements judged against the state before the change.**
- `hood`: take `room.VisualSnapshot()` before setting `Hooded`, then send the
  line with `SendTextVisualToSnapshot`.
- A light or darkness record running out: in the user and mob round ticks,
  just before `Conditions.Trigger`, if a held light or darkness record will
  expire on this trigger, take the room's snapshot and keep it for that
  record; the prune sends the end line against it. Without a kept snapshot
  (a record removed some other way) the line is judged by the room as it is,
  as today. Darkness end lines change too: they were judged after the room
  brightened.
- `SendTextVisualAsLit` and `SendTextVisualAsLitHidingNames` lose their last
  callers and are deleted, with the tests and guard entries that name them.

**4. #221: tidy-ups.** Delete `DarknessTerms`; its one test caller reads the
darkness records another way. The nil guard needs no code (F11); the issue
records why.

## Player-visible lines

| Where | Before | After |
|---|---|---|
| `look`, `who` in a room too dark | You can't see anything! | It is too dark to see. You need light, or eyes that do not need it. |
| Fighting in glare | Nothing | Once per fight: The glare is too bright, so your attacks and defense are weaker. |
| Hood, a light or darkness ending | Judged "as lit" or after the change | Judged by what each watcher could see just before |

## Testing

Each fix lands in its own commit with a test that fails first: the dark and
blind `look` and `who` lines; the dazzle cue once in a fight, not again in
the same fight, again in a new fight, never for a fighter at full sight; the
hood line and a light's and a darkness's end line reaching exactly the
watchers who could see before the change (each shown to fail on today's
sender). Full local gate, lint, and a boot on private ports. No playtest in
this PR: plan 6's closing playtest covers lighting in game, with #332.

## Out of scope

Plan 6's value decisions and its crime-in-the-dark playtest (#372); the
blind notice's per-round cadence.
