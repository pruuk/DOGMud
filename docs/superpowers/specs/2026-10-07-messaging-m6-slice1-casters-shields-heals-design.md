# Messaging M6 slice 1: condition casters, one line per audience, shield and heal identities

Status: design approved by the owner 2026-10-07, spec awaiting owner review.
Issues: #370 (M6 ledger rows 1 and 2), #338 (shield and heal identities), #240 (DoT kills credit nobody).

## Goal

A condition remembers who put it there. That one fact lets three things happen:

1. A poison, bleed or other damage-over-time kill credits the caster (#240).
2. A spell that lands a condition prints one line per audience, written by the
   condition's data and naming the caster (M6 ledger row 1).
3. Shield and heal spells each land their own condition, so a player can tell
   which ward or heal is on them, what a ward blocks, and a second ward or heal
   replaces the first instead of silently overwriting a shared record (#338, M6
   ledger row 2).

## Owner rulings (do not relitigate)

- **R1 (2026-09-11):** "the right end state is one merged line per audience, with
  the code supplying who and the data supplying what."
- **R2 (2026-10-07, #370 Q1):** the 5b ruling governs. A buff's holder line is
  already the Actee line; what buffs lack is the caster (Actor) line.
- **R3 (2026-10-07, #338):** shield spells differ in effect and duration. A shield
  mitigates physical, spell or social (conviction) damage, or a combination, and
  its condition says which. "Minor Shield" for every shield is not acceptable.
  Heals are all heal over time, with variety: short and strong, long and weak,
  single target, area.
- **R4 (2026-10-07, #338):** a shield spell does not stack with another shield
  spell, and a heal spell does not stack with another heal spell. Both stack with
  potion, item and mutation effects.
- **R5 (2026-10-07):** #240 kill credit is in this slice.
- **R6 (2026-10-07):** within a family, **the newest replaces** the older one, and
  the replacement is announced.
- **R7 (2026-10-07):** the shield roster is three wards: a weak physical-only ward
  that reuses the weaker existing spell (Conviction Ward), a new medium ward for
  physical and spell damage, and Chrysalis Cocoon for all three kinds, the
  strongest.
- **R8 (2026-10-07):** heals keep the regen-multiplier model (condition effect
  `regen_mult`); each heal spell gets its own multiplier and duration.
- **R9 (2026-10-07):** the heal roster is the table in section 5, and Chrysalis
  Regeneration and Vital Surge join the heal family.
- **R10 (2026-10-07):** a DoT kill credits the caster wherever they are: moved
  away, logged out or dead since the cast. A mob caster lost to a restart
  credits nobody.
- **R11 (2026-10-07):** the condition's authored text owns the merged line. The
  spell's generic "takes effect" trio is dropped whenever the condition has its
  own start text.

## Facts verified against source (master 7ec29bb34, 2026-10-07)

| Fact | Source |
|---|---|
| `conditions.Condition` has `ConditionId`, `Source` (string), `OnStartWaiting`, `Permanent`, `RoundCounter`, `TriggersLeft`, `TickAmount`, `Magnitude`, `Stacks`, `LightTrim`, `LightOutput`, `Hooded`; no caster field | `internal/conditions/conditions.go:14-44` |
| `events.Condition` has `UserId`, `MobInstanceId`, `ConditionId`, `Source`, `DurationMult`, `Magnitude`, `Triggers`, `TickScale`, `LifeEpoch`; no caster field | `internal/events/eventtypes.go:20-44` |
| Adders: `AddCondition(id, isPermanent)`, `AddConditionScaled(id, mult)`, `AddConditionMagnitude(id, triggers, mag)`, `RefreshCondition(id)` | `internal/conditions/conditions.go:419, 276, 347, 391` |
| One record per condition id; a re-add overwrites `TriggersLeft`, `RoundCounter`, `Permanent` and, on the magnitude door, `Magnitude`/`TickAmount` | `conditions.go:308-313, 351-366, 449-454` |
| A refresh of an already-active condition suppresses its start line (`wasAlreadyActive`) | `internal/hooks/Condition_ApplyConditions.go:86, 130` |
| No family, group, exclusive or category concept exists on `ConditionSpec`; the only multi-copy rule is the `stacking` flag (122 bleeding only) | `internal/conditions/conditionspec.go:99-104, 171-225` |
| Effects from different records combine in `Effect()`: multipliers multiply, `isMax` kinds take the max, others (incl. `mitigation_flat`) sum | `internal/conditions/effects.go:219-259` |
| `Source` reaches the record only on the magnitude door; the scaled and plain doors drop it, and GMCP shows "unknown" | `Condition_ApplyConditions.go:106-112`; `internal/characters/conditions.go:157-165`; `modules/gmcp/gmcp.Char.go:804-859` |
| Condition ticks harm anonymously: `ApplyHarm(pool, -tickAmt, state.ActorRef{})` for health, stamina and conviction, mob and user | `internal/hooks/NewRound_MobRoundTick.go:248, 267, 273`; `internal/hooks/NewRound_UserRoundTick.go:331, 352, 358` |
| Zero-health backstops die anonymously: `Die(state.ActorRef{}, life.TriggerHealthZero)` | `NewRound_MobRoundTick.go:153`; `Condition_ApplyConditions.go:207` |
| `ApplyHarm(pool, amount, source ActorRef)` copies `source` into `CharacterDied.KillerUserId/KillerMobInstanceId` | `internal/characters/pools.go:644, 679-686` |
| `state.ActorRef{UserId, MobInstanceId}` with `IsZero`, `IsPlayer`, `IsMob` | `internal/state/transition.go:7-25` |
| `ResolveAggroTarget(ref)` tolerates a zero ref, a gone mob and an offline user (`Found: false`) | `internal/actions/combat_helpers.go:30-60` |
| User saves persist conditions (`Character.Conditions yaml:"conditions"`); mob instance saves carry no conditions, and `Mob.InstanceId` is runtime only (`yaml:"-"`) | `internal/characters/character.go:147`; `internal/mobs/instance_save.go:22-51`; `internal/mobs/mobs.go:88` |
| The double line: `applySpellConditionEffect` queues the condition event, then sends the spell trio ("Your %s takes effect on %s!" etc., with `critTag()`), under the comment "KNOWN AND DEFERRED ... M6 merges them" | `internal/hooks/spell_help_effects.go:80-111` |
| `ApplyConditions` sends `start_actee` to the holder and `start_observer` to the room via `SendTextVisualHidingNames`, judged on the pre-landing light snapshot | `Condition_ApplyConditions.go:128-183, 244-250` |
| Condition narration fills only `{actee}`; `TokenContext` already has `ActorName`/`ActorPlainName` (`{actor}`); no condition YAML uses `{actor}` | `internal/conditions/narration.go:24-25, 44-50`; `internal/textutil/tokens.go:14-30` |
| 20 spells use `effect_type: condition` (14 buffs, 6 harmful), not the ledger's 17; all 20 conditions have `start_actee`, 8 also `start_observer` | `_datafiles/world/dogmud/spells/*.yaml` `condition_ids` |
| Shield: `applySpellShield` lands hard-coded 119 via `AddConditionMagnitude`, bonus `(stat + round(skill*SkillWeight))/3 * magnitude/100`, min 1 | `spell_help_effects.go:171-190`; `internal/conditions/ids.go:13` |
| Heal: `applySpellHeal` lands hard-coded 120 with `regenMult = max(magnitude, 1)` for `calcSpellDuration/2` (min 6) | `spell_help_effects.go:124-141`; `ids.go:14` |
| 119 "Minor Shield" = `mitigation_flat: magnitude`, silent start, end "Your Minor Shield dissipates."; 120 "Regenerating" = `regen_mult: magnitude`, silent start | `_datafiles/world/dogmud/conditions/119-minor_shield.yaml`, `120-regenerating.yaml` |
| `mitigation_flat` feeds physical mitigation only; magical and conviction mitigation read gear, mutations and the `magical_mitigation`/`conviction_mitigation` statmods | `internal/characters/combat.go:154, 187-271` |
| Damage types physical, mental, social map to mitigation channels physical, magical, conviction | `internal/combatvocab/vocab.go:37-42`; `internal/combat/pools.go:51-61` |
| `spell.ConditionIds` is read only for `effect_type: condition`, so cocoon's `[52]` and mass-mend's `[33]` are dead; cocoon's help promises three kinds, mass-mend's promises regeneration | `spell_help_effects.go:83`; spells and help templates |
| 120 is also used by the mob consume path ("grafted corpse", "consumed corpse") | `internal/mobcommands/consume.go:46, 58` |
| Mob AI calls a target "shielded" when it `HasEffect(mitigation_flat)` | `internal/behaviortree/action_cast_best_in_category.go:214-229`; `internal/combat/ai.go:742` |
| Shield-like statmods that must keep stacking: 61 Ironhide Brew, 62 Mindshield Elixir, 63 Veilguard Tonic, 27 Iron Will, 104 Cocoon mutation | `_datafiles/world/dogmud/conditions/` |
| Heal-over-time items that must keep stacking: 5, 54 Healing Salve, 60 Elixir of Renewal, 42/57/58/90 | `_datafiles/world/dogmud/conditions/` |
| Highest condition id in use is 134 (dogmud) and 39 (default world) | directory listings |
| Local user saves 10, 11, 12 carry the trigger pattern "Your Minor Shield dissipates." | `_datafiles/world/dogmud/users/1[012].yaml` (untracked) |

## 1. The caster on a condition

**Data.** `conditions.Condition` gains `Caster state.ActorRef` (yaml `caster,omitempty`).
`events.Condition` gains the same field. Every door that lands a condition takes
it: the event path through `ApplyConditions`, `AddCondition`, `AddConditionScaled`,
`AddConditionMagnitude` and the character-level wrappers. Producers with no caster
(potions, hazards, mutations, mob consume) pass the zero ref, which is today's
behaviour. While threading the doors, `Source` reaches the record on every door,
not only the magnitude one.

**Persistence.** A player caster is stored by user id and survives a save. A mob
caster's `MobInstanceId` is runtime only, so it is not written to a save; on load
a condition with a mob caster reads as no caster.

**Refresh and replace.** A re-application by a different caster takes the new
caster (the newest application owns the record). A family replacement (section 3)
carries the new record's caster.

**Credit (#240).** The tick paths pass the record's `Caster` to `ApplyHarm` for
health, stamina and conviction ticks, for players and mobs. The two zero-health
backstops pass the caster of the condition whose tick brought health to zero when
one did. Kill credit then flows through the existing paths (`DamageMap`,
`Death_MobKillCredit`, quest kills, faction rep, crimes, bounties) with no new
credit rule.

**Absent casters (R10).** Credit is recorded for a player caster who has moved
away, logged out or died since the cast: kill credit, quest credit and the crime
record if a witness identified the attack. Effects that need a live actor (procs,
on-kill triggers that act on the caster's body or room) are skipped when the
caster is not online. A mob caster that no longer resolves credits nobody. The
plan must find each credit consumer and state which half it falls in; any
consumer that cannot take an offline player is called out in the plan, not
silently skipped.

**Crime.** A DoT kill is a murder by the caster if the assault that applied it was
identified, using the existing assault-to-murder upgrade (`FindRecentAssault` and
`FindRecentUnknownAssault`, which match the victim since #434). The kill happening
in a different room or after the caster left changes nothing about who is
recorded.

## 2. One line per audience (ledger row 1)

**Data.** `ConditionSpec` gains `start_actor`: the caster's line when the caster is
someone other than the holder. `start_actee` and `start_observer` may use
`{actor}`. Condition narration fills `{actor}` from the record's caster through
the sight-aware name path, so a holder or observer who cannot see the caster reads
the hidden form, never the name.

**Rule (R11).** When a spell lands a condition that has authored start text, the
spell's generic trio in `applySpellConditionEffect` is not sent. The condition's
lines are the only lines:

| Case | Caster reads | Holder reads | Room reads |
|---|---|---|---|
| Self-cast | `start_actee` | (same person) | `start_observer` |
| Cast on another | `start_actor` | `start_actee` | `start_observer` |

The crit marker the trio carries today (`critTag()`) moves to the caster's line.
The trio is dropped only when the condition's start lines will actually be sent.
So a condition with no start text (silent start) keeps the generic trio, and so
does a re-cast of a condition already active on the holder: its start lines are
refresh-suppressed as today, and the trio's "takes effect" line tells each
audience the spell renewed it. Every cast therefore prints exactly one line per
audience.

**Content.** Every one of the 20 spell-applied conditions gets a `start_actor`
line, and its `start_actee`/`start_observer` lines are reviewed so that they read
right whether the caster is the holder or someone else. The new shield and heal
conditions (sections 3 to 5) are authored with all three lines, so they use this
same rule instead of the spell trio.

**Darkness.** The pre-landing light snapshot that judges `start_observer` today
also judges `start_actor` and the `{actor}` name in every line.

## 3. Condition families

**Data.** `ConditionSpec` gains `family` (string, optional). This slice ships two
values: `ward` and `heal`.

**Rule (R4, R6).** Landing a condition whose family is set removes every other
active condition of the same family on that holder first, then lands the new one.
The holder reads a replacement line instead of the old record's end line, for
example "Your Conviction Ward fades as Chrysalis Cocoon takes hold." The room
reads the observer form with names through the sight path. Re-landing the same
condition id is a refresh, not a replacement, and keeps today's refresh rule.

Conditions with no family are untouched, so potions, salves, items and mutations
keep stacking with spell wards and heals exactly as today (R4).

The mob AI's "already shielded" test becomes "has an active `ward` family
condition", replacing `HasEffect(mitigation_flat)`. Its heal choice reads the
`heal` family the same way.

## 4. Wards (R3, R7)

**Effects.** Two new effect kinds join `mitigation_flat` (physical):
`mitigation_magical` and `mitigation_conviction`, each read with `magnitude` and
added into magical and conviction mitigation alongside the existing statmods, so
a potion's `magical_mitigation` still sums on top.

| Spell | Cost | Blocks | Strength |
|---|---|---|---|
| Conviction Ward (`conviction-ward`, existing) | cheap | physical | weak |
| Conviction Bulwark (new spell) | medium | physical and spell | medium |
| Chrysalis Cocoon (`chrysalis-cocoon`, existing) | expensive | physical, spell and social | strongest |

Each ward lands its own condition in the `ward` family, named after its spell.
Its description says in words which damage it blocks, and its start and end lines
are authored per section 2. `applySpellShield` stops hard-coding 119 and lands the
spell's own condition; the shield formula stays and feeds each kind the ward
covers. The exact magnitudes, durations and costs are set in the plan's dry run
against the existing shield formula, holding the ordering above.

**Retiring "Minor Shield".** Condition 119 is renamed to the Conviction Ward
condition rather than deleted, so a save that holds 119 still loads. Its old
expiry line goes away, which breaks player triggers that match "Your Minor Shield
dissipates." and web triggers on "status has Minor Shield". The patch notes say
so in player terms. Cocoon's dead `condition_ids: [52]` is removed; 52 Chrysalis
Shell has no other applier and is left in place, unchanged.

## 5. Heals (R3, R8, R9)

Each heal spell lands its own condition in the `heal` family with `regen_mult`,
its own multiplier and duration. `applySpellHeal` stops hard-coding 120.

| Spell | Target | Identity |
|---|---|---|
| Mend Flesh (`heal`) | single | long and gentle: low multiplier, long duration |
| Mend Wounds (`mend-wounds`) | single | short and strong: high multiplier, short duration |
| Mend All (`mend-all`) | area | short and light |
| Communion of Flesh (`communion-of-flesh`) | area | long and steady |
| Mass Mend (`mass-mend`) | area | short and strong, the top area heal |
| Repair Pulse (`repair-pulse`, mob only) | single | as today, on its own condition |

Chrysalis Regeneration (33) and Vital Surge (32) keep their own effects (a
percentage of max health per tick) and join the `heal` family, so any heal spell
replaces any other. Condition 120 Regenerating stays for the mob consume path and
is not in the family. Mass Mend's dead `condition_ids: [33]` is removed.

The exact multipliers and durations are set in the plan's dry run, holding the
identities above. Area heals land the same condition on each target with the
caster recorded on each.

## 6. Help and copy that must match the code

- Chrysalis Cocoon's help (three kinds) becomes true.
- Mass Mend's help drops the regeneration promise.
- Mend Flesh's help stops calling the heal instant.
- Conviction Ward's help shows the real base folds and says physical only.
- A help file for Conviction Bulwark.
- `help` pages for the shield and heal families state the no-stacking rule.
- `docs/PATCH_NOTES.md`: player framing, no raw numbers, no dashes, including the
  trigger warning in section 4.

Player copy follows the dogmud-player-copy rules: no em or en dashes, 80
columns, no raw numbers, no line ending in a colon.

## 7. Display

The `conditions` command and the GMCP `Char.Conditions` map already show a
condition's name and description, so per-spell conditions are enough for "a
player can tell which ward or heal is on them". GMCP `Type` shows the real
`Source` once every door carries it. No web client change is needed.

## Out of scope

- Quest Actee text (moved to #375 by the owner's 2026-10-07 answer to #370 Q2).
- Ledger rows 36 to 59 and the triage of the other open rows (M6 slice 2).
- Reflect protection and other new ward riders.
- The per-tick heal pool model (rejected in favour of R8).
- A caster ref for mobs that survives a restart.

## Testing

- **Caster:** threading through every door; persisted for a player caster;
  dropped for a mob caster on load; zero ref for potions and hazards.
- **Credit:** a DoT kill credits the caster for kill, quest and crime, with the
  caster present, moved away, logged out and dead; no proc for an offline caster;
  a mob caster gone after restart credits nobody.
- **Lines:** for each of self-cast and cast-on-another, exactly one line per
  audience, the crit marker on the caster line, `{actor}` hidden for a holder or
  observer who cannot see the caster, and the generic trio kept for a silent
  condition and for a re-cast of an active one.
- **Families:** the newest ward replaces the older with the replacement line,
  refresh of the same ward is unchanged, a potion's mitigation still sums with a
  ward, and the same for heals with a salve.
- **Wards:** each ward reduces exactly the damage kinds it lists and no others.
- **Heals:** each heal's multiplier and duration match its identity ordering.
- **Guard:** a test that every spell's `condition_ids` is actually applied by its
  effect type, so dead data like cocoon's `[52]` cannot return.
- Boot check and the full gates.

## Gate

The arc spec ends M6 with an adversarial playtest. This slice's playtest covers:
a DoT kill in a dark room credited to a caster who walked away; a cast on another
with one line per audience; a ward swap and a heal swap with the replacement
line; each ward against physical, spell and social attacks; and a potion stacking
with a ward and a salve with a heal.
