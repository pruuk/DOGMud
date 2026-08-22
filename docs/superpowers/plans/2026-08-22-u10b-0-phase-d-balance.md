# U10b-0 Phase D: Balance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Balance progression on **play time**, so that an hour of concerted
grinding yields comparable progress whatever you choose to grind.

**Architecture:** The re-key changed what the curve is keyed to; this phase
changes how fast it moves. The decay constants keep the curve's **shape** and
the per-stat / per-skill multipliers set its **rate**. Rates are derived from
measured uses-per-hour rather than from use counts, because a use is not a
comparable unit across activities: an hour of combat is hundreds of swings, an
hour of crafting is tens of crafts.

**Tech Stack:** `_datafiles/config.yaml` (multipliers, `ObservedCritProgressionBonus`),
plus the Go defaults that shadow them. Models in `tools/balance/u10b_*.py`.

**Spec:** `docs/superpowers/specs/2026-08-21-u10b-0-progression-rank-from-training-design.md`
sections 13.3, 13.4, 14.2. **Phase index:** `2026-08-21-u10b-0-README.md`.

**Branch:** `feature/u10b-0-phase-d-balance`, cut from `master` once PR #57 merges.

---

## Owner rulings that fix the target

**2026-08-22.**

1. **~10% engagement** over a real hour for combat and crafting. The rest is
   travelling to find mobs, gathering materials, walking to stations, recovering.
2. **3 points/hour for combat tracks, 4 for everything else.** Crafting spends
   gold and materials, so it may run a little faster; combat carries risk and
   time-to-find, so it is measured a little cheaper.
3. **The knee is fine, and progressing past the soft cap is wanted.** It has
   implications for titles, rank-band names and balance, and those are accepted.

---

## What is already settled, so nobody re-derives it

**All figures below were computed against the shipped config; the models are
committed under `tools/balance/` so they can be re-run rather than trusted.**

`RoundSeconds` is 4, so an hour is **900 rounds**. Recipe `time_rounds` is mode
4 / median 5. At 10% engagement that is **90 combat rounds** and **18 crafts**
per hour.

**Per-event accounting** (verified in `NewRound_DoCombat_unified.go`,
`defence_multiplier.go`, `progression.go`):

- attacking, per exchange: `strength` +1 and `dexterity` +1 (`emitAttackerStatGain`)
- per clean weapon hit: the combat skill +1, and its primary stat +1
- defending, per round: the defence skill +1 and its stat +1 (parry also `strength`)
- **any skill use also fires `OnStatUse(primary stat)`**
- crafting, per craft: the craft skill +1 and its primary stat +1

**Two traps that already produced wrong answers in this arc, both from grepping
for a string literal where production passes a variable:**

- `OnStatUse("dexterity")` appears **nowhere**. Combat passes the stat as a
  variable, and dexterity is trained on **every attack**.
- `OnSkillUse("manifestation")` appears only in `assess.go`. The cast path picks
  the skill from the spell's **school**, so all 14 `SchoolManifestation` spells
  (`raise-*`, `charm`, `conjure-*`, `summon-*`) train it too.

**Casting is CP-bound over an hour, not cooldown-bound.** `waitrounds` caps the
burst rate; conviction regen caps the sustained rate. A mid character
regenerates ~2,700 CP/hr (2% of a 450 pool every 3 rounds) plus the 450 they
start with, which funds ~79 casts at a typical 40 CP spell against a burst cap
of 900.

---

## Standing rules

1. **Go defaults move with shipped values.** A test binary never loads
   `config.yaml`.
2. **`config.yaml` carries the git skip-worktree bit.** Build the commit from
   `git show HEAD:_datafiles/config.yaml`, never from disk, and never commit the
   local `HttpPort` / `LogLevel` / `LogToFile` overrides. See
   `reference_config_yaml_skip_worktree`.
3. **Decay sets shape, multipliers set rate.** `ProgressionDecayBelowCap` (3.0)
   and `AboveCap` (2.0) already reproduce both documented anchors — a fresh stat
   at ~27% and a stat with 50 trained points at ~1.34%. **Do not touch them.**
4. **The per-stat multiplier is shared with the regen faucet.**
   `CheckRegenProgression` applies `GetStatProgressionMultiplier` and the damper,
   so changing a stat's multiplier for combat pace also rescales what the regen
   tick grants it. This is not separable; it is why Task D3 exists.

---

## Task D1: measure the assumptions before moving any knob

