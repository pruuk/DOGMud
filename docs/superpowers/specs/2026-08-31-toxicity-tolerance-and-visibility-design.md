# Toxicity: Alchemical Tolerance and Graduated Feedback — Design

**Created:** 2026-08-31
**Status:** Design approved by owner 2026-08-31. No plan written yet.
**Origin:** Owner report from play — *"toxicity from drinking potions doesn't
seem to be doing anything. I can drink 4 potions in a row on Meirok and my
toxicity still shows as clear."*

---

## Facts verified against source, 2026-08-31

| Fact | Evidence |
|---|---|
| The wiring is **fine**. `drink.go:168` calls `AddToxicity`, and `AddToxicity` accumulates and clamps correctly | `internal/usercommands/drink.go`, `internal/characters/resources.go:460` |
| `ItemSpec.Toxicity` is **exported with a correct yaml tag** — this is NOT the unexported-field trap | `internal/items/itemspec.go:336` |
| **Nine of the 34 drinkables carry no `toxicity:` field at all.** `drink.go:167` gates on `if itemSpec.Toxicity > 0`, so `AddToxicity` never fires for them | Small Red Potion 30001, Conviction Draught 30012, Herbal Tea 30024, Minor Antidote 30028, Clarity Tonic 30029, Fire Resistance Draught 30030, Berserker Elixir 30032, Catalyst of Unmaking 30067, Phial of Second Birth 40181 |
| **Two of those share a NAME with a toxic alchemy twin** | Conviction Draught 30012 (none) vs 30038 (8); Berserker Elixir 30032 (none) vs 30049 (25) |
| Current max is `ToxicityBaseMax + Vitality/ToxicityVitalityScale` | `internal/characters/resources.go:452` |
| **All five toxicity knobs are ABSENT from `config.yaml`**, so all run on Go defaults: base 100, vitality scale 5, decay 1.0/tick, sickness 0.02, high-decay slow 0.5 | `grep -c -i toxicity _datafiles/config.yaml` → 0; defaults at `internal/configs/config.balance.combat.go:375-389` |
| Bands are 50 / 75 / 90 percent, and `ToxicityBand` mirrors `GetToxicityPenalties` exactly | `internal/characters/resources.go:496`, `:523` |
| Above 90% there is **acute HP damage** that scales past the cap and can kill | `ToxicitySicknessDamage`, `resources.go:476` |
| `{tox}` is **deliberately blank at band 0** | `internal/users/userrecord.prompt.go:685` |
| `status` does not show toxicity at all | no match for `toxic` in `internal/usercommands/status.go` |
| An alchemy skill exists, primary stat perception, progression multiplier 1.56 | `internal/skills/skills.go:40`, `:336`, `:432` |
| Skill level is read by `c.GetSkillLevel(skills.Alchemy)` | `internal/characters/skills.go:170` |
| **Meirok has alchemy 58, vitality base 104** | `_datafiles/world/dogmud/users/3.yaml:825`, `:66` |
| The three mid-tier potions at toxicity 14 | Warrior's Brew 30039, Preacher's Tincture 30040, Windrunner Draught 30041 |
| 🔴 **`toxicity.template` exists but is NOT registered in `keywords.yaml`**, so `help toxicity` is unreachable | `grep -n toxicity _datafiles/world/dogmud/keywords.yaml` → no match |
| 🔴 **`alchemy.template` never mentions toxicity** | `grep -i toxic` on that file → no match |

---

## The problem, in three parts

**1. Nine potions cost nothing.** They are the pre-alchemy generation. The
alchemy system authored `toxicity` on its own 21 new potions (30036-30056) and
never backfilled what already existed. A player can dodge the mechanic entirely
by drinking only old potions, and two potions with the *same name* behave
differently depending on which one is in the pack.

**2. The ceiling is flat and uninteresting.** `100 + Vit/5` means a fresh
character sits at 120 and a veteran at 130 — a 8% spread for a 50% difference in
vitality. Nothing a player does moves it meaningfully, and the number expresses
nothing about them.

**3. Nothing is visible until it already hurts.** The first band is at 50% of
max, so every value from zero to half renders identically as "clear", and `{tox}`
prints nothing at all there. A player has no way to see pressure building, and no
way to learn the mechanic exists.

---

## The model

### Tolerance is earned by brewing, not by being tough

> Owner: *"use Alchemy + Vit / 3 and tune to that instead. It's more interesting
> and also makes sense from a flavor standpoint. You've been dabbling in alchemy
> and have built up a tolerance."*

```
GetToxicityMax() = alchemy/ToxicityAlchemyScale + Vitality/ToxicityVitalityScale
```

Shipped: `ToxicityAlchemyScale: 1.5`, `ToxicityVitalityScale: 3`,
`ToxicityBaseMax: 0`.

