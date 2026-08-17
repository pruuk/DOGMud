# U8 action-cost model evidence

**Status:** The owner-selected package remains the only package Task 2 is
authorized to encode. Corrected actual-admitted accounting re-ranked the
generated comparison rows, but the owner package still passes every gate.

This evidence is generated and asserted by
`tools/balance/unified_resolution_model.py`. The script reads shipped scalars
from `_datafiles/config.yaml`, discovers ranged capacity from the item
state/schema, and reads the tracked anonymized live-character band from
`tools/playtest/profiles/veteran.yaml`. It does not read production saves.

## Owner checkpoint

| Package | Pressure | Special | Shoot | Reload | Sneak | Rhetoric | Grapple |
|---|---:|---:|---:|---:|---:|---:|---:|
| Generated Low | 10.31% | 1.25 | 2 | 1 | 0.5 | 4 | 0.5 |
| Generated Midpoint | 23.93% | 1.5 | 2 | 1.5 | 3.5 | 5 | 0.75 |
| Generated High | 37.54% | 4 | 3.5 | 0.75 | 4 | 5 | 5 |
| **Owner-selected** | **23.85%** | **4** | **2** | **1** | **2.5** | **4** | **2** |

The owner selected the final row from the pre-fix generated Midpoint row.
After review, pressure counts only actually admitted whole-point debits rather
than attempted fractional cost. That correction moved the generated Midpoint
without invalidating the owner package: it remains near the center of the
passing range and passes every corrected gate. The model finds 19,600 passing
packages. Attempted and admitted debit are reported separately so refused
full-pay actions cannot inflate pressure.

| Exact selected config knob | Base |
|---|---:|
| `SpecialMoveBaseStaminaCost` | 4 |
| `ShootBaseStaminaCost` | 2 |
| `ReloadBaseStaminaCost` | 1 |
| `SneakBaseStaminaCost` | 2.5 |
| `RhetoricActionBaseConvictionCost` | 4 |
| `GrappleStaminaCostPerRound` | 2 |

All six are bases before shared `costs.Calc` encumbrance, inverse
governing-skill, and documented action modifiers.

## Inputs and cadence

| Fixture | Stat(s) | Weapon | Unarmed | Ranged | Skullduggery | Rhetoric | Stamina | Conviction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Novice | 100 | 5 | 5 | 5 | 5 | 5 | 405 | 405 |
| Mid-skill | 110 | 25 | 25 | 25 | 25 | 25 | 445 | 445 |
| Veteran | 136 | 69 | 69 | 69 | 69 | 69 | 549 | 549 |
| Synthetic high | 175 | 100 | 100 | 100 | 100 | 100 | 705 | 705 |
| Anonymized live band | mixed | 69 | 57 | 1 | 50 | 55 | 504 | 522 |

The shipped 66% reservation cap leaves usable Stamina of 138, 152, 187, 240,
and 172 and usable Conviction of 138, 152, 187, 240, and 178. Regeneration is
2% of raw maximum each third round; combat Stamina regeneration is quartered.

`SpecialMoveCooldown` is 4. Missing `SneakFailCooldown` remains zero, allowing
one detected attempt per round. Ranged capacity is one projectile because live
state is the single `Item.Loaded` boolean and the schema exposes no capacity.
One builder supplies this cadence to gates, scoring, assertions, and output:

| Round | 1 | 2 | 3 | 6 | 7 | 10 | 11 | 14 | 15 | 18 | 19 | 22 | 23 | 26 | 27 | 30 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Action | shoot | reload | shoot | reload | shoot | reload | shoot | reload | shoot | reload | shoot | reload | shoot | reload | shoot | reload |

## Owner-selected action equivalents

Every selected base is shown per fixture at 50% load. The denominator is one
ordinary swing using the action's governing skill rank. Rhetoric is a
cross-pool comparison. Special uses representative Unarmed Combat; its 4x
ratio is also exact for Weapon Combat and Skullduggery. Grapple includes the
role multiplier before `costs.Calc`.

