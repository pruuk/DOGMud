# internal/messaging

Centralized player-facing-text pipeline. Every `Room.SendText` /
`Room.SendTextVisual` / `UserRecord.SendText` / `Actor.SendText` call
in the engine flows through this package's pipeline before reaching
the recipient's connection.

## Pipeline Stages

1. **Compose** — caller produces `(Category, text)`.
2. **Style normalize** — sentence-start caps, a/an agreement,
   duplicate-word collapse, sentence-end punctuation, ANSI canon for
   names. Per-Category skip table in `normalize.go`.
3. **Sight gate** (visual channel only) — per-recipient: CanSeeClearly,
   CanSeeShapes, or skip-visual-deliver-audio. Consumes the chunk-6
   Perception FSM (see `internal/state/perception/`).
4. **Anonymize** (infrared-only path) — regex strips `username` /
   `mobname` / `petname` ANSI name tags, including suffixed mob tags such as
   `mobname-dup2`, then substitutes "a figure" + the `combat-anon` color alias.
5. **Apply category color tag** — `<ansi fg="<category-alias>">…</ansi>`.
6. **Wrap** at recipient's `UserRecord.LineWidth` (default 80, range
   40–240), ANSI-aware.
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

- `Category` — enum of 59 text classes (combat hits, defense, grapple,
  submissions, specials, spells by school, social, system, environment,
  loot/equipment/buff/mutation/toxin; plus `CategoryCombatSummary` for
  the per-round compact tally emitted by the light-verbosity path).
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
- `RoomVisibility` — minimal interface (`GetVisibility() int`)
  satisfied by `*rooms.Room`.
- `Line` — `{Text string; Cat Category}`, one audience's view of an
  event. Built with `Say(cat, text)`.
- `NoLine` — the zero `Line`. A viewpoint that deliberately has
  nothing to say. Spelled out so a considered silence is legible as
  one; the root guard requires it rather than an omitted field.
- `Trio` — `{Actor, Actee, Observer Line}`, one narrated event as its
  three audiences see it.
- `Recipient` — minimal interface (`SendText(cat, text)`) satisfied by
  `*users.UserRecord` and by `actions.Actor`.
- `Broadcaster`: interface satisfied by `*rooms.Room`:
  `SendTextVisualHidingNames(cat, txt, names, excludeUserIds ...int)` and
  `ParticipantSight(userId int) SightDecision`.
- `Audience`: who is present for one event: `Actor`/`ActorId`/`ActorName`,
  `Actee`/`ActeeId`/`ActeeName`, `Room`. Ids are passed rather than derived
  because `users.UserRecord.UserId` is a FIELD while `actions.Actor` exposes
  `GetUserId()`. The names are exactly as the lines print them; the root guard
  requires both on every literal.
- `NoName`: the empty string, for a side of an Audience with nobody on it.

Functions:

- `RenderForRecipient(in RenderInput) string` — entry point; runs the
  full pipeline for one recipient. Empty return = "don't deliver".
- `CanSeeClearly(observer *characters.Character, room RoomVisibility) bool`
- `CanSeeShapes(observer *characters.Character, room RoomVisibility) bool`
- `ParticipantSight(observer *characters.Character, room RoomVisibility) SightDecision`
  is what a party to an event makes out of the other party. Darkness and
  blindness decide it; sleep does not (a sleeper struck in a lit room is told
  what hit them). Infrared gives `SightShapes`.
- `HideNames(text string, names []string, d SightDecision) string`: replaces
  each name with "a figure" (shapes) or "something" (none), longest name first,
  capitalized at a sentence start. In bare prose the match is exact and
  whole-word, because mob names collide with ordinary words ("guard"). Inside an
  identity tag it ignores case and any duplicate index, and the whole tag is
  replaced: one Audience name then covers both the authored form ("skeleton")
  and the display form the channel defence triad prints ("Skeleton #2"). A match
  inside tag markup itself is never replaced.
- `Normalize(cat Category, text string) string`
- `Anonymize(text string) string`
- `WrapAnsi(text string, maxWidth int) string`
- `Say(cat Category, text string) Line`
- `SendTrio(t Trio, aud Audience)`: delivers one narrated event to
  everyone entitled to it. A line goes out only if it has BOTH text
  and a recipient. The room broadcast ALWAYS excludes the actor and the
  actee. Each role is rendered for its reader: the actor's line hides
  `ActeeName` and the actee's hides `ActorName` by that reader's
  `ParticipantSight`; the observer line hides both for shapes-only observers.

## Two jobs, not one

This package now does two things, and the second is not the first.

1. **The seven-stage pipeline, per recipient.** `RenderForRecipient`.
2. **Fan-out of one event to three audiences.** `SendTrio`.

The fan-out lives here because the import graph rules out both
alternatives. `internal/narration` cannot import `messaging` (`items`
imports `narration`, and `messaging` reaches `items` through
`characters`), and `internal/actions` cannot be imported by
`questengine`, which has to reach the seam in M3. `messaging` hosts it
with **zero new imports**, using the same minimal-interface trick
`predicates.go:11` documents.

⚠️ **The category rides on the `Line`, not on the `Trio`, and that is
load-bearing.** The three roles of one event routinely differ: ten of
twelve player-side special-move verbs send personal lines as
`CategorySystem` and the room line as something verb-specific, `shoot`
uses four categories on the personal side, and `throw` uses three on
each side. A single-`Category` seam would silently recategorise
them, and category decides both the line's colour and whether a
light-verbosity player sees it at all.

## Import-direction discipline

`messaging` imports `internal/characters`, `internal/buffs`,
`internal/state/perception` directly (sight predicates need
Perception FSM state and the NightVision / InfraredVision buff
flags). Everything else — `rooms`, `users`, `mobs`, `combat`,
`hooks` etc. — is consumed via narrow interfaces (`RoomVisibility`,
`Recipient`, `Broadcaster`) so the dependency arrow stays one-way:

- Many packages import `messaging` (combat, hooks, rooms, users,
  actions, behaviortree, questengine, modules, world.go, …).
- `messaging` imports characters/buffs/state/perception ONLY.
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

- `docs/superpowers/specs/2026-05-19-messaging-framework-design.md` —
  full design spec.
- `internal/state/perception/context.md` — the FSM whose state the
  sight gate reads (shipped dormant in chunk 6; this chunk is the
  consumer).
- `_datafiles/world/dogmud/ansi-aliases.yaml` — color aliases.

## Files

The package is the pipeline, one stage per file:

| File | Stage |
|------|-------|
| `messaging.go` | Entry points and the `Category` vocabulary |
| `pipeline.go` | Stage ordering — compose → normalize → anonymize → color → wrap → deliver |
| `normalize.go` | Grammar and article normalisation |
| `anonymize.go` | Replacing names the observer should not see |
| `wrap.go` | 80-column wrapping (uses visible width, not byte length) |
| `predicates.go` | Who should receive a message |
| `verbosity.go` | Per-player verbosity filtering |

Adding a transformation means adding a stage here, not special-casing at a call
site — that centralisation is the point of the package.
