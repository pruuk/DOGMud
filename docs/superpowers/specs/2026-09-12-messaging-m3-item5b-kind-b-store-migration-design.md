# Messaging M3 item 5b: buffs, spells and quests onto the narration core

Date: 2026-09-12. Branch `feature/messaging-m3-item5b-kind-b-migration` off
master `449339957`. Second half of M3 item 5 of the messaging unification arc
(`2026-08-31-messaging-unification-design.md`); the first half, 5a, shipped as
PR #119 and fixed the live defects on today's paths. This half is the
migration the owner split off: a byte-identical refactor, proven by goldens.

Owner rulings taken during this design (2026-09-12, do not relitigate):

- **A buff's holder line occupies the Actee role.** Actor is reserved for the
  caster, which M6 will author once `events.Buff` carries a caster. Spells and
  quests are Actor plus Observer.
- **Approach A**: store doors over a thin `textutil` adapter, with the core
  reached through one function.
- **Name hiding for these stores stays where the arc puts it**: the send-path
  collapse is M4 and the `{source_plain}` leak ruling is M5. Not in this slice.

## Facts verified against source

Every claim below was read from the tree at `449339957` on 2026-09-12.

| Fact | Where |
|---|---|
| The core: `Render(v Variants, tokens map[string]string, pick Picker, indexOverride ...int) Roles` picks ONE index, then substitutes with a `strings.NewReplacer` whose keys are sorted longest-first | `internal/narration/render.go` |
| `Render` calls `pick(n)` on every render, override or not, and `TestRenderIndexOverrideStillConsumesAPick` pins that | `render.go`; `render_test.go:98` |
| `DefaultPicker(n)` calls `util.Rand(n)` for every `n >= 1`; `util.Rand` calls `math/rand`'s `Intn` for every `maxInt >= 1`. **A one-variant pool rendered with the default picker consumes a global random draw** | `internal/narration/picker.go:15-20`; `internal/util/util.go:11,203-208` |
| `itemvoices` calls `ValidateVariants` zero times, so it may hold one-line pools whose draw count must not change | grep of `internal/itemvoices/itemvoices.go` |
| `ValidateVariants(v, minVariants, expected ...Role)`: equal counts, no blank variant, count at least `minVariants` | `render.go` |
| Six buff text fields, all `string` | `internal/buffs/buffspec.go:171-176` |
| Buff token validation only WARNS on an unknown token | `buffspec.go:272-279` |
| `StartUserNotice()` and `EndUserNotice()` are the holder-line doors (secret, `silent-start`, `hidden`, generic fallback) | `internal/buffs/notice.go:31,55` |
| Six spell text fields, all `string`; token validation warns only | `internal/spells/spells.go:66-71,300-307` |
| Quest action text: `ActionDef.SendText` and `ActionDef.RoomText`; reward text: `QuestReward.PlayerMessage` and `RoomMessage` | `internal/quests/triggers.go:48-49`; `internal/quests/quests.go:46-47` |
| Quest room_text validation FAILS on a line without `{source}` and runs inside `Quest.Validate`, which fileloader calls on every parse | `internal/quests/roomtext.go`; `internal/fileloader/fileloader.go:107,219` |
| No shipped quest action sets both `send_text` and `room_text` (545 actions across both worlds, 0 with both) | walked every `_datafiles/world/*/quests/*.yaml`, nested `sequence.on_complete` included |
| `textutil.SubstituteTokens` replaces exactly four tokens with a `strings.NewReplacer`; `ValidateTokens` warns on any other `{token}` | `internal/textutil/tokens.go` |
| `textutil.SendPhaseText(user, room, ctx, colorName, cfg)` substitutes and delivers through two closures; `colorName` is already ignored | `internal/textutil/spelltext.go` |
| `SendPhaseText` has exactly 11 call sites and ZERO test callers (the first draft of this row said 12 while listing 11; corrected 2026-09-12 by the whole-branch review) | `hooks/Buff_ApplyBuffs.go:146`, `hooks/NewRound_UserRoundTick.go:293`, `hooks/NewTurn_PruneBuffs.go:63,125`, `hooks/NewRound_MobRoundTick.go:276`, `hooks/spell_resolution.go:232`, `hooks/NewRound_DoCombat_helpers.go:616,760`, `mobcommands/aid.go:76`, `mobcommands/cast.go:143`, `usercommands/skill.cast.go:348` |
| `SubstituteTokens` has two callers outside textutil: the quest bridge's `RoomText` and the behaviour tree's dialogue action | `internal/questengine/bridge.go:227`; `internal/behaviortree/actions_dialogue.go:37,44` |
| Quest reward messages are sent RAW, no substitution: player line on `CategorySystem`, room line through `sendVisualRoomText` on `CategoryEmote` | `internal/hooks/Quest_HandleQuestUpdate.go:261-267` |
| Quest `send_text` is delivered raw on `CategoryNPCDialogue` via `ActionContext.SendText`; `room_text` via `ActionContext.RoomText`, which substitutes `{source}` and sends `SendTextVisual` on `CategoryNPCDialogue` | `internal/questengine/actions.go:26-27,71-77`; `bridge.go:201-233` |
| `ActionContext` has THREE implementations: `GameBridge`, the `mockActionContext` in `actions_test.go`, and `recordingCtx` (embedded by `panickingCtx`) in `action_abort_test.go`; and `GameBridge.RoomText` has three DIRECT callers in `internal/hooks/quest_room_text_test.go` that bypass the interface. (Corrected 2026-09-12 during Task 8; the first draft of this row said two and missed the direct callers.) | `bridge.go`; `internal/questengine/actions_test.go:23,65,68`; `internal/questengine/action_abort_test.go:10-46`; `internal/hooks/quest_room_text_test.go:24,39,55` |
| Three appliers send a silent-start buff's raw `StartUserText` with no substitution: sleep (buff 15), arrest (buff 88), stun and broken limb (buffs 84 and 83) | `internal/actions/sleep.go:75-76`; `internal/justice/arrest.go:47,408-409`; `internal/hooks/Position_Messaging.go:377-391`; `internal/combat/submission_outcome.go:20,22` |
| Those four start lines carry no token; NO buff user-side text carries a token at all | grep `{` over `*_user_text` in `_datafiles/world/dogmud/buffs`: 0 hits |
| Shipped field counts, dogmud: buffs start_user 96 / start_room 33 / trigger_user 15 / trigger_room 7 / end_user 97 / end_room 9; spells cast_user 58 / cast_room 57 / wait_user 18 / wait_room 2 / magic 0 / 0; quests send_text 134 / room_text 22 / playermessage 66 / roommessage 56 | grep over `_datafiles/world/dogmud/{buffs,spells,quests}` |
| Tokens in shipped text: `{source}` 113, `{source_plain}` 17 (all in buff room lines), `{target}` 2 (both spell `cast_room_text`: charm, repair-pulse); zero tokens in any `send_text`, `playermessage` or `roommessage`; zero tokens anywhere in the default world | grep over both worlds |
| 18 spell and buff files use folded YAML scalars for text; 0 text fields are whitespace-only | grep |
| Delivery channels today: buff start and trigger room lines `SendTextVisual`; buff end room lines `sendBuffEndRoomText` (`SendTextVisualAsLit` for a light buff); spell cast room line `SendTextVisual`; spell wait and magic room lines `Room.SendText` (AUDIO, not sight-gated) | the 11 sites above; `internal/rooms/rooms.go:308,315,338` |
| `SendTextVisual` already gates on sight and anonymizes tagged names for infrared observers; `SendTextVisualHidingNames` additionally hides the given names when untagged | `rooms.go:304-317` and `sendTextVisualJudgedBy` |
| The snapshot harness holds seven goldens; none for buffs, spells or quests. `setupRealStores` loads items, taunt, itemvoices and casting from the dogmud data dir | `internal/narration/snapshot_test.go:130-139,687-710`; `testdata/stores/` |
| Buffs and spells load from `configs.GetFilePathsConfig().DataFiles`, so the harness can load them the same way; quests resolve their root through `questsDataRoot`, a package variable that is a test seam | `buffspec.go:358`; `spells.go:472`; `quests.go:351`; `internal/quests/save.go:19-21` |
| `narration.Render(` is called from exactly FIVE production files (`items/attack_messages.go` imports the package for its picker only) | `combat/taunt_messages.go`, `grapplemessaging/render.go`, `items/defensive_messages.go`, `itemvoices/itemvoices.go`, `spells/casting_messages.go` |
| Test seeding helpers exist for buffs and spells | `internal/buffs/test_helpers.go:6`; `internal/spells/test_helpers.go:6` |
| The root viewpoint guard fingerprints top-level `SendText` / `SendTextVisual` / `sendVisualRoomText` statements by receiver name; a candidate is actor plus exactly one of actee or observer; every entry carries a verdict and a reason | `messaging_surface_guard_test.go:770-830,1004-1024,1315` |
| Today's 11 sites are invisible to that walk because their sends sit inside closures; the three raw silent-start sends are visible and sleep's is already registered | `messaging_surface_guard_test.go:1151` |
| The M2 literal freeze covers 25 verb files; none of the 11 sites is among them | `messaging_surface_guard_test.go:1404-1429` |
| The YAML key registry already lists every text key this slice touches; no key is added or renamed | `messaging_surface_guard_test.go:66-120` |
| `textutil` is imported by 14 production files; `narration` by 6 store files; `narration` imports only `internal/util` | grep |
| `events.Buff` carries no caster: `UserId`, `MobInstanceId`, `BuffId`, `Source`, `DurationMult` | `internal/events/eventtypes.go:20-31` |
| A buff `Validate` error panics the load, as it does today for a bad `tick_pool` | `buffspec.go:277-289,359-362` |

