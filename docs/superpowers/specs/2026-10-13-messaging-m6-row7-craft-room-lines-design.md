# Messaging M6 row 7: craft room lines

Issue #370, ledger row 7 (`docs/superpowers/audits/messaging-m6-content-ledger.md`).
Owner rulings 2026-10-13 are marked R1 to R3.

## Problem

A player who crafts is invisible to the room. Their start, success and failure
send no room line at all, because the per-recipe observer slots
(`success_observer`, `failure_observer`) exist but 0 of 127 recipes set them. A
mob crafting is narrated, but by different rules: a sight-judged start line,
a Go fallback finish line, and a silent failure. Two sibling paths that should
match do not, and the player one is mute.

## Rulings

- **R1. Per-skill generic pools, no per-recipe copy.** Every craft room line
  comes from a pool keyed by the recipe's skill. No recipe file changes.
  Crafting is due for a rework later, so per-recipe copy waits for it.
- **R2. The start is announced too**, for players as well as mobs. One sender
  narrates start, success and failure for every crafter.
- **R3. A reader who cannot see the crafter hears the craft.** A per-skill
  sound line at start and at finish, the same sound for success and failure,
  so the outcome is not given away. Same spirit as M6 slice 4's R5 (a hidden
  cast start is heard as an unnamed murmur).

## Facts verified against source (master `43d4eda93`)

| Fact | Where |
|---|---|
| 127 recipe files: alchemy 37, blacksmithing 19, cooking 9, enchanting 24, jewelcrafting 18, tailoring 20 | `_datafiles/world/dogmud/recipes/<skill>/` |
| The `default` world has no `recipes/` directory; `casting-messages.yaml` ships in both worlds | `_datafiles/world/default/` |
| Observer slots `SuccessRoomMessage` `yaml:"success_observer"`, `FailureRoomMessage` `yaml:"failure_observer"`; no recipe sets either | `internal/crafting/crafting.go:53-54` |
| Validator: room line must contain `{actor}`; tokens checked; boot panic on violation | `internal/crafting/narration.go` `validateNarration` |
| Player tick success room line sent only when authored, via `sendVisualRoomText` | `internal/hooks/NewRound_UserRoundTick.go:672-674` |
| Player tick failure, same shape | `internal/hooks/NewRound_UserRoundTick.go:709-711` |
| Player instant success: `craftDeliverInstant` (SendTrio, Observer = `roles.Observer`, empty today) | `internal/usercommands/craft.go:198`, `:767` (`completeCraft`, enchanting instant) |
| Player start: `craftDeliver`, Observer `messaging.NoLine` | `internal/usercommands/craft.go:205` |
| Mob instant: `room.SendTextVisual(... MobRoomLine(PhaseSuccess, ..., "... works quickly and produces something."))` | `internal/mobcommands/craft.go:56` |
| Mob start: `announceCraftBegin` loops players, `messaging.ReaderSight`, full line names the output via `craftBeginLine`, shapes line says "something", none gets nothing | `internal/mobcommands/craft.go:81-114` |
| Mob tick success: `MobRoomLine(..., "... finishes their work.")` | `internal/hooks/NewRound_MobRoundTick.go:617` |
| Mob tick failure: silent unless the recipe authors a line | `internal/hooks/NewRound_MobRoundTick.go:629` |
| `RecipeSpec.MobRoomLine` is the only consumer of the mob fallback | `internal/crafting/mob_room_line.go:13` |
| Shared tokens are `{actor}`, `{actee}`, `{actor_plain}`, `{actee_plain}` only; there is no item token | `internal/narration/render.go:129-132`, `internal/textutil/tokens.go` |
| `items.SpecDisplayName(itemId int) string` | `internal/items/items.go:502` |
| Pool store precedent: `spells.CastingMessages`, `LoadCastingMessages` (panics on missing/invalid), `CastRoomLine` renders via `narration.Render` with a picker | `internal/spells/casting_messages.go:114`, `:226`; loaded at `main.go:1982` |
| Recipes load at `main.go:1642` (`crafting.LoadRecipeFiles`) | `main.go` |
| One-sender precedent: `actions.NarrateSpellPhase`, guarded by `spell_phase_narration_guard_test.go` | `internal/actions/spell_phase_lines.go:32` |
| Perception helpers: `UnseenBy(room, c) []int`, `PlayersNotPerceiving`, `MobSeenName(mob, room, viewingUserId)`, `MobDisplayName` | `internal/actions/perception.go` |
| `messaging.ReaderSight(observer, room) SightDecision` carries the sleep gate (a sleeper is `SightNone`) | `internal/messaging/predicates.go:225` |
| `Room.SendTextVisualHidingNames(cat, txt, names, exclude...)` hides names for a shapes reader | `internal/rooms/rooms.go:288` |
| `Room.SendTextUnsighted(cat, txt, exclude...)` delivers on the audio channel to every `SightNone` reader, a sleeper included; the combat and spell-disruption sounds use it | `internal/rooms/rooms.go:566` |
| A hidden spell caster's non-perceivers each get one `u.SendText` of an unnamed line | `internal/actions/spell_phase_lines.go` end of `NarrateSpellPhase` |

