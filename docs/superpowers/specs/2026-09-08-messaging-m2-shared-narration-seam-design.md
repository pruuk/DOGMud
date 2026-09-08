# Messaging M2 — The Shared Narration Seam — Design

**Created:** 2026-09-08
**Status:** Design approved by owner 2026-09-08. No plan written yet.
**Parent arc:** [Messaging Unification Arc](2026-08-31-messaging-unification-design.md),
slice M2. Predecessor slices M0, M0b and M1 are merged.
**Inputs:** [M1 viewpoint audit](../audits/2026-09-07-narration-viewpoint-audit.md),
[M1 plan](../plans/2026-09-07-messaging-m1-viewpoint-audit-and-harness.md).

---

## Facts verified against source, 2026-09-08

Read from the tree while writing this document. Re-grep before quoting any of
it in a later slice.

### The import graph, which decides where the seam can live

| Fact | Evidence |
|---|---|
| `items` imports `narration` | `internal/items/defensive_messages.go:7` |
| `messaging` imports `characters` | `internal/messaging/predicates.go:5` |
| `characters` imports `items` | `internal/characters/character.go:12` (also `buffs.go`, `combat.go`) |
| **Therefore `narration` can never import `messaging`** | Would close the cycle `items -> narration -> messaging -> characters -> items` |
| `actions` imports `questengine` | `internal/actions/buy.go:15`, `internal/actions/plant.go:18` |
| `questengine` imports `messaging`, `rooms`, `users`, and not `actions` | Package import set of `internal/questengine/*.go` |
| **Therefore `questengine` can never import `actions`** | Rules out `internal/actions` as the seam's home |
| `messaging` imports only `buffs`, `characters`, `state`, `state/perception` | Package import set of `internal/messaging/*.go` |

### The types the seam has to accept

| Fact | Evidence |
|---|---|
| `UserRecord.UserId` is a **field**, not a method | `internal/users/userrecord.go:35` |
| `actions.Actor.GetUserId()` is a **method** | `internal/actions/actor.go:14` |
| No single interface can read the id from both, so ids are passed explicitly | Follows from the two rows above |
| `func (u *UserRecord) SendText(cat messaging.Category, txt string)` | `internal/users/userrecord.go:435` |
| `actions.Actor.SendText(cat messaging.Category, msg string)`, documented as a no-op for mobs | `internal/actions/actor.go:14` |
| `func (r *Room) SendTextVisual(cat messaging.Category, txt string, excludeUserIds ...int)` | `internal/rooms/rooms.go:307` |
| `messaging.RoomVisibility` already uses the minimal-interface trick to avoid importing `rooms` | `internal/messaging/predicates.go:13`, with the reason in the comment at `:11` |

### The duplication M2 removes

| Fact | Evidence |
|---|---|
| `sendMoveDefenceTriad` exists once per command package, near-identical | `internal/usercommands/skill_move_defence.go:32`, `internal/mobcommands/skill_move_defence.go:30` |
| 12 special-move verb files in `usercommands` | `bash drain gore kick maul pounce rake throttle trip grapple shoot throw` |
| 11 in `mobcommands`, the same list without `throw` | `ls internal/mobcommands/` |
| Every one of the 12 carries the three-send shape | Per-file send counts, table below |
| `throw` has zero actee sends and is correct that way, being an area effect | 17 actor sends, 0 actee, 4 room |
| The render half of the seam already exists and stays put until M3 | `items.DefenseOptions.RenderTriad`, `internal/items/defensive_messages.go` |
| `internal/hooks` broadcasts through a wrapper, not `room.SendText*` directly | `sendVisualRoomText`, `internal/hooks/NewRound_DoCombat_helpers.go:401` |
| **The three roles of one event do not share a `Category`.** 10 of 12 verbs send personal lines as `CategorySystem` and the room line as something verb-specific; `shoot` uses four personal categories and `throw` uses three on each side | Category table in section 1, counted 2026-09-08 |

Send counts, `internal/usercommands/`, counted 2026-09-08:

| verb | actor | actee | room |
|---|---|---|---|
| bash | 9 | 4 | 4 |
| drain | 9 | 8 | 3 |
| gore | 8 | 10 | 4 |
| kick | 8 | 10 | 4 |
| maul | 7 | 8 | 3 |
| pounce | 9 | 10 | 4 |
| rake | 7 | 8 | 3 |
| throttle | 8 | 10 | 3 |
| trip | 13 | 18 | 8 |
| grapple | 16 | 10 | 4 |
| shoot | 26 | 5 | 6 |
| throw | 17 | 0 | 4 |

