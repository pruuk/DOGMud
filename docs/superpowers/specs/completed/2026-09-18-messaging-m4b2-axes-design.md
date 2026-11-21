# Messaging M4b-2: the four axes

Date: 2026-09-18
Arc: [messaging unification](2026-08-31-messaging-unification-design.md), M4b-2
Status: designed, awaiting owner review
Supersedes: [the M4b-2 axes notes](2026-09-17-messaging-m4b2-axes-notes.md)
in full, and within [the M4 flip spec](2026-09-17-messaging-m4-flip-design.md)
the "DefenseType" and "Spell defence convention" bullets of the M4b section and
rulings 8 and 9.

The M4 spec said M4b leaves "one `DefenseType`". The notes found that three
concepts share that type and that `AttackChannel` is two axes flattened into
one enum. This spec fixes the vocabulary: four authored axes, each declared
once in a leaf package, with defence eligibility keyed by the attack-and-damage
pair. It is vocabulary and routing. Every golden stays byte-identical, and the
two places where the truth on disk differs from the truth in the notes are
called out as flagged behaviour commits rather than hidden under that promise.

---

## Facts verified against source

Every row was read from source on 2026-09-18, master `612b85d54`.

### The declarations being replaced

| Declaration | Values | Source |
|---|---|---|
| `items.DefenseType` | dodge, parry, block, quell, defy, plus counter-melee, counter-ranged, counter-quell, counter-defy | `internal/items/defensive_messages.go:15-34` |
| `characters.Defense*` (untyped string consts) | "", dodge, parry, block, quell, defy | `internal/characters/character.go:727-740` |
| `combat.DefenseType` | "", dodge, parry, block (physical only) | `internal/combat/attackresult.go:8-15` |
| `combat.AttackChannel` | melee, ranged, spell-physical, spell-mental, social | `internal/combat/defence_sets.go:8-16` |
| `spells.SpellType` | neutral, harmsingle, harmmulti, helpsingle, helpmulti, harmarea, helparea | `internal/spells/spells.go:19,82-88` |
| `SpellData.TargetDefenseType` | free string; physical, mental, social, none, or absent | `internal/spells/spells.go:35` |
| `combat.DamageChannel` (NOT replaced, see finding 1) | Physical, Magical, Conviction | `internal/combat/damage_pipeline.go:12-18` |

### Where they are consumed

| Consumer | Count | Source |
|---|---|---|
| `AttackChannel` and `Channel*` constants, production | 27 files | grep `AttackChannel\|Channel(Melee\|Ranged\|SpellPhysical\|SpellMental\|Social)` |
| Same, in tests | 29 files | same grep over `*_test.go` |
| `combat.DefenseType` outside its file | `NewRound_DoCombat_helpers.go:322-365`, `NewRound_DoCombat_unified.go:150,178`, `combat.go:566`, `combat_helpers.go:656,1273,1477-1481`, `surprise_narration.go:59-63` | grep |
| `characters.Defense*` | 41 lines in `combat/calculations.go`, `combat/combat_helpers.go`, `combat/defence_multiplier.go`, `combat/defence_sets.go` | grep |
| Defence name as `string` parameter | `GetDefenseScoreFor`, `GetDefenseScore` (`characters/combat.go:279,341`); `defenseCostRequest`, `QuoteDefenseCost`, `DefensePool`, `GetDefenseCostFloat`, `GetDefenseCost` (`characters/resources.go:140-297`); `DefenceSkillAndStat` (`defence_multiplier.go:135`); `bestDefenseResult.defenseType` (`combat_helpers.go:100`); `ChannelDefenceResult.DefenceType` (`defence_multiplier.go:225`); `DefenceSetFor`, `DefenceEntriesFor` return `[]string` (`defence_sets.go:51,151`) | read |
| The unchecked bridge | `items.DefenseType(out.DefenceType)` | `defence_multiplier.go:298` |
| Spell `Type` readers | 33 lines in 8 files: `actions/cast.go` (target resolution switch at `:111-296`, harm gate `:306`), `actions/cast_admission.go:42,50`, `hooks/combat_shared_helpers.go:655`, `hooks/NewRound_DoCombat_helpers.go:631,638`, `hooks/spell_resolution.go` (11 lines), `mobcommands/cast.go:179`, `usercommands/skill.cast.go:246,248`, `usercommands/spells.go:42-83` | grep |
| `SpellType` display methods | `HelpOrHarmString` (`spells.go:103`), `TargetTypeString` (`spells.go:115`); used by `usercommands/spells.go:81,83` and `templates/help/spell.template:7,8` | read |
| `TargetDefenseType` readers | `hooks/combat_shared_helpers.go:87` (mitigation pool), `hooks/spell_resolution.go:168` (uncontested shortcut), `hooks/spell_resolution.go:1245` (`spellAttackChannel`), `templates/templatesfunctions.go:100` (`defensename`, via `spell.template:12`) | grep |
| Attack entry points, all hardcoded constants | 14 `SkillMoveParams.Channel` sites (12 in `internal/actions/combat_*.go`, 2 at `hooks/combat_shared_helpers.go:335,394`); `ResolveChannelAttack` direct at `actions/combat_counter.go:183`, `actions/combat_taunt.go:176`, `usercommands/throw.go:327`; `combat/counter.go:113` (the counter-swing); spells through `spellAttackChannel` | grep |
| Throw | accepts only `items.Throwable` (`throw.go:193`), resolves on `ChannelRanged` (`throw.go:327`), is an AREA effect by its own comment (`throw.go:283-286`), has no counter tier | read |
| Eligibility table today | melee: dodge, parry, block; ranged and spell-physical: dodge, block; spell-mental: quell; social: defy; default: nil | `defence_sets.go:51-65` |

