# Messaging Unification Arc — Design

**Created:** 2026-08-31
**Status:** Design approved by owner 2026-08-31. No plan written yet.
**Predecessor:** [Unified Resolution Roadmap](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md)
(closed) — this arc borrows its refactor-first / flip-once shape.
**Backlog entry:** [`CURRENT_BACKLOG.md`](../../roadmaps/CURRENT_BACKLOG.md),
queued arc 2.

---

## Facts verified against source, 2026-08-31

Everything below was read from the tree on the date of writing. **Do not quote
this table after a slice lands without re-grepping** — this arc exists partly
because a "verified" claim about quell survived two weeks past its own fix.

### Claims this arc was nearly designed on, which are FALSE

| Claim | Verdict | Evidence |
|---|---|---|
| "Quell and defy are unnarrated, fall back to the verb `counter`" | **FALSE** | `items.DefenseType` defines `DefenseQuell`, `DefenseDefy` and four counter pools (`internal/items/defensive_messages.go:15`). `defense-messages/` holds **9** files including `quell.yaml`, `defy.yaml`, `counter-{melee,ranged,quell,defy}.yaml`, with band-split, actor-aware text. Rendered by `combat.RenderChannelDefenceMessages` (`internal/combat/defence_multiplier.go:286`) from **9 production call sites**. |
| "The `counter` fallback proves they are unnarrated" | **Misread** | That fallback is in `sendDefenseMessages` (`internal/combat/combat_helpers.go:1284`), the **melee** path. Quell and defy are not in melee's defence set, and the code says the fallback is *"Unreachable today … made unreachable by construction rather than by argument."* |
| "Combat-state Perception shipped dormant with no consumer" | **FALSE** | `messaging.CanSeeClearly` (`internal/messaging/predicates.go:24`) and `CanSeeShapes` (`:44`) read `Perception.State()` on every sight-gated broadcast; `internal/actions/combat_fire.go:250` reads it; `internal/characters/buffs.go` drives it from the blind buffs. |

### The delivery layer that already exists

| Fact | Evidence |
|---|---|
| 7-stage pipeline: compose → normalize → sight gate → anonymize → color → wrap → deliver | `internal/messaging/pipeline.go:53` |
| **The wrap stage never runs.** `shouldWrap` returns false for every category, deliberately | `internal/messaging/pipeline.go` — and `internal/messaging/context.md` still documents wrap as an active stage (chunk 5.12 material) |
| 60 `Category` values | `internal/messaging/messaging.go:18` |
| 3 verbosity tiers + spectator step-down | `internal/messaging/verbosity.go` |
| Audio (`SendText`) bypasses sight gate and anonymizer by design; Visual (`SendTextVisual`) runs all stages | `internal/rooms/rooms.go:279`, `:307` |
| `Anonymize` strips **only ANSI-tagged** names; its docstring admits bare names leak | `internal/messaging/anonymize.go` |

### The sprawl

**14 narration stores, 6 role conventions, 2 token engines, 3 send helpers,
1 hard bypass.**

> **Counted vs estimated, stated explicitly** because this arc exists partly
> because uncounted claims were trusted. **679 text fields are counted exactly**
> — recipes 252, buffs 170, spells 135, quests 122, by grepping the field keys.
> The remaining **ten** stores are counted **in files, not fields** (69 files:
> combat-messages 20, conversations 18, weather 15, defense-messages 9,
> itemvoices 2, and one each for taunt, grapple, casting, gossip and hints),
> because their fields are nested pools whose sizes were not enumerated.
> The often-quoted "about a thousand fields" is an **estimate** extrapolating
> those 68 files, and M0 replaces it with a real number. Do not quote it as
> measured.