| Fixture | Action | Pool | Skill | Cost | Swings |
|---|---|---|---|---:|---:|
| Novice | special | Stamina | Unarmed | 5.76 | 4.00x |
| Novice | shoot | Stamina | Ranged | 2.88 | 2.00x |
| Novice | reload | Stamina | Ranged | 1.44 | 1.00x |
| Novice | sneak | Stamina | Skullduggery | 3.60 | 2.50x |
| Novice | rhetoric | Conviction | Rhetoric | 4.32 | 3.00x |
| Novice | grapple controller | Stamina | Unarmed | 2.88 | 2.00x |
| Novice | grapple controlled | Stamina | Unarmed | 5.76 | 4.00x |
| Mid-skill | special | Stamina | Unarmed | 5.33 | 4.00x |
| Mid-skill | shoot | Stamina | Ranged | 2.67 | 2.00x |
| Mid-skill | reload | Stamina | Ranged | 1.33 | 1.00x |
| Mid-skill | sneak | Stamina | Skullduggery | 3.33 | 2.50x |
| Mid-skill | rhetoric | Conviction | Rhetoric | 4.00 | 3.00x |
| Mid-skill | grapple controller | Stamina | Unarmed | 2.67 | 2.00x |
| Mid-skill | grapple controlled | Stamina | Unarmed | 5.33 | 4.00x |
| Veteran | special | Stamina | Unarmed | 3.46 | 4.00x |
| Veteran | shoot | Stamina | Ranged | 1.73 | 2.00x |
| Veteran | reload | Stamina | Ranged | 0.86 | 1.00x |
| Veteran | sneak | Stamina | Skullduggery | 2.16 | 2.50x |
| Veteran | rhetoric | Conviction | Rhetoric | 2.59 | 3.00x |
| Veteran | grapple controller | Stamina | Unarmed | 1.73 | 2.00x |
| Veteran | grapple controlled | Stamina | Unarmed | 3.46 | 4.00x |
| Synthetic high | special | Stamina | Unarmed | 2.13 | 4.00x |
| Synthetic high | shoot | Stamina | Ranged | 1.07 | 2.00x |
| Synthetic high | reload | Stamina | Ranged | 0.53 | 1.00x |
| Synthetic high | sneak | Stamina | Skullduggery | 1.33 | 2.50x |
| Synthetic high | rhetoric | Conviction | Rhetoric | 1.60 | 3.00x |
| Synthetic high | grapple controller | Stamina | Unarmed | 1.07 | 2.00x |
| Synthetic high | grapple controlled | Stamina | Unarmed | 2.13 | 4.00x |
| Anonymized live band | special | Stamina | Unarmed | 3.97 | 4.00x |
| Anonymized live band | shoot | Stamina | Ranged | 2.92 | 2.00x |
| Anonymized live band | reload | Stamina | Ranged | 1.46 | 1.00x |
| Anonymized live band | sneak | Stamina | Skullduggery | 2.67 | 2.50x |
| Anonymized live band | rhetoric | Conviction | Rhetoric | 3.04 | 3.00x |
| Anonymized live band | grapple controller | Stamina | Unarmed | 1.98 | 2.00x |
| Anonymized live band | grapple controlled | Stamina | Unarmed | 3.97 | 4.00x |

## Thirty-round ranged and rhetoric summary

Cells are `attempted / admitted / ending pool` after exact hook recovery.

| Package | Fixture | Ranged | Rhetoric |
|---|---|---:|---:|
| Low | Novice | 34.6/34/124 | 34.6/34/138 |
| Low | Mid-skill | 32.0/32/140 | 32.0/32/152 |
| Low | Veteran | 20.7/20/187 | 20.7/20/187 |
| Low | Synthetic high | 12.8/12/240 | 12.8/12/240 |
| Low | Live band | 35.1/35/157 | 24.3/24/178 |
| Midpoint | Novice | 40.3/40/118 | 43.2/43/138 |
| Midpoint | Mid-skill | 37.3/37/135 | 40.0/40/152 |
| Midpoint | Veteran | 24.2/24/183 | 25.9/25/187 |
| Midpoint | Synthetic high | 14.9/14/240 | 16.0/16/240 |
| Midpoint | Live band | 40.9/40/152 | 30.4/30/178 |
| High | Novice | 49.0/48/110 | 43.2/43/138 |
| High | Mid-skill | 45.3/45/127 | 40.0/40/152 |
| High | Veteran | 29.4/29/178 | 25.9/25/187 |
| High | Synthetic high | 18.1/18/240 | 16.0/16/240 |
| High | Live band | 49.7/49/143 | 30.4/30/178 |
| **Owner** | **Novice** | **34.6/34/124** | **34.6/34/138** |
| **Owner** | **Mid-skill** | **32.0/32/140** | **32.0/32/152** |
| **Owner** | **Veteran** | **20.7/20/187** | **20.7/20/187** |
| **Owner** | **Synthetic high** | **12.8/12/240** | **12.8/12/240** |
| **Owner** | **Live band** | **35.1/35/157** | **24.3/24/178** |

