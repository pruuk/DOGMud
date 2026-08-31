# U6b modelling gate — fragment C: counters, the unowned family, defence cost load (items 7–9)

Script: `tools/balance/u6b_model_counters_family_costs.py` (new, self-contained;
imports `tools/balance/unified_resolution_model.py` for cost/regen/trace
machinery, unmodified). Method: **numeric integration** (closed-form normal
CDFs for single-defender contests; 6001-point z-grid over the attacker's roll
for best-of-N sets), cross-checked by **fixed-seed Monte Carlo, 2,000,000
samples/cell** on two anchor cells (agreement within 0.06pp). Sanity anchors
asserted: parity pre-floor win 50.00%, parity pre-floor crit 2.275%.

Shipped values used (from `git show HEAD:_datafiles/config.yaml`): RollSpread
0.15, SkillWeight 5.0, ContestFloor 0.125, ContestCritThreshold 2.0,
MinDefense/AttackCritChance 0.01, MeleeDamageScale 0.52,
GlobalDamageMultiplier 0.5, DefenceBaseStaminaCost 1.0, dodge/parry/block cost
modifiers 1.25/1.10/1.15, dodge/parry/block effectiveness 0.95/0.97/1.05,
CritDamageBase 2.0 + 0.05/rank, PlayerStaminaRegenPct 0.02 (quartered in
combat, hook every 3rd round), SubSkillWeight 1.5 (Go default, absent from
YAML), StealSkillMultiplier 1.0 (Go default, absent from YAML).

Tiers: novice 100/5, journeyman 120/25, adept 135/40, Meirok (Str 115, Dex 110
effective, Per 101, Vit 104; weapon 69, unarmed 57, skullduggery 50). Trash mob
90/1. Ratio-sweep opponents scale the tier's stats AND skills by r.

## Contradictions between the task prompt / spec and code (code wins, mirrored)

1. **Defence base cost is 1.0 shipped, not 2.** The prompt's "base 2" matches
   `QuellBaseConvictionCost`/`DefyBaseConvictionCost` (both 2), not the physical
   `DefenceBaseStaminaCost: 1.0`. All item-9 numbers use 1.0.
2. **Throw's attacker score is `Dex + skullduggery×W`**, not "skullduggery×5"
   alone (`usercommands/throw.go:271`). Defender is `Dex + Per×W×0.5` — the
   ×2.5 is `SkillWeight×0.5`, i.e. config-coupled, not a literal.
3. **The counter tier is parry-crit-only today** (`combat_shared_helpers.go`):
   dodge crit → auto-trip, block crit → auto-bash, each with its own damage.
   The tables price every defensive crit at riposte value
   (`CalcRawDamage(Str, combatSkill, 0.5)`, mitigated) — a stated
   approximation, roughly right in magnitude since trip/bash carry their own
   damage percents.
4. **Grapple initiation uses `GetCombatSkillLevel()`** (equipped combat skill),
   not unarmed specifically (`grapple.go:47`). Immaterial for synthetic tiers.
5. **Grapple drift runs through `combat.RunContest`, so the 12.5% ContestFloor
   applies to drift**: a flipped outcome carries the ±1 sentinel margin, whose
   z is ≈0, i.e. **12.5% of all drift rounds are forced Holds** today and in
   every fix variant. Neither the spec nor the prompt mentions this.
6. **`CritDamageMultiplier` is uncapped by rank** (crit_damage.go:53) — the
   model capped rank at 50; no modelled cell is affected (all relevant ranks
   ≤50), but a rank-69 crit multiplier is 5.45, not 4.5, if anyone reuses these
   numbers for weapon-combat crits.
7. The melee defensive-crit bar is the hardcoded 2.0 in
   `resolveDefenseOutcomeCore` (combat_helpers.go:966), and the **fumble branch
   returns first**, so a fumbled swing can never be defensively crit — modelled
   (costs the defender the 2.28% of swings that would be the easiest crits).

---

## Item 7 — Counter frequency and worth

P(defensive crit) per swing (post-floor, incl. 1% promotion, fumbles excluded);
counter = riposte at defender's Str/combat-skill; attacker swings 1/round
(scale linearly for faster weapons). Defender has all three defences
(dodge/parry/block, effectiveness applied).