## What 5b delivers

The arc's M3 spec found that buffs, spells and quests are Kind B stores: single
strings, no pool, no selection problem. Their lifecycle phase IS the selector.
What they lack is the role model. Today each of their 11 render sites builds a
`TokenContext`, two closures and a `SendTextConfig`, then calls a helper that
substitutes and delivers in one motion; the quest bridge and the reward hook do
it two more ways; three appliers read a text field straight off the spec. That
is the dialect sprawl the arc exists to collapse.

After 5b:

- Each store exposes its text as `narration.Variants` per phase, with the
  roles named, and renders through the core.
- One function substitutes tokens for every store, including the dialogue
  store, which is not migrated but stops having its own engine.
- `SendPhaseText` and `SendTextConfig` are gone. Delivery stays at the sites,
  exactly as it is today, because the send-path collapse is M4.
- Three new goldens freeze what the stores emit, built before the migration.
- No file outside a store reads a store's text field.

Output is byte-identical on shipped data. Every golden, old and new, must come
out unchanged, and `-update` is forbidden in this slice.

## Design

### The core: `narration.FirstPicker`

```go
// FirstPicker always returns 0 and never touches util.Rand. It is the picker
// for a single-variant store, where there is nothing to choose.
func FirstPicker(n int) int { return 0 }
```

