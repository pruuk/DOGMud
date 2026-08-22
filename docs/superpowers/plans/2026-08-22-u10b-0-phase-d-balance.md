# U10b-0 Phase D: Balance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Revision 2**, after three blind adversarial reviews of revision 1. Revision 1's
Task D3 rested on a false premise and its model was wrong in four measured ways.
Every correction is marked **[R2]** and the reason is stated, so nothing is
quietly re-derived.

**Goal:** Balance progression on **play time**, so an hour of concerted grinding
yields comparable progress whatever you grind.

**Architecture:** The re-key changed what the curve is keyed to; this phase
changes how fast it moves. Decay constants keep the curve's **shape**; the
per-stat and per-skill multipliers set its **rate**. Rates derive from measured
uses-per-hour, because a use is not comparable across activities.

**Tech Stack:** `_datafiles/config.yaml`, `internal/skills/skills.go` (the Go
shadow map), and the models in `tools/balance/u10b_*.py`.

**Spec:** `.../2026-08-21-u10b-0-progression-rank-from-training-design.md`
sections 13.3, 13.4, 14.2. **Phase index:** `2026-08-21-u10b-0-README.md`.

**Branch:** `feature/u10b-0-phase-d-balance`, already cut from master.

---

## Owner rulings

1. **~10% engagement** over a real hour for combat and crafting.
2. **3 points/hour for combat tracks, 4 for everything else.**
3. **The knee is fine and progressing past the soft cap is wanted**, with its
   implications for titles and rank bands accepted.

---

## [R2] What revision 1 got wrong

Recorded so it is not repeated, and because two of the four also mislead anyone
reading the surrounding code.

### 1. Both crit knobs are LIVE. Revision 1's Task D3 was a no-op.

`git show HEAD:_datafiles/config.yaml` lines 1003 and 1009 ship
`CritProgressionBonus: 2.0` and `ObservedCritProgressionBonus: 0.5`, added by
`81061c6b4` on 2026-08-19. Revision 1 read the **working-tree** copy, which
carries the git skip-worktree bit and predates U6b.

So the crit-toughen faucet (physical -> vitality, magical -> willpower,
conviction -> charisma) has been live for weeks, and the doer-side crit/fumble
bonus tier is live at 2.0. **Neither needs enabling; both must be *modelled*.**

Two stale comments propagated this and are corrected in Task D0:
- `internal/configs/config.balance.progression.go:33` — "*is why
  ObservedCritProgressionBonus sits at 0 in production*"
- `internal/characters/progression_faucet_test.go:48` — "*an absent key (as in
  the shipped config.yaml today)*"

### 2. `OnCritReceived` is dead code, not the crit path.

It has **zero production callers**. U9 replaced it with the event seam:
`NewRound_DoCombat_unified.go:704-721` builds `progression.Outcome{ToughenStat:
...}`, `progression.BonusEvents` emits it, `applyBonusProgression`
(`progression.go:748-785`) rolls it. `defence_multiplier.go:489-543` does the
same for non-melee channels. Model against the seam, not the dead function.

### 3. The model missed the defence faucet and the offhand fist.

`AwardDefenceProgression` calls `OnSkillUse(skill)` — which itself fires
`OnStatUse(primary)` — and **then** `OnStatUse(stat)`. Since weapon-combat and
unarmed-combat both map to dexterity: dodge gives dexterity **+2**, parry
dexterity **+2** and strength +1, block dexterity +1 and strength +1. And
`collectAttackWeapons` supplies a **fist for an empty offhand**, so a 1H
character produces two `WeaponHits` entries. Strength and dexterity uses/hour
roughly **double**; `unarmed-combat` is trained by every armed character.

### 4. Crafting and casting are not multiplier 1.0.

Both use `OnSkillUseScaled` with a difficulty bonus applied **to the skill roll
only** (the stat roll inside always passes 1.0):
`craftBonus = 1 + skill_minimum x 0.02` (median **1.40** over 126 recipes),
`spellBonus = 1 + difficulty x 0.01` (median **1.25**; **1.35** over the 14
manifestation spells). **Salvage does not get it** — it calls bare `OnSkillUse`
— so salvage and the crafts cannot share a multiplier.

### 5. Smaller factual corrections

- `search` has a **2-round cooldown** (`actions/search.go:53`), so its concerted
  ceiling is **450/hour**, not the 180 revision 1 assumed.
- `skullduggery` has **no search faucet** — steal, plant, shadow, defuse,
  surprise-attack, throw.