| defender | attacker | atkScore | P(defWin) | **P(defCrit)/swing** | riposte | ctr dmg/rd |
|---|---|---|---|---|---|---|
| novice | trash mob | 95 | 86.8% | **45.4%** | 21.2 | 9.6 |
| novice | parity | 125 | 68.3% | 4.4% | 21.2 | 0.9 |
| novice | 1.5× | 188 | 22.5% | 0.11% | 21.2 | 0.02 |
| journeyman | trash mob | 95 | 87.5% | **85.5%** | 37.7 | 32.2 |
| journeyman | parity | 245 | 68.3% | 4.4% | 37.7 | 1.7 |
| adept | trash mob | 95 | 87.5% | **85.5%** | 48.9 | 41.9 |
| Meirok | trash mob | 95 | 87.5% | **85.5%** | 44.9 | 38.3 |
| Meirok | parity | 455 | 66.1% | 4.1% | 44.9 | 1.8 |
| Meirok | 0.8× | 364 | 85.3% | 31.5% | 44.9 | 14.1 |
| Meirok | 1.2× | 546 | 40.9% | 0.59% | 44.9 | 0.26 |
| Meirok | 2× | 910 | 14.1% | 0.01% | 44.9 | 0.00 |

Mob counters against players are symmetric in shape but tiny in practice:
a trash mob defending a journeyman+ attack crits ≈0.00% (novice attacker:
0.08%); a trash mob ATTACKING any journeyman+ defender eats an 85.5% counter
per swing.

**Findings, including the against-the-spec ones:**

- **Veteran-vs-trash is already degenerate at the defensive-crit ceiling.** The
  floor caps P(defCrit) at 87.5% (=1−ContestFloor); every non-parity-down
  matchup from journeyman up sits pinned there. A Meirok standing still takes
  ~38 counter damage per incoming trash swing-round — about **0.43 free
  half-swings/round** (his own swing raw is ~90) — before he spends a single
  action. Against a trash mob's ~365 HP pool that is ~10%/round of passive
  kill rate. This is TODAY's melee behaviour, not new to U6b; **what U6b does
  is multiply the surfaces** (specials, shots, spells, defy) that feed it. If
  the counter fires per §4.4 on all channels, farming veterans convert every
  enemy action of any kind into damage. Expect "AFK farming" to get visibly
  stronger; either accept it as mastery expression or bound counter frequency
  (e.g. once per round) — the model says once-per-round capping changes
  nothing at parity (4%) and only trims the pinned 85.5% cells.
- **At parity the counter is well-behaved**: 4.1–4.4% of swings, ~1–2
  dmg/round expected — flavour, not a second weapon.
- The prompt's hypothesis "defensive crit spikes when the defender outclasses
  the attacker" is confirmed with a sharp knee: 0.8× attacker → ~32%, 1.0× →
  4.4%, 1.2× → 0.6%. The transition band is roughly ±20% of score.

### Cross-room immunity (kiting)

Counter avoided per SHOT by shooting from the adjacent room (target defends
with dodge; shot cadence 1/4 rounds steady-state, capacity 1 / cd 4):

| shooter | target | P(defCrit) | ctr avoided/shot | shot dmg | avoided/shot ÷ shot dmg |
|---|---|---|---|---|---|
| novice | trash | 0.12% | 0.02 | 42.4 | 0.00 |
| novice | parity | 1.45% | 0.31 | 42.4 | 0.01 |
| novice | 1.5× | 44.3% | 15.3 | 42.4 | **0.36** |
| novice | 2× | 86.4% | 42.6 | 42.4 | **1.00** |
| adept | 1.5× | 44.3% | 35.0 | 97.9 | **0.36** |
| adept | 2× | 86.4% | 91.0 | 97.9 | **0.93** |

