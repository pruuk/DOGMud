# internal/messaging

Centralized player-facing-text pipeline. Every `Room.SendText` /
`Room.SendTextVisual` / `UserRecord.SendText` / `Actor.SendText` call
in the engine flows through this package's pipeline before reaching
the recipient's connection.

## Pipeline Stages

1. **Compose** — caller produces `(Category, text)`.
2. **Style normalize** — sentence-start caps, a/an agreement,
   duplicate-word collapse, sentence-end punctuation, ANSI canon for
   names. Per-Category skip table in `normalize.go`. The a/an stage
   looks through any run of `<ansi …>` open tags between the article
   and the noun (`a <ansi fg="itemname">Ivory Fan</ansi>` becomes
   `an …`), keeping the tags byte for byte. It tests the first letter,
   not the sound, so "a useful" becomes "an useful"; a known limitation.
3. **Sight gate** (visual channel only) — per-recipient: CanSeeClearly,
   CanSeeShapes, or skip-visual-deliver-audio. Consumes the chunk-6
   Perception FSM (see `internal/state/perception/`).
4. **Anonymize** (infrared-only path) — regex strips `username` /
   `mobname` / `petname` ANSI name tags, including suffixed mob tags such as
   `mobname-dup2`, then substitutes "a figure" + the `combat-anon` color alias.
5. **Apply category color tag** — `<ansi fg="<category-alias>">…</ansi>`.
6. **Wrap** at recipient's `UserRecord.LineWidth` (default 80, range
   40-240), ANSI-aware, but only for the narration categories
   `shouldWrap` admits (46 of the 62 `Category` values as of this
   writing, `CategoryLight` among them: lighting plan 3d's transition
   notices wrap exactly as `CategoryTimeOfDay` does). Pre-formatted
   output is excluded by category: mixed
   buckets that mix refusals or chat with tables, ASCII art or a
   banner (`System`, `Broadcast`, `Splash`, `SkillProgress`), the
   side-by-side minimap block (`RoomDescription`), categories that
   already wrap themselves at a hardcoded 80 (`Speech`, `Whisper`,
   `Shout`, `Emote`), system output owned by a later stage (`Error`,
   `Warning`), and categories with zero production senders
   (`GrappleHigh`, `Login`, `OOC`, `Toxin`). `shouldWrap` in
   `pipeline.go` is the authoritative list. `WrapAnsi` closes and
   reopens the whole stack of open ansi tags across a line break and
   measures width in runes, not bytes.
7. **Deliver** to the recipient's connection.

## Channels

| Channel  | Helper            | Sight-gated | Stages run            |
|----------|-------------------|-------------|-----------------------|
| Audio    | `SendText`        | no          | 1, 2, 5, 6, 7         |
| Visual   | `SendTextVisual`  | yes         | all 7                 |

Audio bypasses the sight gate and the anonymizer; visual runs the
full per-recipient pipeline.

## Public API

Types and constants:

- `Category`: enum of 62 text classes (combat hits, defense, grapple,
  submissions, specials, spells by school, social, system, environment,
  loot/equipment/condition/mutation/toxin; plus `CategoryCombatSummary` for
  the per-round compact tally emitted by the light-verbosity path, and
  `CategoryCombatBlindWarning` for the per-round "you can't see clearly"
  notice, M4d PR 2 Task 4, sent by `internal/hooks`'
  `flushBlindCombatNotices`, and for the once-per-fight glare notice, #319,
  sent by `flushGlareCombatNotices`). `CategoryCombatBlindWarning` is in
  `verbosity.go`'s `suppressibleAtLight` only (owner ruling, M4d PR 2
  followup): suppressible at Light, not at Medium, since at Medium the
  player still reads the swing prose the notice explains, while Light is
  a deliberate near-silence preference the notice should not override.
  `Category.String()` no longer spells the three defence names as local
  literals: `CategoryDodge` / `CategoryParry` / `CategoryBlock` return
  `string(combatvocab.DefenceDodge)` / `DefenceParry` / `DefenceBlock`, the
  same one-declaration constants messaging M4b-2 gave `internal/combat`,
  `internal/characters` and `internal/items` (see
  `internal/combatvocab/context.md`).
- `CategoryLight`: lighting plan 3d's transition notices (when a room's
  light crosses a band for a given observer), sent by
  `internal/lightnotice`. Appended AFTER `CategoryToxin` rather than grouped
  beside `CategoryTimeOfDay`, so no existing `Category` value shifts, but
  treated exactly as `CategoryTimeOfDay` everywhere that matters: it wraps
  (`shouldWrap`), skips the same normalisation stages (`normalize.go`'s
  skip table), and no verbosity tier suppresses it. Alias `light` in
  `ansi-aliases.yaml`, value 179.