### Spell data on disk

| Fact | Value | Source |
|---|---|---|
| Shipped spells | 59 in `_datafiles/world/dogmud/spells/` | `ls` |
| `type:` | harmsingle 15, harmarea 7, helpsingle 21, helparea 4, neutral 12; harmmulti and helpmulti 0 | grep |
| `target_defense_type:` | physical 11, mental 9, social 1 (charm), none 13, absent 25 | grep |
| The 13 `none` | 5 conjure, 6 raise (all `neutral`), summon-hive-swarm and summon-steppe-spirit (`helpsingle`); all carry `summon_mob_id` | table |
| The 25 absent | 19 `helpsingle`, 4 `helparea`, identify (`neutral`), core-drain (`harmarea`, `drain_area`) | table |
| `world/default/spells` | 8 files: harmsingle 2, harmmulti 1, helpsingle 4, helpmulti 1; no `target_defense_type` at all | grep |
| Loader | `LoadSpellFiles` reads `<DataFiles>/spells`, `Validate()` error panics the boot; yaml unknown keys are ignored (no `KnownFields`) | `spells.go:315,503-510`; grep `KnownFields` |
| Test binaries that load spells from disk | only `narration/snapshot_test.go:163`, which points `DataFiles` at dogmud first | grep |
| Schema doc | `docs/schemas/spell.md:35,43` documents `type` (required) and `target_defense_type` | read |
| Unused multi resolvers | `resolveMobHarmMultiTargets` (`cast.go:492`), `resolveMobHelpMultiTargets` (`cast.go:518`), live code, zero shipped spells | grep |

### Finding 1: the pool is a hidden fifth spelling, and it is not the damage type

| Fact | Value | Source |
|---|---|---|
| Damage scaling | every spell scales on `ChannelMagical`, physical spells included | `hooks/combat_shared_helpers.go:52`; `combat/calculations.go:90` |
| Mitigation pool for spells | chosen by `TargetDefenseType`: physical uses physical mitigation, mental uses magical, anything else 0 with cap 0.75 | `hooks/combat_shared_helpers.go:87-96` |
| Toughen stat on a defensive crit | `channelDamageChannel`: melee and ranged "physical", BOTH spell channels "magical", social "conviction"; the comment warns that mapping spell-physical to "physical" toughens the wrong stat | `combat/defence_multiplier.go:753-778` |
| The string it feeds | `characters.ToughenStatFor(string)`: physical, magical, conviction | `characters/progression.go:511-521` |
| Situational attack multiplier | prone and stamina penalties on melee and ranged only | `combat/situational.go:35-51` |
| `DamageChannel` consumers | 14 files; `DamageScale` (`:48`), `CalcRawDamage` (`:72`), `MitigationCap` (`:119`) | grep |
| Species reflect | its own two-value `ReflectChannel` (physical, magical) from `return_damage_channel` | `combat/reflect_damage.go:29-49` |
| Analytics | `DamageChannelForType` maps move NAMES ("spell", "taunt", "unarmed") to report buckets | `combat/analytics.go:369-380` |

### Finding 2: a help spell cast at a mob is contested today