| Store | Size | Role convention | Loader | Validator |
|---|---|---|---|---|
| `recipes/` (crafting) | **252 fields** (126 `success_message`, 126 `failure_message`) across 126 files | **none — no audience split at all** | crafting | — |
| `buffs/` | **170 fields** across 101 files | `*_user_text` / `*_room_text` | buffs | — |
| `spells/` | **135 fields** across 59 files | `*_user_text` / `*_room_text` | spells | — |
| `quests/` | **122 fields** (66 `playermessage`, 56 `roommessage`) | `playermessage` / `roommessage` | questengine | — |
| `combat-messages/` | 20 files | `toattacker`/`todefender`/`toroom`, split `together`/`separate` | `internal/items/itemspec.go` | ✅ |
| `defense-messages/` | 9 files | `toattacker`/`todefender`/`toroom` | `internal/items/itemspec.go` | ✅ |
| weather emotes | 15 files | **ambient — no actors** | `modules/weather/content` | test-only |
| `conversations/` | 18 files | speaker A / B | `internal/conversations` | — |
| `hints.yaml` | 250 lines | — | scattered, 15 files | — |
| `itemvoices/` | 2 files | owner only | `internal/itemvoices` | ✅ |
| `taunt-messages/` | 1 file | `{source}`/`{target}`, `toattacker`/`todefender` | `internal/combat/taunt_messages.go` | ✅ |
| `messaging/grapple_outcomes.yaml` | 1 file | **`controller`/`controlled`/`observers`** | `internal/grapplemessaging` | ✅ |
| `casting-messages.yaml` (tree root) | 1 file | caster only | `internal/spells/casting_messages.go` | **none** |
| `gossip_templates.yaml` | 1 file | — | inline in `internal/hooks/MobIdle_HandleIdleMobs.go` | — |

### Fragmentation, precisely

| Finding | Evidence |
|---|---|
| **Three `DefenseType` declarations.** The combat one stops at block — no quell, no defy | `internal/combat/attackresult.go:9`, `internal/items/defensive_messages.go:15`, and bare `string` constants at `internal/characters/character.go:726` |
| They are joined by a **raw string cast** that compiles for any string and yields an empty pool on drift | `items.DefenseType(out.DefenceType)`, `internal/combat/defence_multiplier.go:290` |
| **Two token engines with an overlapping vocabulary.** `items.TokenName` has 16 tokens, 4 consuming files; `textutil` has 4 tokens, 13 consuming files. `{source}` and `{target}` exist in **both** | `internal/items/itemspec.go:200-216`; `internal/textutil/tokens.go` |
| **Three send helpers** | `SendText`, `SendTextVisual`, and `textutil.SendPhaseText` (`internal/textutil/spelltext.go:21`) |
| **Two randomness sources.** `util.Rand` in defence/attack/taunt/itemvoices; stdlib global `rand.Intn` in casting (`internal/spells/casting_messages.go:96`) and grapple (`internal/grapplemessaging/render.go:52`) | — |
| **Only one determinism seam exists** — `indexOverride` on `RenderDefenseMessage` (`internal/items/defensive_messages.go:107`). The other pickers have none | — |
| **Progression bypasses the pipeline entirely.** Six raw `events.AddToQueue(events.Message{…})` calls, so no Category, no color, no normalize, no sight gate, no verbosity | `internal/characters/progression.go` |
| **A config-driven decoration layer** wraps enter/exit messages from `config.yaml` | `EnterRoomMessageWrapper`, `ExitRoomMessageWrapper`, `internal/configs/config.textformats.go` |
| 54 Go files carry in-code text pools | — |

### Two live defects this audit found

| Defect | Evidence |
|---|---|
| **`{source_plain}` / `{target_plain}` produce deliberately untagged names, which `Anonymize` is structurally unable to strip.** **14 buff files** use `{source_plain}` in `*_room_text`, so an infrared-only observer in a dark room reads the actor's real name in full while every correctly-tagged message renders "a figure" | `internal/textutil/tokens.go:12-13`; `_datafiles/world/dogmud/buffs/` |
| **The Observer role exists twice with contradictory rules.** Text observers get a per-recipient sight gate. Knowledge observers — `crimes.WitnessesInRoom` (`internal/crimes/crimes.go:198`) — filter on **faction membership only**, with no sight, darkness, blindness or hidden check, and `IdentifiedPerp` names the perpetrator the moment that list is non-empty | — |

### The Actee gap

**Buffs, spells and quests are all Actor + Observer with no Actee: 427 text
fields that cannot address the person the thing happens to.** Crafting's 252
have neither Observer nor Actee.

---

## Why this arc exists

The stores are all the same abstraction wearing different costumes:

> **(event key) × (band, optional) → one variant, rendered for Actor, Actee and
> Observer from a single coordinated index, with tokens substituted.**

The *text* is all keepers. The scaffolding around it is six arbitrary dialects,
two token engines, two randomness sources and three send helpers. The goal is to
**unify the messaging and simplify the underlying mechanics as much as possible
without losing the flavor and complexity we already have** (owner, 2026-08-31).