## Exact owner-selected recovery/state traces

These novice tables use the same asserted event builders as gates and scoring.
Pool is after actions and then any eligible regeneration.

### Combined special, attack, and defence

| Round | Resolved | Pool | Regen |
|---:|---|---:|---:|
| 1 | attack, defence, special | 129 | 0 |
| 2 | attack, defence | 126 | 0 |
| 3 | attack, defence | 125 | 2 |
| 4 | attack, defence | 122 | 0 |
| 5 | attack, defence, special | 113 | 0 |
| 6 | attack, defence | 112 | 2 |
| 7 | attack, defence | 108 | 0 |
| 8 | attack, defence | 105 | 0 |
| 9 | attack, defence, special | 98 | 2 |
| 10 | attack, defence | 95 | 0 |
| 11 | attack, defence | 92 | 0 |
| 12 | attack, defence | 90 | 2 |
| 13 | attack, defence, special | 81 | 0 |
| 14 | attack, defence | 78 | 0 |
| 15 | attack, defence | 77 | 2 |
| 16 | attack, defence | 74 | 0 |
| 17 | attack, defence, special | 65 | 0 |
| 18 | attack, defence | 63 | 2 |
| 19 | attack, defence | 60 | 0 |
| 20 | attack, defence | 57 | 0 |
| 21 | attack, defence, special | 50 | 2 |
| 22 | attack, defence | 47 | 0 |
| 23 | attack, defence | 43 | 0 |
| 24 | attack, defence | 42 | 2 |
| 25 | attack, defence, special | 33 | 0 |
| 26 | attack, defence | 30 | 0 |
| 27 | attack, defence | 29 | 2 |
| 28 | attack, defence | 25 | 0 |
| 29 | attack, defence, special | 16 | 0 |
| 30 | attack, defence | 15 | 2 |

### Detected sneak and awareness reset

Every resolved row pays and follows `Visible -> Concealing -> Visible`; the
last state explicitly represents the detected-attempt awareness reset.

| Round | State | Pool | Regen |
|---:|---|---:|---:|
| 1 | Visible -> Concealing -> Visible | 135 | 0 |
| 2 | Visible -> Concealing -> Visible | 131 | 0 |
| 3 | Visible -> Concealing -> Visible | 136 | 8 |
| 4 | Visible -> Concealing -> Visible | 132 | 0 |
| 5 | Visible -> Concealing -> Visible | 128 | 0 |
| 6 | Visible -> Concealing -> Visible | 133 | 8 |
| 7 | Visible -> Concealing -> Visible | 129 | 0 |
| 8 | Visible -> Concealing -> Visible | 126 | 0 |
| 9 | Visible -> Concealing -> Visible | 130 | 8 |
| 10 | Visible -> Concealing -> Visible | 126 | 0 |
| 11 | Visible -> Concealing -> Visible | 123 | 0 |
| 12 | Visible -> Concealing -> Visible | 127 | 8 |
| 13 | Visible -> Concealing -> Visible | 124 | 0 |
| 14 | Visible -> Concealing -> Visible | 120 | 0 |
| 15 | Visible -> Concealing -> Visible | 124 | 8 |
| 16 | Visible -> Concealing -> Visible | 121 | 0 |
| 17 | Visible -> Concealing -> Visible | 117 | 0 |
| 18 | Visible -> Concealing -> Visible | 122 | 8 |
| 19 | Visible -> Concealing -> Visible | 118 | 0 |
| 20 | Visible -> Concealing -> Visible | 114 | 0 |
| 21 | Visible -> Concealing -> Visible | 119 | 8 |
| 22 | Visible -> Concealing -> Visible | 115 | 0 |
| 23 | Visible -> Concealing -> Visible | 112 | 0 |
| 24 | Visible -> Concealing -> Visible | 116 | 8 |
| 25 | Visible -> Concealing -> Visible | 112 | 0 |
| 26 | Visible -> Concealing -> Visible | 109 | 0 |
| 27 | Visible -> Concealing -> Visible | 113 | 8 |
| 28 | Visible -> Concealing -> Visible | 110 | 0 |
| 29 | Visible -> Concealing -> Visible | 106 | 0 |
| 30 | Visible -> Concealing -> Visible | 110 | 8 |

