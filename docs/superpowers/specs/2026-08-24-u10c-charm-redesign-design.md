# U10c — Charm Redesign (design)

**Status:** APPROVED 2026-08-24, **REVISED after blind adversarial review the same day.**
**Sections 11, 12 and 13 supersede 1-10 where they conflict — read them FIRST.**
**Section 13 further supersedes 11.3.2. Section 14 is the Slice B design gate.**
Section 2.5 is known FALSE; section 4.1 would have shipped a double contest.
**Arc:** U0–U12 unified resolution. Sequenced after U10b-0; depends on U9 and U6b.
**Roadmap row:** `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, U10c.

---

## 1. Summary

Charm becomes **a companion with a clock**. It keeps its companion slot, its
conviction reservation and its AutoAssist exactly as today. What changes is that
the bond now ends, and when it ends the creature **breaks free and turns on the
caster**. The player is never told how long they have.

The contest joins the seam it escaped: charm resolves through
`ResolveChannelAttack(ChannelSocial)` and is answered by **defy**, with the
arc's uniform skill weight. The dead resist ladder is deleted outright.

---

## 2. The problem, verified against code

Every claim here was checked in the source, not taken from the roadmap.

### 2.1 The skill weight escaped the flip

```go
// internal/hooks/charm_spell.go:51
attackScore := ch.Stats.Charisma.ValueAdj +
    ch.GetSkillLevel(skills.Manifestation) * 25
