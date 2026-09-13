# Conditions unification, slice 1: one model for timed state

Date: 2026-09-12. Branch `feature/conditions-unification-slice-1-model` off
master `230041292`. First of three slices in the conditions unification arc,
ordered by the owner on 2026-09-12: **model, then rename, then wire**.

The owner's complaint (2026-08-15): "I don't really care for the duality of
buffs and combat conditions in the codebase. It kind of sent me down a lot of
alleys." Ruling 2026-09-12: "They really should be one thing", called
**conditions**, and the documentation must make sure no new buff or other
parallel path gets created.

## Owner rulings taken during this design (2026-09-12, do not relitigate)

- **Buffs absorb conditions.** The Go enum and its tick path are deleted; the
  ten conditions become YAML records; the unified thing is called a condition.
- **The rename goes all the way**, including the 13 YAML key spellings and the
  player save format, but as its own slices: slice 2 is the Go and
  player-facing rename, slice 3 the wire format and save migration. This slice
  keeps today's Go names (`buffs`, `BuffSpec`, `Buff`) so the behaviour change
  and the rename never share a diff.
- **GMCP: one payload, `Char.Conditions`**, carrying the richer Affects shape;
  `Char.Affects` is retired and the web client's two panels read the one list.
- **Scaling stays in the appliers.** The record carries base values; each
  applier hands a duration multiplier and a magnitude through ONE door. Which
  formula computes them is unchanged and belongs to the spell scaling arc.
- **The two one-round penalties stay visible** in the list, as today, and are
  silent at both ends through a new flag rather than announcing themselves
  every round.
- The playtest may reshape the testers: loadouts, known spells, starting rooms.

## Facts verified against source

Every claim below was read from the tree at `230041292` on 2026-09-12.