- `Verbosity`, `ParseVerbosity`, `(Verbosity).Suppresses` — combat-text
  verbosity primitives in `verbosity.go`. The allowlists
  (`suppressibleAtMedium`, `suppressibleAtLight`) declare which
  categories each level may drop. Suppression is applied by the combat
  hooks (`internal/hooks/combat_verbosity.go`), not by this pipeline
  itself — the pipeline delivers whatever the hook passes through.
- `Channel` — `ChannelAudio`, `ChannelVisual`.
- `SightDecision` — `SightFull`, `SightShapes`, `SightNone`.
- `RenderInput` — bundles Category, Text, Channel, SightDecision,
  LineWidth for one recipient's pipeline pass.
- `RoomVisibility` — minimal interface (`LightLevel() int`)
  satisfied by `*rooms.Room`. Renamed from `GetVisibility() int` by the
  graded lighting arc's plan 1, task 4; `internal/rooms.Room.LightLevel()`
  reports the room's light on the graded -100 to 100 scale (see
  `internal/rooms/context.md`).
- `Line` — `{Text string; Cat Category}`, one audience's view of an
  event. Built with `Say(cat, text)`.
- `NoLine` — the zero `Line`. A viewpoint that deliberately has
  nothing to say. Spelled out so a considered silence is legible as
  one; the root guard requires it rather than an omitted field.
- `Trio` — `{Actor, Actee, Observer, RemoteObserver Line}`, one narrated
  event as its FOUR audiences see it. `RemoteObserver` is the second
  room's line: ranged combat narrates to both the attacker's room and
  the defender's room, and until M4d that second audience had no seat
  here at all — it travelled outside the pipeline. The root guard
  `TestEveryTrioLiteralNamesAllThreeRoles` (repo root,
  `messaging_surface_guard_test.go`) still requires only the original
  three (`Actor`/`Actee`/`Observer`) on every `messaging.Trio{}`
  literal; `RemoteObserver` is deliberately NOT added to it. A census
  on 2026-09-20 found 150 `Trio` literals across 31 non-test files —
  requiring the fourth field on all of them would leave 149 carrying
  an always-empty field forever, which teaches an author to paste it
  unread rather than reason about it. The narrower, PAIRED guard that
  replaces it is `TestRemoteRoomIsPairedWithRemoteObserver` (same
  file): within one function, setting `RemoteRoom` on an `Audience`
  literal without also setting `RemoteObserver` on a `Trio` literal in
  that function fails, because `SendTrio` silently delivers nothing to
  a `RemoteRoom` whose `Trio` never named `RemoteObserver` — the exact
  "wired the mechanism, dropped the narration" defect class the M1
  viewpoint audit found repeatedly. Proved capable of failing with a
  probe in `internal/combat/combat.go`; see the M4d Task 5 report.
  A fifth field, `ObserverSound`, is what an observer who sees nothing
  hears of an event heard as well as seen (a spell disruption, #242 owner
  ruling R4). It names nobody; `NoLine`, the usual case, keeps the event
  silent to such a reader.
- `SoundChantBreaksOff`, `SoundSpellSputtersOut` (`disruption_sounds.go`):
  the two sound lines every spell-disruption path shares, mob and player
  caster alike (broken concentration or interrupt; fizzle or falter).
- `Recipient` — minimal interface (`SendText(cat, text)`) satisfied by
  `*users.UserRecord` and by `actions.Actor`.
- `Broadcaster`: interface satisfied by `*rooms.Room`:
  `SendTextVisualHidingNames(cat, txt, names, excludeUserIds ...int)`,
  `SendTextUnsighted(cat, txt, excludeUserIds ...int)` and
  `ParticipantSight(userId int) SightDecision`.
