# M4b-2 design notes: the four axes, and the counters slice

Date: 2026-09-17
Status: **SUPERSEDED 2026-09-18 by
[the M4b-2 spec](2026-09-18-messaging-m4b2-axes-design.md).** Kept as the
record of the conversation. Two claims below did not survive verification:
`DamageChannel` is not the damage type (it is a derived pool, and it stays),
and the mob-loop uncontested shortcut is a behaviour change, because a help
spell cast at a mob is contested today. Open questions 1 to 4 are ruled in the
spec.
Originally: working notes, not a spec, written at the end of the session so the
next one starts here.
Arc: [messaging unification](2026-08-31-messaging-unification-design.md), M4b-2
Supersedes, within M4: the M4 spec's "one `DefenseType`" wording.

---

## Where the arc stands

| Slice | State |
|---|---|
| M4a, one token engine and vocabulary | **SHIPPED**, PR #142 |
| M4b-1, role keys, shipped-data guards, loader policy | **SHIPPED**, PR #143, master `612b85d54` |
| **M4b-2, the axes (this document)** | **NEXT: spec, then plan** |
| Counters slice (new, owner-approved 2026-09-17) | after M4b-2 |
| M4c bands, M4d send path, M4e Go narration to YAML | unchanged |
| M5 quality, M6 content, M7 refusals | unchanged |

Nothing is deployed. The stack is #111 to #143.

---

## The finding that reframed M4b-2

The M4 spec says M4b leaves "one `DefenseType`". Taken literally that is wrong:
three different concepts share one Go type today, and the defence names
themselves are declared four times, not two.

**Verified on master `612b85d54`:**

| Declaration | Values | file:line |
|---|---|---|
| `items.DefenseType` | dodge, parry, block, quell, defy **plus** counter-melee, counter-ranged, counter-quell, counter-defy | `internal/items/defensive_messages.go:15` |
| `characters.Defense*` (untyped strings) | none, dodge, parry, block, quell, defy | `internal/characters/character.go:727` |
| `combat.DefenseType` | none, dodge, parry, block (physical only, predates quell and defy) | `internal/combat/attackresult.go:8` |
| `combat.AttackChannel` | melee, ranged, spell-physical, spell-mental, social | `internal/combat/defence_sets.go:10` |
| `combat.DamageChannel` | Physical, Magical, Conviction | `internal/combat/damage_pipeline.go:15` |
| `spells.SpellData.TargetDefenseType` | physical, mental, social, none | `internal/spells/spells.go:35` |

So the same three damage types are spelled **physical/mental/social** in one
place and **Physical/Magical/Conviction** in another, with a third copy in spell
YAML. And `AttackChannel` is not an axis at all: it is two axes flattened into
one enum, which is why its five values look lopsided. Melee physical allows
parry; ranged physical does not. Parry is gated by the ATTACK type (reach),
quell and defy by the DAMAGE type.

The counter values are not defences. `defensive_messages.go:24-29` says so in
its own comment: they ride the defence type only because they share its loader,
shape and validator.

---

## The model (owner, 2026-09-17)

Four axes, each named once, in a leaf package every consumer can import.

| Axis | Values | Replaces |
|---|---|---|
| `AttackType` | melee, ranged, thrown, spell, rhetoric, none | the attack half of `AttackChannel`; today hardcoded per call site, never data |
| `DamageType` | physical, mental, social, non_harm | the damage half of `AttackChannel`, all of `DamageChannel`, all of `TargetDefenseType`, and the harm/help half of spell `type:` |
| `Targeting` | self, single, multi, area | the rest of spell `type:` |
| `Defence` | dodge, parry, block, quell, defy | `items.DefenseType`'s first five, `characters.Defense*`, `combat.DefenseType` |

**Defence eligibility is keyed by the (AttackType, DamageType) pair**, replacing
`DefenceSetFor`'s flattened switch (`internal/combat/defence_sets.go:51`).
Today's table, restated in the new terms:

| attack_type | damage_type | eligible defences |
|---|---|---|
| melee | physical | dodge, parry, block |
| ranged, thrown | physical | dodge, block |
| spell | physical | dodge, block |
| spell | mental | quell |
| rhetoric | social | defy |

**Spell `type:` is retired.** `type: helpsingle` becomes
`damage_type: non_harm` plus `targeting: single`; `type: harmarea` becomes the
spell's real damage type plus `targeting: area`. Keeping `type:` beside
`damage_type: non_harm` would be a fourth spelling of harm versus help.

**Owner ruling: the counter pools keep existing, but keyed by the defence that
won**, not by channel. A dodge counter should not read like a block counter.

### Where the leaf package goes

