# Messaging M6 slice 4: the spell target's own cast line

**Date:** 2026-10-12
**Issue:** #370 (M6), ledger row 9 of
`docs/superpowers/audits/messaging-m6-content-ledger.md`
**Status:** design approved by the owner in session, 2026-10-12 (rulings R1
to R4 below)

## Facts verified against source (master `eb0d57624`, 2026-10-12)

| Fact | Where |
|---|---|
| `SpellData` authors six text keys: `cast_actor`, `cast_observer`, `wait_actor`, `wait_observer`, `magic_actor`, `magic_observer`; no `*_actee` key | `internal/spells/spells.go:88-93` |
| `SpellData.Narration` returns `Actor` and `Observer` only; its comment says "Actee is empty; the target's own line is authored in M6" | `internal/spells/narration.go:21-35` |
| `validateNarration` refuses a blank variant through `narration.ValidateVariants`, skipping a phase with no Actor and no Observer | `internal/spells/narration.go:43-62` |
| A unit test pins the empty Actee: `TestNarrationCastPutsTheCasterInActor` fails if `v.Actee` is non-empty | `internal/spells/narration_test.go:22-33` |
| Player cast start: `cast_actor` to the caster by `SendText`, `cast_observer` to the whole room except the caster by `SendTextVisual`; the target is part of the room | `internal/usercommands/skill.cast.go:310-339` |
| Mob cast start: `cast_observer` to the whole room by `SendTextVisual`; the mob's own line is rendered and dropped | `internal/mobcommands/cast.go:118-140` |
| Mob `aid` sends `aidskill`'s `cast_observer` the same way; `aidskill` exists only in `world/default`, not in the shipped `world/dogmud` | `internal/mobcommands/aid.go:58-73`, `_datafiles/world/default/spells/aidskill.yaml`, `_datafiles/config.yaml:240` (`DataFiles: _datafiles/world/dogmud`) |
| Neither cast-start site checks whether a mob caster is hidden | grep `Hidden` in both files: no match |
| Every resolution applier already sends the target a Go-built line through `messaging.SendTrio`: damage "`<caster>'s <spell> strikes you! (<damage words>)`", heal "`<caster>'s <spell> envelops you in healing energy...`", shield and the rest likewise | `internal/hooks/spell_effects.go:262-311`, `internal/hooks/spell_help_effects.go:163-205` |
| A condition spell's landing line for the holder is the condition's `start_actee` (#470) | `internal/hooks/condition_cast_lines.go:104-140` |
| Every dogmud heal spell (heal, mend-wounds, mass-mend, mend-all, communion-of-flesh, repair-pulse) carries `condition_ids`, so ledger row 9's "heal spells with no condition" names none | `_datafiles/world/dogmud/spells/*.yaml` |
| `messaging.SendTrio` sends Actor, Actee and Observer; the Actee half hides the actor's name by the actee's `ParticipantSight`; the Observer half is `SendTextVisualHidingNames` and excludes both parties' user ids | `internal/messaging/trio.go:118-159` |
| A hidden mob reads as `messaging.UnseenFigure(messaging.SightNone)` to a reader who does not `Perceives` it | `internal/hooks/NewRound_DoCombat_helpers.go:405-436` |
| 64 dogmud spells: 40 `targeting: single`, 12 `self`, 12 `area` | `_datafiles/world/dogmud/spells/` |
| A help single-target spell takes a named player or an allied mob, else defaults to the caster | `internal/actions/cast.go:244-262` |
| fold-anchor, fold-recall and purge-affliction resolve in Go hooks; fold-anchor and fold-recall act on the caster only | `internal/hooks/spell_resolution.go:254-262`, `internal/hooks/spell_foldanchor.go:10-16` |
| summon-hive-swarm and summon-steppe-spirit summon a companion (`summon_mob_id`) and target no one | their YAML |
| `heal.yaml` (dogmud) authors no cast text at all | that file |
| The YAML dash guard scans `combat-messages`, `defense-messages`, `narration`, `messaging`, `rooms`, `mobs`, `taunt-messages`; not `spells` | `prose_dash_guard_test.go:20-28` |
| `core-drain.yaml` carries ` -- ` in its `description` and a comment | `_datafiles/world/dogmud/spells/core-drain.yaml:7,25` |
| The shipped-narration guard already loads `spells` as a flat store | `shipped_narration_data_guard_test.go:185-186` |

## The gap, restated

Ledger row 9 says damage and heal spells with no condition have no target
line. At resolution that is not so: every applier tells the target in Go, and
every heal spell carries a condition. The real gap is the cast start. When a
creature begins a targeted spell, the target reads the room's line ("The
shaman stares intently, a faint pressure building in the air.") and nothing
says the spell is aimed at them. That holds for every single-target spell,
condition or not, cast by a player or a mob.

## Scope

One PR. A new `cast_actee` key, its delivery at the two shipped cast-start
sites (and `aid` for parity), and authored lines for the spells listed below.
Out of scope: per-spell landing lines replacing the Go templates (owner chose
cast start only), wait and magic phase actee lines, area spells.

## 1. Data

- `SpellData` gains `CastActeeText string \`yaml:"cast_actee,omitempty"\``,
  beside `cast_actor` in `spells.go`.
- `Narration(PhaseCast)` returns it as `Actee: textutil.Pool(...)`. Wait and
  magic stay Actor and Observer. The comment loses "authored in M6".
- `validateNarration` treats a phase as authored when any of Actor, Actee or
  Observer is present, so a whitespace-only `cast_actee` is refused.
- Tokens: `{actor}` is the caster, `{actee}` the target, as the other cast
  keys already render them.

## 2. Delivery

Both cast-start sites build one `messaging.Trio` and one
`messaging.Audience` and call `messaging.SendTrio`, in place of the hand-sent
lines:

- Actor: `cast_actor` to a player caster (a mob caster has no client, as now).
- Actee: `cast_actee` to the first target when that target is an online
  player other than the caster and the line is authored. `SendTrio` hides
  `{actor}` by the target's sight: "a figure" at shapes, "Something" at none
  (**R1**: a target in the dark still reads their line, with the caster as
  "Something", as the owner's 10-10 rule has a hidden caster's lines read).
- Observer: `cast_observer` to the room, sight-gated and name-hidden as
  `SendTrio` does. The target is excluded from the room line only when they
  received their own line; with no `cast_actee` authored, `Audience.Actee` is
  left unset and the target reads the room line as today.
- A mob caster that is hidden from the target (`!target.Perceives(mob)`) is
  named `messaging.UnseenFigure(messaging.SightNone)` in the target's line.
  The plan's first task checks whether a hidden mob reaches cast start still
  hidden; if it does, the room line takes the same per-reader rule, since
  that is a live name leak in the code this slice rewrites.
- **R2:** a mob target reads nothing (no client). Area spells get no actee
  line; their room line already describes a blast that reaches everyone.
- Category stays `CategorySpellFold` (`CategorySpellVital` for `aid`).

Self-cast (target is the caster) sends no Actee, so a caster never reads two
lines.

## 3. Copy

`cast_actee` is authored for every `targeting: single` dogmud spell whose
effect can land on someone else: the 40 single-target spells minus
fold-anchor, fold-recall, summon-hive-swarm, summon-steppe-spirit (target no
one) and heal (authors no cast text), so 35 spells.

Rules for each line:
- Second person, names `{actor}`, describes what the target sees or feels
  aimed at them, never the outcome (the landing line states it).
- Matches the spell's existing `cast_observer` imagery so the two lines read
  as one moment from two seats.
- **R3:** help spells read warm ("{actor} turns to you, warm light gathering
  in their hands."); harm spells read as a threat.
- No dashes (em, en, spaced hyphen, double hyphen), no raw numbers, the
  plain-word idiom list, under 80 columns rendered with a typical name,
  "their" for the caster, no colon in an unquoted YAML value (quote every
  line).

## 4. Guards and tests

- **R4:** a root guard test loads the shipped spells and fails when a spell
  in the authored set lacks `cast_actee`, or a spell outside it (self, area,
  or the five exclusions) authors one. The exclusion list is named in the
  test.
- `spells` joins `proseDashDirs`; the two `core-drain.yaml` ` -- ` uses are
  rewritten in the same commit.
- `narration_test.go`: the empty-Actee assertion flips to the authored line;
  a whitespace-only `cast_actee` fails validation.
- Delivery tests at both sites: player caster on a player target (target
  reads Actee, not Observer; a bystander reads Observer), shapes-only target
  ("a figure"), blind target ("Something"), unauthored spell (target reads
  Observer), self-cast (no Actee), mob caster on a player, hidden mob caster.
- Gate: full `go test ./...`, the YAML shape diff against master, lint
  `--new-from-merge-base`, a boot.

## 5. Ledger and docs

Row 9 is marked `done (PR #___)` with the correction that landing lines
already existed as Go templates. `internal/spells/context.md` documents
`cast_actee`. PATCH_NOTES gains a line.