| Fact | Value | Source |
|---|---|---|
| Player casting at a mob | `resolveAgainstMob` runs the contest unconditionally through `spellAttackChannel` | `spell_resolution.go:406` |
| Routing for an absent defence type | `ChannelSpellMental`, so quell | `spell_resolution.go:1260-1262` |
| Help spells can target mobs | `HelpSingle` allows companions (`cast.go:218-230`); `HelpArea` applies to ally mobs (`spell_resolution.go:109-118`) | read |
| What follows a "defended" heal | the defence triad is narrated and nothing applied; a fumble backfires on the caster; a defensive crit fires the counter tier at `:445` | `spell_resolution.go:406-448` |
| Mob-to-mob heals | already bypass the contest, but only for `EffectType == "heal"` | `spell_resolution.go:1546-1550` |
| Player-target loop | shortcut on `TargetDefenseType == ""` | `spell_resolution.go:168` |

### Counters and targeting

| Fact | Value | Source |
|---|---|---|
| Counter pool chosen by | the ORIGINAL attack's channel: ranged, both spell channels (counter-quell), social, default melee | `combat/counter.go:161-172` |
| `CounterResult.Channel` | `AttackChannel`, kept only for pool selection | `counter.go:24-26` |
| Area attacks earn counters per target | `actions/combat_drain.go:302`; `spell_resolution.go:445,963,1566,1791` | read |
| Shipped pools | dodge, parry, block, quell, defy, counter-defy, counter-melee, counter-quell, counter-ranged | `ls _datafiles/world/dogmud/defense-messages` |
| Test fixture | `items.MinimalDefenseMessageFixture` seeds all nine | `items/test_helpers_combat.go:54-58` |

### Docs and text that name the retiring symbols

`internal/{items,characters,actions,combat,conditions,spells,hooks}/context.md`,
`docs/schemas/spell.md`, `_datafiles/world/dogmud/templates/help/spell.template`,
`.claude/skills/dogmud-combat/SKILL.md:181`. Roadmap mentions are historical
and stay.

---

## Rulings

Owner, 2026-09-17, from the notes:

1. Four authored axes: `AttackType`, `DamageType`, `Targeting`, `Defence`.
   Each declared once, in a leaf package.
2. Defence eligibility is keyed by the (attack type, damage type) pair.
3. Spell `type:` and `target_defense_type:` retire in favour of `attack_type`,
   `damage_type`, `targeting`. Harm versus help is `damage_type: non_harm`,
   not a fourth spelling.
4. Targeting does not change eligibility. It changes how many contests
   happen, the defence text (content, later), and whether a counter is earned
   (the counters slice).
5. Counter pools survive, and the counters slice re-keys them to the defence
   that won. Area attacks stop earning counters there, with targeting carried
   on the attack so the gate cannot be bypassed by omission.
6. `AttackChannel` is replaced outright, not kept as an alias.
7. core-drain declares `physical` and routes through the physical spell
   pairing, dropping parry. A balance change in its own flagged commit
   (M4 spec ruling 9, carried).
8. `world/default` spells are changed minimally so they cause no error
   (M4 spec ruling 10, carried).

Owner, 2026-09-18, this session:

9. **`thrown` is a real attack type.** Grenades are thrown, and the throw
   command passes it. Eligibility is dodge and block, the same as ranged.
10. **`none` is the attack type of a non-harm cast.** A heal is not an
    attack. `attack_type: none` pairs with `damage_type: non_harm`, the
    validator requires them together, and that pair is the uncontested
    shortcut. It has nothing to do with condition ticks.
11. **Targeting travels with the attack, not the item.** No `ItemSpec` field.
12. **The pool is derived, not authored.** `DamageChannel` (Physical,
    Magical, Conviction) is what the damage pipeline scales, mitigates and
    toughens with. It is not the damage type: a physical spell is dodged but
    scales magically. The axes exist to make that derivable, so the three
    hand-rolled switches collapse onto derivation functions and
    `DamageChannel` itself is neither merged nor renamed.

---

## Design

### 1. The leaf package: `internal/combatvocab`

Imports nothing but the standard library, so `characters`, `items`, `combat`,
`spells`, `templates` and `hooks` can all import it. (`characters` imports
`items`, not the reverse, which is why neither of those could own it.)

```go
type AttackType string   // melee, ranged, thrown, spell, rhetoric, none
type DamageType string   // physical, mental, social, non_harm
type Targeting  string   // self, single, multi, area
type Defence    string   // dodge, parry, block, quell, defy; "" is DefenceNone
```