`internal/characters` imports `internal/items`, not the reverse, so the shared
types cannot live in `characters`. The only packages all four consumers already
import are `util`, `configs` and `mudlog`. **A new leaf package** (working name
`internal/combatvocab`, importing nothing but stdlib) is the recommendation, so
the vocabulary is not buried in a grab bag. Owner leaned toward replacing
`AttackChannel` outright rather than keeping it as a derived alias.

---

## The targeting axis, reasoned through (owner, 2026-09-17)

Question: does targeting change defence eligibility? Cases considered:
grenades, AoE spells of each damage type, a hypothetical AoE social attack,
sweeping melee cleaves, ranged volleys.

**Conclusion: no.** Targeting changes how many contests happen, not what may
roll in one. A cleave is still parryable; a volley is ranged, which already
excludes parry for reach reasons. The counter-argument was built and did not
hold.

**What targeting DOES change:**

1. **The defence text.** Today every victim of one blast reads the
   single-target line, so six people are each told about "the bolt" as though
   it were aimed at them. Content gap, not a mechanism gap.
2. **Whether a counter is earned at all.**

### AoE and counters: a live behaviour, not a new rule

🔴 **AoE attacks grant counters PER TARGET today.** Verified:
`internal/actions/combat_drain.go:302` calls `counterSkillMoveExit` inside the
per-player loop ("each player's own crit defence earns their own counter"), and
area spells fire `fireSpellCounterTier` per target through `resolveAgainstMob`
and `resolveAgainstPlayer` (`internal/hooks/spell_resolution.go:445,963,1566,1791`).
One room-wide cast into five people can eat five counters.

**Owner ruling: disallow counters from area attacks.** Note this is a BUFF to
area attackers, and it lands on Meirok's core-drain fights.

**Plumbing, decided:** targeting travels with the attack and the counter gate
reads it, plus a guard test asserting no area attack produces a counter. The
cheap alternative (just do not call the tier at the two area call sites) is
rejected: the next sweep or volley someone writes reintroduces the counter by
omission.

---

## Slice split (owner, 2026-09-17)

**M4b-2: the axes.** Vocabulary and routing only, no behaviour change, provable
by goldens.
- The leaf package with the four enums.
- The eligibility table keyed by the pair; `DefenceSetFor` and
  `DefenceEntriesFor` move onto it.
- `AttackChannel`, `DamageChannel`, `characters.Defense*` and
  `combat.DefenseType` all die.
- `items.DefenseType` keeps only the counter pools, renamed to say so.
- Spell YAML: `target_defense_type` and `type` retire in favour of
  `attack_type`, `damage_type`, `targeting`. Charm and the 13 summons take
  `damage_type: non_harm` per the earlier ruling.
- The `non_harm` uncontested shortcut at `internal/hooks/spell_resolution.go:168`
  reads the new value, and the mob-target loop at `:136` gains the same check
  (the lead M4a filed and M4b-1 did not reach).

**Counters slice: behaviour and content.** Owner ruled it belongs in this arc,
because it sends messages to the player.
- Re-key the counter pools from channel to the defence that won.
- Author the missing per-defence counter text (a parry counter and a block
  counter do not exist as text today; there are four channel pools now and five
  defence pools after).
- Gate counters off for area attacks.
- Ends with the adversarial playtest, which covers the whole slice at once.

---

## Open questions for the next session

1. **`thrown` as an attack type.** Today `throw.go:327` passes `ChannelRanged`,
   so thrown is not distinct. Introducing it is a real value, and it changes
   nothing until something keys off it. Keep it in the enum, or leave it out
   until a mechanic wants it?
2. **`none` as an attack type.** Needed for a condition tick or an ambient
   source that damages without an attacker. Confirm against
   `internal/conditions` before adding it.
3. **Does `Targeting` belong on the item side too?** A grenade is an item, and
   its area-ness is a property of the item, not of the command.
4. **The `DamageChannel` to `DamageType` merge changes mitigation lookups**
   (`MitigationCap`, `internal/combat/damage_pipeline.go:119`). Physical maps to
   physical and mental to magical cleanly, but `non_harm` has no cap and must
   never reach that function. Decide whether the merge is a rename or a
   conversion at the boundary.

---

## Carried forward from earlier slices, still unpaid

- `textutil.ValidateTokens`' pattern is `\{[a-z_]+\}`, so `{Actor}` or
  `{actor1}` still ship raw. No shipped data has one today.
- `casting-messages.yaml` uses `{spell}`, which is not in the known-token set.
  Pointing the validator at that store as-is would reject every line.
- Quest `send_text` and reward messages are still unchecked for unknown tokens;
  only the room line is.
- `position_control.yaml`'s `gradient_messages` and `transition_messages` are
  authored text no Go code reads. Kept and renamed in M4b-1. Deleting them is
  still open, and would want an M6 ledger row.
- `_datafiles/world/default` is out of bounds by owner ruling, and the larger
  cleanup (repoint the Go default at `world/dogmud`, delete the tree) is filed
  in the M4 spec, not scheduled.