| Fact | Where |
|---|---|
| `ConditionType` is a ten-value iota enum; `CombatCondition{Type, Duration int, Magnitude float64, Source string}`; API `AddCondition` (returns bool, refuses poisoned under `PoisonImmunity`), `HasCondition`, `GetConditionMagnitude`, `RemoveCondition`, `DecrementCondition`, `GetConditionDuration`, `TickConditions` | `internal/characters/conditions.go` (203 lines) |
| `Character.Conditions []CombatCondition` is tagged `yaml:"-"`: **conditions do not survive a logout or restart**; `Character.Buffs` persists | `internal/characters/character.go:145,300` |
| The death cascade clears the slice (`c.Conditions = nil`) | `internal/hooks/Life_Cascades.go:78` |
| `TickConditions` runs once per round for every player and mob | `internal/hooks/NewRound_UserRoundTick.go:370`; `NewRound_MobRoundTick.go:599` |
| 🐛 **Minor Shield decays twice per round in combat**: `TickConditions` decrements every timed condition, and `handlePlayerShieldDecay` / the mob branch decrement it again while in combat; the "dissipates" line exists only on the combat path, so a shield lapsing out of combat ends silently | `internal/hooks/NewRound_DoCombat_helpers.go:508-517`; `NewRound_DoCombat.go:308-316`; `conditions.go:187-203` |
| 🪦 **`ConditionBlinded` has no producer.** Its readers (`GetDefenseScore` dodge multiplier, `HasAnyBlindSource`) exist; nothing anywhere under `internal/` or `modules/` calls `AddCondition(ConditionBlinded, ...)`. Perception is driven by buffs 3 and 77 | grep; `internal/characters/combat.go:291`; `sight.go:19-44`; `internal/state/perception/transitions.go:27-28` |
| Poison immunity is checked in BOTH systems: `Buffs.AddBuffScaled` refuses a `poison`-flagged buff, `AddCondition` refuses `ConditionPoisoned` | `internal/buffs/buffs.go:245`; `conditions.go:96-99` |
| `condition-mirror` is a display-only flag carried by exactly two files (79, 80) and read by exactly two loops that skip the record | `internal/buffs/buffspec.go:85-90`; `internal/usercommands/conditions.go:44`; `modules/gmcp/gmcp.Char.go:588`; the two YAML files |
| Production callers by API (excluding the definitions): `AddCondition` 29 sites in 8 files; `HasCondition` 23 in 8; `GetConditionMagnitude` 14 in 4; `RemoveCondition` 4; `DecrementCondition` 2; `GetConditionDuration` 2; `TickConditions` 2 | grep, non-test |
| Consumers: swing count caps to 1 under `RecoveryPenalty`; damage mean and crit base × (1 + warcry magnitude); defense score × (1 + rally magnitude) and × grapple-exposure magnitude; physical mitigation adds `int(shield magnitude)`; dodge × blinded magnitude (dead); regen hook multiplies `HealthPerRound()` by regen magnitude when > 1 out of combat and unconditionally in combat, and emits "Your wounds knit closed."; poison and bleed apply `int(magnitude)` (min 1) health harm with a fixed line each, then `cancelCraftOrSalvageOnDamage` and `cancelDamageBuffs`, **on every THIRD round, not every round** (corrected during execution): the whole `AutoHeal` body is behind `if evt.RoundNumber%3 != 0 { return }` (`NewRound_AutoHeal.go:34`) while the duration these conditions counted down ran every round, so a duration of `n` landed about `n/3` hits and the exact count depended on the round the condition was applied on; `Validate` subtracts `floor(max × magnitude)` from the pool named by `Source` for withdrawal, after the reservation clamp | `internal/combat/combat_helpers.go:232,491-494,745-757`; `internal/characters/combat.go:185,291`; `internal/hooks/NewRound_AutoHeal.go:191-255,332-352,400-415`; `internal/characters/validate.go:187-221` |
| Other readers: mob AI casts a ward when unshielded; the behaviour tree skips a shield spell when shielded; death cause reads poisoned then bleeding; Purge Affliction cancels `poison`-flagged buffs AND removes the poisoned condition | `internal/combat/ai.go:741`; `internal/behaviortree/action_cast_best_in_category.go:210-225`; `internal/hooks/Death_PlayerAnnouncement.go:124-130`; `internal/hooks/spell_purgeaffliction.go:101-102` |
| Producers and their numbers: warcry and rally bonus = clamp(0.05 + 0.15 × sqrt((rhetoric/75) × (charisma/175)), 0.05, 0.20), × (1 + shout amp); duration 25 × (1 + amp); applied to self, party members in the room, and charmed mobs, each time as `AddCondition` PLUS `AddBuff(79 or 80)` | `internal/actions/combat_warcry.go:98-119`; `combat_rally.go`; `internal/usercommands/warcry.go:50-60,112-124`; `rally.go` |
| Shield: `(caster stat + round(spellcasting × SkillWeight)) / 3`, min 1, × `effect_magnitude` / 100 when set, × 1.5 on crit; duration `calcSpellDuration(baseFolds, skill, stat)`; player and mob targets | `internal/hooks/spell_resolution.go:1147-1165,1521` |
| Regen: `effect_magnitude` floored at 1.0, crit doubles the part above 1; duration `calcSpellDuration / 2` floored at 6; mob corpse feeding sets 2.0 for 6 rounds, a flesh golem 3.0 for 10 | `spell_resolution.go:1047-1062,841,1482`; `internal/mobcommands/consume.go:46,55` |
| Poison (spell dot): magnitude = `effect_magnitude` as flat damage per round; duration `calcSpellDuration / 3` floored at 3 | `spell_resolution.go:615-637,1640-1653` |
| Bleeding: maul strength/8 min 3 for 5 rounds; hamstring for 5; rake for 4; drain strength/12 min 2 for 4; throttle strength/10 min 2 for 3; item proc `magnitude` param min 2, `dur` default 4 | `internal/actions/combat_{maul,hamstring,rake,drain,throttle}.go`; `internal/hooks/item_procs.go:205-215` |
| Recovery penalty: `AddCondition(RecoveryPenalty, 1, 1.0)` on every recovering round; grapple exposure: `AddCondition(DefensePenalty, 1, 0.85)` on a weak failure; withdrawal: `AddCondition(EnchantWithdrawal, config rounds, reservePct, reservePool)` | `internal/characters/skills.go:73,96,100`; `internal/combat/grapple_move.go:56`; `internal/usercommands/skill.disenchant.go:62-72` |
| `calcSpellDuration(baseFolds, spellcasting, stat) = round(folds × (10 + stat/20 + skill/2))`, min 10 | `spell_resolution.go:35-44` |
| The buff instance: `Buff{BuffId, Source, OnStartWaiting, PermaBuff, RoundCounter, TriggersLeft, TickAmount}`; `TickAmount` is a per-instance SNAPSHOT set by `SetTickAmount` on the most recently added buff of that id | `internal/buffs/buffs.go:13-23,475` |
| The spec: `BuffSpec{BuffId, Name, Description, Secret, TriggerNow, TriggerRate, RoundInterval, TriggerCount, StatMods, Flags, ProgressMult, six text fields, TickPool, TickPercent, TickVariance, TickMin, StartRemoveBuffs}`; statmods are summed ints; flags carry no strength (readers test presence and apply a fixed or config number) | `internal/buffs/buffspec.go:150-190`; `buffs.go:84-90`; `combat_helpers.go:217,482`; `NewRound_DoCombat_unified.go:364` |
| `Buffs.Trigger` ticks records with `RoundInterval >= 1` only; the player tick applies `TickAmount` (computing and caching it when 0) with ApplyRestore for positive and ApplyHarm for negative; the mob tick mirrors it | `buffs.go:359-408`; `NewRound_UserRoundTick.go:296-345`; `NewRound_MobRoundTick.go` |
| 🪤 `Buffs.HasFlag(flag, expire=true)` MUTATES (expires matching buffs); the door must never use it | `buffs.go:122-190` |
| `events.Buff{UserId, MobInstanceId, BuffId, Source, DurationMult}`; `UserRecord.AddBuffScaled(id, mult, source)` queues it; `Buff_ApplyBuffs` applies and narrates the start through `StartUserNotice` | `internal/events/eventtypes.go:20-31`; `internal/users/userrecord.go:436-449`; `internal/hooks/Buff_ApplyBuffs.go` |
| Scaling today lives in the appliers, not the YAML: potions scale duration by aging potency (age, bottle multiplier, craft skill) × (1 + craft skill/100) and compute the tick at 1.0; spell `effect_type: buff` scales the tick by `SkillMultiplier(spellcasting)` × weapon spell damage × gear effectiveness and does NOT scale duration; shield, regen and dot conditions use `calcSpellDuration` and `effect_magnitude` | `internal/usercommands/drink.go:145-157,250-287`; `spell_resolution.go:1085-1109,762,1492` |
| The `conditions` command lists buffs (skipping `hidden` and `condition-mirror`) then the condition slice with `DisplayName`/`Description` and rounds left; the template shows a qualitative duration | `internal/usercommands/conditions.go`; `_datafiles/world/dogmud/templates/character/conditions.template` |
| GMCP: `Char.Affects` is a name-keyed map of `{Name, Description, DurationMax, DurationLeft, Type, Mods}` built from buffs (skipping hidden and mirror); `Char.Conditions` is a list of `{Type, Description, Duration word}` built from the slice; the web client builds statuses from `Char.Affects` and its status panel from `Char.Conditions` | `modules/gmcp/gmcp.Char.go:562-631,719-731,776-800`; `_datafiles/html/public/webclient-pure.html:2136-2189`; `static/js/renderguard.js:6-11` |
| The notice guard requires authored start and end text on every non-secret dogmud buff, with two flag exemptions: `silent-start` (start only) and `hidden` (end only, stealth semantics) | `buff_notice_guard_test.go:17-30` |
| Existing overlap records: 3 Blinded (perception -40, 3 rounds), 77 Flashbang Blindness, 39 Venom and 115 Rending Bleed (health ticks with variance), 84 Stunned (statmods), 92 Bloom Withdrawal (statmods), 33 Chrysalis Regeneration (positive tick) | `_datafiles/world/dogmud/buffs/` |
| Shipped statmod keys across buffs: six stats, three recoveries, `magical_mitigation`, `conviction_mitigation`, `physical_mitigation`, `defense`, `combatmodifier` | grep |