Each type has `Valid() bool` and a `Parse` that rejects anything else. The
zero value of every type is invalid, so an unset field cannot pass silently.

**`Attack` is the value that travels.** It replaces `AttackChannel` at every
call site and is what the counters slice reads targeting from.

```go
type Attack struct {
    Type      AttackType
    Damage    DamageType
    Targeting Targeting
}
```

Constructors build the only pairs that exist, so Go call sites cannot invent
an unknown pairing: `Melee(t)`, `Ranged(t)`, `Thrown(t)`,
`Spell(d DamageType, t)`, `Rhetoric(t)`, `NonHarm(t)`. Spells build theirs
from validated data through `SpellData.Attack()`.

**Eligibility** is one table, keyed by the pair, replacing `DefenceSetFor`'s
switch:

| attack type | damage type | eligible defences |
|---|---|---|
| melee | physical | dodge, parry, block |
| ranged | physical | dodge, block |
| thrown | physical | dodge, block |
| spell | physical | dodge, block |
| spell | mental | quell |
| spell | social | defy |
| rhetoric | social | defy |
| none | non_harm | (empty: uncontested) |

`EligibleDefences(a Attack) ([]Defence, bool)`. The second value is false for
a pair not in the table. Today's `default: nil` silently made an unknown
channel uncontested; the seam keeps that outcome for safety (a panic mid-round
would be worse) but logs it at error level, and the pair can only arise from a
struct literal, because the constructors and the spell validator both refuse
it. `Pairs()` returns the table for the guards.

Charm is the one spell on the `(spell, social)` row. Today it reaches defy
through `ChannelSocial`, the same channel as taunt, so that row is a
restatement, not a change.

`multi` stays in `Targeting` because `resolveMobHarmMultiTargets` and its
help twin are live code, even though no shipped spell uses them. Deleting them
is a separate decision.

### 2. Derived pools, in `combat`

`DamageChannel` stays where it is with its three values. Two functions replace
three switches, and every consumer of those switches moves onto them:

- `ScaleChannelFor(AttackType) DamageChannel`: melee, ranged, thrown are
  Physical; spell is Magical; rhetoric is Conviction. This is also the toughen
  channel: `channelDamageChannel` (`defence_multiplier.go:769`) becomes
  `ScaleChannelFor(a.Type).ToughenName()`, where `ToughenName` returns the
  string `characters.ToughenStatFor` already expects. The mapping is identical
  to today's, spell-physical to magical included.
- `MitigationChannelFor(DamageType) (DamageChannel, bool)`: physical is
  Physical, mental is Magical, social is Conviction, non_harm is false. The
  switch at `combat_shared_helpers.go:87` reads this.
- `SituationalAttackMult` switches on `a.Type`: melee, ranged and thrown take
  the prone and stamina penalties; spell, rhetoric and none do not.

`non_harm` never reaches the damage pipeline, because a non-harm cast takes
the uncontested path before any damage helper runs. The plan must trace the
six callers of `calcSpellDamageForCharacter` by effect type and prove that no
shipped spell reaches the old `default: 0 mitigation, cap 0.75` arm after
classification. If one does, that is a behaviour change to flag, not to hide.

`ReflectChannel` and `analytics.DamageChannelForType` are out of scope: the
first is a species data string with its own defaulting rule, the second buckets
move names for a report. Neither is a defence or attack vocabulary.

### 3. The seam and its callers

- `ResolveChannelAttack(a combatvocab.Attack, side, attacker, defender)`.
- `SkillMoveParams.Channel` becomes `Attack combatvocab.Attack`.
- `DefenceEntriesFor(a, defender, opts) []combatvocab.Defence`; the equipment
  gate is unchanged.
- `ChannelDefenceResult.DefenceType` becomes `Defence combatvocab.Defence`,
  and the unchecked cast at `defence_multiplier.go:298` disappears: the store
  lookup takes a `Defence` and converts to its pool key itself.
- `CounterResult.Channel` becomes `Attack combatvocab.Attack`;
  `ExecuteCounter` takes the attack. `counterPoolFor` keys on `a.Type` for
  now (melee; ranged and thrown to counter-ranged; spell to counter-quell;
  rhetoric to counter-defy), which is today's mapping restated. Re-keying to
  the defence that won is the counters slice.
- The 17 hardcoded call sites pass constructors. Every special move is
  `Melee(Single)` except `ExecuteDrainArea`, which is `Melee(Area)` in the
  byte-identical commit and moves per ruling 7 in its own commit. Fire is
  `Ranged(Single)`. Throw is `Thrown(Area)`. Taunt and the counter-taunt are
  `Rhetoric(Single)`. The counter-swing inside `ExecuteCounter` is
  `Melee(Single)`.