The rates in D2 rest on five assumptions. Three are the owner's or come from
data; **two are estimates I invented and they drive the largest changes in the
table.** Measure them before shipping a 2.5x nerf on an estimate.

| assumption | source | trust |
|---|---|---|
| 10% engagement | owner ruling | settled |
| 5-round median craft | recipe data | settled |
| 4-second round | `Timing.RoundSeconds` | settled |
| **180 utility uses/hour** | my estimate | **measure** |
| **CP haircuts on casting** | my estimate | **measure** |

**Files:** none — this is a measurement task.

- [ ] **Step 1: Instrument, do not eyeball**

Progression banners are too rare to count by hand at these rates. Add a
temporary debug counter, or read the existing `mudlog.Debug("Progression", ...)`
lines out of the server log, which already carry `chance`, `roll` and
`threshold` per event. The log is the cheaper route and needs no code change.

- [ ] **Step 2: Run `mid` on each track for a measured wall-clock period**

`mid` is the right instrument and `veteran` is not: `mid` has every stat at
Training 0, no companions, and ordinary kit, where `veteran` one-shots bandits
and yielded one event in 18 kills.

```text
/playtest local --checkout <abs> feature-tester <goals-file>
```

Drive one track per run: combat, crafting, utility search, casting. Record
**uses per wall-clock minute**, not gains — gains are the thing being solved
for, uses are the input.

- [ ] **Step 3: Feed the measurements back into the model**

Edit the `uses/hour` figures at the top of
`tools/balance/u10b_time_solve.py` and re-run. If a measured rate differs from
the estimate by more than about 30%, the solved multiplier moves materially and
**the table in D2 must be regenerated rather than used as written.**

---

## Task D2: the per-stat and per-skill multipliers

**Files:** Modify `_datafiles/config.yaml` **and `internal/skills/skills.go`.**

⚠️ **Skills have a Go-side shadow default and stats do not.** Verified:

- `GetStatProgressionMultiplier` returns **1.0** when the config has no entry.
  Config is the only source; there is nothing to keep in sync.
- `GetSkillProgressionMultiplier` returns `(0, false)` meaning *"use the
  hardcoded default"*, and `skills.GetProgressionMultiplier` then falls back to
  the `SkillProgressionMultipliers` map in `internal/skills/skills.go`.

So a skill multiplier changed only in `config.yaml` leaves the old value live for
any config that omits the key — including **every test binary**, which never
loads `config.yaml`. **Update both**, exactly as the standing rule about Go
defaults requires.

Note the two maps also disagree today: the Go map has `WeaponCombat: 0.3` while
`config.yaml` ships `weapon-combat: 0.23`. Bring them into line rather than
preserving the split.

- [ ] **Step 1: Apply the solved table**

Regenerate rather than transcribe:

```bash
python tools/balance/u10b_time_solve.py
```

Target values as solved on 2026-08-22, at rank 25:

| track | shipped | solved |
|---|---|---|
| `weapon-combat` | 0.23 | **1.24** |
| `unarmed-combat` | 0.23 | **1.24** |
| `ranged-combat` | 0.50 | **2.49** |
| `spellcasting` | 0.63 | **1.87** |
| `rhetoric` | 0.58 | **1.12** |
| `manifestation` | 0.38 | **2.99** |
| `skullduggery` | 2.00 | **0.83** |
| `search` | 2.00 | **0.83** |
| `bartering` | 2.00 | **1.49** |
| `salvage` | 2.00 | **8.30** |
| `blacksmithing`, `alchemy`, `tailoring`, `cooking`, `jewelcrafting`, `enchanting` | 3.50 | **8.30** |
| `strength` | 0.20 | **0.55** |
| `dexterity` | 0.15 | **0.28** |
| `perception` | 1.00 | **0.37** |
| `willpower` | 1.00 | **0.83** |
| `charisma` | 0.22 | **0.66** |

- [ ] **Step 2: Rewrite the comment block above each map**