## What this slice delivers

One collection of timed state on a character, data-authored, persisted,
narrated through the doors M3 item 5b built, displayed once, read by combat
through one function. The enum, its tick, the double-decaying shield and the
mirror flag are gone. Every live number is unchanged, proven by a test that
runs both systems side by side before the old one is deleted.

## Design

### The record

The instance gains one field:

```go
type Buff struct {
	...
	TickAmount int     `yaml:"tickamount,omitempty"` // unchanged
	Magnitude  float64 `yaml:"magnitude,omitempty"`  // per-instance strength the spec's effects read
}
```

`omitempty` means a save written before this slice loads unchanged and a save
written after loads on the previous build; slice 3 owns the format proper.

The spec gains one map:

```yaml
effects:
  damage_mult: magnitude     # or a literal number
  defense_mult: 0.85
  dodge_mult: magnitude
  regen_mult: magnitude
  mitigation_flat: magnitude
  pool_max_pct: magnitude    # the pool rides on the instance's Source, as today
  attacks_cap: 1
```

Seven keys, closed. `Validate` refuses any other key and a non-numeric value
other than the word `magnitude`. (A third rule was proposed here and struck
during execution: "`effects` on a record that also declares a statmod for the
same concern". The two vocabularies do not overlap key for key, so "the same
concern" had no mechanical definition to test, and the rule would have had
nothing to refuse. What `validateEffects` DOES refuse besides the two above is
`tick_from_magnitude` without a `tick_pool`, or alongside a non-zero
`tick_percent`.) A value of `magnitude` on an instance whose magnitude is 0
contributes nothing (multipliers read 1, flats read 0, caps are ignored).