### 4. The defence enum, everywhere

`combatvocab.Defence` replaces all three declarations. `characters.Defense*`
and `combat.DefenseType` are deleted, and every `string`-typed defence
parameter and field listed in the facts table takes `Defence` instead:
`GetDefenseScoreFor`, `GetDefenseScore`, the five cost functions,
`DefenceSkillAndStat`, `bestDefenseResult.defenseType`,
`AttackResult.DefenseUsed`, `DefenseAttempts`, `SwingEvent.DefenseUsed`,
`SwingDefence.Defence`, and the two narration switches at
`combat_helpers.go:1477` and `surprise_narration.go:59`.

Untyped string constants in the 13 test files still compile against a
`Defence` parameter, so those tests need no edits, and a defence name spelt
wrong in a test still fails the assertion it always did.

`actionspec.ActionDodge` and friends stay as they are: they are cost-registry
keys that happen to share a spelling, and `defenseCostRequest` remains the one
place that maps a `Defence` onto its action.

### 5. The message store keeps only its pool keys

`items.DefenseType` is renamed `items.DefencePool`: the key of the
`defense-messages/` loader map. Its values are the five defences' pool names
plus `CounterPoolMelee`, `CounterPoolRanged`, `CounterPoolQuell`,
`CounterPoolDefy`. `PoolFor(combatvocab.Defence) DefencePool` is the one
conversion. `RenderDefenseMessage` takes a `DefencePool`. The fixture, the
loader validation and the goldens are unchanged in content: the YAML files and
their keys do not move.

### 6. Spells: data

`SpellData` gains three required fields and loses two:

```yaml
attack_type: spell        # melee | ranged | thrown | spell | rhetoric | none
damage_type: physical     # physical | mental | social | non_harm
targeting: single         # self | single | multi | area
```

`Validate()` requires all three, requires the pair to be in the eligibility
table, and requires `attack_type: none` exactly when `damage_type: non_harm`.
The `Type` and `TargetDefenseType` fields are deleted from the struct. Because
the yaml decoder ignores unknown keys, a shipped-data test also fails the
build if any file under `spells/` still carries `type:` or
`target_defense_type:`, in both worlds.

The rewrite is mechanical, from the two old keys:

| old `type` | old `target_defense_type` | attack_type | damage_type | targeting |
|---|---|---|---|---|
| harmsingle | physical / mental / social | spell | same | single |
| harmarea | physical / mental | spell | same | area |
| harmmulti | (default routing was mental) | spell | mental | multi |
| helpsingle, helpmulti, helparea | any | none | non_harm | single, multi, area |
| neutral | any | none | non_harm | self |
| core-drain (harmarea, absent) | | spell | physical | area (ruling 7) |

`self` means no target is resolved and the argument text passes through,
which is what `neutral` meant: summons and identify. It is not "defaults to
the caster"; that is `single` with the caster as the default, which is what
`helpsingle` meant. Under this mapping the 13 summons, identify and the other
23 help spells, 37 in all, become `none` / `non_harm`, which is the M4 spec's
ruling 8 restated in the new vocabulary.

The tool is `tools/spell_axes_rewrite.py`: text-line edits with a temp-file
swap, never a yaml round trip (the M4a rule), a `--dry-run`, and a `--check`
that fails on any legacy key or any missing new key. It runs over both
`world/dogmud/spells` and `world/default/spells`, which satisfies ruling 8
with the same pass.

### 7. Spells: code

Every `Type == HarmSingle || Type == HarmArea || Type == HarmMulti` reads
`spellData.IsHarm()`, which is `Damage != NonHarm`. The target-resolution
switch in `cast.go` and `admitCastAim` switch on `Targeting` and `IsHarm()`.
The self-cast progression penalty at `NewRound_DoCombat_helpers.go:631` reads
`Targeting == Single && !IsHarm()`. `playerHarmTargetPermitted` takes the
spell.

`spellAttackChannel` is deleted; `SpellData.Attack()` returns the value built
from the three fields. `spellAttackSideFor` and the four `fireSpellCounterTier`
sites read it.

Display strings are derived so the `spells` listing and the help template
print exactly what they print today:

- `HelpOrHarmString`: harm is "Harmful"; non_harm with `self` is "Neutral";
  non_harm otherwise is "Helpful".