- `rhetoric` has **no conversation faucet** — taunt and the defy defence only.
- Manifestation is **cast-time bound** at 10% engagement (90/4 = 22 casts/hr),
  not CP-bound; the CP sustain is higher than the engaged burst cap.
- Anchor drift is **1.822x per 10 ranks** (`exp(3 x 10/50)`), not 1.5x. A fresh
  character progresses at **4.48x** the target.
- Eleven Go-map / config entries disagree, not one, and **three skills have no
  config entry at all**.

### 6. [R2] "Past the soft cap buys accuracy and zero damage" was WRONG

`SkillMultiplier` clamps at the soft cap, so **base-hit** damage plateaus. But
two damage paths do not clamp:

- **`CritDamageMultiplier` is linear and uncapped**: `2.0 + 0.05 x rank`. Rank 50
  gives 4.5x, rank 69 gives 5.45x, rank 100 would give 7.0x.
- **`CritBarFor` uses raw unclamped ranks**: `bar = 2.0 - 0.05 x (atkRank -
  defRank)`, floored at `CritBarFloor` 1.5, so out-skilling keeps raising crit
  frequency until a 10-rank advantage binds.

`crit_bar.go:31` records the double count as deliberate. So past the knee, skill
buys accuracy, crit **frequency** and crit **magnitude** — growth shifts from
every-swing to crit-weighted, making high-skill characters spikier rather than
merely more accurate. **`CritDamagePerSkill` is the knob to watch** if past-cap
characters feel explosive; it has no ceiling.

---

## Standing rules

1. **[R2] Some tests DO load `config.yaml`.** `withRepoRoot`
   (`internal/characters/poolmax_test.go:24-44`) chdirs to the repo root and
   calls `configs.ReloadConfig()`. Thirteen test files use it. Revision 1's
   blanket "a test binary never loads config.yaml" is false.
2. **[R2] The disk copy and the committed blob must move together.** Because of
   rule 1 plus the skip-worktree bit, a change made only to the git blob is
   **invisible locally but live in CI**, and a commit built from disk **deletes**
   whatever the disk is missing. Task D0 exists for this.
3. **Decay sets shape, multipliers set rate.** `ProgressionDecayBelowCap` (3.0)
   and `AboveCap` (2.0) reproduce both documented anchors. **Do not touch them.**
4. **Go defaults move with shipped values** — and for skills the Go map in
   `internal/skills/skills.go` is a real fallback, authoritative wherever
   `config.yaml` has no key.
5. **The per-stat multiplier is shared with the regen faucet.**
   `CheckRegenProgression` applies `GetStatProgressionMultiplier` and the damper.

---

## Task D0: repair the config desync FIRST

**This is the live hazard that produced revision 1's central error, and it is
still armed.** The working-tree `_datafiles/config.yaml` is missing at least
twelve knobs the committed blob has: `ConcentrationFloor`,
`ConcentrationDamageThresholdPct`, `CritBarSkillSlope`/`Floor`/`Ceiling`,
`CounterDamagePercent`, three `Grapple*Mod` knobs, and the three U10
`*KnockdownFactor` knobs — while still carrying the retired `*KnockdownChance`
ones. A commit built from disk silently reverts U6b and U10, because every one
of those defaults on absence rather than erroring.

- [ ] **Step 1:** Rebuild the disk copy from `git show HEAD:_datafiles/config.yaml`,
      re-applying only the three intentional local overrides: `HttpPort: 8090`,
      `LogLevel`, `LogToFile: false`. Confirm with a `--strip-trailing-cr` diff
      that nothing else differs.
- [ ] **Step 2:** Fix the two stale comments named in [R2] item 1.
- [ ] **Step 3:** Fix the false paragraph in `tools/balance/u10b_time_solve.py`'s
      header if any remains, and the "dormant" claims in
      `tools/balance/u10b_vitality_model.py`.
- [ ] **Step 4:** Commit. This is a correctness fix that stands alone and should
      land before any tuning.

---

## Task D1: measure uses/hour before moving any knob

The whole table is uses/hour multiplied by arithmetic. Revision 1 got two of
those inputs wrong by 2.5x and the correction moved `perception` from 0.37 to
0.15. **Measure before shipping.**

| input | source | trust |
|---|---|---|
| 10% engagement | owner | settled |
| 4-second round, 5-round median craft | config / data | settled |
| search 450/hr ceiling | 2-round cooldown | **is the ceiling realistic play?** |
| combat per-round accounting | code-derived | **verify empirically** |
| craft / spell difficulty bonus medians | data | settled |

- [ ] **Step 1:** Read realised uses out of the `mudlog.Debug("Progression", ...)`
      lines rather than counting banners; they carry `chance`, `roll`, `threshold`.