The precedent is direct: unifying combat resolution and progression surfaced a
long list of fragile and broken things that no amount of reasoning had found —
five floor pairs that were one value by coincidence, a registry with zero
production callers, five knobs silently inert at zero, a phase boundary that had
never once been true. This audit has already repeated the pattern twice, finding
the `{source_plain}` leak and the split Observer role. **The defect list is an
output of unification, not an input to it.**

---

## The model

### Three roles, and the roles are the audiences

**Actor, Actee, Observer.** There is no separate audience axis; the existing
`toattacker`/`todefender`/`toroom` and `controller`/`controlled`/`observers`
pairs are the same three slots with domain labels on top.

Grapple's controller/controlled and combat's attacker/defender remain as
**authoring aliases**, so no shipped file is rewritten to make the flip work.

### The Observer role carries a perception verdict

Computed once per observer per event, read by **both** consumers: the narration
pipeline picks full text, anonymized text or silence; the knowledge and crime
path decides whether that observer learned anything. One answer to "who
perceived this", two consumers — replacing today's two answers that disagree.

#### Owner ruling, 2026-08-31: darkness defeats crime reporting

> *"It'd be fairly hard to observe a crime in the dark and identify who did it,
> but it's probable that you'd know something is happening. … For now, I'd say
> no crime reporting in the dark unless you have nightvision/etc."*

**This arc applies the ruling rather than filing it.** Crime witnessing stops
being a faction-membership test and starts reading the same per-observer
perception verdict the narration pipeline reads.

🔑 **The three tiers already have machinery.** `crimes.PerpUnknown`
(`internal/crimes/types.go:22`) is a real recorded state, not a null: a crime row
is written and every downstream consumer — arrest (`internal/justice/arrest.go`),
justice (`internal/justice/justice.go`), faction rep
(`internal/hooks/MobDeath_FactionRep.go`), and knowledge-writing — gates on
`perp.Type == crimes.PerpPlayer`. So "a crime happened and nobody can pin it on
you" is already supported end to end.

The only reason the middle tier is unreachable is that `IdentifiedPerp`
(`internal/crimes/crimes.go:232`) derives identification from **whether the
witness list is empty**, not from what those witnesses could see:

```go
if len(witnesses) == 0 { return Perpetrator{Type: PerpUnknown} }
return Perpetrator{Type: PerpPlayer, Id: userId}
```

Target mapping, using the sight verdicts the pipeline already computes:

| Verdict | Witness outcome | Perpetrator |
|---|---|---|
| `SightFull` — lit, or NightVision | witnessed and identified | `PerpPlayer` (today's behavior) |
| `SightShapes` — InfraredVision in the dark | knows something happened, cannot say who | `PerpUnknown` **with a non-empty witness list** — the combination that cannot occur today |
| `SightNone` — dark, no aid | not witnessed | no crime row |

⚠️ **Mobs need a perception verdict, and today only players get one.**
`CanSeeClearly`/`CanSeeShapes` take a `*characters.Character`, which a mob has,
so the predicate applies unchanged — but every one of the 13 call sites checking
`buffs.NightVision` passes a **user**, never a mob.

🔴 **BLOCKING FINDING — `buffs.NightVision` is granted by NOTHING.** The flag is
defined at `internal/buffs/buffspec.go:62` and **13 production sites read it**,
but a grep across the entire data tree finds **no buff, no mutation, no species
and no item that grants it**. `internal/mutations/describe.go:128` even carries
finished player copy for it — *"You see clearly in the dark."* — that no mutation
triggers. `InfraredVision` fares barely better: one buff (`85-infraredvision`),
carried by exactly **one** mob file.

Three consequences, all of which must be settled before this ruling is
implemented:

1. **Every darkness gate in the game currently reduces to "is the room lit?"**
   The nightvision branch of all 13 checks is dead in practice, including the two
   fixed in `go.go` on 2026-08-31 and `canSeeInDark` in `mobcommands/darkness.go`.
2. **The ruling's escape hatch does not exist.** "No crime reporting in the dark
   unless you have nightvision/etc" currently means *no crime reporting in an
   unlit room, full stop*, for every actor in the world.
3. **The middle tier would apply to one mob.** `SightShapes` requires
   `InfraredVision`, which exactly one mob carries.

**This is a content/balance gap, not a messaging defect**, and it is the owner's
call: either something must grant `NightVision` (a mutation is the obvious home,
since the copy is already written), or the ruling should be restated in terms of
room lighting alone. **Do not implement the crime gate until that is decided** —
shipping it as-is would make every unlit room a free-crime zone and would look
like a messaging-arc regression.

⚠️ **This is a real behavior change and does not belong in a refactor stage.** It
lands in **M5**, as its own commit, after the perception verdict has a single
owner — not inside M2 or M3, where every change must be snapshot-provable.

### One band model

The six existing systems — `items.Intensity`, `TauntIntensity`, `AgingPhase`,
defence z-score bands, defence normalized-margin bands, weather's felt threshold
— collapse to one ordered scale. Each store declares what *feeds* it. The
mapping becomes a parameter that M4 flips, not a branch.

### One coordinated pick

`DefenseMessageTriad` already guarantees all audiences come from the same
variant index, enforced by a validator requiring equal-length lists, at least
five variants, and no empty strings. That property becomes universal — it is why
the room never sees a different event than the participants.

### Deliberately out of scope

- **Help templates, room descriptions, dialogue trees, quest `description`
  fields, `localize/`.** Authored content a player reads on request, not events
  narrated at them. The *delivery* of `talk`/`ask` is in scope; the dialogue
  text is not.
- **Weather stays adjacent, not absorbed.** Same renderer, tokens and pipeline;
  it keeps its actorless shape rather than pretending an ambient line has an
  Actor.
- **Weather-driven mechanics** (wet conditions affecting items or players) are a
  different arc entirely (owner, 2026-08-31).

---

## Stages

Everything through M3 is provably no-op against snapshots. The behavior change is
one commit.

> **Plan granularity.** This is an arc spec, not a single implementation plan.
> Each stage gets its own plan, written when the stage is next, so that later
> stages are planned against what earlier ones actually found. M3 in particular
> should not be planned in full up front: its whole purpose is to surface
> defects, and a plan written before those defects exist would be fiction.

### M0 — The sweep *(a slice, not a preamble)*

A categorized inventory of every narration site and every delivery-layer stage,
as a standalone reviewed document before any code. Covers stores and delivery
layer **in one pass** — they are coupled through `Category`, and auditing the
stores blind to what the pipeline does to their output would be auditing half a
system.

**Enumeration must be mechanical, by property, not curated.** A curated list
failed twice during this design: the first pass missed buffs, spells, quests,
crafting, progression, the second token engine and the third send helper.
Enumerate every YAML key matching a text pattern, every Go string pool, every
send call site, every token vocabulary.

**Deliverable beyond the document: a registered surface list plus a guard test
that fails when an unregistered text-bearing surface appears.** Completeness has
to be testable, not asserted, or this inventory decays exactly as the quell claim
did.

### M1 — The snapshot harness

Freeze what every narration path emits today, including paths nobody plays. No
production behavior changes.

**First task: one seeded pick used by every store.** Two of the fourteen stores
use the stdlib global `rand.Intn` and are unreachable by any seam controlling
`util.Rand`; until that is fixed the net has holes.

The mechanism is an **injected picker**, not a global seed: the core takes a
`pick(n int) int` and production supplies the engine's randomness while the
harness supplies a deterministic sequence. A global seed would be process-wide
and would make snapshot runs order-dependent, which is the trap that already
bites this repo's test binaries (all tests share one binary, so relative state
passes or fails by order). The existing `indexOverride` on
`RenderDefenseMessage` is the same idea and becomes redundant once the picker is
injected.

Captured at two levels:

1. **Raw render** — every `(store × event key × band × role)` tuple, tokens
   substituted, all roles from the same index.
2. **Post-pipeline** — the same events at each sight verdict (full / shapes /
   none) and each verbosity tier, catching "the text survived but the delivery
   changed".

**Empty cases must be asserted to stay empty.** `RenderDefenseMessage` returns an
empty triad on a missing or malformed pool; a refactor that makes that path start
emitting text is also a regression. Both directions, like the existing site
guards.

> **Acknowledged gap:** snapshots cannot capture the *order and interleaving* of
> messages within a round. That is player-visible and real. It stays a playtest
> question, which is part of why M5 ends with the adversarial gate.

### M2 — The shared core, bug-compatible

One narration seam reproducing today's behavior **including its
inconsistencies**, taking them as parameters rather than branches: band input,
pool shape, token set, role split. The core is **extracted from the defence
store**, which is already closest to the target shape, rather than designed
beside it. Snapshots green throughout.

### M3 — Store migration

Each store moves onto the core, its old path deleted at the moment it migrates.
No dual maintenance, no final sweep. Snapshots green at every step. This is where
the fragile and broken things surface.