**Finding:** against equal-or-weaker targets, cross-room immunity is worth
≈nothing (≤1% of a shot's damage) — kiting is NOT incentivised for ordinary
play. Against superior targets it is worth **~36% of a shot's damage at 1.5×
and ~100% at 2×**: exactly the boss fights where the counter was meant to
threaten the attacker, a shooter who steps one room out deletes the entire
counter mechanism and loses nothing (there is no cross-room counter by
design, §4.4). If bosses are meant to punish ranged chip, this is a hole the
spec should name: the declared "coherent" no-counter case is also the optimal
strategy against every up-tier enemy.

---

## Item 8 — The unowned family at ×5

### Flee (×25 → ×5, both sides)

| fleer | blocker | P BEFORE | P AFTER | Δ |
|---|---|---|---|---|
| novice | trash mob | 86.7% | 77.8% | **−8.9** |
| novice | parity | 50.0% | 50.0% | 0 |
| novice | veteran (Meirok) | 12.5% | 12.5% | 0 (floor) |
| Meirok | trash mob | 87.5% | 87.5% | 0 (ceiling) |
| any | 1.5× / 2× | 13.2% / 12.5% | 13.2% / 12.5% | 0 |

**The headline is that there is no headline.** Because both sides carry the
same weight and σ scales with the attacker's score, multiplying both skill
terms by the same constant is nearly probability-invariant; and every large
mismatch is already pinned to the 12.5%/87.5% floor/ceiling. Novice-fleeing-
veteran: 225v1535 → 125v395, both floor. Veteran-fleeing-trash: 1360v115 →
360v95, both ceiling. The only visible movement is where skill MIX differs:
a novice fleeing a skill-1 trash mob **loses 8.9 points** (their skull-5 was
worth 125 at ×25, now 25). So ×25→×5 mostly changes flee's *future tunability*
(and deletes an in-`internal/` literal), not current outcomes — the "who
gains" answer is: **the lower-skilled side gains, and only against low-skill
blockers; nobody's floor/ceiling cell moves at all.**

### Grapple initiation (×1 → ×5)

Parity: 50.0% → 50.0%. Vs trash: novice 67.6% → 77.8%, adept 86.6% → 87.5%
(ceiling). Vs 1.5×: 13.2% → 13.2% (floor). The ×5 flip mainly hands
skill-carrying players the ceiling against skill-1 mobs one tier earlier.
Crit-failure (attacker falls prone): today P(fail ∧ self z≤−2) ≈ 0.4–2.0%
depending on matchup; the spec keeps fumble self-relative → flat ~2.28%,
i.e. veterans grappling trash will fall on their face MORE often after the
change (0.43% → 2.28%) — small but a player-visible oddity worth naming.

### Submission (×1.5 → ×5; crit self-z → margin)

| attacker | defender | P BEFORE | P AFTER | crit BEFORE | crit AFTER |
|---|---|---|---|---|---|
| journeyman | trash | 30.2% | **78.2%** | 1.8% | **18.0%** |
| adept | trash | 59.6% | **86.2%** | 2.0% | **48.0%** |
| Meirok | trash | 62.9% | **87.1%** | 2.0% | **61.8%** |
| any | parity | 12.5–12.8% | 12.5–19.1% | ~0–0.2% | ~0–0.1% |

Note the defender's Str+Vit double-stat means "parity" is a 2:1 score gap —
submissions vs parity are floor-bound before AND after. The big move is
vs trash: **submission crit (which stuns) goes from ~2% to 18–62%** once crit
derives from margin. Grapple → drift-to-mount → near-automatic stunning
submission becomes the strongest farming loop against skill-1 mobs. If that
is not intended, submission's crit needs its own bar or the stun needs a cap;
the model argues §4.5's "crit from the contest margin" is the single most
distorting line in the family.

### Throw (firebomb, itemMult 0.85, per hostile target)

| thrower | target | P(hit) BEF | E[dmg] BEF | E[dmg] AFTER | P(crit) AFTER | counter risk/throw |
|---|---|---|---|---|---|---|
| novice | trash | 12.5% | 4.5 | **42.1** | 22.0% | 0.02 |
| journeyman | trash | 19.2% | 12.3 | **165.5** | 73.3% | 0.00 |
| adept | trash | 58.3% | 48.5 | **280.3** | 81.1% | 0.00 |
| Meirok | parity | 49.0% | 35.8 | 48.7 | 1.6% | 1.6 |
| any | 1.5× | 12.5% | 8–10 | 8–18 | ~0% | 15–44 |

Today's defender pseudo-skill (`Per×2.5`) makes trash defence 315 vs a novice's
125 — grenades are nearly useless down-tier and that is a *bug-shaped* state.
But the AFTER state overshoots: with a skill-1 dodge defence and a margin crit
tier, **a journeyman firebomb does ~165 expected damage per trash target per
throw** (crit bypasses mitigation at ×3.25), versus ~30–40 for a melee swing —
and it is AoE. At ×5 + crit, throw becomes the best down-tier farming tool in
the game by a wide margin. The counter column shows the intended counterweight
(same-room def-crit → melee counter vs the thrower) only bites UP-tier
(15–44 dmg/throw vs 1.5× targets), where throw is already floor-bound. If
§8-Q5 lands on "AoE gets a defence set", the damage/crit half needs a
concurrent look — model recommends flagging grenade crit-bypass specifically.

### Steal (sqrt-curve×25 → ×5; defender raw Per → Per + skill×5 [ASSUMPTION: defender skill = combat skill])

| thief | mark | P BEFORE | P AFTER |
|---|---|---|---|
| novice | trash | 84.2% | 77.8% |
| novice | parity | 81.1% | **50.0%** |
| journeyman | parity | 83.2% | **50.0%** |
| Meirok | parity | 85.4% | **50.0%** |
| Meirok | 1.5× | 72.8% | **12.7%** |

Steal is currently an ~81–87% mechanic against everything up to 1.5× marks
(the old sqrt-curve×25 attacker bonus vs a skill-less defender). Uniform ×5
with any defender skill term is a **hard nerf at and above parity**: −31 to
−35 points at parity, −60 at 1.5×. Down-tier stealing barely moves. Whether
that is a fix or a break depends on intent, but it will be *felt* — flag for
the owner. (The spec does not name the defender's skill for steal; the model
assumed combat skill. With NO defender skill the parity numbers land ~62–74%
instead of 50%.)

### Grapple drift (√2 fix ÷1.414; reweight 2.2/2.0 → 5/5; floor forces 12.5% Hold in ALL variants)

Parity journeyman pair (controller = aggressor):

| variant | hold | P(move) | P(reversal) | P(escape) | E[steps/rd] |
|---|---|---|---|---|---|
| current (2.2/2.0, no √2) | 36.5% | 63.5% | 12.2% | 5.3% | **+0.196** |
| √2 fix only | 45.7% | 54.3% | 9.8% | 1.4% | +0.153 |
| reweight only (5/5) | 36.7% | 63.3% | 14.1% | 6.9% | **0.000** |
| both | 46.0% | 54.0% | 11.9% | 2.0% | **0.000** |

Adept controller vs trash: E[steps] 2.52 → 2.54 (both); P(move) ~87% in all
variants (floor-dominated hold at 12.6–13.5%).

- **The √2 fix alone** cuts P(any position change) by ~9pp (63→54%) and cuts
  the tail outcomes (escape / 3-step advance) by ~3.5× (6.9%→2.0%). That is
  the "grapples develop more slowly" change, and it is the fix of a live
  defect, not a retune.
- **The reweight alone deletes the aggressor's initiative edge entirely.**
  The 2.2-vs-2.0 coefficient split is the ONLY thing that makes a parity
  drift non-symmetric; at 5/5 E[steps] is exactly 0. The +0.196 steps/round
  a parity journeyman aggressor currently enjoys goes to zero. If the
  initiative edge is wanted, it must be re-expressed (e.g. a stat-side or
  multiplier edge), because uniform ×5 cannot carry it. **This is a design
  decision U6b's spec does not currently acknowledge.**
- Compounded, a parity pair after U6b: 46% hold, symmetric ±, 2% escapes —
  materially "stickier" grapples than today at parity, virtually unchanged
  dominance down-tier.

---

## Item 9 — Defence cost load

Defence priced per code: 1.0 × encumbrance(load 0.5)=1.333 × inverse-skill ×
dodge-modifier 1.25 (worst case). In-combat regen: `max(1, 2%·max)÷4` every
3rd round. 30-round fight, defender also swings once/round (partial-pay).
Scenario A: melee attacker adds 1 defended special per 3 rounds. Scenario B:
full-ranged attacker on the script's live schedule (**8 defended shots per 30
rounds**, capacity 1 / cooldown 4).