- `Audience`: who is present for one event: `Actor`/`ActorId`/`ActorName`,
  `Actee`/`ActeeId`/`ActeeName`, `Room`, and `RemoteRoom`. Ids are passed
  rather than derived because `users.UserRecord.UserId` is a FIELD while
  `actions.Actor` exposes `GetUserId()`. The names are exactly as the lines
  print them; the root guard requires both on every `Trio` literal.
  `RemoteRoom` is the second room for a ranged event (the defender's room);
  nil sends nothing, which is every event except ranged combat. A nil
  recipient/broadcaster must be assigned as the interface, never as a
  typed-nil pointer — a `(*users.UserRecord)(nil)` stored here is a
  non-nil interface value and `SendTrio` would call through it and panic.
- `NoName`: the empty string, for a side of an Audience with nobody on it.

Functions:

- `RenderForRecipient(in RenderInput) string` — entry point; runs the
  full pipeline for one recipient. Empty return = "don't deliver".
- `SightThroughWindow(light, strength, reach int, blindBelow, dimBelow int) SightDecision`
  (`window.go`): the pure function behind `ParticipantSight`'s optics
  decision; see the window-model description under `ParticipantSight`
  below. No locks, no global state, no config read: its caller owns
  fetching `blindBelow`/`dimBelow` from `Balance`.
- `clampShift(strength int) int` (`window.go`): bounds an ability's window
  shift to `[0, windowShiftCap]`. Shared by `SightThroughWindow`,
  `BandThroughWindow` (added lighting plan 3d, `band.go`) and
  `ExitThroughWindow` so they can never clamp differently.
- `ExitThroughWindow(light, strength, exitsAbove int) bool` (`window.go`,
  lighting plan 5c): whether an observer sees THROUGH an exit.
  `exitsAbove` (`LightExitsAbove`) is the normal-eyes edge; night-vision
  strength moves it down exactly as it moves the blind and dim edges. Infra
  reach plays no part in this light test; heat has its own path,
  `SensesHeatThroughExit`. Shipped edge: 55 (lighting plan 6, owner ruling
  O5; the Go default stays 65).
- `SeesThroughExit(observer, room) bool` (`predicates.go`, lighting plan
  5c): `ParticipantSight` is not `SightNone` AND `ExitThroughWindow` at
  the room's light and the observer's strength. `look <direction>` and
  `scan` gate on it; it replaced a nightvision-FLAG waiver.
- `SensesHeatThroughExit(observer, here, next RoomVisibility) bool`
  (`predicates.go`, lighting plan 6, owner ruling O6): infravision through an
  exit. True when the observer has infra reach, some sight `here` (not
  `SightNone`, so a blinded observer senses nothing) and the `next` room's
  light is at or above minus the reach. A caller asks it only after
  `SeesThroughExit` refused, so it never upgrades a view the light grants,
  and renders shapes only: the roster's `UnseenFigure(SightShapes)` per
  occupant the roster would list (`actions.FiguresSensedIn`, the same
  `Character.Perceives` filter scan uses), never a name, the room's
  description or its items. `look <exit>` by heat omits the room's title
  too; `scan` keeps its existing direction label, `north (Town Square): a
  figure`, exactly as its lit-but-too-dark line already named the title.
  Every occupant counts as warm: the codebase has no cold-body concept, and
  own-room infravision shows everyone too.
- `FixedLight` (`predicates.go`, lighting plan 5c): an `int` that
  satisfies `RoomVisibility`. A caller judging many observers in one room
  reads `room.LightLevel()` once and passes `FixedLight`, as
  `actions.CalcSneakScoreVsObserver`'s callers do.
- `Band` (`band.go`), added lighting plan 3d: one step finer than
  `SightDecision`, splitting full sight into reading faces and being
  dazzled. `BandDark`, `BandShapes`, `BandFaces`, `BandDazzled`, ordered
  DARKEST TO BRIGHTEST (the opposite of `SightDecision`'s best-to-worst
  order), because the one consumer (`internal/lightnotice`) asks "did it
  get darker?" and an ordered comparison should read that way. `Band`
  itself still carries no score penalty and is narration-only; since
  lighting plan 5b the actual dazzle cost is priced independently by
  `ComfortDistance`/`SightScoreMultiplier` below, which do not consult
  `Band` at all.
  - `BandThroughWindow(light, strength, reach, blindBelow, dimBelow,
    dazzleAbove int) Band` is `SightThroughWindow` with the full tier
    split at the observer's shifted dazzle edge (`dazzleAbove` minus the
    clamped `strength`). It never moves a lower edge: dark, shapes and
    faces-or-dazzled are still `SightThroughWindow`'s answers.
    `dazzleAbove` is the caller's config knob (`Balance.LightDazzleAbove`
    via `Lighting.DazzleAbove`), a plan 5b knob, not a package constant;
    the unexported `windowDazzleEdge` constant this used to read is GONE,
    replaced by this parameter.
- `LightTrimTarget(strength, dazzleAbove int) float64` (`window.go`),
  added lighting plan 5a and reparameterized in 5b: the brightest room
  light an observer with this night-vision strength reads without being
  dazzled, one point under the shifted dazzle edge
  (`dazzleAbove - clampShift(strength) - 1`). An adjustable light
  (`internal/rooms.(*Room).TrimLightFor`) trims toward it. One point, not
  half, so a room at 74.5 cannot round up onto the edge.