### The door

```go
// Effect combines every held, unexpired record's contribution for one kind.
// Multipliers multiply (identity 1), flats and pool fractions sum, caps take
// the minimum (0 meaning none). Never touches HasFlag's expire form.
func (bs *Buffs) Effect(kind EffectKind) float64
func (bs *Buffs) HasEffect(kind EffectKind) bool
```

Consumers move onto it one per line:

| Today | After |
|---|---|
| `HasCondition(RecoveryPenalty)` → swings = 1 | `if cap := Effect(AttacksCap); cap > 0 { swings = min(swings, cap) }` |
| `1 + GetConditionMagnitude(Warcry)` | `Effect(DamageMult)` (the record stores 1 + bonus) |
| `1 + GetConditionMagnitude(Rally)` | `Effect(DefenseMult)`, which also folds the 0.85 exposure |
| `int(GetConditionMagnitude(Shield))` | `int(Effect(MitigationFlat))` |
| regen `if mag > 1 { × mag }` / in combat `× mag` | `Effect(RegenMult)` with the same two branches |
| withdrawal loop over `Conditions` by `Source` | loop over records with `pool_max_pct`, pool from `Source` |
| mob AI `HasCondition(Shield)` | `HasEffect(MitigationFlat)` |
| death cause `HasCondition(Poisoned)` / `Bleeding` | `HasBuffFlag(Poison)` / `HasBuffFlag(Bleeding)` |
| purge `RemoveCondition(Poisoned)` | deleted; `CancelBuffsWithFlag(Poison)` already ran |

### The producers

One door, at the character and on the event:

```go
func (c *Character) AddBuffMagnitude(buffId int, durationMult float64, magnitude float64, source string) error
// events.Buff gains Magnitude float64; Buff_ApplyBuffs passes it through.
```