`ToxicityBaseMax` **stays as a knob** and is set to zero rather than deleted, so
a flat floor can be restored without a code change.

| | Alchemy | Vit | Max | Low tier (8) | Mid tier (11) |
|---|---:|---:|---:|---|---|
| Noob | 0 | 100 | 33 | 3rd bites | 2nd |
| Veteran, no alchemy | 0 | 150 | 50 | 4th | 3rd |
| Mid alchemist | 25 | 125 | 58 | 4th | 3rd |
| **Meirok (real save)** | **58** | **104** | **73** | **5th** | **4th** |

This meets the owner's target — *"a vet should be able to drink like 4 of the low
tier pots… the 5th would push them into a toxicity effect"*, and mid-tier one
earlier — measured against a **real character file**, not an invented one.

🔑 **The flavour falls out of the maths**: a veteran who has never brewed is
*less* tolerant than a mid-level dabbler. Toughness is not tolerance. That is the
point of the change.

### The ratio problem, and why a potion value has to move

⚠️ **No choice of scale can satisfy both targets with today's values.** For "5th
low-tier" and "4th mid-tier" to hold simultaneously, mid must cost about
**1.25×** low. Today it is **14 vs 8 = 1.75×**, so at *any* threshold where the
5th salve bites, the **3rd** Preacher's has already bitten.

This is a ratio, not a scale — moving the ceiling moves both together and never
separates them.

**Resolution: mid tier 14 → 11** on Warrior's Brew, Preacher's Tincture and
Windrunner Draught.

The alternative was raising low tier 8 → 11, which preserves mid-tier's cost but
makes the basic healing salve more taxing — and that lands hardest on new
players, who have the least tolerance under the new formula. Lowering is the
gentler correction.

### Backfill for the nine

Scaled to effect strength against the alchemy range as yardstick.

| Potion | Toxicity | Reasoning |
|---|---:|---|
| Herbal Tea 30024 | 3 | trivial comfort drink |
| Small Red Potion 30001 | 6 | basic heal; deliberately under the alchemy salve's 8 |
| Minor Antidote 30028 | 6 | small remedy |
| Clarity Tonic 30029 | 8 | low-tier utility |
| Conviction Draught 30012 | 8 | **matches its twin 30038** |
| Fire Resistance Draught 30030 | 11 | mid-tier ward |
| Berserker Elixir 30032 | 25 | **matches its twin 30049** |
| Catalyst of Unmaking 30067 | 60 | sits with purging draught and essence of growth |
| Phial of Second Birth 40181 | 80 | sits with mutagen brew |

⚠️ **The two twins take their partner's value exactly.** Same name, same cost, so
the matcher picking either one stops mattering. Merging the duplicate items is
explicitly **out of scope** — players own both, and item merges are a migration.

### Graduated feedback

> Owner: *"show a textual description at different levels so you can see it
> coming, but not be too worried about it until you are close."*

| Range | Shown | Penalty |
|---|---|---|
| under 15% | nothing | none |
| 15–30% | a faint sourness | **none** |
| 30–50% | unsettled | **none** |
| 50–75% | queasy | existing, unchanged |
| 75–90% | sick | existing, unchanged |
| 90%+ | critical | existing, unchanged, plus acute HP damage |

🔑 **The two new bands are FEEDBACK ONLY.** `GetToxicityPenalties` and
`ToxicitySicknessDamage` keep their 50 / 75 / 90 thresholds untouched. This
changes what a player can *see*, never what they *suffer*.

⚠️ `ToxicityBand` and `GetToxicityPenalties` currently mirror each other exactly,
and the code says so. **That relationship is deliberately broken by this
change** — bands become finer than penalties. Both functions must carry a comment
saying so, or the next reader will "restore" the symmetry and silently delete the
warning tiers.

Surfaced in two places: `{tox}` starts rendering from the faint band upward
(still blank below 15%, so a default prompt stays quiet for an unaffected
player), and `status` gains a toxicity line.

🔑 **`status` is the documented exception to the no-hard-numbers rule** (it is a
deliberate mechanical display), but this line stays **descriptive** anyway, for
consistency with every other player-facing toxicity string.

### Toxicity clears on death

**Today it does not, and nothing clears it.** The only two `Toxicity = 0` sites
are floor clamps: one in `AddToxicity` (`resources.go:466`) and one in the decay
tick (`NewRound_AutoHeal.go:86`). Death resets Health, Stamina and Conviction to
5% and leaves toxicity exactly where it was.

**Owner ruling, 2026-08-31:** it must clear, and the reasoning is consistency
rather than mercy:

> *"We clear conditions, so the potions that the player used to generate the
> toxicity are no longer affecting the player, so they can't also be affected by
> toxicity. They also lose the cost/time from creating the potions because the
> effects got cleared on death."*