- [ ] **Step 2:** Run `mid` on each track for a measured wall-clock period.
      `mid` has every stat at Training 0 and no companions.
      **[R2] `mid` measures at rank 0-18, and the solve is anchored at rank 25.**
      Uses/hour is rank-independent so measuring *uses* is valid — but do **not**
      use realised *gains* as a pass/fail gate, because at rank 0 they run 4.48x
      the target by design.
- [ ] **Step 3:** Feed measured uses/hour into `tools/balance/u10b_time_solve.py`
      and regenerate. Any input off by >30% moves its multiplier materially.

---

## Task D2: the multipliers

**Files:** `_datafiles/config.yaml` **and** `internal/skills/skills.go`.

`GetStatProgressionMultiplier` returns 1.0 when config has no entry, so stats
live only in config. `GetSkillProgressionMultiplier` returns `(0, false)` meaning
"use the hardcoded default", so **skills have a Go fallback that must move too**.
**[R2] `ranged-combat`, `skullduggery` and `search` have no config entry at all**
— D2 must *add* those keys. Eleven entries currently disagree between the two.

- [ ] **Step 1:** Regenerate rather than transcribe:
      `python tools/balance/u10b_time_solve.py`

Solved 2026-08-22 (revision 2), at rank 25:

| track | uses/hr | bonus | shipped | solved |
|---|---|---|---|---|
| `weapon-combat` | 105 | 1.00 | 0.23 | **1.07** |
| `unarmed-combat` | 75 | 1.00 | 0.23 | **1.49** |
| `ranged-combat` * | 45 | 1.00 | 0.50 | **2.49** |
| `spellcasting` | 45 | 1.25 | 0.63 | **1.99** |
| `rhetoric` | 90 | 1.00 | 0.58 | **1.24** |
| `manifestation` | 38 | 1.35 | 0.38 | **2.95** |
| `skullduggery` * | 100 | 1.00 | 2.00 | **1.49** |
| `search` * | 450 | 1.00 | 2.00 | **0.33** |
| `bartering` | 100 | 1.00 | 2.00 | **1.49** |
| `salvage` | 18 | 1.00 | 2.00 | **8.30** |
| the six crafts | 18 | 1.40 | 3.50 | **5.93** |
| `strength` | 150 | 1.00 | 0.20 | **0.33** |
| `dexterity` | 330 | 1.00 | 0.15 | **0.15** (unchanged) |
| `perception` | 450 | 1.00 | 1.00 | **0.15** |
| `willpower` | 45 | 1.00 | 1.00 | **1.11** |
| `charisma` | 100 | 1.00 | 0.22 | **0.66** |

`*` = no config key today; add it.

- [ ] **Step 2:** Rewrite the comment above each map. The current text explains
      the values as compensation for firing frequency; that reasoning is now
      inverted — combat's low per-hour yield is exactly why its multiplier rises.
      State the engagement assumption, the 3/4 targets, and point at the model.

---

## Task D3: [R2] the structural problem the solve cannot fix

**One multiplier per stat, many feeder tracks with 25x different use rates.**
Fitting `perception` to `search` at 450/hr gives 4.07 pts/hr from searching and
**0.16 pts/hr** from alchemy, cooking, enchanting or salvage — the same stat,
25x apart, and the gap lands on crafters, the group the 4/hr target was meant to
favour.

| training perception by | uses/hr | pts/hr at m=0.15 |
|---|---|---|
| search | 450 | 4.07 |
| consider / look | 60 | 0.54 |
| ranged-combat | 45 | 0.41 |
| alchemy / cooking / enchanting / salvage | 18 | 0.16 |

Dexterity has the same shape, milder (combat 330 vs crafts 18).

**A scalar cannot serve tracks 25x apart. Pick a lever, do not pretend the
multiplier solves it:**

- [ ] **Option A (recommended): fix `search`, not `perception`.** Its 2-round
      cooldown is the outlier. Lengthen it, or damp perception specifically on
      the search path, then fit `perception` to a mid-rate faucet (~60-100/hr),
      which puts it near **0.6-1.0** and leaves crafters whole.
- [ ] **Option B:** accept the spread and document that stats are trained
      primarily by their dominant activity.
- [ ] **Option C:** per-faucet stat multipliers. Most correct, most work, new
      config surface.

**Needs an owner decision before D2 ships**, because Option A changes
`perception`'s solved value by ~5x.

---

## Task D4: vitality, with both faucets live

Vitality has two faucets and **both are already live** — revision 1 wrongly
treated the crit one as dormant.