- `DarknessTrimTarget(strength, reach, blindBelow int) float64`
  (`window.go`), lighting plan 5d: the room light an adjustable darkness
  trims to, one point inside the darkest light an observer can still use.
  `-reach + 1` with infravision, else `blindBelow - clampShift(strength)`
  floored at `windowFloor`, plus one. One point inside, mirroring
  `LightTrimTarget`, so a small downward drift in sky light after the trim
  does not tip the bearer into the dark (the 5d playtest).
- `ComfortDistance(observer *characters.Character, room RoomVisibility)
  (dark, bright float64)` (`comfort.go`), lighting plan 5b: how far the
  room's light sits outside the observer's own comfortable band, as two
  fractions of the way to the cap. `dark` is nonzero below the dim edge (1
  at the blind edge); `bright` is nonzero above the dazzle edge (0 at the
  edge itself, 1 one ramp-width beyond it). At most one is non-zero. A nil
  observer or room reads comfortable (0, 0); a `Blinded` observer reads
  fully dark (1, 0). The bright ramp is exactly as wide as the dark one, so
  a strong window is punished by excess light as fast as it is helped by
  faint light. **Since lighting plan 5c**, `ComfortDistance` also calls the
  unexported `infraDarkCap(light, reach, floor, reachCap, darkCap)`
  (`comfort.go`) and takes the lesser of the natural `dark` and its result:
  infravision's sight multiplier (`Lighting.InfraPenaltyFloor` at the first
  point of reach, rising linearly to 1.0 at `Lighting.InfraReachCap`)
  expressed as a dark fraction against `Lighting.DarkCap`
  (`Balance.DarknessCombatPenalty`), so `SightScoreMultiplier` reads exactly
  that multiplier with no change to it or to any caller. `infraDarkCap`
  never touches `bright`: glare costs an infravision creature in full
  (owner ruling 2026-09-28), and it returns `ok == false` (no cap applied)
  when reach is zero, the light sits below minus the reach, or
  `Lighting.DarkCap` is `>= 1` (no dark penalty exists to ease).
- `SightScoreMultiplier(dark, bright float64, bal configs.Balance)
  float64` (`sight_mult.go`), lighting plan 5b: the ONE place the sight
  ramp becomes a number. Runs from 1.0 at the edge of the comfortable band
  down to `Balance.DarknessCombatPenalty` at the blind edge and to
  `Balance.DazzleCap` one ramp-width above the dazzle edge; never returns
  below 0. It replaced the flat three-verdict penalty (`DarknessScoreMultiplier`,
  now deleted, and the knob it read, `DarknessShapesCombatPenalty`, also
  retired). It prices every opposed or difficulty roll, not only combat;
  the voice contests (taunt, demoralize, rally, warcry, defy) are exempt by
  never calling it.
- `SightMult(c *characters.Character, room RoomVisibility) float64`
  (`sight_mult.go`), lighting plan 5b: the sight ramp for one roller in one
  room, composing `ComfortDistance` and `SightScoreMultiplier` against
  `configs.GetBalanceConfig()` so no call site can pair the two
  differently. Apply it to the score of whoever needs to SEE for the roll
  (the observer in a detection roll, the thief in a theft, the actor in a
  search, track, defuse, forage, craft or concentration roll).
  - `LightBand(observer *characters.Character, room RoomVisibility) Band`
    is `ParticipantSight`'s band-grained twin: optics only, does not
    consult sleep, a `Blinded` observer reads `BandDark`, a nil observer
    or room reads `BandFaces`. Reads the narrow `configs.GetLightingConfig()`
    rather than the 400-field `Balance` copy `ParticipantSight` takes; both
    carry the same two edges.