Verified: `internal/hooks/Life_Cascades.go:62` calls
`c.CancelBuffsWithFlag(buffs.All)` in the `Alive → Dead` branch, so **every**
buff is stripped on death, potion effects included.

🔑 **So this is not a free purge.** The player has already lost the potions
themselves, the materials, the brewing time, and every effect they paid for.
Leaving the toxicity behind would charge them a second time for a benefit they no
longer hold. Toxicity is the price of an effect; with no effect there is no
price.

**Implementation:** set `c.Toxicity = 0` in the `Alive → Dead` cascade in
`internal/hooks/Life_Cascades.go`, immediately alongside the existing
`CancelBuffsWithFlag(buffs.All)`, so the two always move together. A comment must
tie them, because separating them is what re-creates the bug.

⚠️ **This also closes a death spiral that the new formula would have sharpened.**
`ToxicitySicknessDamage` deals acute HP damage above 90% that scales past the cap
and can kill, and decay HALVES above 75% — slowest exactly where it is most
dangerous. Without clearing, a player could die of toxicity, respawn at 5% health
still at critical toxicity, and die again with no way out. The new formula makes
that worse, not better: Meirok's ceiling falls from about 130 to about 73, and a
low-alchemy character sits near 33, so the danger band is far easier to reach.

### Documentation

The mechanic is undocumented in practice: its helpfile cannot be reached, and
the page that should point at it does not mention it.

- **Register `toxicity` in `keywords.yaml`** with aliases (`toxic`, `poison`,
  `tolerance`).
- **Cross-link it** from `alchemy`, `craft` and `health`.
- **Update `toxicity.template`** for the new formula, naming alchemy as the
  tolerance source and describing the bands in words.
- **Update `alchemy.template`** to say that brewing raises tolerance.

⚠️ **Template function names resolve at PARSE time and help templates parse
LAZILY**, so a typo passes build and boot and reaches a player as
`[TEMPLATE ERROR]`. Any template edit must be checked by rendering the page, not
by booting.

---

## Config

All five knobs are absent today. This adds them explicitly, with the new sixth,
so the subsystem stops running on invisible Go defaults.

```yaml
  ToxicityBaseMax: 0            # flat floor; 0 = tolerance is entirely earned
  ToxicityAlchemyScale: 1.5     # divisor on alchemy skill
  ToxicityVitalityScale: 3      # divisor on vitality
  ToxicityDecayPerTick: 1.0     # unchanged from the Go default
  ToxicitySicknessDamagePct: 0.02
  ToxicityHighDecaySlowMult: 0.5
```

⚠️ **`config.yaml` carries `skip-worktree`.** Build the commit from the
`git show HEAD:` blob with only the intended lines spliced in, and restore the
flag afterwards. The working copy holds local-only `HttpPort`, `LogLevel` and
`Playtest` settings that must not be committed.

🔴 **CONFIRMED BLOCKER, verified in source.** `config.balance.combat.go:379`
reads:

```go
if b.ToxicityBaseMax <= 0 {
    b.ToxicityBaseMax = 100
}
```

**`ToxicityBaseMax: 0` is overwritten back to 100 on load**, which restores the
flat ceiling and silently reverts this entire model. Nothing would fail; the
numbers would simply be wrong, and the fifth-potion target would miss by a wide
margin with no error anywhere.

**The validation must be changed to accept an explicit zero before any of the
tuning is written**, or the change cannot be tested at all. This is the same
family as the recorded trap where `if x < 0 || x > 1.0 { x = default }` can never
fire because an absent key unmarshals to 0 — a guard whose shape makes a legal
value unreachable. See the config-audit backlog entry.

⚠️ The same guard shape protects `ToxicityVitalityScale` and the new
`ToxicityAlchemyScale`. Those are divisors and must never be zero, so their
guards are correct and must stay.

---

## Out of scope

- **Merging the duplicate potion items.** Players own both; that is a migration.
- **Changing the penalty thresholds or the acute damage curve.** Feedback only.
- **Retuning decay.** `ToxicityDecayPerTick` stays at 1.0/tick; the model was
  fitted to a burst of potions, not to sustained load, and changing both at once
  would make neither measurable.

## Done when

1. All 34 drinkables carry a toxicity value, and the two same-named pairs agree.
2. `GetToxicityMax` reads alchemy, and Meirok's real save yields the fifth
   low-tier potion as the one that bites.
3. Two feedback bands exist below the first penalty band, and no penalty
   threshold has moved.
4. **Toxicity is zero after death**, pinned by a test, and the clear sits beside
   the buff strip it is justified by.
5. `help toxicity` resolves, and alchemy, craft and health link to it.
6. Boot clean, and a test pins the four-tier table above so a later knob edit
   cannot silently move where the fifth potion lands.