| tier | pool | def cost/use | added drain A (per rd) | added drain B | stamina@30 BEF-A → AFT-A | BEF-B → AFT-B | rounds-to-exhaustion |
|---|---|---|---|---|---|---|---|
| novice | 405 | 1.80 | 0.60 | 0.48 | 328 → 310 | 382 → 368 | never (all cases) |
| journeyman | 485 | 1.67 | 0.56 | 0.44 | 415 → 399 | 465 → 452 | never |
| adept | 545 | 1.47 | 0.49 | 0.39 | 486 → 472 | 530 → 519 | never |
| Meirok | 430 | 1.24 | 0.41 | 0.33 | 387 → 375 | 425 → 415 | never |

**Finding — this argues AGAINST the spec's implied fear:** at shipped costs
(DefenceBaseStaminaCost 1.0) and typical load, §4.3's new charges add only
**0.3–0.6 stamina/round** — roughly the size of in-combat regen — and NO tier
gets within 60% of exhaustion in 30 rounds, let alone into U8 skill-strip
territory (strip requires the pool to be unable to cover a ~2-point quote,
i.e. near-zero). Defending specials and shots is affordable. The number that
matters is not the drain but the parity: dodging a bash finally costs what
dodging a sword does.

Stress case (66% max-reserved pool + crushed load 1.0, where the cost product
clamps at 6.0/use): exhaustion at rounds 12–18, defence first short-pays at
rounds 12–18. So the loop IS reachable — but only via reservation+overload,
which are player choices, not the attacker's rotation.

