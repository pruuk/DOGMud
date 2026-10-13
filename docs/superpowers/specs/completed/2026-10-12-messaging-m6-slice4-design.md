# Messaging M6 slice 4: one body for a caster's own spell lines

**Date:** 2026-10-12
**Issue:** #370 (M6), ledger row 9 of
`docs/superpowers/audits/messaging-m6-content-ledger.md`
**Status:** design approved by the owner in session, 2026-10-12 (rulings R1
to R7 below). Rewritten the same day: the first version covered the target's
cast-start line only; reading the hidden-caster path showed seven sites
narrating a caster's own spell lines under four different rules, and the
owner asked for them to be unified so they cannot drift again.

## Facts verified against source (master `eb0d57624`, 2026-10-12)

| Fact | Where |
|---|---|
| `SpellData` authors six text keys: `cast_actor`, `cast_observer`, `wait_actor`, `wait_observer`, `magic_actor`, `magic_observer`; no `*_actee` key | `internal/spells/spells.go:88-93` |
| `SpellData.Narration` returns `Actor` and `Observer` only; its comment says "Actee is empty; the target's own line is authored in M6"; a unit test pins the empty Actee | `internal/spells/narration.go:21-35`, `internal/spells/narration_test.go:22-33` |
| Root guard: no file outside `internal/spells` reads the six text fields | `store_text_fields_guard_test.go:32` |
| **Site 1**, player cast start: `cast_actor` to the caster, `SendTextVisual(cast_observer)` to the room except the caster; no hidden rule; no room line when the spell authors none | `internal/usercommands/skill.cast.go:310-339` |
| **Site 2**, mob cast start: `SendTextVisual(cast_observer)` to the whole room, no hidden rule; then ALWAYS `"<name> begins weaving a spell."` by `SendTextVisual` with the raw name | `internal/mobcommands/cast.go:118-140,173-174` |
| **Site 3**, mob `aid` cast start: `SendTextVisual(cast_observer)`; `aid` clears hiding first (`aidskill` exists only in `world/default`) | `internal/mobcommands/aid.go:48,58-75` |
| **Sites 4 and 5**, player wait line, in `handlePlayerFoldCasting` (the completing round and each still-casting round): `wait_observer` by `r.SendText`, the AUDIO channel, so the caster is named to blind readers and to readers who do not perceive a hidden caster | `internal/hooks/NewRound_DoCombat_helpers.go:709-722,852-866` |
| **Site 6**, mob fold rounds: `sendMobWindupWarning` (`wait_observer`, only for `min_cast_rounds` spells) else `sendMobWeaving` (`"<name> weaves magic with focused intent."`), visual, named by `mobSeenName`, skipping `mobActUnseenBy` | `internal/hooks/NewRound_DoCombat_helpers.go:556-585,1024-1025` |
| **Site 7**, player magic line: `magic_observer` by `SendText` (audio), no hidden rule; no mob equivalent; no shipped spell authors `magic_*` | `internal/hooks/spell_resolution.go:195-221` |
| Landing lines already share one body (`spellEffectCtx`), naming a hidden mob caster "Something" to the room (`roomName`, `actorHiddenFromRoom`) | `internal/hooks/spell_effects.go:96-145,262-311` |
| Disruption lines (break, fizzle, falter) are heard as well as seen, with `mobSubjectName` / `playerSubjectName` reading a hidden caster as "Something" (owner ruling R4, #242) | `internal/hooks/NewRound_DoCombat_helpers.go:505-620` |
| Owner ruling R3 (#274): "a hidden actor does not emote. Silence, not an anonymous line." Implemented as `mobActUnseenBy` / `conditionLineUnseenBy` / `playersNotPerceiving` | `NewRound_DoCombat_helpers.go:512-520`, `Condition_ApplyConditions.go:245-250`, `Logout_AwarenessCleanup.go:63-71` |
| `Character.IsHidden()` reads the Awareness FSM; `Character.Perceives(other)` is true unless other is hidden and the reader lacks `SeeHidden` | `internal/characters/character.go:892,966` |
| Neither cast command clears hiding. A hidden caster is revealed only by the Awareness cascade on its own CombatPhase going Idle to Engaging | `internal/hooks/Awareness_Cascades.go:30-41` |
| A player's harmful spell enters combat when it lands (`commitHarmfulSpellAggro` commits the caster when not in combat), so a hidden player is revealed then | `internal/hooks/spell_effects.go:211-222` |
| A mob's harmful cast commits targeting at cast start so the fold steps in the combat loop (#242), so a hidden mob is revealed at cast start, after its cast lines went out | `internal/mobcommands/cast.go:176-190` |
| `targeting.Commit` calls `SetAggro`, whose CombatPhase transition carries `TriggerAttackCommand` (or `TriggerSurpriseAttack`) | `internal/targeting/commit.go:62-68`, `internal/characters/engagement_storage.go:154-170` |
| `messaging.SendTrio` hides the actor's name from the actee by the actee's sight; its observer half is `SendTextVisualHidingNames`; `Trio.ObserverSound` is a heard line that names nobody | `internal/messaging/trio.go:38-136` |
| `casting-messages.yaml` pools: `already_casting`, `cast_started`, `cast_continuing`, `concentration_slipped`, ...; loaded into `spells.CastingMessages`; `concentration_slipped` carries an em dash and the dash guard does not scan the file | `_datafiles/world/dogmud/casting-messages.yaml`, `internal/spells/casting_messages.go:17-25`, `prose_dash_guard_test.go:20-28` |
| The dash guard does not scan `spells/` either; `core-drain.yaml:7` has ` -- ` | `prose_dash_guard_test.go:20-28` |
| Every dogmud heal spell carries `condition_ids`; every landing applier already sends the target a Go-built line | `_datafiles/world/dogmud/spells/`, `spell_effects.go`, `spell_help_effects.go` |
| 64 dogmud spells: 40 `targeting: single`, 12 `self`, 12 `area` | `_datafiles/world/dogmud/spells/` |
| `hooks` imports `actions`; `usercommands` and `mobcommands` import `actions` | import blocks |

## The gap, restated

Ledger row 9 says damage and heal spells with no condition have no target
line. At landing that is not so: every applier tells the target, and every
heal spell carries a condition. The real gaps are two:

1. At cast start the target reads only the room's line; nothing says the
   spell is aimed at them.
2. The caster's own lines (cast start, each fold round, magic) are sent from
   seven sites under four rules: some visual, some audio; some silent to a
   reader who does not perceive a hidden caster, some naming them; generic
   room lines for mobs only, and doubled on a mob's cast start. A hidden
   player is named on audio every fold round.

## Scope

One PR. In: the target's cast-start line, one delivery body for every
caster's own spell lines, one hiding rule, one home for the perception
helpers, the guards and the copy. Out: per-spell landing lines replacing the
Go templates; target lines for the wait and magic phases; area spells'
target lines; landing and disruption lines, which already share their own
bodies (they only move onto the shared perception helpers, section 4).

## Owner rulings

- **R1** A target who cannot see the caster for darkness still reads their
  `cast_actee` line, the caster as "a figure" at shapes and "Something" at
  none.
- **R2** A mob target reads nothing (no client). Area spells get no target
  line.
- **R3 (applied, #274)** A hidden caster's own lines are silent to a reader
  who does not perceive them, and name them to one who does. This covers the
  room line and the target's line alike.
- **R4 (applied, #242)** What is heard reaches readers who do not see, as a
  line that names nobody.
- **R5** A hidden caster's cast start is HEARD: every player in the room who
  does not perceive the caster, target included, reads one line from a new
  pool `cast_started_unseen` that names nobody and does not say who is
  targeted. Cast start only; the fold rounds stay silent to them.
- **R6** One hiding rule: a harmful spell breaks its caster's hiding when it
  lands, for every caster. Starting a cast never does.
- **R7** The generic room lines ("begins weaving a spell", "weaves magic
  with focused intent") are the fallback for ANY caster whose spell authors
  no room line for that phase, so a player's cast reads to the room as a
  mob's does; a mob no longer gets both the authored and the generic line.

## 1. Data

- `SpellData` gains `cast_actee` (`CastActeeText`), returned as the `Actee`
  variant of `Narration(PhaseCast)`. Wait and magic stay Actor and Observer.
  `validateNarration` treats a phase as authored when any role is present, so
  a whitespace-only `cast_actee` is refused. The text-field guard learns the
  seventh field.
- `casting-messages.yaml` gains three pools, loaded into
  `spells.CastingMessages` and validated like the rest: `cast_started_unseen`
  (R5), and `cast_started_room` and `cast_continuing_room`, which carry the
  two generic room lines (R7) with `{actor}` instead of being spelled in Go.
  Each starts with the existing wording and gains a few variants.

## 2. One body for a caster's own lines

`actions.NarrateSpellPhase(caster Actor, spell *spells.SpellData, phase
spells.Phase, targetUserIds, targetMobIds []int, room *rooms.Room, cat
messaging.Category)` is the only code that sends a spell's cast, wait or
magic lines. Sites 1 to 7 call it; their hand-sent lines and
`sendMobWeaving` / `sendMobWindupWarning`'s bodies go.

For one call:
- **Caster** (a player) reads the phase's `*_actor` line.
- **Target** (cast phase only; a player other than the caster; the spell
  authors `cast_actee`) reads `cast_actee`, the caster hidden by the
  target's sight (R1). A target who does not perceive a hidden caster reads
  no `cast_actee` (R3).
- **Room** reads the phase's `*_observer` line, or the generic pool line
  when none is authored (R7): visual only, names hidden by each reader's
  sight, the caster and an addressed target excluded, and silent to every
  player who does not perceive a hidden caster (R3).
- **Unperceiving readers** of a hidden caster, cast phase only, read one
  `cast_started_unseen` line (R5). Sound, so darkness does not stop it.
- The audio sends at sites 4, 5 and 7 are gone.

## 3. One hiding rule (R6)

- The landing body reveals the caster of a harmful spell explicitly, for
  every caster, in `commitHarmfulSpellAggro`, instead of relying on the
  caster's own combat entry (which a caster already in combat never makes).
- A mob's fold-only combat entry at cast start no longer reveals it: that
  commit carries its own combat-phase trigger (`TriggerCastFold`), and the
  Awareness cascade does not reveal on it. The plan reads how a reason
  reaches `SetAggro`'s trigger and picks the narrowest route (a new
  `targeting.Reason` mapped to the trigger, without a new legacy
  `AggroType` if one can be avoided).
- `aid` keeps clearing hiding, since it is a hands-on act, not a cast from
  hiding; its cast lines go through the body like the rest.

## 4. One home for the perception helpers

The questions "does this reader perceive this caster", "which players do not
perceive them" and "what does this reader call them" are answered once, in
`actions` (which `hooks`, `usercommands` and `mobcommands` all import):
`playersNotPerceiving`, `mobSeenName`, and the hidden checks inside
`mobHiddenFrom`, `actorHiddenFromRoom` and `playerSubjectName`. The `hooks`
copies become calls into them, so the landing, disruption, condition and
cast-phase lines read the same source. Behaviour of the landing,
disruption and condition lines does not change.

## 5. Copy

- `cast_actee` for the 35 single-target spells that can land on someone
  else: the 40 minus fold-anchor, fold-recall, summon-hive-swarm,
  summon-steppe-spirit (target no one) and heal (no cast text). Second
  person, `{actor}` named, what the target sees or feels aimed at them, not
  the outcome; help spells warm, harm spells a threat; matched to the
  spell's `cast_observer` imagery.
- `cast_started_unseen`: school-neutral (a murmur fits an incantation, not a
  psionic stare), names nobody, does not say who is targeted. Draft:
  "You hear a faint murmur nearby, though you see no one." / "The air
  prickles, as if someone close by is gathering power." / "A low whisper
  rises somewhere close, then fades."
- `cast_started_room`, `cast_continuing_room`: the existing two lines plus
  variants, `{actor}` as subject.
- All copy: no dashes, no numbers, plain words, "their" for the caster, every
  YAML value quoted.

## 6. Guards and tests

- A root guard fails when anything outside `internal/actions` and
  `internal/spells` calls `Narrate` or `Narration` on a `SpellData` with a
  phase, so a new site cannot hand-roll delivery again.
- A root guard on the `cast_actee` set (R4 of the first version): every
  aimed single-target spell has one, nothing else does, the five exemptions
  named.
- The dash guard scans `spells/` and `casting-messages.yaml`; the
  `core-drain.yaml` and `concentration_slipped` dashes are rewritten.
- `NarrateSpellPhase` tests, each phase and caster kind: caster, target,
  bystander; shapes and blind targets; self-cast; unauthored phase (generic
  pool line); hidden caster with a perceiving and a non-perceiving reader
  (named line vs `cast_started_unseen` at cast start, silence on a fold
  round); no audio leak to a blind bystander on a wait line.
- Hiding tests: a hidden mob starting a harmful cast stays hidden; a hidden
  caster (player and mob) is revealed when the harmful spell lands; a help
  spell landing reveals nobody.
- The existing hidden-mob room-line tests (`hidden_mob_room_lines_test.go`,
  `spell_channel_sight_test.go`) keep passing against the moved helpers.
- Gate: full `go test ./...`, a YAML shape diff against master, lint
  `--new-from-merge-base`, a boot, and a playtest.

## 7. Ledger and docs

Row 9 is marked done, with the correction that landing lines already
existed. `internal/spells/context.md` documents `cast_actee` and the new
pools; `internal/actions/context.md` documents `NarrateSpellPhase` and the
perception helpers; `internal/hooks/context.md` notes the moved helpers.
PATCH_NOTES gains lines for the target line, the heard murmur, and the
player cast now showing in the room.