Every `AddCondition` site becomes one call, with the record id and the exact
magnitude and duration it computes today. Warcry and rally stop calling
`AddBuff(79)` next to `AddCondition`: the single record carries the magnitude.
Poison and bleed producers set the tick snapshot to the same integer they
passed as magnitude, so the per-round damage is unchanged to the unit.

### The ten migrations

| Condition | Record | `effects` | Flags | Notice |
|---|---|---|---|---|
| Warcry | 79 (existing) | `damage_mult: magnitude` | drop `condition-mirror`; keep `silent-start` | authored today |
| Rally | 80 (existing) | `defense_mult: magnitude` | same | authored today |
| Grapple exposure | new | `defense_mult: 0.85` | `quiet` | none; listed for its round |
| Prone recovery | new | `attacks_cap: 1` | `quiet` | none; listed for its round |
| Minor Shield | new | `mitigation_flat: magnitude` | | "dissipates" is the end line, sent everywhere; start line is the spell's existing narration (`silent-start`) |
| Regenerating | new | `regen_mult: magnitude` | `silent-start` (the heal spell narrates) | short end line; the regen hook keeps its tick line |
| Poisoned (spell) | new | tick: `tick_pool: health`, snapshot = magnitude | `poison`, `silent-start` | "The poison burns through your veins!" as `trigger_user_text`; end line authored |
| Bleeding | new | tick record, snapshot = magnitude | `bleeding` (new), `silent-start` | "Blood seeps from your wounds!" as trigger text; end line authored |
| Enchant withdrawal | new | `pool_max_pct: magnitude` | | authored start and end |
| Blinded (dodge) | none | | | dead code deleted; buffs 3 and 77 keep driving perception |

A `quiet` flag is new: listed, no start or end line. The notice guard accepts
it. It exists for records reapplied every round they persist, where any line
would repeat each round.

### Display

- The `conditions` command lists the one collection; the mirror skip is gone.
- `Char.Conditions` carries the unified list in the Affects shape, plus the
  qualitative duration word the status panel uses today. `Char.Affects` is
  retired: its builder is deleted, the identifier strings updated, the web
  client's statuses read `Char.Conditions`. A client asking for `Char.Affects`
  gets nothing, as it would for any unknown module.

### Deleted

`ConditionType`, `CombatCondition`, `Character.Conditions`, the seven methods,
`TickConditions` and both call sites, `handlePlayerShieldDecay` and the mob
shield branch, `ConditionMirror` and both skips, the `Life_Cascades` clear, the
dead blinded readers, the duplicate poison immunity check.

### Behaviour changes this slice makes, stated rather than hidden

1. Minor Shield lasts its authored duration (it decayed twice per round in
   combat) and narrates its end out of combat too.
2. Former conditions persist across logout and restart, like every record.
3. Poison and bleed damage moves from the regen hook to the tick path, so it
   lands EARLIER in the round (the round ticks run before `DoCombat`; AutoHeal
   ran after it). The cadence is unchanged: records 121 and 122 ship
   `triggerrate: 3 rounds`, and every producer converts its rounds-literal
   duration with `buffs.TickTriggers(rounds)` (`rounds/3`, minimum 1), which
   reproduces the `RoundNumber%3` gate the hook had. The per-tick integer is
   identical. What DOES change is the exact hit count, because the old one was
   phase-dependent: a 4 or 5 round bleed landed 1 or 2 hits depending on which
   round it started, a 20 round dot 6 or 7. The record always lands the floor
   of that range, and its first tick comes 3 rounds after application rather
   than on the next round divisible by 3. Whether the cadence SHOULD be every
   third round is a filed owner call for a balance pass; this slice reproduces
   what shipped.
4. The two one-round penalties stop appearing in `Char.Conditions` as bare
   words and appear as records; no line is sent for them.
5. Every damaging tick (Venom, Spore Toxin, Rending Bleed and the new records)
   now wakes a sleeper and cancels `cancel-on-damage` records, as the poison
   and bleed hook already did; the tick path never called those two helpers.