```

×25 against the arc's uniform 5.0. At manifestation 50 that is **1250** from
skill alone, versus 250 under the uniform weight.

### 2.2 The resist ladder is unreachable

The ladder at `internal/hooks/NewRound_MobRoundTick.go:394-450` is gated on
`comp.CharmDuration > 0`. The **only** assignment to that field is at line 423,
**inside the ladder itself**, so it can never bootstrap. Nothing sets it at cast
time.

Everything it implements is therefore dead: the periodic re-roll, the
`effectiveness` decay on `CharmRerolls`, the "your control is slipping" warnings
at 3 and 5 re-rolls, and break-free-and-turn-on-you.

### 2.3 Charm is permanent via a magic number

```go
targetMob.Character.Charm(user.UserId, 99999, "")
```

A `CharmPermanent = -1` sentinel already exists and means "never expires". The
spell ignores it and passes 99999 rounds instead.

### 2.4 There are TWO duration systems — not in the roadmap

| Field | Ticked? | Set at cast? |
|---|---|---|
| `CharmInfo.RoundsRemaining` | yes, every round (`tickMobCharmDuration`) | yes — to 99999 |
| `CompanionInfo.CharmDuration` | only inside the dead ladder | **never** |

`CharmInfo.RoundsRemaining` already has expiry handling at
`NewRound_MobRoundTick.go:376`.

### 2.5 Charm never joined the unified seam — also not in the roadmap

U6b's stated achievement is that *every* attack channel resolves through one
seam. Charm does not: it builds its own attack and defence scores and calls
`combat.RunContest` directly. Its spell YAML has **no `target_defense_type`**,
so nothing routes it either. It is the one attack the flip missed.

### 2.6 The scoring expression is duplicated

`Charisma + manifestation*25` appears at `charm_spell.go:51` **and again** at
`NewRound_MobRoundTick.go:401-410`. This is precisely the drift class that let
the admin dashboard's chance display diverge from production for months
(U10b-0 Phase E).

### 2.7 Reservation ignores what you charmed

```go
reserve := ch.CalcCompanionReserve(characters.CompanionReserveBase(0))
```

The `0` is the pet multiplier. The code comment states the consequence plainly:
a charmed creature "has no pet multiplier and reserves the unscaled default".
`CompanionReserveDefault` is **280**. A sewer rat and the Elemental King cost
the same.

---

## 3. Design decisions and their rationale

All decisions are the owner's, recorded 2026-08-24.

### 3.1 Charm stays a companion, with a duration

Not a temporary combat-control effect and not a permanent pet. Slot,
reservation and AutoAssist unchanged; the bond simply ends.

### 3.2 Expiry produces a grudge

The creature breaks free and turns on the caster. This behaviour is **already
written** — it is the dead ladder's break-free branch, including its
player-facing copy:

- to the owner: `%s breaks free of your control!`
- to the room: `%s snarls and turns on %s!`

The redesign keeps that block as the **unconditional** expiry outcome and
deletes the re-roll machinery wrapped around it.

### 3.3 No warning before expiry

The player must not know when the clock runs out. That uncertainty is the
mechanic. The ladder's "your control is slipping" / "eyes flash with defiance"
warnings are deleted rather than repurposed.

### 3.4 Duration scales with the margin of victory

The more decisively you won the contest, the longer the bond holds. Same caster
against the same target produces a different result each cast.

### 3.5 Defence is defy; charm joins the seam

Charm resolves through `ResolveChannelAttack(ChannelSocial)`, answered by
**defy** (`Wil + rhetoric × SkillWeight`).

Note this reframes the roadmap's "Willpower or Charisma?" question: **both** seam
defences are Willpower-based, so Willpower remains the defending stat either
way. The real choice was which skill resists, and whether charm keeps a bespoke
contest at all. It does not.

Charm inherits crit, counterattack and coordinated narration for free.

### 3.6 No power gate, and the size term is deleted

`StatPoolTotal × 0.10` goes. There is no cap on what may be charmed.

Owner rationale, recorded because it is the reason this looks under-defended and
is not: **the risk is the balance.** A charmed elite trains its stats and skills
while it serves you, and keeps any gear you hand it. When the bond breaks it
turns on you with your equipment and its improved skills, at a moment you cannot
predict. Most players will stick to conjured and raised companions. Anyone
willing to take an Elemental King should be allowed to try, and to die for it.

**Consequence:** the shipped spell description promises "Stronger creatures
resist more fiercely." That becomes false and **must be rewritten**.

### 3.7 No re-charm restriction

You may re-cast charm on a creature that has just thrown you off. The existing
friction is judged sufficient: the in-combat penalty (`×0.75` when it is
fighting you), the 120 conviction cost, and the round spent casting instead of
fighting or fleeing.

`CharmRerolls` and its `1 - 0.01r²` decay are therefore deleted, not salvaged.

### 3.8 Reservation stays flat — DELIBERATE

Reservation remains `CompanionReserveDefault`, skill-reduced, regardless of what
was charmed. A rat and the Elemental King reserve the same conviction.

**This is a deliberate choice and must be commented as such in the code**, or a
future reader will file it as the bug it resembles. The reasoning: charm is
already a risky game, and juggling multiple charmed NPCs is challenge enough on
its own. A power-scaled price would add bookkeeping without adding tension.

### 3.9 The grudge dies with the player

On player death, any charm-grudge aggro against them is cleared.

Without this, a hostile elite with patrol and `pathto` behaviour could hunt a
player across zones indefinitely. That is griefing, not risk.

### 3.10 A bond that lapses while you are absent does not hunt you

If the duration expires while the caster is not present, the creature reverts to
normal behaviour and does **not** acquire aggro on them. The grudge only bites
if you are there to receive it. Same anti-grief reasoning as 3.9.

---

## 4. Mechanics

### 4.1 The contest

| | Today | After |
|---|---|---|
| Resolution | bare `RunContest`, hand-built scores | `ResolveChannelAttack(ChannelSocial)` |
| Attack | `Cha + manifestation × 25` | `Cha + manifestation × SkillWeight` (5.0) |
| Defence | `Wil + StatPoolTotal × 0.10` | **defy** = `Wil + rhetoric × SkillWeight` |
| In-combat penalty | `×0.75` vs caster, `×0.85` vs other | **unchanged** |
| `CharmImmune` gate | present | **unchanged** |

Success is `!out.Defended`. **`AttackerFumble` must be resolved BEFORE success**
— the seam documents that a fumbled attack aborts even when the roll won.

Verified: `ChannelSocial` always yields **exactly one** defence entry. Defy is
not equipment-gated (`DefenceEntriesFor` routes quell and defy through its
default branch, which appends unconditionally), so there is no empty-defence-set
case where the contest silently does not run and charm auto-succeeds.

### 4.2 Duration

```
m        = attack-positive normalized margin
duration = Min + (Max - Min) * clamp(m / 2.0, 0, 1)
```

`2.0` is the crit bar at parity. Shipped tuning **Min 30, Max 450** rounds; at
`RoundSeconds: 4` that is:

| Margin | Rounds | Real time |
|---|---|---|
| barely won (0.1) | 51 | 3.4 min |
| clear (0.5) | 135 | 9 min |
| strong (1.0) | 240 | 16 min |
| crushing (2.0+) | 450 | 30 min |

The 16-minute figure at a strong margin is deliberate: it reproduces the dead
ladder's own `50 + Cha/2 + manifestation*3` (~235 rounds for a veteran), which is
the only prior art in the codebase for what a charm duration should be.

**A floored outcome takes `Min`.** `res.Floored` means a contest floor changed
the result, and the margin is then a ±1 sentinel rather than a real roll. A
floor-granted win is by definition a scrape and must not read as dominance.

### 4.3 The seam does not currently expose this margin — a required addition

`ResolveChannelAttack` sets `NormalizedDefenceMargin` **only when the defence
won**: the assignment sits below `if res.Success { return out }`. On an attack
win the field is zero, so charm cannot read it.

U10c must add an attack-positive twin to `ChannelDefenceResult`, populated on the
win path:

```go
// AttackerNormalizedMargin is the ATTACK-POSITIVE opposed margin, populated
// only when the attacker WON. NormalizedDefenceMargin is its defence-positive
// counterpart and is populated only when the defence won, so neither is a
// substitute for the other and neither is meaningful on the other's path.
//
// Zero on a floored outcome: the margin is then a +-1 sentinel, not a roll.
AttackerNormalizedMargin float64
```

Set as `res.Margin / (res.DefenseRoll.StdDev * math.Sqrt2)`, guarded on
`res.DefenseRoll.StdDev > 0` and skipped when `res.Floored`, mirroring the
existing `DefenseRollZScore` guard.

This is a small, additive change to shared combat code. It must not alter any
existing field's behaviour.

### 4.4 One clock, not two

`CharmInfo.RoundsRemaining` becomes the single duration. It is already ticked
every round and already has expiry handling.
`CompanionInfo.CharmDuration` is deleted.

### 4.5 Reservation

Unchanged: `CalcCompanionReserve(CompanionReserveBase(0))`. Released when the
bond ends, including on the grudge.

---

## 5. What gets deleted

- The resist ladder, `NewRound_MobRoundTick.go:394-450` (~60 lines).
- `CompanionInfo.CharmDuration`.
- `CompanionInfo.CharmRerolls` and the `1 - 0.01r²` effectiveness decay.
- The "control is slipping" and "eyes flash with defiance" warnings.
- The `StatPoolTotal × 0.10` defence term.
- The duplicated scoring expression in the tick file.
- The `99999` magic number.

---

## 6. Config knobs

| Knob | Default | Meaning |
|---|---|---|
| `CharmDurationMinRounds` | 30 | Duration at a barely-won contest, and on a floored win |
| `CharmDurationMaxRounds` | 450 | Duration at or above the crit bar |

Both new. Existing knobs (`CompanionReserveDefault`, the in-combat penalties,
`SkillWeight`, `PoolReservationCapPct`) are reused unchanged.

Per project convention, **Go defaults and `config.yaml` values must move
together**, and `config.yaml` edits are built from the `git show HEAD:` blob
because the file carries `skip-worktree`.

---

## 7. Traps

1. **Margin sign.** `contest.Result.Margin` is ATTACK-positive.
   `bestDefenseResult.margin` in `internal/combat` is DEFENCE-positive. The
   contest package's own docs record that mixing them "compiles cleanly and
   silently puts crit on the losing side". The new field is attack-positive;
   `NormalizedDefenceMargin` is not.
2. **Fumble precedes success.** A fumbled attack aborts even when the roll won.
3. **Floored outcomes carry sentinel margins**, not rolls.
4. **No second copy of the scoring expression.** It exists once, in the seam.
   The duplicate is what this slice deletes; do not reintroduce one.
5. **Two duration fields exist today.** Deleting the wrong one leaves the live
   clock unticked.
6. **The flat reservation is deliberate** (3.8). Comment it at the call site.
7. **No migration is needed** — owner confirms no veteran character uses charm.

---

## 8. Testing

- **Duration is margin-monotonic**: larger margin never yields a shorter bond;
  clamped at both ends; a floored win takes exactly `Min`.
- **Sign guard**: a dominant win produces a LONG duration. A test that would pass
  under an inverted sign is worthless, so assert the direction explicitly.
- **Expiry produces the grudge**: companion removed, reservation released,
  aggro set on the caster, both messages sent.
- **Absent caster**: expiry with the caster elsewhere reverts the creature and
  sets no aggro.
- **Grudge dies with the player**: after caster death, no charm-grudge aggro
  remains.
- **Seam routing**: charm resolves through `ResolveChannelAttack(ChannelSocial)`
  and the defence that answered is defy. A guard test that no charm code path
  calls `RunContest` directly, mirroring the arc's existing site guards.
- **Uniform weight**: manifestation contributes `× SkillWeight`, not `× 25`.
- **`AttackerNormalizedMargin`**: zero when the defence won, zero when floored,
  correctly signed and scaled when the attack won.
- **No behaviour change to existing channels** from the new field.
- **Helpfile served, not just written**: `help charm` renders and no longer
  claims the duration scales with charisma and manifestation skill.

---

## 9. Out of scope

- Charm on players. Charm targets mobs only today and continues to.
- Rebalancing summon or raise, which share the companion slots.
- The wider U10b progression-firing convention, which is a separate slice and
  remains unshipped.

---

## 10. Player-facing copy — REQUIRED FOR COMPLETION

Owner ruling 2026-08-24: **the slice is not done until both of these land.** They
are plan steps, not follow-ups.

### 10.1 `_datafiles/world/dogmud/spells/charm.yaml`

The description promises "Stronger creatures resist more fiercely", which 3.6
makes false. Rewrite so it conveys that a strong-*willed* creature resists and a
strong-*bodied* one may not, and that the bond does not last forever.

### 10.2 `_datafiles/world/dogmud/templates/help/charm.template`

**Read this file before writing the new one — it is the closest thing to a
design document the original intent ever had, and this slice largely makes it
true.** It already describes behaviour that has never run:

- "The creature will follow you and fight at your side **until the hold on its
  mind fades**" — a duration. The code ships 99999 rounds.
- "**Your hold gradually loosens over time. When the charm finally breaks, the
  creature reverts to its natural state and may turn hostile.**" — the ladder
  and the grudge. Both dead.
- "If the charm breaks **while the creature is near you**, expect it to become
  hostile immediately." — this all but states 3.10's absent-caster rule.

Lines that become WRONG and must change:

| Line | Why |
|---|---|
| `Defense: Mental (opposed by target's willpower)` | Now the **social** channel, answered by defy. Still Willpower-based, but "mental" names the wrong channel and the wrong skill. |
| "your charisma and manifestation skill against the creature's willpower and **mental fortitude**" | The defending skill is now **rhetoric**, not a vague fortitude. |
| `Duration: Scales with charisma and manifestation skill` | Duration now comes from **how decisively you won**, not from your stats. This is the single most important change for a player to understand, because it is why the same caster gets a different result each cast. |
| "The stronger your charisma and the higher your manifestation skill, the longer the charm endures." | Same — delete or replace with the margin framing. |

Lines that stay TRUE and must be preserved: charm cannot target players;
`CharmImmune` creatures are beyond reach; creatures already in combat resist
more strongly; the companion limit and `dismiss`.

**Do not tell the player the duration formula or the remaining rounds.** Per 3.3
the uncertainty is the mechanic. The copy should convey that the hold is
finite and that a decisive victory buys a longer one, without ever being
quantitative.

Consider adding a line reflecting 3.6 honestly: a powerful creature is not
harder to charm, but it is far more dangerous when the bond breaks.

---

# 11. REVISION 2 — after blind adversarial review (2026-08-24)

**Sections 11 and 12 SUPERSEDE sections 1-10 where they conflict. Read them
first.** Three independent blind reviewers each made the same finding their
number one, and it falsifies a premise sections 2.5 and 4.1 both assert.

## 11.1 Section 2.5 is FALSE — charm already resolves through the seam

Section 2.5 claims charm "never joined the unified seam" because `charm.yaml`
declares no `target_defense_type`. That reasoning is backwards.

Verified in source:

- `spellAttackChannel` (`internal/hooks/spell_resolution.go:1076-1081`) maps an
  **absent** `target_defense_type` to `ChannelSpellMental`. Absent is the
  DEFAULT, not an escape.
- The mob-target loop (`spell_resolution.go:129-143`) calls `resolveAgainstMob`
  **unconditionally** — there is no effect-type guard. The *player* loop has
  one, which is what made the false reading look plausible.
- `resolveAgainstMob` calls `runSpellChannelAttack` =
  `combat.ResolveChannelAttack`, with `spellAttackSideFor` building
  `Cha + manifestation × SkillWeight` — **the exact AttackSide section 4.1
  proposed to add.**

**Charm has been on the seam, on the uniform weight, since U6b.** What escaped
the flip is not the cast: it is that the seam's verdict is *discarded*
(`effect_type: "charm"` falls through `applyMobEffect_default`) and
`resolveCharmSpell` then runs a second, private `RunContest` to decide the
outcome.

Adding a `ResolveChannelAttack(ChannelSocial)` call as section 4.1 specified
would therefore give one cast **two** seam contests: two defence quotes and
charges, two progression awards, two crit/fumble tiers, two narrations, and two
verdicts free to contradict each other. That is precisely the "second
independent contest on top of the channel's primary roll" shape U6b was written
to delete.

## 11.2 The corrected design — consume the contest, do not add one

Owner ruling: **the direction is unification.** Charm therefore:

1. **Declares `target_defense_type: social` in `charm.yaml`.**
   `spellAttackChannel` gains a `"social"` case returning `ChannelSocial`.
   One line of routing replaces the whole hand-rolled contest.
2. **Consumes the existing seam result.** `resolveAgainstMob` threads its
   `ChannelDefenceResult` into the charm effect — either by giving charm its own
   `applyMobEffect` arm or by passing `out` to `resolveCharmSpell`. **No new
   `ResolveChannelAttack` call is added anywhere.**
3. **Deletes the private contest** in `resolveCharmSpell` entirely: the attack
   score, the defence score, the aggro multipliers and the `RunContest` call.
   The in-combat penalty moves onto the AttackSide `Mult` that
   `spellAttackSideFor` already builds, composed with
   `combat.SituationalAttackMult` as every other channel caller does.

This is strictly less code than section 4.1 proposed, and it removes a contest
rather than adding one.

**Note:** with charm on `ChannelSocial`, the `AttackerNormalizedMargin` addition
from section 4.3 is still required — the seam still does not surface the opposed
margin on an attack win, and the duration still needs it.

## 11.3 Rulings on the review's other blockers

### 11.3.1 The 12.5% contest floor — ACCEPTED, no change

`ContestFloor: 0.125` means charm succeeds on at least 12.5% of casts against
any target, so roughly 8 casts will charm anything in the game, and no boss
carries `charm_immune`.

**Owner ruling: this is fine.** A one-in-eight chance of charming a hostile
creature that will, in time, become genuinely hostile again is an acceptable
trade. Section 3.6 stands unchanged: no power gate, no cap.

### 11.3.2 Logout must NOT release the grudge — CLOSE IT

`PlayerSpawn_HandleJoin.go:43-63` destroys every charmed mob on login and strips
the companion record, so `quit` currently discharges the bond with no
consequence. Since the player is never told how long they have (3.3), the
rational play would be to relog after every fight, and the entire risk half of
the design becomes opt-out.

**Owner ruling: close it.** Logging out must not be an escape from the grudge.

**Known and accepted consequence:** a creature that survives its owner's logout
can be walked into a town first and abandoned there. **That griefing path is
accepted and will be policed socially rather than prevented mechanically.**
Do not add a mechanic for it.

### 11.3.3 Non-combatants and shopkeepers — ALREADY SAFE, pin it

Requirement: shopkeepers and other non-combatants must not be charmable.

Verified already true, by two independent gates:

- **All 97 shop mobs** carry `charm_immune: true` and/or `non_combatant: true`.
  Zero are unprotected.
- `non_combatant` blocks charm *before* it is reached:
  `playerHarmTargetPermitted` → `mobs.CheckPlayerHarm` →
  `HarmBlockedNonCombatant` (`internal/mobs/harm_authorization.go:48-49`), and
  the target is skipped from the loop.
- `charm_immune` is a second gate at `charm_spell.go:30`.

**The deliverable is a regression TEST, not a mechanism**: assert that every mob
carrying a `shop:` block is `charm_immune` or `non_combatant`, so a future
shopkeeper authored without either is caught at test time.

### 11.3.4 The margin mechanic is NOT decorative — the reviewer overstated

The reviewer concluded the duration mechanic is decorative because a veteran
clamps to maximum duration on ~90% of successful casts.

**Owner ruling: that conclusion is drawn from the wrong targets.** It holds
against common wilderness mobs, where the duration hardly matters. The
reviewer's own figures contradict it where it counts: against The Core Guardian
(`statpool: 2800`) a veteran reaches maximum duration on only **13%** of wins,
with an expected hold around 287 rounds against a 450 ceiling. High-end
creatures carry enormous stat pools, so even an archetype-slanted share puts
Willpower high enough to land `D/A` in the band where the margin varies.

The mechanic is legible exactly where the interesting decisions are. **No change
to 3.4 or 4.2.**

One genuine correction survives from that finding, independent of the above:
**`charmCritBar = 2.0` is wrong.** `CritBarFor` (`internal/combat/crit_bar.go`)
subtracts `CritBarSkillSlope × (atkRank - defRank)` and clamps to
`[CritBarFloor 1.5, CritBarCeiling 3.0]`. Because mob rhetoric is 1, the bar is
**1.5** for any caster past manifestation 10. Either divide by the real
`CritBarFor` or drop the "same threshold as a crit" justification and call 2.0
what it is: an arbitrary two-sigma constant. **Do not ship the claim that they
coincide.**

### 11.3.5 Item duplication — GUARD IT

`SaveMobInstance` (`internal/mobs/instance_save.go:83-86`) early-returns on
`mob.Character.IsCharmed()`. That has always held for a charmed mob's whole life
because charm was permanent. Once bonds expire, the ex-companion is uncharmed
**while still wearing the player's equipment**, and the next save pass writes it
to `mobs.instances/`. The gear becomes a persistent world object: kill it, loot
it, charm another, repeat.

**Owner ruling: guard it.** Extend the skip to any mob that was ever charmed:

```go
	// EverCharmed, not just IsCharmed: once a bond expires the ex-companion is
	// uncharmed while still wearing the equipment its owner handed it. Saving
	// it would bake player gear into a world mob permanently -- kill, loot,
	// re-charm, repeat. The betrayal stays real in-session (it fights you with
	// your own gear) but nothing is written to disk, so a reboot clears it.
	if mob.Character.IsCharmed() || mob.Character.EverCharmed {
		return nil
	}
```

`EverCharmed` (`internal/characters/character.go:138`) already exists and is
documented as surviving dismiss, which is exactly the semantics needed.

## 11.4 Implementation findings to carry into the re-plan

Not design decisions; all confirmed, and all owed by the slice.

1. **`contest_site_guard_test.go` has two allowlist rows naming U10c as their
   owner** (`:76-77`). The guard asserts both directions, so deleting the two
   contest sites without deleting the rows turns `internal/combat` red. Add
   `internal/hooks/charm_spell.go` to `legacyLiteralFiles` once its `×25` is
   gone.
2. **Deleting the ladder drops its `SourceType == CompanionCharmed` guard**
   (`NewRound_MobRoundTick.go:399`). Five other systems set `Charmed` —
   summons, brood spawns, the homunculus, `befriend`, and behaviour-tree
   companions. Two reach `RoundsRemaining == 0` immediately: logout
   (`PlayerDespawn_HandleLeave.go:136-142`) and link-dead
   (`users.go:347-352`, which does it to **the tutorial Guide mob**). The
   expiry path MUST test the source type.
3. **The reservation is not released by `RemoveCompanion` alone.** It is applied
   during `RecalculateStats()` (`validate.go:261-284`). Section 4.5's "nothing
   to release" is wrong: call `RecalculateStats()` and publish the vitals
   change, as `dismiss` does.
4. **Expiry runs below the active-zone gate.** `tickMobCharmDuration` is in the
   idle lane but `tickMobCharmState` is not, so a bond lapsing in a cold zone
   parks at zero and fires when a player next enters — inverting 3.10 into an
   ambush on return.
5. **Charm bypasses `SituationalAttackMult`**, which both other `ChannelSocial`
   callers compose. Fixed for free by 11.2 step 3.
6. **`dismiss` sets the grudge unconditionally, with no room check**, while 3.10
   makes expiry aggro conditional on presence. Reconcile the two exits.
7. **Charm prints two contradictory outcome messages per cast today**, because
   the discarded seam contest still narrates. 11.2 fixes this by construction.
8. **`context.md` for `internal/combat` and `internal/hooks`** both document the
   charm `RunContest` sites and "charmed companions are permanently Active".
   Both become false.
9. **The helpfile carries four em dashes.** Project convention forbids them in
   player copy, and section 10.2's rewrite list only covers one of the four.
10. **Section 10.2 lists a line to "preserve" that is false**: "Merchants,
    powerful named creatures, and certain others are immune". The 372
    `charm_immune` mobs are townsfolk; no boss carries it. Rewrite rather than
    preserve.

---

# 12. Follow-up filed by this review

**The instance-save system deserves its own examination.** Owner observation,
2026-08-24: it has been the source of a large share of this project's
headaches, alongside the quest system. Finding 11.3.5 is the third distinct
instance-save trap this arc has hit, after stale saves shadowing template edits
and the legacy-training migration. A dedicated audit slice is likely worth more
than continuing to patch it per-incident. Not scoped here.

---

# 13. REVISION 3 — logout destroys the creature, by design (2026-08-24)

**This section SUPERSEDES 11.3.2.** Owner ruling, same day.

## 13.1 The rule

**On logout, a charmed creature is destroyed. There is no grudge, and nothing
persists.** This is the existing behaviour
(`PlayerSpawn_HandleJoin.go:43-63` — `DestroyInstance`, plus the charmed
`CompanionInfo` being dropped) and it is **kept deliberately**, not by
accident.

The codebase already states this philosophy at that site:

```go
// Remove charmed companions — they don't persist through restart.
// Charmed mobs are temporary by nature (borrowed, not created).
```

**Borrowed, not created.** A bond that does not survive the owner leaving is
the correct reading of what charm is, and the rest of this design follows from
it.

## 13.2 Why this supersedes 11.3.2

11.3.2 ruled that logout must not release the grudge, and accepted the
resulting griefing path — a hostile creature walked into a town and abandoned
there — as something to police socially.

Destroying the creature instead is better on every axis:

- **The griefing path closes mechanically.** Nothing is left behind to kill
  newcomers, so nothing needs policing.
- **No grudge has to persist.** `Character.Charmed` is `yaml:"-"` and there is
  no persisted charm clock, so carrying a grudge across a logout would have
  meant inventing persistence for it. That work disappears.
- **Item duplication loses its main vector.** Gear on the creature dies with
  it. (The `EverCharmed` save guard in 11.3.5 is still required for the
  in-session case, where a bond expires while the owner is online and the
  ex-companion keeps their equipment.)

## 13.3 What this costs, stated plainly

Quitting still lets a player dodge the betrayal. It is **priced, not closed**:
they lose the creature, the 120 conviction, and any equipment they gave it.

This is judged acceptable because **the maximum bond is 30 minutes**. Almost
every charm begins and ends inside a single session, so logout-during-bond is
an edge case rather than a viable strategy. And since the player is never told
how long they have (3.3), playing around it means quitting constantly and
re-buying at 120 conviction each time, with gear never worth giving.

What that degrades charm into — a disposable, per-session borrowed ally — is a
reasonable thing for charm to be, and it leaves the mid-session betrayal this
design exists for completely intact. You can still train a creature, gear it,
and have it turn on you at the worst possible moment. You simply cannot carry
that bond across a logout.

## 13.4 Implementation wrinkle — do NOT read rounds-at-zero as one thing

`PlayerDespawn_HandleLeave.go:136-142` signals logout by calling
`Charmed.Expire()`, which sets `RoundsRemaining` to **0** — the identical state
a natural expiry produces. `users.go:347-352` does the same on link-dead, to
the **tutorial Guide mob**.

So the expiry path must distinguish *"the clock ran out"* from *"the owner
left"*. Reading rounds-at-zero as a natural expiry would fire the grudge on a
player who is mid-logout, and on a newcomer whose connection dropped.

The expiry handler must therefore gate on **both**:

1. `SourceType == characters.CompanionCharmed` — five other systems set
   `Charmed` (summons, brood spawns, the homunculus, `befriend`, behaviour-tree
   companions) and none of them should ever produce a grudge; and
2. an owner who is present and **not leaving** — the logout and link-dead paths
   must reach destruction, never the grudge.

---

# 14. Charm may not target players — decided 2026-08-24 (Slice B gate)

## 14.1 The decision

**Charm refuses a player target, with a message, at cast time.** The helpfile
has always claimed this (`charm.template:30` — "Charm cannot be used on other
players") and nothing enforced it.

## 14.2 Why this had to be decided before Slice B could be coded

Declaring `target_defense_type: social` (11.2) changes the player branch, not
just the mob branch.

Today `spell_resolution.go:161-164` shortcuts any spell with an empty
`TargetDefenseType` into `applyPlayerEffect` with a fabricated uncontested win.
`applyPlayerEffect` has no `charm` arm, so a player-targeted charm is a
**silent no-op that costs the caster 120 conviction**.

Declaring `social` moves it to `resolveAgainstPlayer` (`:166`), which runs a
real `ChannelSocial` contest. That would have:

- charged the victim conviction for a defy they never chose to mount,
- awarded them rhetoric and willpower progression,
- let them defensively crit into a counterattack against the caster,
- let the caster fumble into a self-damaging backfire,

all for an effect that still does nothing. Shipping that would have been a PvP
mechanic arriving by accident, as a side effect of a routing change.

## 14.3 Why refusal rather than making it work

Making charm work on players is a real PvP feature — mind control of another
character, with a duration, a grudge, and a companion slot. It needs its own
design, its own consent rules, and its own review. It is not U10c's to invent.

Refusal is also **strictly better than today**: the player gets a clear message
instead of silently losing 120 conviction to nothing.

## 14.4 Where the guard goes

At **cast time**, in target validation, not at resolution. A 36-fold channel
should not be spent before the refusal arrives.

`internal/actions/cast.go` already refuses player targets for other reasons in
the same place (self-target on a harm spell, and the `room.CanPvp` check), so
this is one more row in an existing guard rather than a new mechanism.

## 14.5 What this does NOT change

Mob targets are unaffected. `CharmImmune` and `non_combatant` continue to gate
mob targets exactly as before (11.3.3).