### The guard M2 extends

| Fact | Evidence |
|---|---|
| The AST guard is at the **repo root**, `package main` | `messaging_surface_guard_test.go`, 1368 lines |
| It carries two tests today | `TestEveryTextSurfaceIsRegistered:483`, `TestNarrationSitesMatchViewpointAudit:1311` |
| It already models a `narration / content / config` scope split | `surfaceScope` type at the head of the file |
| M1's goldens live under `internal/narration/testdata/stores/` and cover the message **stores** only | Six golden files; none reference a special-move verb |

### Documents that are wrong today

| Claim | Verdict | Evidence |
|---|---|---|
| `internal/messaging/context.md` lists `UserSender`, `ProgressionKind`, `FormatProgression`, `SendProgression` | **All four absent.** There is no `progression.go` in the package | Public surface of `internal/messaging/*.go`; `grep -rn UserSender` finds only a comment |
| `internal/banner/banner.go:7` points at `internal/messaging/progression.go` | **File does not exist** | `ls internal/messaging/` |
| `internal/narration/` carries a `context.md` | **It does not.** The package holds `picker.go`, `picker_test.go`, `snapshot_test.go`, `testdata/` only | `ls internal/narration/` |
| `docs/README.md:75` said the M1 scanner's labels are wrong 18% of the time | **26%**, and **already corrected in the same change as this spec**, so the row now reads 26. The audit says "64 of the 247 labels (26%) were wrong" | `docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md:201` |

### The quest bridge's missing actee seam

| Fact | Evidence |
|---|---|
| `GameBridge` exposes `SendText` (actor) and `RoomText` (observer), and no actee seam | `internal/questengine/bridge.go:200`, `:205` |
| Quest YAML has no actee key | `playermessage` 67 uses, `roommessage` 57, `room_text` 22, across `_datafiles/world/dogmud/quests/` |

---

## What M2 delivers

One seam that fans a narrated event out to its three audiences, every
special-move verb on both the player and mob side migrated onto it, and the
five defects M1 found, across seven sites, closed.

M2 does **not** touch rendering: band selection, pool shape, token vocabulary
and the authored stores are all M3's. The split is deliberate and it is where
the risk lives. Band selection in particular is still split between `zScore`
(melee, via `GetDefenseMessage`) and `defensiveCrit` plus
`normalizedDefenceMargin` (every other channel, via `RenderDefenseMessage`).
Unifying those changes which band fires, which is a behavior change, so it
stays out of M2 exactly as PR #112 left it.

### Why this seam is worth building

M1 ruled all 247 hand-rolled narration sites. 240 are correctly incomplete.
Seven are defects, and they are five distinct bugs. **Every one of the five is
the same shape: a duplicated code path that copied the mechanical effect and
dropped the narration beside it.** Not one came from an author deciding a
viewpoint should not exist.

If applying an effect and narrating it were one call, none of the five could
have happened. That is the argument, and it is stronger than "the code is
scattered".

---

## 1. The seam

New file, `internal/messaging/trio.go`.

```go
// Line is one audience's view of an event: what they are told, and under
// which category. The category rides on the line rather than on the Trio
// because the three roles do NOT share one. See "Categories are per role".
type Line struct {
	Text string
	Cat  Category
}

// Say builds a Line. It exists so call sites read as prose rather than as
// struct literals.
func Say(cat Category, text string) Line { return Line{Text: text, Cat: cat} }

// NoLine marks a viewpoint that deliberately has nothing to say. Spelled out
// so the guard can tell a considered absence from a forgotten one.
var NoLine = Line{}

// Trio is one narrated event as its three audiences see it.
type Trio struct{ Actor, Actee, Observer Line }

// Recipient is anything that can be sent a categorized line. Satisfied by
// *users.UserRecord and by actions.Actor without either changing.
type Recipient interface {
	SendText(cat Category, text string)
}

// Broadcaster is anything that can broadcast to a room minus some ids.
// Satisfied by *rooms.Room without change.
type Broadcaster interface {
	SendTextVisual(cat Category, txt string, excludeUserIds ...int)
}

// Audience is who is present for one narrated event.
//
// Ids are passed rather than derived: UserRecord.UserId is a field while
// actions.Actor exposes GetUserId(), so no one interface can reach both.
type Audience struct {
	Actor   Recipient // nil on the mob side, where the actor has no client
	ActorId int
	Actee   Recipient // nil when the actee is a mob
	ActeeId int
	Room    Broadcaster
}

// SendTrio delivers one narrated event to everyone entitled to it.
func SendTrio(t Trio, aud Audience)
```