This is the one addition to `internal/narration`. It exists because `Render`
always draws, `DefaultPicker` always calls `util.Rand`, and `util.Rand(1)`
still calls `rand.Intn`. Rendering roughly 750 single-string fields through the
default picker would add one global random draw per narrated buff, spell or
quest phase and shift every later combat roll. The core cannot special-case a
pool of one instead: itemvoices never validates its pool sizes, so a one-line
voice pool exists legitimately and its draw count must not change either.

### The adapter: `textutil`

`textutil` keeps the four-token vocabulary. The two vocabularies (this one and
`items.TokenName`) unify in M4, not here.

```go
// Tokens is the vocabulary as the core takes it. All four keys are always
// present, so an absent target renders as an empty string, exactly as
// SubstituteTokens has always rendered it.
func (ctx TokenContext) Tokens() map[string]string

// Pool is a single-variant pool: nil for an empty string, else the one line.
// Whitespace is kept so the validator can refuse it.
func Pool(text string) []string

// Narrate renders one single-variant event. It is the ONLY way a Kind B store
// reaches narration.Render, and it always passes FirstPicker.
func Narrate(v narration.Variants, ctx TokenContext) narration.Roles

// SubstituteTokens is unchanged in signature and result, and is now a
// one-variant Narrate, so one engine substitutes for every store.
func SubstituteTokens(text string, ctx TokenContext) string
```

`spelltext.go` is deleted with `SendTextConfig` and `SendPhaseText`. The
package's `context.md` is rewritten to match. `textutil` gains an import of
`narration`; `narration` imports only `util`, so no cycle is possible.

Substitution equivalence, stated once: both engines are a single-pass
`strings.NewReplacer` over the same four pairs. No token is a prefix of another
because each ends in `}`, so the order of the pairs cannot change the result,
and an empty value replaces to an empty string in both. Identical for every
input.

