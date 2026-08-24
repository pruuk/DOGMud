# U10c — Charm Redesign (design)

**Status:** APPROVED 2026-08-24.
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