- `TargetTypeString(short)`: self "Self"; single "Single" / "Single Target";
  multi "Group" / "Group Target"; area "Area" / "Area Target".
- `DefenceNames()` replaces the template's `defensename` helper and is
  derived from the eligibility table: "dodge or block", "quell", "defy", or
  "" for non_harm, which suppresses the line as today.

`spell.template` moves from `.Type.HelpOrHarmString` to `.HelpOrHarmString`,
and from `defensename .TargetDefenseType` to `.DefenceNames`.

**The uncontested shortcut** at `spell_resolution.go:168` reads
`Attack().Type == None`. The mob-target loop at `:136` and
`resolveMobSpellAgainstMob`'s `EffectType == "heal"` check gain the same test,
in the flagged commit below.

### 8. Guards, each proven capable of failing before its green run counts

1. **Eligibility parity.** The old `DefenceSetFor` table, pinned as a literal
   in the test, equals `EligibleDefences` for each old channel's pair, and
   the two new rows (thrown, spell/social) equal their stated sets.
2. **Pool parity.** The three deleted switches, pinned as literals, equal the
   two derivation functions across every value.
3. **Display parity.** The seven legacy `SpellType` values, mapped through
   the rewrite table, produce the same `HelpOrHarmString` and both
   `TargetTypeString` forms as before, pinned as literals.
4. **Shipped spells.** Every file in both worlds parses with all three keys,
   a valid pair, the `none`/`non_harm` pairing rule, and no legacy key.
5. **One declaration.** A test greps non-test Go for a string literal
   `"dodge"`, `"parry"`, `"block"`, `"quell"` or `"defy"` assigned to a
   const or var and fails on any file other than `combatvocab` and
   `actionspec`. Proven by sabotage that compiles.
6. **Call-site coverage for the companion-heal fix.** A unit test drives
   `resolveAgainstMob` with a non-harm spell and asserts no contest ran, with
   production-side sabotage (the M4a lesson: a golden covers the store, not
   the path production calls).
7. **Goldens byte-identical**, `-update` never run. Boot check by
   `Server Ready` and the panic patterns, never `$?`.

### 9. Docs

New `internal/combatvocab/context.md`. Updated: the seven `context.md` files
in the facts table, `docs/schemas/spell.md`, `docs/README.md`,
`.claude/skills/dogmud-combat/SKILL.md`, and `docs/PATCH_NOTES.md` for the
two behaviour commits only. The rewrite tool is indexed in `docs/README.md`.

---

## Behaviour changes, each its own commit, flagged in the PR

Everything above is byte-identical. These two are not, and the PR description
names them.

1. **core-drain** (ruling 7): `ExecuteDrainArea` moves from `Melee(Area)` to
   `Spell(Physical, Area)`, so its victims dodge or block and no longer parry.
   Pinned by a test on its defence set. Lands on Meirok's core-drain fights.
2. **Help spells at mobs stop being contested** (finding 2): the mob-target
   loop and the mob-to-mob path take the uncontested shortcut for every
   `none` / `non_harm` cast, not only `heal`. Today a heal on a companion can
   be "defended", can backfire, and can earn the companion a counter-swing at
   its owner. This is the M4 spec's "the mob-target loop gets the same check"
   with the trace it asked for done, and the answer is that it was reachable.

---

## Out of scope, carried

To the counters slice: re-keying the pools to the winning defence, the
missing parry and block counter text, the area gate, the playtest.
To M4c: the bands. To M6: per-targeting defence text (six victims of one
blast each read the single-target line).

Still unpaid from earlier slices, unchanged by this one:
`textutil.ValidateTokens` accepts only `\{[a-z_]+\}`; `casting-messages.yaml`
uses `{spell}`; quest `send_text` and reward messages are unchecked for
tokens; `position_control.yaml`'s dead `gradient_messages` and
`transition_messages` blocks; the `world/default` repoint-and-delete.

---

## Proof of done

- `go build ./... && go test ./...` green, with guards 1 to 6 each shown red
  under sabotage first.
- Every golden byte-identical to master's.
- `tools/spell_axes_rewrite.py --check` clean over both worlds.
- Boot check reaches `Server Ready` with no `PANIC`.
- `grep -rn "AttackChannel\|combat\.DefenseType\|characters\.Defense\|TargetDefenseType\|spells\.SpellType"` over non-test Go returns nothing, and the grep is shown capable of matching before the sweep.
- The two behaviour commits sit last in the branch, separately titled.