### Store doors

Each store gets the same three things: a phase selector, a `Narration(phase)`
that assembles the `Variants` (the assembly rule: the store decides what goes in
each role), and a `Narrate(phase, ctx)` that renders through `textutil.Narrate`.
Empty fields are omitted from the `Variants`, so a phase with no room line
renders an empty Observer and a phase with no text at all renders nothing,
exactly as `SendPhaseText` sent nothing for an empty string.

**buffs** (`internal/buffs/narration.go`):

```go
type Phase uint8
const (PhaseStart Phase = iota; PhaseTrigger; PhaseEnd)

// Narration: the holder's line is the ACTEE (the buff happens to them); the
// room line is the Observer. Actor is empty and reserved for the caster.
// Start and End use StartUserNotice / EndUserNotice, so the secret, hidden,
// silent-start and generic-fallback rules stay in their one door.
func (b *BuffSpec) Narration(p Phase) narration.Variants
func (b *BuffSpec) Narrate(p Phase, ctx textutil.TokenContext) narration.Roles

// AuthoredStartLine renders start_user_text as written, ignoring the notice
// rules. It is the door for the applier of a silent-start buff, which
// narrates the start itself (sleep, arrest, stun, broken limb).
func (b *BuffSpec) AuthoredStartLine(ctx textutil.TokenContext) string
```

**spells** (`internal/spells/narration.go`): `Phase` (`PhaseCast`, `PhaseWait`,
`PhaseMagic`); `Narration` puts the caster's line in Actor and the room line in
Observer; `Narrate` as above.

**quests** (`internal/quests/narration.go`):

```go
// An action's send_text is the Actor line, its room_text the Observer line.
func (a ActionDef) Narration() narration.Variants
func (a ActionDef) Narrate(ctx textutil.TokenContext) narration.Roles
// A reward's playermessage is the Actor line, its roommessage the Observer.
func (r QuestReward) Narration() narration.Variants
func (r QuestReward) Narrate(ctx textutil.TokenContext) narration.Roles
```

### Validation at load

Each store's `Validate` runs `narration.ValidateVariants(v, 1)` over every
phase that has any text, on the RAW fields (a whitespace-only `start_user_text`
must fail even on a silent-start buff, where the notice would hide it). A
failure is a load panic, the same as a bad `tick_pool` today. Shipped data in
both worlds passes; a whitespace-only line would fail boot, which it cannot
today. Token validation stays a warning; upgrading it is filed, not done.

Quests gain one rule alongside the existing `room_text` rule: an action that
sets both `send_text` and `room_text` is refused. Today `ExecuteAction` would
silently drop the room line of such an action; after this slice the action
narrates both. Zero shipped actions set both, so the rule refuses nothing and
the behaviour that changes is unreachable.

### The quest interface

`ActionContext` drops `SendText(cat, text)` and `RoomText(text)` for one
method, `Narrate(v narration.Variants)`. `ExecuteAction` calls it with
`a.Narration()` when either text is set. `GameBridge.Narrate` renders with the
player's tagged and plain name and delivers exactly as today: Actor on
`CategoryNPCDialogue` to the player, Observer on `CategoryNPCDialogue` through
`SendTextVisual` excluding the player. The test mock records the rendered
roles instead of two text lists. The reward hook renders
`questInfo.Rewards.Narrate(ctx)` and delivers Actor on `CategorySystem` and
Observer through `sendVisualRoomText` on `CategoryEmote`, as now.

`send_text`, `playermessage` and `roommessage` are substituted for the first
time. Shipped data carries zero tokens in them, so the goldens prove the
output unchanged; a future line that names `{source}` in one of them will
render the name instead of the literal, which is the behaviour quest 77's
`room_text` bug showed is wanted.

### The sites

Each of the 11 `SendPhaseText` sites becomes:

```go
roles := spec.Narrate(phase, tCtx)
if roles.Actee != "" && u != nil { u.SendText(cat, roles.Actee) }      // holder
if roles.Observer != "" { room.SendTextVisual(cat, roles.Observer, excl) }
```

with today's exact category, channel and exclusion at every site, and no
closures. In particular: buff end room lines keep `sendBuffEndRoomText`; spell
wait and magic room lines keep `Room.SendText` on the audio channel (filed
below, not changed); the mob holder and mob caster sites render both roles and
deliver only the room line, as they do now. The three silent-start appliers
call `AuthoredStartLine` instead of reading the field.