- **Regen tick.** [R2] `OnRegenTick` runs for **both** Health and Stamina, and
  vitality is in both lists, so it gets **two rolls per tick**, not one.
  Revision 1's "1 gain per 72 rounds" is ~2x pessimistic on its regen component.
- **Taking a physical crit**, via the bonus-event seam at
  `ObservedCritProgressionBonus: 0.5`.

Phase C already cut vitality without touching its multiplier: the regen damper
was pinned at 1.0 because nothing moved vitality's rank, and is now **0.43x at
Training 14**, **0.09x at Training 40**.

- [ ] **Step 1:** Correct `u10b_vitality_model.py` (two regen rolls; crit path
      live, not dormant) and re-run.
- [ ] **Step 2:** Solve `vitality` against both live faucets at the 4/hr target.
      **Do not apply spec 13.3's "start at ~1.0" literally** — that instruction
      predates the damper biting and predates knowing the crit path was live.
- [ ] **Step 3:** Re-check `willpower` and `charisma`, which also receive
      crit-toughen and regen contributions the D2 solve does not model.
      `charisma` is the most affected: regen alone adds ~1.4 pts/hr at m=0.66.

---

## Task D5: [R2] consequences the solve does not contain

- [ ] **Mobs and companions share these multipliers.** `MobProgressionRate` 0.5
      multiplies on top. A ~5x combat raise takes a companion's `weapon-combat`
      1 -> 25 from ~194 play-hours to ~36. **This falsifies the config comment
      Phase C shipped**, which justifies `MobSkillTrainingCap: 25` on the grounds
      that ~17,500 rounds makes it unreachable. Either revisit the cap or rewrite
      that comment; do not leave it asserting something now false.
- [ ] **Existing characters.** Of ~102 saves, six are past a soft cap. Meirok
      (`users/3.yaml`) has `perception` Training **51** — the only stat past 50 on
      any character — and perception is primary for five of his six top skills.
      Under D2 as written he takes a 6.7x cut there. **A migration is not
      possible** (multipliers are not stored per character), so this is a
      deliberate, uncompensated redistribution and the patch notes must say so
      plainly.
- [ ] **The low end.** At 5.93 a fresh craft skill's chance is
      `0.12 x 5.93 x 1.40 = 0.996`, clamped to 1.0 — the first craft always
      levels, and the clamp wastes part of the multiplier exactly where new
      players are.
- [ ] **Titles.** Meirok's skill total is 848 against a `GetSkillTier`
      denominator of 850, and the two skills gating "grandmaster" (`manifestation`
      48, `ranged-combat` 1) receive two of the three largest raises. Confirm
      with the owner against these numbers rather than the general ruling.

---

## Task D6: gates

- [ ] `gofmt`, `go build ./...`, full `go test ./... -count=1`.
- [ ] **[R2] `TestGetProgressionMultiplier`
      (`internal/characters/progression_test.go:258-277`) hard-asserts the Go map
      values** and will fail. Revision 1 did not name it.
- [ ] **[R2] `TestStatChance_ReproducesTheDocumentedAnchors`** — revision 1's
      remedy ("use a stat whose multiplier is still 1.0") is **impossible**; after
      D2 no stat has 1.0. Either re-anchor the expected values or use a name
      absent from the map, and say which.
- [ ] **[R2] `CLAUDE.md` states the same anchors as fact** ("roughly 27% per
      use... roughly 1.3%"). The doc moves with the test.
- [ ] Boot test in an isolated detached worktree (`mkdir -p _datafiles/logs`;
      exit **124** is success; never grep the bare word `panic`).
- [ ] Patch notes: player-facing, no raw numbers, no em dashes, 80 columns. Must
      be honest that searching improves much more slowly and that a
      perception-built character is materially affected.
- [ ] Adversarial playtest with `mid`. Measure **uses**, not gains (see D1).
- [ ] PR to `pruuk/DOGMud`, hand over, do not merge.

---

## Self-Review

**Spec coverage:** 13.3 -> D4 · 13.4 -> standing rule 3 and D2 · 14.2 -> D2,
recast as time rather than uses.

**Known-weak points:**
- D3 is an unresolved design decision, not a task with a known answer. D2 cannot
  ship correctly until it is settled.
- The bonus tier (`CritProgressionBonus` 2.0 doer, 0.5 observer) adds skill and
  stat rolls on crit and fumble rounds that the D2 solve does not count. Against
  an outclassed target, where U6's margin-driven crit rate is high, this is not
  a rounding error.
- Rank 25 is the anchor; the realised rate is 4.48x target at rank 0 and 0.22x
  at rank 50. Cross-track *parity* holds at every rank, but the absolute target
  does not.