Call sites read:

```go
messaging.SendTrio(messaging.Trio{
	Actor:    messaging.Say(messaging.CategorySystem, "Your shield bash strikes Grix!"),
	Actee:    messaging.Say(messaging.CategorySystem, "Kesh's shield bash strikes you!"),
	Observer: messaging.Say(messaging.CategoryBash, "Kesh bashes Grix with their shield!"),
}, aud)
```

### Semantics

- A line is delivered only if it has **both** text and a recipient. Either
  half being absent is a silent, correct skip.
- The room broadcast **always** excludes `ActorId` and `ActeeId`. The caller
  can no longer get the exclusion list wrong, which is a thing call sites
  currently do by hand every time.
- Nothing is rendered, chosen, banded or tokenised here. Callers pass finished
  strings, exactly as they build them today.
- `SendTrio` is the only function in the file. It is deliberately small; its
  value is that it cannot be called with a viewpoint missing.

### Categories are per role, not per event

The seam takes no `Category` argument, because **the three roles do not share
one.** Counted across the twelve player-side verbs on 2026-09-08:

| Shape | Verbs |
|---|---|
| Personal lines `CategorySystem`, room line verb-specific | 10 of 12: `bash` (`CategoryBash`), `kick` (`CategoryKick`), `trip` (`CategoryTrip`), `grapple` (`CategoryGrappleFlow`), and `drain gore maul pounce rake throttle` (all `CategoryHitNaturalSharp`) |
| Several categories on the personal side | `shoot`: `CategorySystem`, `CategoryHitRanged`, `CategoryDodge`, `CategorySurpriseAttack` |
| Several on **both** sides | `throw`: personal `CategorySystem`, `CategoryDodge`, `CategorySpellDisruption`; room `CategoryHitRanged`, `CategoryDodge`, `CategorySpellDisruption` |

A single-`Category` signature would have handled ten verbs and silently
rewritten the categories of the other two, which reach the verbosity
suppression allowlists in `internal/messaging/verbosity.go` and decide what a
player on medium or light verbosity actually sees. That is a player-visible
change, and M2 claims bug-compatibility, so the category rides on the `Line`.

This is the same fact M1 recorded from the other end: `messaging.Category`
cannot classify a narration site, because `give.go` sends one event across two
categories.

### What goes through the seam, and what does not

**Narrated events** go through `SendTrio`: anything describing something that
happened in the world. This includes actor-only detail lines riding on a parent
event, which write `Actee: messaging.NoLine, Observer: messaging.NoLine` so the
guard can see the silence was chosen.

**Refusals and mechanical feedback stay plain `SendText`.** "You need a shield
equipped to perform a shield bash!", "Your target is gone!", cost refusals, and
the cooldown notice describe why nothing happened, and have no actee or
observer to reason about. M1 ruled 21 actor-only refusals in a single slice as
correctly incomplete; forcing them into a `Trio` would add two `NoLine` fields
per site and teach the guard nothing.

The line between the two is whether the message would still make sense to
somebody who was not the actor. If yes, it is an event. If it only answers
"why did my command not work", it is a refusal.

### Why `internal/messaging`

The two edges above rule out both obvious alternatives. `internal/narration`
cannot import `messaging`, so the deliver half cannot join the render half.
`internal/actions` cannot be imported by `questengine`, so it cannot serve the
quest bridge when M3 gets there.

`messaging` can host it with **zero new imports**, because the package already
avoids importing `rooms` and `users` on purpose and reaches them through
minimal interfaces. `Recipient` and `Broadcaster` are one more instance of a
pattern `predicates.go` established, for the same reason, with the reason in a
comment.

The one real objection is that the package's stated job is a seven-stage
pipeline **for one recipient**, and fan-out to three audiences is a different
job. The owner ruled that acceptable on 2026-09-08: the `context.md` needs a
correction pass regardless, since four of the symbols it documents do not
exist.

---

## 2. The net, built before anything moves

**The arc spec says M2 keeps "snapshots green throughout". That is not
available here, and this design does not claim it.**

M1's goldens snapshot the five message stores. They do not touch a single one
of the 23 files M2 rewrites, whose text is hand-rolled `fmt.Sprintf` literals.
M1 chose that deliberately and correctly: building roughly 247 render fixtures
for text M2 might replace outright was the most expensive thing in the arc and
bought the least.