### Rhetoric cadence and recovery

| Round | Event | Pool | Regen |
|---:|---|---:|---:|
| 1 | rhetoric | 134 | 0 |
| 2 | idle | 134 | 0 |
| 3 | idle | 138 | 8 |
| 4 | idle | 138 | 0 |
| 5 | rhetoric | 134 | 0 |
| 6 | idle | 138 | 8 |
| 7 | idle | 138 | 0 |
| 8 | idle | 138 | 0 |
| 9 | rhetoric | 138 | 8 |
| 10 | idle | 138 | 0 |
| 11 | idle | 138 | 0 |
| 12 | idle | 138 | 8 |
| 13 | rhetoric | 133 | 0 |
| 14 | idle | 133 | 0 |
| 15 | idle | 138 | 8 |
| 16 | idle | 138 | 0 |
| 17 | rhetoric | 134 | 0 |
| 18 | idle | 138 | 8 |
| 19 | idle | 138 | 0 |
| 20 | idle | 138 | 0 |
| 21 | rhetoric | 138 | 8 |
| 22 | idle | 138 | 0 |
| 23 | idle | 138 | 0 |
| 24 | idle | 138 | 8 |
| 25 | rhetoric | 133 | 0 |
| 26 | idle | 133 | 0 |
| 27 | idle | 138 | 8 |
| 28 | idle | 138 | 0 |
| 29 | rhetoric | 134 | 0 |
| 30 | idle | 138 | 8 |

## Grapple and admission evidence

Ten-round novice cells are `controller/controlled` after maintenance and
combat regeneration. Role ratio remains 2:1 at every modeled load.

| Package | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Low | 138/137 | 137/136 | 138/136 | 138/135 | 137/133 | 138/134 | 137/132 | 137/131 | 138/132 | 137/130 |
| Midpoint | 137/136 | 136/134 | 137/134 | 136/132 | 135/130 | 136/130 | 135/127 | 134/125 | 135/125 | 134/123 |
| High | 131/124 | 124/110 | 119/97 | 112/83 | 104/68 | 99/56 | 92/42 | 85/27 | 80/15 | 72/0 |
| **Owner** | **136/133** | **133/127** | **132/123** | **129/117** | **126/112** | **125/108** | **122/102** | **119/96** | **119/93** | **116/87** |

The model asserts refusal no-mutation, separate attempted/admitted counters,
fractional carry, partial-payment writeoff, recovery, explicit modifier zero,
and negative-modifier rejection. Owner full-pay novice examples are:

| Pool | Affordable | Exhausted | Recovered |
|---|---|---|---|
| Stamina | paid, 133 | refused, unchanged at 0 | round 3, paid, 3 |
| Conviction | paid, 134 | refused, unchanged at 0 | round 3, paid, 4 |

## Owner rationale

> The listed values are config bases before the shared `costs.Calc`
> encumbrance, inverse governing-skill, and documented modifiers. Grapple
> initiation is another special move and therefore uses
> `SpecialMoveBaseStaminaCost = 4`. `GrappleStaminaCostPerRound = 2` applies
> only to ongoing per-round grapple maintenance; the existing
> controller/controlled role multiplier applies to that base before
> `costs.Calc`. The owner considers base 2 appropriate for ongoing grapple
> actions.

The plan shorthand `SpecialMoveCostBase` and `GrappleCostBase` maps to the
exact config names above. Task 2 must not charge grapple initiation at the
maintenance base or apply the maintenance multiplier after `costs.Calc`.
