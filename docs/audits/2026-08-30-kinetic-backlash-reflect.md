# Kinetic Backlash: a dead mob staggers its killer, and the message names no source

**Date:** 2026-08-30
**Status:** filed as a follow-up, not fixed
**Reported by:** owner, from live play in the planar oasis instance
**Severity:** low mechanically, moderate for comprehension

Point-in-time report. Verify against source before acting.

---

## The symptom

`An unseen force slams into you, sending you staggering!` arrives while fighting
elementals in the planar oasis, **sometimes after the fight is already over**.

## Where it comes from

| Layer | Where |
|---|---|
| The message | `start_user_text` on buff **109 "Reeling"**, `_datafiles/world/dogmud/buffs/109-reeling.yaml:8` |
| What it does | Dexterity **-15**, `triggerrate: 1 round`, `triggercount: 2` |
| What applies it | The **Kinetic Backlash** mutation (`_datafiles/world/dogmud/mutations/kinetic-backlash.yaml`), ethereal cluster, rarity 7, via `on_reflect_buff: 109` alongside `reflect_damage: 12` |
| Wiring | `mutations.GetReflectRiderBuffs` (`internal/mutations/mutations.go:643`), called from `internal/hooks/NewRound_DoCombat_unified.go:406` |
| Origin | `42d4f366c` (2026-07-12), "content(ethereal): fill cluster". The commit says outright: *"True knock-flat/knockdown-on-reflect deferred (staggering debuff approximates)."* |

**No oasis mob authors this mutation.** The six elemental files (`318`-`322`,
`377`) reference no mutations at all, so they are acquiring Kinetic Backlash at
runtime through mob mutation acquisition. That is why it appears irregularly and
feels sourceless.

## Two distinct defects

### 1. The reflect has no death guard — a corpse hits you back

`NewRound_DoCombat_unified.go:393-410` gates only on `returnDmg > 0`:

```go
if returnDmg > 0 {
    atkChar.ApplyHarm(characters.PoolHealth, returnDmg, charActorRef(defChar))
    emitReturnDamageText(atk, def, returnDmg)
    for _, buffId := range mutations.GetReflectRiderBuffs(defChar.Mutations) {
        atk.AddBuff(buffId, "mutation")
    }
}
```

There is **no check that the defender is still alive**. Damage is applied
upstream (the `res.DamageToTarget <= 0` gate at `:349`), and since U5c death is
immediate at the harm site, a killing blow leaves the defender **already Dead**
by the time this block runs. Reflect damage and the rider buff both land anyway.

This is why it fires "after the fight is over", and it gets more visible the
harder you hit: reflect is proportional to damage dealt, so a high-damage
character both kills in one blow *and* takes the largest recoil from the thing
it just killed.

**Open design question, not a foregone conclusion.** A dying creature's
telekinetic skin lashing out one last time is defensible flavour. If it is kept,
the message has to name it (defect 2). If it is not, the fix is a liveness check
before the reflect block.

### 2. The copy hides a source the player can plainly see

The victim's line is written as though the cause were invisible:

> `An unseen force slams into you, sending you staggering!`

It is not unseen. It is the elemental being punched. The room-facing line is
already better, because an observer genuinely does not see the cause:

> `{source_plain} staggers as though struck by something unseen.`

Compounding it: the buff carries no attribution, so when it fires on a killing
blow the player gets a debuff from nothing, attached to nothing, after combat
has ended.

**Owner, 2026-08-30:** *"It'd be better to have a source for it because it fires
after the fight is over in some cases."*

## Suggested fix

Smallest change that resolves the confusion, whichever way the design question
above is settled: rewrite `start_user_text` to name the recoil rather than an
unseen force, e.g. something on the shape of *"The force sheathing {source}
rebounds through you, and you stagger."* — a phrasing that still reads correctly
when the source has just died.

If the death guard is also wanted, add a liveness check on `defChar` before the
reflect block at `NewRound_DoCombat_unified.go:393`.

## Not caused by U11

Predates it by seven weeks. Recorded here because the U11 gate is what surfaced
the question.

**Note the related family:** `GetReflectRiderBuffs`'s own docstring names the
Ironhide Reflect Skin flavours (Molten burn, Frostbite chill, Voltaic shock) as
carrying the same effect type. Any fix should check whether those messages have
the same sourceless problem, rather than fixing 109 alone.