The current comments explain the multipliers as compensation for firing
frequency ("Combat skills fire many times per round, so they get a low
multiplier"). That reasoning is now **inverted**: the numbers are solved so that
an hour of grinding any track yields comparable progress, and combat's low
per-use rate is exactly why its multiplier is now HIGH rather than low. Say
that, state the engagement assumption and the 3/hr and 4/hr targets, and point
at `tools/balance/u10b_time_solve.py` so the next person re-solves rather than
guesses.

- [ ] **Step 3: Note the two entries that change character most**

`salvage` moves 2.00 -> 8.30 because it is craft-paced, not utility-paced, and
was grouped wrongly. `search`, `skullduggery` and `perception` drop to ~0.4x:
searching while travelling was paying better per hour than fighting. Both belong
in the patch notes.

---

## Task D3: vitality, the damper, and the crit faucet — solved together

**These are three changes to one number and they pull in different directions.
Do not make them independently.**

Vitality has **two** faucets and neither is `OnStatUse`:

- the **regen tick**, live today
- **taking a physical crit**, via `OnCritReceived` -> `ToughenStatFor`, which is
  **dormant** because `ObservedCritProgressionBonus` is absent from
  `config.yaml` and its validator uses the `< 0` idiom, so it stays at 0

Phase C already cut vitality hard without touching its multiplier: the regen
damper was previously pinned at exactly 1.0 because nothing ever moved
vitality's rank. It is now **0.43x at Training 14** and **0.09x at Training 40**.

So enabling the crit knob *adds* a faucet at the same moment the damper
*removes* pace. Modelled at the shipped 4.5, with the knob set to 0.5, a
character fighting at half health gains vitality about once per 72 rounds at
Training 14, slowing to once per 139 by Training 25.

- [ ] **Step 1: Set `ObservedCritProgressionBonus: 0.5` in `config.yaml`**

Absent means 0 means the path is dead for **vitality, willpower and charisma**
alike. Setting it is what makes "taking a beating toughens you" real.

- [ ] **Step 2: Do NOT cut `vitality` to ~1.0**

Spec 13.3 says to start at ~1.0. That instruction predates the damper biting and
predates the crit path being enabled; taken literally it would be a third cut on
top of two. Re-run `tools/balance/u10b_vitality_model.py` and set the multiplier
against the measured result, holding the same 4/hr target the other non-combat
tracks use.

- [ ] **Step 3: Check willpower and charisma too**

Both gain a crit-toughen faucet the moment the knob is set, on top of the D2
solve which assumed only their existing faucets. Re-run the model with the crit
path live and adjust if either overshoots its target.

---

## Task D4: gates

- [ ] `gofmt -l internal/ modules/` prints nothing; `go build ./...`; full
      `go test ./... -count=1`. Note `internal/usercommands` and
      `internal/mobcommands` are independently flaky on master — rerun before
      assuming a failure is yours.
- [ ] **Confirm the documented anchors still hold.**
      `TestStatChance_ReproducesTheDocumentedAnchors` pins a fresh stat at ~27%
      and Training 50 at ~1.34%. Those are properties of the CURVE, so a
      multiplier change must not move them for `perception` (multiplier 1.0 ->
      0.37 **will** move them; update the test to a stat whose multiplier is
      still 1.0, or re-anchor the expected values, but do not delete it).
- [ ] Boot test in an isolated detached worktree (`mkdir -p _datafiles/logs`
      first; exit **124** is success; never grep the bare word `panic`).
- [ ] Patch notes: player-facing, no raw numbers, no em dashes, 80 columns. The
      honest statement is that how quickly things improve now depends on the time
      spent rather than on which activity happens to fire most often, that
      fighting and crafting improve faster than before, and that searching
      improves more slowly.
- [ ] **Adversarial playtest with `mid`, not `veteran`.** Re-measure one track
      and confirm the realised rate is near the target.
- [ ] Ship via PR to `pruuk/DOGMud`, hand over, do not merge.

---

## Self-Review

**Spec coverage:** 13.3 (vitality vs the right faucet) -> D3 · 13.4 (decay sets
shape, multipliers set rate) -> standing rule 3 and D2 · 14.2 (solve
multipliers for pace) -> D2, re-cast as time rather than uses.

**Not in this phase, deliberately:** the dashboard (E), and any change to which
code paths train what. The manifestation companion-tick idea the owner raised is
a **flavour** change, not a balance one — the multiplier sits with the crafts
without it — so it belongs in its own slice.

**Known-weak points, stated rather than hidden:**
- The whole table rests on uses-per-hour, and two of those figures are estimates.
  D1 exists because of that, and D2 is not safe to ship until D1 runs.
- A point is not worth the same in every track. Combat trains two stats plus a
  skill at once, so "3/hr each" is really ~9 points of total growth per hour
  against crafting's ~6. The owner accepted this deliberately: combat carries
  risk and consumable cost.
- Rank 25 is the reference point. The solve holds exactly there and drifts either
  side, because the curve's shape differs from the old model's. Anchoring
  elsewhere moves every number by roughly 1.5x per 10 ranks.