Inlining the sends makes them visible to the root viewpoint guard for the
first time. Each newly surfaced site is registered with a verdict and a
reason: a buff phase has no separate actee because the holder is the reader;
a spell phase has no separate actee because the target is reached by the room
line or by the effect's own narration; a mob holder has no client.

## The net

### Goldens first

Three goldens under `internal/narration/testdata/stores/`, built from
PRE-migration code and committed before any production file changes:

- `buffs.golden`: for every dogmud buff, sorted by id, one row per non-empty
  line, keyed by the AUTHORED key: `buff|39|start_user_text => ...`. The
  `*_user_text` rows record what the holder is SENT, which for start and end is
  the notice (the authored line or the generic fallback), and the row exists
  only when that is non-empty. Silent-start buffs get an extra
  `buff|15|authored_start_line` row.
- `spells.golden`: `spell|charm|cast_room_text => ...` for every non-empty
  field, plus a `|notarget` row for the two lines that name `{target}`,
  rendered with an empty target, freezing the empty substitution.
- `quests.golden`: `quest|14|rewards|playermessage`, `quest|14|trigger0|action2|send_text`,
  nested sequence actions as `...|sequence|action0|room_text`. The pre-migration
  builder sends raw text where today's code does and substitutes where it does.

Stand-ins: source `<ansi fg="username">Aliceia</ansi>` / `Aliceia`, target
`<ansi fg="mobname">Targetticus</ansi>` / `Targetticus`. `setupRealStores`
additionally loads buffs, spells and quests.

After the migration the builders switch to the store doors and every one of
the ten goldens must be byte-identical. Keying rows by the authored key is what
lets a golden catch an Actee/Observer swap; re-recording under the core's
vocabulary would bake a swap in invisibly.

### Sabotage probes, each proven red before it is trusted

1. Swap Actee and Observer inside `buffs.Narration`: `buffs.golden` goes red on
   the row that changed.
2. Change `textutil.Narrate` to pass `DefaultPicker`: the root guard below goes
   red.

### Root guards

- `TestKindBStoresRenderOnlyThroughTextutilNarrate`: `narration.Render(` may
  appear only in the five store files that call it today plus `textutil`; the
  `textutil` call must pass `narration.FirstPicker`. A registry of callers,
  the same shape as slice A's.
- `TestStoreTextFieldsAreReadOnlyByTheirStore`: no production file outside
  `internal/buffs` reads the six buff field names, none outside
  `internal/spells` reads the six spell field names, and none outside
  `internal/quests` reads `Rewards.PlayerMessage` or `Rewards.RoomMessage`.
  Quest action fields are exempt because `ExecuteAction` legitimately tests
  them for dispatch; combat result types share the `RoomMessage` spelling,
  which is why the reward check is qualified.
- The viewpoint registry gains one entry per newly visible site.

### Gate

Full suite green (`internal/playtestrun` standalone if it reds under load),
gofmt, the boot check, and one playtest lane reusing 5a's lane A in lit room
5343: quest 76 `look disc`, chrysalis-glow self-cast, cleansing-wave area
purge. That run touches quest send_text and room_text, spell cast lines, and
buff start and end lines in one sitting, and the expected lines are recorded
verbatim from the 5a run, so the check is equality, not judgement.

## Out of scope, filed

- Delivery through `messaging.SendTrio` and name hiding for these stores: M4
  (send path) and M5 (`{source_plain}` in 17 buff room lines).
- Actee text for buffs, spells and quests, and a caster on `events.Buff`: M6.
- One token vocabulary: M4.
- Unknown-token validation upgraded from warn to fail for buffs and spells.
- The two shipped `wait_room_text` lines (and any future `magic_room_text`)
  still go out on the audio channel, unsighted observers included. A D1
  sibling; behaviour change, so not here.
- The dialogue store keeps `SubstituteTokens`; migrating it is item 7.

## Documentation

`context.md` for `narration` (FirstPicker; consumers), `textutil` (rewritten),
`buffs`, `spells`, `quests` and `questengine` (the interface change). Rows for
this spec and its plan in `docs/README.md`. No patch note: nothing a player
reads changes.