**Correction to the design as presented:** the design said a sleeper reads
nothing. The engine's one sound channel (`SendTextUnsighted`) reaches sleepers,
and every other room sound (combat, spell disruption) already does. The craft
sound follows that channel rather than adding a sleeper exception; a sleeper
hears the hammer as they hear a fight. Noise waking is #336's business.

## Design

### Store: `crafting-messages.yaml`

New file `_datafiles/world/dogmud/crafting-messages.yaml`, one block per skill:

```yaml
blacksmithing:
  start_observer:
    - "{actor} sets to work at the forge on {item}."
  success_observer:
    - "{actor} quenches {item} with a hiss and sets it aside, finished."
  failure_observer:
    - "{actor} curses as the metal cracks, and the work is ruined."
  start_heard:
    - "You hear the ring of a hammer on metal nearby."
  finish_heard:
    - "The hammering nearby falls silent."
```

(Lines illustrative; final copy is a plan task.)

- Each pool holds 3 to 4 variants, plain words, no dashes, no `: ` in an
  unquoted scalar (all lines quoted), within the 80-column wrap.
- `start_observer` and `success_observer` must contain `{actor}`; they may
  contain `{item}`. `failure_observer` must contain `{actor}` and may not
  contain `{item}` (the item was never made). Heard pools contain no token at
  all: they name nobody.
- `{item}` is a crafting-store token only. The store substitutes it before
  handing the line to `narration.Render`, and the store's validator accepts it
  where the shared `textutil.ValidateTokens` would not.
- Go side, in `internal/crafting`: `CraftingMessages` (map of skill to
  `SkillPools`), `LoadCraftingMessages()` (panics on a missing, unparseable or
  invalid file, like `LoadCastingMessages`), called from `main.go` right after
  `crafting.LoadRecipeFiles()`. Validation: every pool present and at least 3
  deep, token rules above, and every skill used by a loaded recipe has a
  block. An unknown skill block is an error too, so a misspelling cannot hide.
- The `default` world gets its own copy of the file (as with
  `casting-messages.yaml`), so a default-world boot does not panic. It loads no
  recipes, so the skill cross-check passes trivially.
- Render functions pick through the store's own `narration.Render` with a
  picker, as `spells.CastRoomLine` does, never `textutil.Narrate` (which always
  picks the first variant).

### One sender: `actions.NarrateCraftPhase`

```go
func NarrateCraftPhase(crafter Actor, recipe *crafting.RecipeSpec, phase crafting.Phase, room *rooms.Room)
```