The consequence is that M2's actual refactor has no automated protection until
it builds some.

**Task 0 captures a literal freeze over the 23 files**, as a third test in the
existing root-level guard, before the first migration commit lands. Every
narration string literal in those files is inventoried by AST. A refactor that
drops a line, reorders a pool, or alters a string then fails the build.

This is a new test in an established file, not new machinery: the guard already
walks the tree with `go/ast`, already classifies sites by scope, and already
locks a 141-entry registry against an audit document.

The six deliberate output changes at the end of M2 update the freeze in their
own commits, so every change to authored text is one reviewable hunk.

### What the net still does not cover

Snapshots and freezes cannot capture the **order and interleaving** of messages
within a round. That is player-visible and real, it was already acknowledged as
a gap in the arc spec, and it is part of why M2 now ends with a playtest.

---

## 3. Migration order

23 files, one commit each, `gofmt` plus the touched package's tests plus the
freeze green after every one.

| Order | What | Why here |
|---|---|---|
| 0 | Literal freeze over the 23 files | The net, before anything moves |
| 1 | `internal/messaging/trio.go` plus unit tests | Seam with no callers yet |
| 2 | The two `sendMoveDefenceTriad` copies collapse to one | **The proof.** Near-identical duplication across two packages, the exact shape that produced all five defects |
| 3-14 | `internal/usercommands/` times 12 | `bash drain gore kick maul pounce rake throttle trip grapple shoot throw` |
| 15-25 | `internal/mobcommands/` times 11 | Same list without `throw` |
| 26 | Documentation corrections | Section 6 |
| 27-32 | The six output changes | Last, one commit each |

### The mob side

Mobs have no client, so `actions.Actor.SendText` is a documented no-op for
them. Mob call sites therefore write `Actor: messaging.NoLine` and leave
`Audience.Actor` nil. That is categorically true rather than a per-site
judgment, so it needs no case-by-case reasoning during migration.

The mob side is also where the **actee is a real player**, which makes its
defects the ones players actually experience. It is not the lower-value half.

### `shoot` and `throw`

M1 held both out of the trio family for needing separate handling. `NoLine`
resolves both without a special case. `throw` is an area effect against mobs
and writes `Actee: messaging.NoLine` honestly. `shoot` only ever differed by
naming its actee `p`, which is why the M1 scanner, which matches the actee by
variable name, never saw it.

---

## 4. The six output changes

Held to the end of the slice, one commit each, so the refactor and the behavior
changes never share a diff.

| # | Site | Change |
|---|---|---|
| 1 | `actions/salvage.go:202`, `:207` | The room broadcast sits on the `else if` mob branch, so a mob butchering a corpse is narrated and a player is invisible. Moved to the shared path |
| 2 | `hooks/spell_resolution.go:1075` | `case "buff":` never calls `sendVisualRoomText`; its sibling `case "heal":` at roughly `:1044` does. Gains the broadcast |
| 3 | `usercommands/rally.go:72` and `usercommands/warcry.go:76` | The Resonant Larynx fold loops apply `AddCondition` and `AddBuff` to every party member and never `SendText` them. Mirrored in both files |
| 4 | `usercommands/equip.go:218` | The arm-slot path drops the room line the shared path sends at `:274` and `:277` |
| 5 | `usercommands/admin.zap.go:82` | The engaged-target path puts a player on 1 health with no message; the explicit-target path at `:46` does tell them |
| 6 | `usercommands/throttle.go:77` | An interrupted cast is told to actor and actee and not to the room. Gains a room line under the detail-line ruling below |

**Number 6 is a scope add, not a bug M1 found.** M1 ruled that site `correct`
but noted the convention was unsettled. The ruling below settles it, and the
consequence is a new player-facing line. It is called out here so it is not
mistaken for one of the five.

---

## 5. Rulings

### Detail lines riding on a parent event

The arc spec requires M2 to settle whether a detail line inherits its parent
event's viewpoints. The four real sites split cleanly into two kinds, so the
ruling is by kind rather than by position.

**A detail line that describes a world event inherits the parent's viewpoints
and becomes a full trio.**

- `throttle.go:77`, a spell collapsing. The room can see this happen.
- `grapple.go:112`, a disarm. Already a full trio today, and correct.

**A detail line that reports private knowledge stays actor-only and writes
`NoLine` twice, explicitly.**