6. The poison and bleed tick lines go out on the record category
   (`CategoryBuffApply`) instead of `CategoryToxin`: a colour change only.
7. **A pre-existing PLAYER-side bug is fixed on the way**, and it changes live
   numbers for buffs this slice never touches. `UserRoundTick` gated the whole
   tick body, harm included, on `!buff.Expired()`, so a record's FINAL trigger
   applied nothing; the mob tick never had the defect. Every player-held tick
   record now lands one more tick: 97 Arc Trap 1 becomes 2 (+100%), 89
   Throttled 2 becomes 3 (+50%), 39 Venom / 40 Spore Toxin / 78 Toxic Cloud 4
   become 5 (+25%), 5 Minor Potion Healing 5 becomes 6 (+20%), 50 Greater
   Healing 9 becomes 10 (+11%). Fixing it was not optional: `TickTriggers`
   yields exactly one trigger for every ordinary bleed, so a bleed would
   otherwise have applied no damage at all.
8. Regenerating and Minor Shield narrate their END. Neither did before: the
   regen condition ended silently, and the shield's "dissipates" line existed
   only on the combat decay path.
9. A death by a Venom, Spore Toxin or Toxic Cloud tick now names "poison". The
   enum only knew its own spell dot, so those three killed while reporting
   "their own foolishness". The cause is carried by `Character.LastTickCause`,
   stamped where the harm lands.
10. The conditions list and `Char.Conditions` show the INSTANCE's real
    remaining duration. `GetDurations` read the spec's authored
    `TriggerCount`, so a 50-trigger Enchant Withdrawal on a record authored
    with 10 reported a negative duration.
11. `Char.Conditions[].type` is now the record's `Source` (the applier's
    string: "spell", "heal spell", "prone recovery"), which is what
    `Char.Affects` always carried there. The retired `Char.Conditions` list
    put `ConditionType.DisplayName()` there; a client switching on `type` to
    identify a condition must stop.

Recorded by the whole-branch review (2026-09-12), not called out above when
each landed:

(a) A player's Minor Shield end now also sends the room line
    (`end_room_text`); the deleted combat-decay helper sent a user line only.
(b) The two quiet penalties' displayed names changed from "Defense
    Penalty"/"Recovery Penalty" to "Off Balance" (117) and "Recovering" (118).
(c) `Char.Conditions` changed from a JSON array to a name-keyed map carrying
    the same keys `Char.Affects` used; a third-party client iterating it as
    an array breaks.
(d) `GetDurations` now reads the instance (`buff.TriggersLeft`) for EVERY
    buff, not only former conditions, so any record applied with a
    non-default trigger count (a scaled potion, a spell buff) shows a
    different remaining duration than the spec's authored `TriggerCount`
    would have reported.
(e) `messaging.CategoryToxin` has no sender left; its ansi alias ("toxin")
    and channel setting are inert now that the poison and bleed tick lines
    ride `CategoryBuffApply` (change 6 above).
(f) The trigger line lands on every tick, including the last one that also
    expires the record (this commit; see the execution findings below).

Everything else is number-identical, and the equivalence test proves it.

### Findings recorded while planning (2026-09-12)