**One iteration of the feedback loop** (defence quote unaffordable → U8 strips
the skill term), vs a parity melee attacker:

| tier | P(defWin) full → stripped | P(defCrit) full → stripped | extra damage fraction/swing |
|---|---|---|---|
| novice | 68.3% → 38.1% | 4.4% → 0.4% | +0.260 |
| journeyman | 68.3% → 14.1% | 4.4% → 0.01% | **+0.423** |
| adept | 68.3% → 13.0% | 4.4% → 0.00% | +0.430 |
| Meirok | 66.1% → 12.5% | 4.1% → 0.00% | +0.412 |

Once stripped, a mid-tier defender falls from a 68% defence to the 12.5%
floor and takes ~42% more of every incoming swing — the loop's one iteration
is brutal, which is exactly why it matters that the added §4.3 load cannot
trigger it on its own at shipped costs. The gate to watch is any future
retune of `DefenceBaseStaminaCost` upward: at base 4+ (special-move-priced
defences) the A-scenario drain quadruples and the 30-round margins start to
matter for the novice tier.

---

## Five most decision-relevant numbers

1. **85.5% defensive-crit rate (floor-capped) for any journeyman+ defender vs
   trash attacks** → ~32–42 free counter damage/round; U6b extends this
   already-live melee degeneracy to every channel.
2. **Journeyman firebomb vs trash: expected damage per throw 12 → 165**
   (÷ mitigation-bypassing 73% crit rate) once throw gets a real defence set
   plus the crit tier — the family's largest single swing.
3. **Submission crit (stun) vs trash: ~2% → 18–62%** when crit moves from
   self-z to margin.
4. **Flee ×25→×5 is a ~no-op**: every headline cell (novice-flees-veteran,
   veteran-flees-trash) is floor/ceiling-pinned before AND after; only
   novice-vs-trash moves (−8.9pp).
5. **§4.3's cost load adds 0.3–0.6 sp/round at load 0.5 — nobody reaches
   exhaustion or skill-strip in 30 rounds**; but a stripped mid-tier defender
   drops 68%→13% defence and takes +42% damage/swing, so the loop is safe
   only while defence stays at base cost 1.0.

Plus two structural flags: the **reweight deletes drift's aggressor edge
(E[steps] +0.196 → 0.000 at parity)**, and **12.5% of all drift rounds are
floor-forced Holds** (the sentinel margin lands in the Hold band) — neither is
in the spec.