- `grapple.go:107`, "they had little chance to resist". This explains the
  actor's roll to the actor.
- `grapple.go:131`, "Your failed attempt leaves you exposed!". Same.

The rejected alternative was a single flat rule that every detail inherits its
parent's roles. It fails on the second kind: it would oblige M6 to author a
room line for "Your failed attempt leaves you exposed!", which means inventing
an observation nobody in the room actually made, addressed to a room that
already watched the parent event get narrated.

### The quest bridge's missing actee seam

**Correctly absent. M2 records the ruling and does not build the seam.**

`GameBridge` exposes an actor seam and an observer seam. Quest YAML has
`playermessage`, `roommessage` and `room_text`, and no actee key anywhere, so
an actee method added now would be a slot nothing fills. M3's own migration
table already schedules "the Actee slot appears here" for the buffs, spells and
quests step, and M6 authors the text. Building it early would put an untested,
uncalled method in a bridge that quest scripts reach.

---

## 6. Documentation corrections

One commit, alongside the migration.

- **`internal/messaging/context.md`** loses the four symbols that do not exist
  (`UserSender`, `ProgressionKind`, `FormatProgression`, `SendProgression`) and
  gains the seam. The Public API section is re-derived from the package rather
  than edited in place, since it has already drifted once.
- **`internal/banner/banner.go:7`** loses its comment pointing at
  `internal/messaging/progression.go`, which does not exist.
- **`internal/narration/context.md`** is written. The package has none, which
  the project convention requires of every package under `internal/`.
- **`docs/README.md:75`** was corrected from 18% to 26% in the same change as
  this spec, matching the audit it summarises. Listed here so the correction is
  recorded rather than silent.
- **`docs/README.md`** gains a row for this spec.

---

## 7. Verification and acceptance

Per commit:

- `gofmt -l internal/ modules/` prints nothing.
- `go build ./...`.
- Tests for every package touched.
- The literal freeze passes, or is updated in the same commit with the change
  visible in the diff.

Before the PR:

- Full suite.
- Boot test in an isolated detached worktree per the pre-push SOP. Exit code
  124 is the pass.
- `docs/PATCH_NOTES.md` entry, player-facing framing.

**Then the adversarial playtest gate.** The six output changes put new
player-facing lines into the world, which is authored content under the project
content SOP. A boot-clean build cannot tell whether a newly broadcast salvage
line reads well, whether the rally and warcry party lines fire at a sensible
moment, or whether the new throttle room line crowds the round it lands in. The
playtest must exercise: a player butchering a corpse in a room with a witness,
a buff spell cast in company, `rally` or `warcry` with a real party, an
arm-slot equip, and a throttle that interrupts a cast.

### Acceptance

M2 is done when:

1. `SendTrio` is the only path by which the 23 files narrate an **event**.
   Refusals and mechanical feedback still use plain `SendText`, per the
   boundary in section 1.
2. `sendMoveDefenceTriad` exists once, not twice.
3. The literal freeze covers all 23 files and passes.
4. Every `Trio` literal in the tree names all three roles, enforced by the
   guard with no exceptions list.
5. No category changed anywhere in the 23 files. The per-role categories in
   section 1 are carried across verbatim, `shoot` and `throw` included.
6. The seven defect sites narrate every viewpoint they should.
7. The playtest has run and its findings are fixed or filed.

---

## Divergences from the arc spec

Both are deliberate, both were approved on 2026-09-08, and both are recorded in
the arc spec itself so a later slice does not plan against the superseded text.

**1. "Snapshots green throughout" is replaced by a literal freeze built in
Task 0.** The arc spec assumed M1's harness would protect M2's refactor. It
does not: M1 snapshots the message stores, and M2 rewrites hand-rolled literals
that M1 explicitly and correctly chose to inventory rather than render. Without
Task 0, the 23-file migration would have no automated protection at all. This
is the same class of gap that let the melee defence bug survive M1, where
goldens froze `RenderDefenseMessage` and never called `GetDefenseMessage`, so
the melee seam had zero snapshot coverage.

**2. M2 ends with the adversarial playtest gate, which the arc spec places at
M5.** The arc spec put M2 down as bug-compatible with no output changes, so no
gate was needed. M2 as scoped closes the five defects M1 found across seven
sites, plus one more from its own detail-line ruling, which adds
player-facing lines, and the project content SOP requires a playtest whenever
player-facing content is authored. M5 keeps its own gate for the
crime-in-the-dark ruling.