Order — cheapest proof first, hardest last:

| | Store | What it proves |
|---|---|---|
| 1 | defence → extract the core | Already the target shape: coordinated triad, band model, real validator |
| 2 | itemvoices, casting-messages | The degenerate case: single role, no band. Proves roles and bands are genuinely optional. **casting gains the validator it has never had** |
| 3 | taunt-messages | Token-vocabulary aliasing: `{source}`/`{target}` read as Actor/Actee without rewriting the file |
| 4 | grapple_outcomes | Role *and* audience aliasing, and it forces the second randomness source to die |
| 5 | buffs, spells, quests | The `*_user_text`/`*_room_text` and `playermessage`/`roommessage` families. The Actee **slot** appears here |
| 6 | crafting, progression | The two that need *new* behavior. Crafting gains the Actee and Observer **slots** (authoring their text is M6); progression comes inside the pipeline and gains a `Category` |
| 7 | conversations, gossip_templates, hints | The non-combat narration tail. Conversations already have a speaker-A/speaker-B split that maps to Actor/Actee with no Observer; `hints` is reached from 15 files and needs a single owner before it can migrate at all |
| 8 | combat-messages | Largest, and the only `together`/`separate` split. Done on a core proven seven times over |
| 9 | weather emotes | Adjacent: joins renderer, tokens and pipeline, keeps its actorless shape |

### M4 — The flip

One reviewable, revertable commit collapsing the parameters: one role
vocabulary, one token engine (resolving the `{source}`/`{target}` overlap), one
band model, one send path, one loader and layout, one `DefenseType`.

**Snapshots change here, deliberately and visibly — the diff is the review.** The
flip changes *parameters*, not text. A snapshot diff that touches wording means
something is wrong.

### M5 — Quality pass

On a single path: the `{source_plain}` anonymizer leak (held here by the owner
rather than fixed standalone), the wrap decision, the `world/default` template
shadowing, `internal/messaging/context.md`'s phantom wrap stage, and whatever M3
turned up.

**Also here, as its own commit: the crime-in-the-dark ruling.** Crime witnessing
starts reading the shared perception verdict, per the owner ruling recorded
above. It is a behavior change, so it cannot live in a snapshot-provable stage.

**Ends with the adversarial playtest gate** per the content SOP. That playtest
must exercise a crime committed in an unlit room, which no previous playtest has
done.

### M6 — Content pass

Authoring the text for slots that M3 and M4 created but left empty: Actee lines
for the 427 buff, spell and quest fields that have never had one, and Actee plus
Observer lines for crafting's 252.

**The split is slot versus content, and it is drawn at the same place
everywhere.** M3 and M4 make the slot exist; M6 fills it. Crafting is the
clearest case: M3 gives it an audience model, M6 writes the lines that model can
carry.

> **This is sequencing, not descoping.** Making the Actee *slot* exist
> everywhere is M3/M4; *authoring* several hundred new lines is a content project
> that can land incrementally. A store with no Actee text renders exactly as it
> does today, so nothing is broken by the split. Conflating them would stall the
> structural work behind a writing project.

**Playtested on both sides** (owner, 2026-08-31): M5's adversarial playtest is
the "before", and M6 ends with its own.

---

## Risks

| Risk | Answer |
|---|---|
| Moving files → `Filepath()` mismatch → **boot panic**, not a soft failure | Loader accepts both old and new locations through M3; the old path dies only at M4 |
| Mechanical edits to prose are exactly how flavor gets lost | The snapshots **are** the flavor; nothing through M3 may change one |
| Two randomness sources leave 2 of 14 stores unfreezable | M1's first task, before any snapshot is trusted |
| M4's snapshot diff too large to review | The flip changes parameters, not text; wording changes are defects |
| Message ordering within a round is invisible to snapshots | Acknowledged; M5's playtest owns it |
| The inventory is still not provably complete | M0's guard test, which fails on any unregistered text surface |

## How we know it worked

1. **Snapshots green through all of M3** — the no-op proof.
2. **M4's snapshot diff reviewed line by line**; wording changes are defects, not
   deliverables.
3. **Guard tests**, in the style this repo already uses: no unregistered text
   surface; no second token engine; no raw `events.Message` outside the pipeline
   plumbing; one `DefenseType`.
4. **M5 and M6 each end with an adversarial playtest.** Boot-clean and
   snapshots-green both verify the *system*, never the experience.