- `ParticipantSight(observer *characters.Character, room RoomVisibility) SightDecision`
  is THE optics primitive, added M4d (`01bbee127`). It answers what an
  observer can make out and nothing else — blindness, room light,
  NightVision, InfraredVision — and deliberately does NOT consult sleep,
  because sleep is an attention property, not an optical one: a sleeping
  character's eyes work, they are simply not reading. A nil observer sees
  fully.

  Since the graded lighting arc's plan 1, sight is decided against two
  `Balance` config thresholds: `LightBlindBelow` (default 25) and
  `LightDimBelow` (default 50). Plan 2 replaced the band switch plus flag
  shortcuts that used to sit here with a WINDOW model. `ParticipantSight`
  now hands `room.LightLevel()`, the observer's two vision numbers and
  those two thresholds straight to `SightThroughWindow` (the pure function
  in `internal/messaging/window.go`) and returns whatever it decides; there
  is one decision now, not a band switch followed by a flag fallback.

  `SightThroughWindow(light, strength, reach int, blindBelow, dimBelow
  int) SightDecision` reads: `strength`
  (`Character.NightVisionStrength()`, `internal/characters/vision.go`)
  shifts BOTH `blindBelow` and `dimBelow` down by that many points,
  capped at the unexported `windowShiftCap` (24) and floored at zero, so
  an ability trades bright-light comfort for dark-light acuity rather
  than simply granting sight: the same shift that lets its holder read a
  dim room by candlelight does nothing extra in a pitch dark one, because
  the shifted window still has a floor. That floor is the unexported
  `windowFloor` (1), which bounds the SHIFTED window only, not `reach`.
  **Since lighting plan 5c** (the owner's ruling on 5b call 3), `reach`
  (`Character.InfraReach()`, the separate heat-sensing number) no longer
  consults `windowFloor` at all: heat-sensing reads SHAPES, never faces, at
  any light down to minus `reach`, wherever the window itself reads worse.
  A faint room (light 2 to 12, dark to a nightvision-only observer under
  the old gate) now reads shapes to a reach-bearing observer by
  construction; this closed 5c's opening bug. `windowFloor` still binds an
  operator's never-blind escape hatch (`LightBlindBelow` set below 1): no
  ability shift can push the blind edge under it, pinned by
  `TestWindowFloorHoldsForANeverBlindConfig`. The dazzle edge,
  `LightDazzleAbove` (default 75),
  marks where the perfect band ends and too-bright begins. It lived as the
  unexported constant `windowDazzleEdge` from plan 1 through plan 5a; plan
  5b retired that constant and made the edge a `Balance` knob instead, once
  it had something for an operator to retune. `SightThroughWindow` itself
  still never consults it: `SightDecision` has no dazzled value, so a
  return of `SightFull` at or above the edge carries no penalty from
  `SightThroughWindow` alone. Lighting plan 3d's `BandThroughWindow`
  (`band.go`, see above) reads it, as the `dazzleAbove` parameter now
  rather than the old constant, to compute `Band`, so a light-crossing
  notice can tell a player the light stabs at their eyes, even though
  `Band` itself still changes no `SightDecision`. Lighting plan 5a added
  `LightTrimTarget` (`window.go`) as a second reader, to set the level an
  adjustable light trims to. **Since lighting plan 5b**, the edge finally
  prices something directly: `ComfortDistance` (`comfort.go`) reads it (as
  `cfg.DazzleAbove`), independently of `Band`/`SightDecision`, to measure
  how far past the edge a room's light sits. `SightScoreMultiplier`
  (`sight_mult.go`) does NOT read the edge itself; it takes
  `ComfortDistance`'s fractions and reads only the two ramp caps,
  `Balance.DarknessCombatPenalty` and `Balance.DazzleCap`, to turn that
  distance into a score penalty on every opposed or difficulty roll; see
  the `ComfortDistance`/`SightScoreMultiplier`/`SightMult` entries above.

  🔴 **Structural fact worth knowing before reading a bug into it:** with
  `windowShiftCap` at 24 and `LightDimBelow` at 50, no ability can shift
  `dimBelow` down to or below light 0 (50 − 24 = 26, still positive), so
  nobody sees fully in pitch darkness at any strength. A test that needs
  `SightFull` at light 0 has to override config to reach an otherwise
  unreachable scenario (see `internal/mobs`'s heat-sensing guard test,
  which raises `InfraReach` instead of asking for full sight). See
  `internal/mutations/context.md` for how a rank-4 vision mutation is
  authored to land exactly at `windowShiftCap` and no further.

  One semantic worth recording so the next reader does not mistake it for
  a bug: a NightVision holder does NOT automatically read `SightFull` just
  by holding the ability. Whether a given room's light clears the
  SHIFTED `dimBelow` is arithmetic, not a flag check, so a holder in a
  room whose light sits between the shifted `blindBelow` and the shifted
  `dimBelow` reads `SightShapes`, same as an unaided observer would in an
  ordinary dim room. `Room.LightLevel()` only ever returns 0, 60 or 70
  today, and both nonzero values clear every possible shifted `dimBelow`
  (60 and 70 both exceed 50, and shifting only lowers the edge further),
  so this in-between case cannot occur in the shipped game yet; the
  graded lighting arc's plan 3, which makes the scale continuous, makes
  it reachable.
- `CanSeeClearly`, `CanSeeShapes`, `CanSeeSightImpairedOnly` — each is now a
  ONE-LINE POLICY over `ParticipantSight` that composes its own attention
  rule, not three independently-implemented predicates:
  - `CanSeeClearly(observer, room) bool` = awake AND `ParticipantSight ==
    SightFull`. Read by the room-broadcast sight gate (visual channel).
    Sleep-gated: a sleeping player stops receiving visual room lines.
  - `CanSeeShapes(observer, room) bool` = awake AND (`ParticipantSight ==
    SightFull` OR `== SightShapes`). Also sleep-gated: closed eyes see no
    shapes either.
  - `CanSeeSightImpairedOnly(observer, room) bool` = `ParticipantSight ==
    SightFull`, WITHOUT the sleep gate. This is the one `internal/combat`
    used to read (as `CanSeeClearly`, before M4d) to drive
    `Balance.DarknessCombatPenalty`; adding the sleep gate to
    `CanSeeClearly` on 2026-08-31 would otherwise have applied a phantom
    darkness penalty to a sleeping defender standing in a LIT room, and
    corrupted `combat-analytics.jsonl`'s contest telemetry with a term
    nobody asked for. M4d closed that gap for good: combat no longer reads
    a messaging predicate by name at all (see `internal/combat/context.md`,
    "Sight: the verdict and the ramp").
  - Its non-test readers are the round's combat sight gates in
    `internal/hooks` (`NewRound_DoCombat_resolution.go`,
    `NewRound_DoCombat_unified.go`), which gate
    `Balance.DarknessCombatPenalty`. `internal/behaviortree`'s `mobCanSee`
    stopped reading it in lighting plan 5d (ruling D8): mob decisions accept
    `SightShapes` from `ParticipantSight` directly, and this predicate stays
    `SightFull` only so the combat penalty does not move.
  - `sleep_policy_test.go` pins the contract by absence: a sleeper reads
    NOTHING from `CanSeeClearly`/`CanSeeShapes` (both false regardless of
    light), and `CanSeeSightImpairedOnly` ignores sleep entirely.
    `optics_pin_test.go` pins all three against an eight-row truth table
    (light x blind x infrared x asleep).
- `HideNames(text string, names []string, d SightDecision) string`: replaces
  each name with "a figure" (shapes) or "something" (none), longest name first,
  capitalized at a sentence start. In bare prose the match is exact and
  whole-word, because mob names collide with ordinary words ("guard"). Inside an
  identity tag it ignores case and any duplicate index, and the whole tag is
  replaced: one Audience name then covers both the authored form ("skeleton")
  and the display form the channel defence triad prints ("Skeleton #2"). When
  the whole identity tag is hidden, one directly following adjective span
  (` <ansi fg="black-bold">(...)</ansi>`, as `FormattedName.String` prints it,
  with the adjective colour-patterned rune by rune into nested tags) goes with
  it, so a caller may pass formatted names through the seam without leaking
  "(dead)" or "(♥friend)". A match inside tag markup itself is never
  replaced.
- `UnseenNoun(d SightDecision) string`: the word `HideNames` substitutes, "a
  figure" at shapes and "something" otherwise. `UnseenFigure(d)` is the same
  word in the `combat-anon` tag, for a list entry that stands alone; the room
  roster (`rooms.GetDetails`), `scan` and `search`'s found-hider list use it.
- `NameHider` (sight gates slice 5b): the shape `HideNames` and
  `HideSpeakerNames` share (`func(text string, names []string, d
  SightDecision) string`), so a room sender that hides names on the audio
  channel can take either interchangeably: a sound
  (`messaging.HideNames`, "Something lets out a roar!") or a speaker
  (`messaging.HideSpeakerNames`, "Someone says, ...").
- `HideSpeakerNames(text string, names []string, d SightDecision) string`
  (sight gates slice 5b): hides a SPEAKER's name in a speech line, never the
  spoken words. Only a name standing as a whole identity tag is replaced,
  capitalised at a sentence start, as "a figure" at shapes or "someone"
  otherwise (a voice belongs to a person, so the unseen word differs from
  `HideNames`' "something", owner ruling 3); a bare mention inside the
  quoted speech itself is untouched ("I am Kesh" stays "I am Kesh"), and a
  player's words cannot forge an identity tag because the wrappers escape
  them first (`util.EscapeAnsiTags`). Clear sight returns `text` unchanged.
- `SpeakerNoun(d SightDecision) string`: the word `HideSpeakerNames`
  substitutes, "a figure" at shapes and "someone" otherwise; exported for
  GMCP say's `sender` (#252).
- `StripNameAdjectives(text string) string` (#415): drops the adjective span
  ("(dead)", "(♥friend)") after every identity tag, keeping the tag, for a
  narrated line that names a creature rather than lists it.
  `combat.RenderChannelDefenceMessages` applies it to both identities; the
  melee and counter paths build their names without adjectives instead
  (`combat.meleeIdentityTag`). The condition start, trigger and end lines
  for a mob holder apply it to `mobDisplayName` (hooks, #453); a player
  holder's `GetCharacterName(true)` carries no span to begin with.
- `Normalize(cat Category, text string) string`
- `Anonymize(text string) string`: the pipeline's infrared fallback for every
  visual line. Replaces each identity tag with "a figure" and takes the
  adjective span behind it (same pattern as `HideNames`), because
  `rooms.go` anonymizes BEFORE it hides names and the span would otherwise
  survive as "a figure (dead)".
- `HideWeapons(text string, d SightDecision) string`
  (`hideweapons.go`, owner ruling R8, 2026-10-08): below `SightFull` every
  `fg="item"` or `fg="itemname"` tag in a combat line becomes `WeaponWord`
  ("weapon"), nested display-name tags included, with "an" before it turned
  to "a". It keeps a natural weapon ("fists", any species' `UnarmedName`),
  and nothing else (since #446 `combat.hideIdentitiesInPersonalLines`
  skips the attacker's own lines instead). The pipeline runs it at
  `SightShapes` for `isCombatNarration` categories only, so every spectator
  combat line is covered and a non-combat item line is not;
  `combat.hideIdentitiesInPersonalLines` runs it on the defender's personal
  lines.
  Untagged item names pass through, which is why the combat templates tag
  every weapon token. The converse holds too: a non-item must not wear the
  item tag. A move name in `{attack}` renders untagged through
  `items.RenderMoveDefenseMessage`, and `{bodypart}` is never tagged.
- `WrapAnsi(text string, maxWidth int) string`
- `Say(cat Category, text string) Line`
- `SendTrio(t Trio, aud Audience)`: delivers one narrated event to
  everyone entitled to it — FOUR roles since M4d (`44cc90ceb`): Actor,
  Actee, Observer, and RemoteObserver. A line goes out only if it has BOTH
  text and a recipient. The room broadcast (`Observer`) and the remote-room
  broadcast (`RemoteObserver`, delivered to `aud.RemoteRoom` when set) ALWAYS
  exclude the actor and the actee. Each role is rendered for its reader: the
  actor's line hides `ActeeName` and the actee's hides `ActorName` by that
  reader's `ParticipantSight`; the observer and remote-observer lines hide
  both, judged per-observer by their own room's `ParticipantSight`. An
  `ObserverSound` goes to `aud.Room.SendTextUnsighted` with the same
  exclusions, reaching only the observers who see nothing.

## Two jobs, not one

This package now does two things, and the second is not the first.

1. **The seven-stage pipeline, per recipient.** `RenderForRecipient`.
2. **Fan-out of one event to its audiences (three, or four for a ranged
   event with a remote room).** `SendTrio`.

The fan-out lives here because the import graph rules out both
alternatives. `internal/narration` cannot import `messaging` (`items`
imports `narration`, and `messaging` reaches `items` through
`characters`), and `internal/actions` cannot be imported by
`questengine`, which has to reach the seam in M3. `messaging` hosts it
with **zero new imports**, using the same minimal-interface trick
`predicates.go:11` documents.

⚠️ **The category rides on the `Line`, not on the `Trio`, and that is
load-bearing.** The three roles of one event routinely differ: `shoot`
uses four categories on the personal side and `throw` uses three on
each side (hit, dodge, interrupt). A single-`Category` seam would
silently recategorise them. Category decides the line's colour and
whether it wraps at 80 (`CategorySystem` never wraps, which is why #449
moved the special-move actor and actee lines off it onto the move's own
category; only refusals and cost notices stay on `CategorySystem`).
Verbosity suppression is NOT driven by a Trio line's category: it applies
only to the combat-round drains in `internal/hooks/combat_verbosity.go`.

## Import-direction discipline

`messaging` imports `internal/characters`, `internal/conditions`,
`internal/state/perception` directly (sight predicates need
Perception FSM state and the NightVision / InfraredVision condition
flags). Everything else — `rooms`, `users`, `mobs`, `combat`,
`hooks` etc. — is consumed via narrow interfaces (`RoomVisibility`,
`Recipient`, `Broadcaster`) so the dependency arrow stays one-way:

- Many packages import `messaging` (combat, hooks, rooms, users,
  actions, behaviortree, questengine, modules, world.go, …).
- `messaging` imports characters/conditions/state/perception, plus
  `internal/combatvocab` (added M4b-2, `d4e5ab20d`) for the three
  defence-name constants `Category.String()` returns. `combatvocab`
  imports nothing but the standard library, so this adds no cycle risk.
- `messaging` also imports `internal/configs` (added by the graded
  lighting arc's plan 1, task 4), so `ParticipantSight` can read the
  `LightBlindBelow` / `LightDimBelow` band thresholds off
  `configs.GetBalanceConfig()`. `configs` imports nothing from
  `messaging`, so this adds no cycle risk either.
- `messaging` also imports `internal/species` (added by spec F2,
  `hideweapons.go`), so `HideWeapons` can keep every species'
  `UnarmedName` as a natural weapon. `species` imports `configs`,
  `fileloader`, `items`, `mudlog`, `stats` and `util`, none of which
  imports `messaging`, so this adds no cycle risk.
- Nothing in `characters` imports `messaging` (would close a cycle).

> **Corrected 2026-09-08.** This file previously documented four
> symbols that do not exist and never did in this package:
> `UserSender`, `ProgressionKind`, `TierChange`, `FormatProgression`
> and `SendProgression`, the last two described as living in a
> `progression.go` the package does not contain. A comment in
> `internal/banner/banner.go` pointed at that same missing file.
> Verify a `context.md` against `Select-String -Path
> internal\<pkg>\*.go -Pattern '^(func|type|const|var)\s'` before
> trusting it; a file describing an invented API is worse than none,
> because someone codes against it.

## Adding a new Category

1. Add a constant to the enum in `messaging.go`. Append at the end of
   its section.
2. Add the matching string in `Category.String()`.
3. Add the color alias in `_datafiles/world/dogmud/ansi-aliases.yaml`
   named `<category-name>` where `<category-name>` is the string the
   enum returns.
4. If the new Category needs style-normalization skips, edit the
   `normalize.go` skip table.

## See Also

- `docs/superpowers/specs/completed/2026-05-19-messaging-framework-design.md` —
  full design spec.
- `internal/state/perception/context.md` — the FSM whose state the
  sight gate reads (shipped dormant in chunk 6; this chunk is the
  consumer).
- `_datafiles/world/dogmud/ansi-aliases.yaml` — color aliases.

## Files

The package is the pipeline, one stage per file, plus the fan-out (`trio.go`):

| File | Stage |
|------|-------|
| `messaging.go` | Entry points and the `Category` vocabulary |
| `pipeline.go` | Stage ordering: compose, normalize, sight gate, anonymize, color, wrap, deliver |
| `normalize.go` | Grammar and article normalisation |
| `anonymize.go` | Replacing names the observer should not see (infrared fallback, whole-line) |
| `hidenames.go` | `HideNames`, `NameHider`, `HideSpeakerNames` (sight gates slice 5b): replacing specific names in bare prose, longest-first, whole-word |
| `hidenames_tagged.go` | Identity-tag-aware name replacement `HideNames` and `Anonymize` share, including the trailing adjective span |
| `wrap.go` | `WrapAnsi`, ANSI-aware folding at a caller-supplied width measured in visible runes; called by the pipeline for the categories `shouldWrap` admits, and directly by `motd.go` for its box-bordered banner |
| `predicates.go` | `ParticipantSight` (the optics primitive) plus `CanSeeClearly`/`CanSeeShapes`/`CanSeeSightImpairedOnly`, the one-line attention policies built on it; `SeesThroughExit` and `FixedLight` (lighting plan 5c); `SensesHeatThroughExit` (lighting plan 6) |
| `window.go` | `SightThroughWindow`, the pure window-model function `ParticipantSight` calls, `clampShift`, `ExitThroughWindow` (lighting plan 5c), `LightTrimTarget` (lighting plan 5a, reparameterized in 5b to take `dazzleAbove` instead of reading the now-retired `windowDazzleEdge` constant), `DarknessTrimTarget` (lighting plan 5d), plus its two remaining unexported constants (`windowShiftCap`, `windowFloor`) |
| `band.go` | `Band`, `BandThroughWindow`, `LightBand` (lighting plan 3d): the band-grained twin of `SightDecision`/`SightThroughWindow`/`ParticipantSight`, adding the dazzled tier for `internal/lightnotice` |
| `comfort.go` | `ComfortDistance` (lighting plan 5b): how far a room's light sits outside the observer's comfortable band, as dark/bright fractions of the way to the cap; `infraDarkCap` (lighting plan 5c), the unexported cap it applies to the dark fraction for an observer with infra reach |
| `sight_mult.go` | `SightScoreMultiplier` and `SightMult` (lighting plan 5b): the sight ramp as a score multiplier, replacing the deleted `DarknessScoreMultiplier` |
| `verbosity.go` | Per-player verbosity filtering |
| `trio.go` | `Line`/`Trio`/`Audience`/`SendTrio` — fan-out of one narrated event to its four audiences |
| `disruption_sounds.go` | `SoundChantBreaksOff`, `SoundSpellSputtersOut`: the shared sound lines of a spell disruption (#242, owner ruling R4) |

Adding a transformation means adding a stage here, not special-casing at a call
site — that centralisation is the point of the package.
