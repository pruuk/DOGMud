# U6b — Finish the Flip

**Created:** 2026-08-19
**Arc:** [`UNIFIED_RESOLUTION_ROADMAP.md`](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md) (U0–U12)
**Parent spec:** [`2026-08-12-unified-contest-resolution-design.md`](2026-08-12-unified-contest-resolution-design.md)
**Depends on:** U9
**Blocks:** U11 (the arc's closer cannot run while its own criteria are false)
**Size:** L
**Behaviour change:** Yes, and it is the largest in the arc

---

## 1. Why this exists

U6 was "THE FLIP": uniform ×5 skill weight, multiplier defence, designed
defence sets, **all legacy parameters deleted**. It converted **melee and
taunt** and stopped.

Two of this arc's own "Done when" criteria have been false ever since:

> 2. Defence skill weight is ×5 in every channel; `SpellAttackSkillFactor` is
>    gone from the attack path.

Both are prose in a roadmap, so nothing failed when they stopped being true.
The gap survived U7, U7b, U8 and most of U9 before U9's progression work
tripped over it. **That is as much a process defect as a code one, and §9
addresses it.**

The owner's framing, 2026-08-19, is the requirement:

> "It says right on the tin UNIFY. melee/ranged/spell/taunt/special/non-combat
> all the same hit path, the same crit path, the same progression path, the
> same damage path (if applicable)."

U9 delivered the progression path. This delivers the other three.

---

## 2. Measured state, 2026-08-19

Verified against source, not inferred. This table is the spec's foundation and
an earlier informal version of it was wrong in two places, so it is stated
precisely.

| Channel | On `RunContest` | Margin-scaled damage | Defence **set** | Atk skill weight | Def skill weight |
|---|---|---|---|---|---|
| Melee | yes | yes | dodge / parry / block | ×5 | ×5 |
| Taunt | yes, via `ResolveChannelDefence` | yes | defy | ×5 | ×5 |
| **Spell** | **TWO contests** | mitigation only | quell, damage-only, **skipped on crit** | **×15** | **×0** on the gate, ×5 on mitigation |
| **Ranged** | yes, via `ExecuteSkillMove` | yes | **none** (flat scalar) | ×1 | ×1 + flat shield bonus |
| **Special ×14** | yes, via `ExecuteSkillMove` | yes | **none** (flat scalar) | ×1 | ×1 |

### 2.0 Crit is a third axis of divergence, and the worst of the three

Audited 2026-08-19 after the owner asked "what about crit?". An earlier draft
of this spec assumed crit was uniform once the contest was, which is false.

| Channel | Attacker crit | Derived from | Defensive crit | Counterattack tier |
|---|---|---|---|---|
| Melee | yes | contest margin | yes | riposte / auto-trip / auto-bash |
| Taunt | yes | contest margin | yes, negates | **none** |
| Spell | yes | **the GATE's margin**, not the defence contest | yes, negates | **none** |
| **Ranged** | **NONE** | — | damage multiplier only | **none** |
| **Special ×14** | **NONE** | — | damage multiplier only | **none** |

**`combat.ExecuteSkillMove` computes no crit whatsoever.** `SkillMoveResult` is
`{Hit, Damage, StatusApplied, KnockedDown, TargetMaxHP}` — there is no `Crit`
field, and `AttackContestCrit` appears in that file zero times.

**Consequence: 16 of the game's attacks cannot critically hit.** Every special
move (bash, trip, kick, hamstring, rake, maul, pounce, gore, drain, throttle,
grapple, and the rest) and every aimed shot. A player who masters
ranged-combat gets no crit tier at all, while an autoattacking sword user does.
Nothing documents this as a design decision; it reads as `ExecuteSkillMove`
predating the crit model and never being revisited.

The **counterattack tier is melee-only** too. `ParryCritDetected`,
`DodgeCritDetected` and `BlockCritDetected` are set in exactly one place
(`combat/combat_helpers.go:1169-1173`, the melee swing path) and consumed in
`hooks/combat_shared_helpers.go`. A defensive crit against a spell, a taunt, a
shot or a bash negates the damage and does nothing else. The parent spec is
explicit that this tier is what a defensive crit IS:

> The defensive crit is the **counterattack tier**. Damage avoidance is the
> continuous curve; crit is the qualitative tier that unlocks something and
> answers crit attacks.

So four of five channels have the curve and not the tier.

Two corrections to earlier informal claims, recorded so they are not repeated:

- **Ranged is NOT off the contest core.** `actions/combat_fire.go` calls
  `combat.ExecuteSkillMove`, which uses `RunContest` and
  `defenceDamageMultiplier`. It folds the defender into one scalar
  (`DefenseSkill: int(rangedDefenseScore(defChar))`, `DefenseStat: 0`). The
  defect is the missing defence **set** and the ×1 weight, not the core.
- **Quell is not dead.** It runs on ordinary spell hits and scales damage. It
  is the *hit gate* that ignores the defender's skill entirely, and the crit
  branch that skips quell altogether.

### 2.1 The spell channel in detail

`resolveAgainstPlayer` / `resolveAgainstMob` (`internal/hooks/spell_resolution.go`):

```
contest 1  runPlayerSpellContest(spellAttack, [spellDefenseValue(...)])
           attacker: CalcSpellAttack = Wil + spellcasting x SkillWeight(5) x SpellAttackSkillFactor(3)
           defender: RAW Willpower.ValueAdj   <-- no skill term at all
           if !success -> "fizzles", return          <-- BINARY HIT GATE
           isCrit := AttackContestCrit(atkMargin, atkRoll)

contest 2  if !isCrit { runPlayerSpellDefence(channel, ...) }   <-- quell lives here
           scales damage only; never runs on a crit
```

Consequences:

1. **A defender's spellcasting does nothing to whether a spell lands.** It only
   softens damage, and not at all on a crit. This is the arc's opening
   complaint, still live: *"skill weight on the defending side: melee ×5,
   ranged ×1, spell ×0, taunt ×5."*
2. **Crit is decided before any defence is contested**, which the parent spec
   names as a trap: *"An attack crit forces a hit. Any crit adjustment
   evaluated before the hit outcome is final becomes an undeclared second hit
   floor."*
3. `SpellAttackSkillFactor` ships at **3**, so attacker skill is weighted ×15
   against a defender weighted ×0. That is a 15:0 asymmetry inside one contest.

---

## 3. The target: one shape, five channels

Every channel resolves the same way:

```
score      attacker = stat + skill x SkillWeight, times situational modifiers
           defender = per-defence, from DefenceSetFor(channel)
contest    combat.RunContest(atkScore, entries)          -- ONE contest
crit       from that contest's MARGIN (AttackContestCrit / DefenseContestCrit)
damage     defenceDamageMultiplier(res) scales it        -- same curve everywhere
progression events from the same margin                  -- already true, U9
```

**One contest. Crit from its margin. Damage from its margin. Every channel has
a crit tier. No channel opts out of its own defence.**

### 3.1 Defence sets, unchanged from U6's design

`DefenceSetFor` already returns these and is already correct. Ranged's entry
exists and has never been wired.

| Attack type | Applicable defences | N |
|---|---|---|
| Melee | dodge, parry, block | 3 |
| Ranged | dodge, block | 2 |
| Spell, physical damage | dodge, block | 2 |
| Spell, mental | quell | 1 |
| Taunt / social | defy | 1 |

### 3.2 What stays per-channel, deliberately

Unification is of the **shape**, not of every value. These differences are
designed and survive:

- **The attacking STAT differs by channel.** Melee autoattack is Dexterity,
  aimed `shoot` is Perception (deliberate: an aimed shot is not a swing),
  spells use the spell's `primarystat` (U9), taunt is Charisma. Only the
  *weight* on the skill term is uniform.
- **Defence sets differ by channel.** You cannot parry a bolt.
- **`ChannelScale` damage constants differ** and are config, not code.
- **Non-contest categories stay out.** Crafting, salvage and the flat
  `util.Rand` sites are Category C: a craft is a probability against a recipe,
  not a contest against an opponent. `picklock` is Category D and permanently
  out. The owner's "non-combat" in the directive is satisfied by those already
  resolving through `contest.Run` / `contest.AgainstDifficulty`, which is the
  same core; they simply have no opponent and therefore no defence set.

---

## 4. Scope

### 4.1 Spell: collapse two contests into one

**Delete the binary hit gate.** The channel defence contest becomes THE
contest:

- `spellDefenseValue` is deleted. `DefenceSetFor(spellAttackChannel(spell))`
  supplies the entries, so a mental spell is answered by quell at
  `Wil + spellcasting×5` and a physical spell by dodge and block.
- `isCrit` comes from that contest's margin, so a crit means the attack beat
  the best defence decisively. **The `if !isCrit` branch disappears**: there is
  only one contest and it always runs.
- The "fizzles" outcome becomes the ordinary defence outcome: a defence win
  scales damage down via `defenceDamageMultiplier`, and a **defensive crit**
  negates, exactly as melee does. Whether "fizzle" survives as a *message* is
  §8's question, not a mechanical one.
- `CalcSpellAttack` drops `SpellAttackSkillFactor`; attacker skill weights ×5
  like everything else. **`SpellAttackSkillFactor` is deleted from the config
  and from `internal/configs`.**

**This is the largest single balance change in the arc.** Attacker spell score
falls from `Wil + skill×15` to `Wil + skill×5`, while defender score rises from
raw Willpower to `Wil + spellcasting×5`. Both directions compound. §7 is the
modelling gate.

### 4.2 Ranged and the special-move family: real defence sets

`ExecuteSkillMove` currently takes pre-computed scalars:

```go
AttackStat, AttackSkill, DefenseStat, DefenseSkill int
```

It builds `attackerScore = AttackSkill + AttackStat` and a single defender
entry. Change it to take a **channel** and build its entries from
`DefenceSetFor`, so a bash can be dodged, parried or blocked and a shot can be
dodged or blocked, each with its own score and its own narration.

- All 14 callers migrate. They currently pass `GetSkillLevel(...)` raw, which
  is the ×1 weight; they pass the skill and the seam applies `SkillWeight`.
- `rangedDefenseScore` is deleted. Its flat `RangedShieldDefenseBonus` is
  superseded by `block` being a real defence with its own score; **that knob is
  deleted with it** unless modelling says shields need a ranged-specific
  adjustment, in which case it becomes a defence-set modifier rather than a
  flat addend to a scalar.
- `DefenseStat: 0` folding disappears. Nothing folds a defence into a scalar
  any more.

**The roadmap's own pre-U6 gate applies here and was never discharged:**

> Applied naively that moves 14 sites from ×1 to ×5 on both sides at once.
> Against mobs, which all carry combat skill 1, a weapon-combat-30 player's
> bash goes from `130 vs 101` to `250 vs 105`. Nobody has modelled that.

§7 discharges it.

### 4.3 Crit: one path, all five channels

This is the axis §2.0 found and it is the largest player-facing part of the
slice.

1. **`ExecuteSkillMove` gains crit**, derived from its contest margin like
   everywhere else. `SkillMoveResult` gains `Crit bool` and `Fumble bool`.
   Sixteen attacks that have never been able to crit gain the tier: all 14
   special moves plus aimed `shoot` (which routes through the same function).
2. **Spell crit moves to the defence contest's margin.** Once §4.1 collapses
   the two contests there is only one margin, so this falls out rather than
   being a separate change. It is named because it changes what a spell crit
   *means*: today it means "beat a raw stat", afterwards "beat the defender's
   quell decisively".
3. **The counterattack tier extends to every channel, gated on REACH.**
   Owner decision, 2026-08-19.

   The tier is universal; what varies is whether you are close enough to
   answer. **A defensive crit in the same room earns a melee counter.** The
   mechanism already exists: parry's riposte is a free counter-swing at half
   weapon damage through `CalcRawDamage(..., ChannelPhysical)`
   (`hooks/combat_shared_helpers.go`). It is wired to parry only; U6b wires it
   to a defensive crit on any channel where the defender is in reach.

   **Do NOT model the counter as "interrupting" the attack.** An earlier draft
   of this spec suggested a quell crit interrupts the caster, which the owner
   correctly rejected: by the time quell answers, the spell has already
   resolved. There is nothing left to interrupt. A defensive crit is not
   prevention, it is a decisive defence that leaves you an opening. The counter
   is what you do with the opening.

   Reach, measured 2026-08-19:

   | Channel | Attacker's position | Melee counter possible |
   |---|---|---|
   | Melee | same room, always | yes |
   | Spell | **same room only** (no adjacent-room targeting exists) | yes |
   | Taunt | same room | yes |
   | Ranged, same-room shot | same room | yes |
   | **Ranged, cross-room shot** | adjacent room (`shoot <target> <direction>`) | **no** |
   | Special moves | same room, always | yes |

   The cross-room shot is the only case with no counter, and that is a
   **coherent asymmetry rather than a hole**: you cannot punch someone in the
   next room. It is also a real property of the weapon, so it reads as an
   advantage of shooting from cover rather than as a missing feature. Document
   it as designed; do not invent a substitute so the table looks symmetrical.

   **A defy crit produces a counter-taunt, REPLACING the melee counter.**
   Owner decision, 2026-08-19. Answering a social attack socially is more
   legible than answering it with a fist, and taunt already has a full
   resolution path to reuse. Defy is therefore the one channel whose counter is
   not a melee swing.

   **A ranged counter is deliberately OUT.** In theory a defender wielding a
   ranged weapon could answer a cross-room shot in kind, which would close the
   one gap in the reach table. Owner decision, 2026-08-19: leave it out. It
   would add a second conditional counter path (wielding-dependent as well as
   reach-dependent) for a narrow case, and the cross-room shot being
   uncounterable is already coherent as a property of the weapon. Recorded here
   so a later reader sees it was considered and declined, not missed.

   **Messaging is a hard requirement, not polish.** The owner's condition was
   explicitly "as long as the messaging to the combatants and observers makes
   sense". A melee counter following a quell crit must read as a person putting
   a working down and stepping in, not as a generic riposte string pasted under
   a spell. Each channel needs its own attacker, defender and room text on the
   existing `defense-messages/` triad shape. Shipping the mechanic without it
   repeats the gap U8 had to close for quell and defy.
4. **Fumble stays self-relative** (`roll.ZScore <= -ContestCritThreshold`), not
   margin-derived. A fumble is a property of one bad roll rather than of the
   gap between two, and that is already consistent across the channels that
   have one. Audit whether `ExecuteSkillMove` has any fumble concept; if not,
   it gains one with crit.

### 4.4 Deletions

By the end, none of these exist:

| Symbol | Where | Why |
|---|---|---|
| `SpellAttackSkillFactor` | config + `characters.CalcSpellAttack` | the ×15 asymmetry; a "Done when" criterion names it |
| `spellDefenseValue` | `hooks/spell_resolution.go` | the binary hit gate's defender score |
| `runPlayerSpellContest` / `runMobSpellContest` | `hooks/` | the second contest |
| `rangedDefenseScore` | `actions/combat_fire.go` | the folded scalar |
| `RangedShieldDefenseBonus` | config | superseded by block as a real defence |
| the `if !isCrit` defence skip | `hooks/spell_resolution.go` | a channel opting out of its own defence |

**Standing rule 5 of the arc — "no legacy parameter survives U6" — is what this
table discharges.**

---

## 5. Out of scope

| Deferred to | What |
|---|---|
| **U10** | Concentration and disruption. |
| **U10b** | Progression firing consistency, Category C routing, and the melee-versus-channel defence-award divergence. |
| **U10c** | Charm, including its skill weight of **25** and its defence stat. Charm has its own resolution path and is redesigned wholesale there rather than half-converted here. |
| **U12** | Targeting simplification. |

**Charm is explicitly NOT in U6b** even though its ×25 weight is the same class
of defect, because U10c is rewriting the function around it and converting it
twice is waste.

---

## 6. Behaviour changes, named

Every one is deliberate and must be called out individually in the PR.

| Change | Direction | Who |
|---|---|---|
| Spell attacker skill ×15 → ×5 | **Large cut** to spell accuracy | Players and mobs |
| Spell defender gains `spellcasting×5` on the hit contest (was ×0) | **Large increase** in spell defence | Players and mobs |
| A critting spell now faces its defence; quell can negate it | Cut to spell crit reliability | Players and mobs |
| Ranged and 14 special moves: attacker skill ×1 → ×5 | Increase | Players and mobs |
| Ranged and 14 special moves: defender skill ×1 → ×5, and a real defence SET | Increase, and multi-defence | Players and mobs |
| `RangedShieldDefenseBonus` flat addend replaced by block as a contested defence | Situational | Defenders with shields |
| Special moves become dodgeable / parryable / blockable with narration | New player-visible text | Everyone |
| **16 attacks gain a crit tier that has never existed** (14 special moves + aimed shot) | **Large increase** in their damage ceiling | Players and mobs |
| Spell crit now means "beat quell decisively", not "beat a raw stat" | Cut to spell crit rate | Players and mobs |
| Counterattack tier extended to every channel, gated on reach | **Increase** to defender value on 4 channels that had none | Players and mobs |
| Cross-room shots remain uncounterable, now as a documented property | Advantage to shooting from an adjacent room | Ranged attackers |

**Mobs all carry combat skill 1**, so every ×1→×5 change widens the
player-versus-mob gap far more than the player-versus-player gap. That
asymmetry is the single most important thing for modelling to quantify.

---

## 7. The modelling gate — MANDATORY, before any code

No task in this slice may change a weight until this is done and reviewed. It
discharges a gate the roadmap raised before U6 and that U6 shipped without.

Model, at minimum:

1. **Spell accuracy across the range.** Novice, journeyman, adept and Meirok
   (Wil 148, spellcasting 51) casting at: a trash mob (skill 1), a parity
   opponent, and a gold-scaled instance boss. Before and after. The attacker
   loses two thirds of its skill term while the defender gains one from zero;
   report the net hit rate, not each half.
2. **Spell crit rate**, before and after, given crit moves from a gate margin
   to a contest margin that now includes a real defence.
3. **The 14 special moves at ×5 on both sides.** Reproduce the roadmap's own
   example (`130 vs 101` becoming `250 vs 105`) and extend it across the skill
   range. Against mobs at skill 1 the defender's ×5 buys almost nothing while
   the attacker's buys a great deal.
4. **Ranged**, including the loss of the flat shield bonus and the gain of
   block as a contested defence.
5. **Defence-set width.** A bash answered by three defences instead of one
   scalar is strictly better for the defender; quantify by how much before
   assuming it is acceptable.
6. **The counterattack tier on four new channels.** A defensive crit currently
   negates damage and nothing more outside melee; afterwards it also earns a
   free counter-swing. Model how often a defensive crit actually occurs per
   channel post-flip (it is margin-driven, so it rises sharply when the
   defender outclasses the attacker) and what that counter is worth. **Cross-
   room shots are excluded by reach**, so quantify how much that is worth to a
   ranged attacker who stays in the next room; if it is large, that is a real
   incentive to kite and worth knowing before players find it.
7. **The new crit tier on 16 attacks.** These have never crit. Model their
   damage ceiling before and after, especially the beast moves (rake, maul,
   pounce, gore) which mobs use heavily and which would gain a crit tier
   against players at the same time players gain one against them. Combine
   with item 3: a special move getting BOTH ×5 skill weight and a crit tier in
   one slice is a compounding change, and the two must be modelled together
   rather than separately.

Use `tools/balance/unified_resolution_model.py`, which already exists for this
arc. **Deliverable: a modelling document alongside this spec, reviewed before
the implementation plan is written.** If the numbers say a knob needs
retuning, that is a `config.yaml` edit, per standing rule 1.

---

## 8. Open questions for spec review

1. **Does "fizzle" survive as player copy?** Mechanically it becomes an
   ordinary defence win. A caster who currently reads "Your spell fizzles"
   would read a quell defence message instead. That is arguably better
   (it names who stopped it and how) but it is a visible change to every
   failed cast.
2. **Does `shoot` keep Perception?** `CLAUDE.md` documents it as deliberate:
   aimed shots are deliberate-move actions, not swings. This spec assumes YES,
   because only the weight is being unified, not the stat. Confirm.
3. **`RangedShieldDefenseBonus`: delete or convert?** This spec assumes delete,
   on the grounds that block as a real defence supersedes it. If modelling says
   shields need ranged-specific help, it becomes a defence-set modifier.
4. **Do the special moves get per-defence narration?** They become dodgeable
   and parryable, which the existing `defense-messages/` data shape already
   supports. Shipping the mechanic without the text would leave players losing
   to something unnarrated, which is the gap U8 had to close for quell and defy.
5. ~~The counterattack tier: extend or scope?~~ **RESOLVED 2026-08-19.** The
   tier extends to every channel, gated on reach: a melee counter by default,
   a counter-taunt REPLACING it for defy, the cross-room shot documented as the
   one coherent exception, and a wielding-dependent ranged counter considered
   and declined. See §4.3 item 3. Nothing open here.
6. **Should 16 attacks gaining crit ship in the same slice as ×1→×5?** They
   compound. A case exists for landing the weight unification first, measuring
   it in play, then adding the crit tier. That splits U6b in two and delays the
   "Done when" criteria being true, so this spec assumes ONE slice, but the
   modelling in §7 item 6 may argue otherwise.

---

## 9. The process defect, and the fix

This slice exists because U6 was declared done while two of its own completion
criteria were false, and **nothing detected that for four slices.**

The criteria live as prose in a roadmap. Prose does not fail.

**U6b ships them as a test**, and U11 inherits it:

```go
// Every channel's DEFENCE skill weight must be the uniform SkillWeight.
// This is a "Done when" criterion of the arc, expressed as something that
// can fail. It was prose from U6 until 2026-08-19, during which time it was
// false for spell, ranged and the special-move family and nothing noticed.
func TestEveryChannelUsesUniformDefenceSkillWeight(t *testing.T)

// SpellAttackSkillFactor must have no readers. Deleting a knob is not the
// same as deleting its use; this asserts the use is gone.
func TestNoLegacyPerChannelSkillFactorSurvives(t *testing.T)
```

The second is an AST or grep-shaped guard, on the model of U9's
`internal/progression/seam_guard_test.go` and U5b's pool-mutation guard, both
of which have already caught real regressions in this arc.

**Recommendation beyond this slice:** every remaining "Done when" criterion
should be expressed as a test before U11 declares the arc finished. U11's row
already carries this.

---

## 10. Testing

- **Per-channel contest-shape tests**: for each of melee, ranged, spell-mental,
  spell-physical and social, assert one contest runs, its entries come from
  `DefenceSetFor`, and crit derives from that contest's margin.
- **No-opt-out test**: assert no channel skips its defence contest on a crit.
  This is the specific regression that produced the spell path's shape.
- **The two Done-when guards** from §9.
- **Parity damage-per-swing within ±10%** of pre-U6b at light, mid and BIS
  armour, per the arc's completion criterion 5 — or a documented, modelled,
  owner-approved deviation where the whole point was to change it.
- Existing tests: **expect many to need updating.** Unlike U9, this slice is
  contracted to change behaviour. Each changed test must be named in the PR
  with what it was pinning.
- Isolated boot test, per the pre-push SOP.

---

## 11. Playtest gate

**This slice REQUIRES an adversarial playtest**, unlike U9. It changes how
every attack in the game resolves and adds player-visible defence narration to
16 actions that previously had none.

Additionally, the owner's pre-deploy manual pass gains items for: spell
accuracy at veteran level, special moves being defended against for the first
time, and whether the widened player-versus-mob gap from ×1→×5 reads as
mastery or as trivialisation.

Per the arc's standing rule, **merging is not deploying**. Nothing deploys
until the whole arc is done and tested with the AI harness plus a full manual
pass.