`crafting.Phase` gains `PhaseStart` alongside `PhaseSuccess` and
`PhaseFailure`. The sender sends ROOM lines only; the crafter's own lines stay
where they are (recipe `success_actor` / `failure_actor`, the "You begin
crafting" line). For each player in the room other than the crafter:

| Reader | Gets |
|---|---|
| Perceives the crafter, `SightFull` | seen line, crafter named, `{item}` = output display name |
| Perceives the crafter, `SightShapes` | seen line, crafter name hidden, `{item}` = "something" |
| `SightNone` (dark, blinded, asleep) | heard line: `start_heard` at start, `finish_heard` at success or failure |
| Does not perceive a hidden crafter (`UnseenBy`) | the same heard line, whatever their sight |

Composition, reusing existing senders:

1. Render one seen line twice: `full` (item named) and `vague` (item
   "something"). The crafter's name is the same in both; name hiding is the
   room sender's job.
2. Partition the room's players by `messaging.ReaderSight`, then send `full`
   via `room.SendTextVisualHidingNames` excluding everyone but the full-sight
   readers, and `vague` the same way for the shapes readers. Both exclude the
   crafter and `UnseenBy`.
3. Send the heard line via `room.SendTextUnsighted`, excluding the crafter.
4. Send the heard line to each `UnseenBy` reader who can see (not `SightNone`,
   so step 3 has not already reached them) with `u.SendText`, as
   `NarrateSpellPhase` does for R5.

`{item}` resolves to `items.SpecDisplayName(recipe.Output.ItemId)` in the
`itemname` colour, or "something" when the recipe has no output item (an
enchant recipe) or the id does not resolve, the fallback `craftBeginLine` uses
today. A mob crafter is named by `MobSeenName(mob, room, 0)`; a player by
`GetCharacterName(true)`.

Categories: seen lines `messaging.CategoryEmote` for a player crafter and
`messaging.CategoryMobIdle` for a mob, as each path uses today. Heard lines use
the same category as their seen line.

### Sites moved onto the sender

| Site | Today | After |
|---|---|---|
| Player start, `usercommands/craft.go` `result.Initiated` | no room line | `PhaseStart` |
| Player instant, `craft.go:198` and `completeCraft` `:767` | empty Observer via `craftDeliverInstant` | `PhaseStart` then `PhaseSuccess`; `craftDeliverInstant` keeps only the crafter's line |
| Player tick success/failure, `NewRound_UserRoundTick.go:672`, `:709` | authored-only `sendVisualRoomText` | `PhaseSuccess` / `PhaseFailure` |
| Mob instant, `mobcommands/craft.go:56` | `MobRoomLine` with "produces something" fallback | `PhaseStart` then `PhaseSuccess` |
| Mob start, `announceCraftBegin` | sight-judged Go line | `PhaseStart` |
| Mob tick success/failure, `NewRound_MobRoundTick.go:617`, `:629` | fallback "finishes their work" / silent failure | `PhaseSuccess` / `PhaseFailure` |

An instant craft narrates start and success together, so the room is told the
crafter began and finished, matching a timed craft's two moments.

### Deleted

`RecipeSpec.MobRoomLine` and `mob_room_line.go`, `announceCraftBegin`,
`craftBeginLine`, the Observer half of `craftDeliverInstant`, and both mob
fallback strings. The per-recipe `success_observer` / `failure_observer`
fields STAY in the schema, unread by the sender; the crafting rework decides
whether they return as an override or go. `RecipeSpec.Narration` keeps
returning them so the validator still checks any that get authored, and the
`textSurfaceRegistry` and snapshot entries for them stay.

## Testing

- **Store tests** (`internal/crafting`): shipped `crafting-messages.yaml`
  loads; every recipe skill has a block; each pool has 3 or more variants;
  `{actor}` required where stated; `{item}` refused in failure and heard
  pools; no token in heard pools; unknown skill block refused; no dashes; each
  rendered line fits 80 columns with a long name.
- **Sender tests** (`internal/actions`), each proven able to fail first:
  full-sight reader gets the item named; shapes reader gets "something" and no
  crafter name; dark reader gets the heard line and no seen line; sleeper gets
  the heard line; hidden crafter: a lit non-perceiver gets only the heard
  line, a see-hidden reader gets the seen line; the crafter gets no room line;
  enchant recipe renders "something" for a full-sight reader; failure and
  success give a dark reader the same `finish_heard` pool.
- **Root guard** `craft_phase_narration_guard_test.go`: no file but
  `internal/actions/craft_phase_lines.go` reads a craft observer pool or calls
  the crafting store's room renderers; no site calls `Narrate(...).Observer`
  on a recipe.
- **Goldens and snapshot**: re-record whatever the store snapshot harness
  emits for the new store; existing recipe rows unchanged.
- **Full gate**: `go test ./...` (root guards key on line numbers, so run
  `go test .` after every batch), lint, two boots (dogmud and default world).
- **Playtest**: a player crafts a timed and an instant recipe in a lit room,
  a dim room and a dark room with a second player watching; a mob does the
  same, made to run `craft` by an admin command, since no shipped mob is
  scripted to craft (`crafter: true` mobs restock without the command); a
  hidden player crafts with a lit watcher. Each reader's lines checked against the table above.

## Out of scope

- Per-recipe copy (R1; the crafting rework).
- A room line between start and finish of a long craft.
- Enchanting naming the target item it enchants.
- The crafter's own lines, which are already authored per recipe.