- 🪦 **The prone recovery penalty never reaches combat today.** `UserRoundTick`
  applies it at the stand attempt (`NewRound_UserRoundTick.go:246`) and its
  own `TickConditions` at line 370 decrements the one-round duration to zero
  and removes it, all before `DoCombat` runs (hooks register in the order
  `UserRoundTick`, `MobRoundTick`, then `DoCombat`; `hooks.go:43-49`). The mob
  tick has the same shape. The record migrates it FAITHFULLY (one trigger,
  expired by the same tick's `Trigger`, skipped by the door), so the swing cap
  stays inert; making it bite is an owner call, filed.
- The tick path applies `TickAmount` through `ApplyHarm` but never calls
  `cancelDamageBuffs` or `cancelCraftOrSalvageOnDamage`; the poison hook did.
  Change 5 above.

### Findings recorded during execution (2026-09-12)

Seven things the plan did not know. Each one changed the work.

- 🪤 **The AutoHeal gate.** The planning fact table said poison and bleed
  applied their harm "per round". `NewRound_AutoHeal.go:34` returns early on
  `evt.RoundNumber%3 != 0`, so the whole hook, dot included, ran on every
  third round while the duration ticked down every round. Task 8 migrated the
  dot at a one-round `triggerrate` and passed the rounds figure straight
  through, tripling its damage; caught and fixed by moving records 121 and 122
  to `triggerrate: 3 rounds` and adding `buffs.TickTriggers`. The lesson is
  the standing one: the cadence of a mechanism is not readable from the
  mechanism, it is readable from its caller's gate.
- 🐛 **The final-trigger hole, player side only.** `UserRoundTick` gated harm
  and flavour text together on `!buff.Expired()`, and `Buffs.Trigger`
  decrements before returning, so a record's last trigger applied nothing.
  Invisible for years because every existing tick record had a trigger count
  far above 1; the one-trigger bleed `TickTriggers` produces made it fatal.
  Behaviour change 7 above lists the seven buffs whose damage or healing this
  raised.
- 🐛 **The death cause had two independent holes**, both from the same root:
  the killing tick's record is already expired. The by-flag read skipped
  expired records, and `PruneBuffs` runs on every `NewTurn` and could remove
  the record entirely before the QUEUED death event was handled. A same-tick
  fix would not have survived the prune. Fixed with a by-id read plus the
  `LastTickCause` / `LastTickCauseRound` stamp.
- 🪤 **`GetDurations` read the SPEC, not the instance.** It computed
  `TriggerCount * RoundInterval - RoundCounter` from the authored default, so
  a 50-trigger Enchant Withdrawal on a record authored with 10 reported a
  negative duration in the list and in GMCP. It now reads `TriggersLeft`.
- 🪤 **The fixture trap.** `Character.AddBuffMagnitude` calls `Validate`,
  which calls `RecalculateStats`, which overwrites `.Value` from
  `Base + Training + Mods`. A pin that set only `StatInfo.Value` had its
  fixture erased on the first call and then multiplied its withdrawal
  fraction against a floor of 1. Seed `.Base`.
- 🪤 **The apply-path guard cannot see the prone recovery producers.**
  `buffAddCallPattern`'s `primitivePackages` exempts `internal/characters`
  wholesale, because that is where `Character.AddBuff` is defined; the three
  `AddBuffMagnitude(BuffIdRecovering, ...)` calls in
  `internal/characters/skills.go` (lines 76, 99, 103) sit inside that
  exemption and are therefore neither reported nor allowlisted. The guard was
  taught to tell the character door from the user door by method name during
  this slice, but the package exemption still stands and is the reason those
  three sites are invisible to it. Not a defect introduced here; worth
  knowing before trusting its allowlist as a complete census of direct adds.
  The whole-branch review corrected the guard's own header comment to say so
  explicitly rather than implying every producer is hand-allowlisted.
- 🔑 **`AddBuffMagnitude`'s second argument counts TRIGGERS, not rounds.** It
  is assigned straight to `TriggersLeft`. For a one-round-interval record the
  two coincide, which is why the plan's "exact rounds" wording survived
  review; for the three-round dot and bleed it is a 3x error. Renamed
  throughout, with `events.Buff.Rounds` becoming `events.Buff.Triggers`.

### Findings recorded by the whole-branch review (2026-09-12)

- 🐛 **The Task 9 text gate silenced every one-trigger bleed.** Task 9 fixed
  the harm half of the final-trigger hole (finding above) but left the
  flavour text gated on `!buff.Expired()`. `buffs.TickTriggers(rounds)`
  returns 1 for every `rounds` under 6, and every combat bleed producer
  passes 3, 4 or 5, so a mauled player took a silent tick and read only the
  record's end line ("Your wounds stop bleeding.") with no "Blood seeps from
  your wounds!" ever shown. Fixed by dropping the expiry gate from the text
  condition in both `NewRound_UserRoundTick.go` and
  `NewRound_MobRoundTick.go`, so the trigger line lands whenever the tick
  fires and the end line still follows as its own, later line.
- 🐛 **The disenchant start line was undelivered.** Record 123 (Enchant
  Withdrawal) carries `start_user_text`, but `Disenchant` applies it through
  `Character.AddBuffMagnitude`, which is synchronous and never travels
  `events.Buff`, so `Buff_ApplyBuffs`' start notice never ran and the line
  reached no one. Fixed by having `Disenchant` render and send the line
  itself through `buffs.AuthoredStartLine`, the same door sleep (15), arrest
  (88), stun (84) and broken limb (83) already use for the same reason.

## The net

- **Pins first.** Tests written against the OLD code with literal
  expectations for the live numbers (mitigation from a shield of 12, pool
  maxima under a withdrawal fraction, the swing cap, the poison tick and the
  death cause, the regen multiplier). Each migration task rewrites only the
  SETUP lines of its pin to the record API and must keep every literal; a
  reviewer who sees a literal change has found a number change. The producer
  side is checked by reading: every task's replacement carries the exact
  duration and magnitude expression it replaces, and the reviewer compares
  them line by line. Durations are passed as exact integer rounds, never a
  multiplier (`3.3 × 10` is 32.999 in binary).
- **Goldens**: `buffs.golden` gains the new records' rows; recorded once in
  Task 0 after the records exist and before any site moves; `-update` forbidden
  after that.
- **Round-order test**: a character at 1 health with a poison record dies the
  same round as before and the death cause reads "poison".
- **Guards**: the compiler enumerates every consumer once the enum is deleted.
  A root guard forbids any new timed-state collection on `Character` (a field
  of slice type whose element carries a `Duration`) and any `HasCondition`
  spelling. The notice guard learns `quiet`. The flag guard learns `bleeding`
  and `quiet`. The existing store-field guard and render-door guard are
  unchanged.
- **Content**: six new records with lines under the copy rules; the shield
  and the two dot lines reuse today's wording.
- **Playtest lane** (the testers may be reshaped: a rhetoric-trained shouter,
  a caster with a ward, a heal and a poison spell, a mid-strength fighter with
  maul, staged in Sable's arena): warcry and rally in a fight that survives
  rounds, with the list showing one entry each; a ward cast then rested past
  its duration out of combat, with the end line arriving; a poison spell at a
  mob and a maul on a player, with ticks and, for one victim, the death cause;
  a logout and login mid-condition, with the record still held.

## Out of scope, filed

- Slice 2: the Go and player-facing rename; slice 3: YAML keys and saves.
- Scaling formulas and the two gaps above (potion tick ignores potency; buff
  spells get no duration scaling): the spell scaling arc, which now has one
  seam to plug into.
- Per-spell ward and heal names in the list (owner ask, 2026-08-15): content,
  now possible because each spell can name its own record.
- The GMCP dark-room leak: its own slice.
- `internal/hooks` shuffle-order flakes: pre-existing, filed with 5b.

## Documentation

There is no `internal/conditions` package today and so no `context.md` for
conditions: the enum lives in `internal/characters/conditions.go`, and
`internal/characters/context.md` describes it in five scattered places (the
regen notes, the prone recovery section, the grapple section, the perception
section and the file table). After this slice the ONE home for the model is
`internal/buffs/context.md`; slice 2 renames it to
`internal/conditions/context.md`. The characters file loses its condition
sections and keeps one pointer.

`context.md` for `buffs` (the model, the door, the effects vocabulary, the
rule that no second timed-state collection may exist), `characters`
(conditions sections removed, pointer to buffs), `hooks` (tick and regen),
`combat`, `gmcp`; the web client's comment on statuses; `docs/README.md` rows.
A patch note: shields last as long as they say, and afflictions survive a
short absence.
