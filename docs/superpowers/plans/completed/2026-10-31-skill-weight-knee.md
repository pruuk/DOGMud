# Skill Weight Knee and Crit Cap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** `SkillWeight` applies at 5.0 up to a knee rank (65) and at a second weight (1.0) above it, through one helper used by every site; crit damage stops growing at a rank cap (65) and `CritDamagePerSkill` ships 0.025.

**Spec:** `docs/superpowers/specs/completed/2026-10-31-skill-weight-knee-design.md` (owner rulings 2026-10-31). Facts table there; the 24 sites are listed in it.

**Branch / worktree:** `feat/skill-weight-knee`, worktree `C:/tmp/dogmud-skill-knee`, draft PR #732.

## House rules for every executor

- Edit repo files only with the Edit and Write tools; never write a repo file from the shell. Scratch goes in your session scratchpad.
- `git add` named paths only; never `-A`, `.`, `commit -a` or `--no-verify`.
- No em or en dashes in prose, comments, player text or commit messages.
- Test binaries load Go defaults (knee and cap off). Pin knobs with `configs.SetConfigForTest` in any test whose subject they move.
- Every new test is seen failing first, for the stated reason.
- Every task's gate includes `go test . ./internal/guards/ -count=1`.

---

## Task 1: Knobs and the `WeightedSkill` helper

**Files:** `internal/configs/config.balance.go` (fields after `CritDamagePerSkill`, line 338), `internal/configs/config.balance.misc.go` (validator beside the `SkillWeight` default at 362), `internal/configs/config.balance.combat.go` (crit validator, 418-423), new `internal/configs/skill_weight.go`, new `internal/configs/skill_weight_test.go`.

- [ ] **Step 1: Fields**, aligned with their neighbours:

```go
	SkillWeightKneeRank             ConfigFloat `yaml:"SkillWeightKneeRank"`             // Rank above which skill counts at SkillWeightAboveKnee instead of SkillWeight; 0 = no knee (default 0)
	SkillWeightAboveKnee            ConfigFloat `yaml:"SkillWeightAboveKnee"`            // Weight per rank above SkillWeightKneeRank; below 0 resets to SkillWeight (default: SkillWeight)
	CritDamageRankCap               ConfigFloat `yaml:"CritDamageRankCap"`               // Crit damage stops growing past this rank; 0 = no cap (default 0)
```

- [ ] **Step 2: Validators.** In `config.balance.misc.go`, after the `SkillWeight <= 0` reset (and mirror it after the duplicate in `config.balance.progression.go:63-64` only if that validator is the one that runs for this field; read both and keep one source of truth, noting which in the commit):

```go
	if b.SkillWeightKneeRank < 0 {
		b.SkillWeightKneeRank = 0
	}
	if b.SkillWeightAboveKnee < 0 {
		b.SkillWeightAboveKnee = b.SkillWeight
	}
```

`SkillWeightAboveKnee` absent unmarshals to 0, which is a legal value (flat above the knee) but only matters when the knee is on; with the knee at 0 it is never read. In `config.balance.combat.go` after the crit resets: `if b.CritDamageRankCap < 0 { b.CritDamageRankCap = 0 }`.

- [ ] **Step 3: Failing tests** in `internal/configs/skill_weight_test.go`, table-driven over `WeightedSkill`, each case pinning the three knobs with `SetConfigForTest`:
  - knee 0, SW 5: rank 0 -> 0, 50 -> 250, 89 -> 445, 101 -> 505 (today's line).
  - knee 65, SW 5, above 1.0: 50 -> 250, 65 -> 325, 66 -> 326, 89 -> 349, 101 -> 361.
  - knee 65, SW 5, above 0: 89 -> 325.
  - fractional rank 70.5 with knee 65 / above 1.0 -> 330.5 (some sites pass a float).
  - negative rank -> 0 (never a negative contribution).

  Run `go test ./internal/configs/ -run WeightedSkill -count=1`: build failure (undefined). Red.

- [ ] **Step 4: The helper**, `internal/configs/skill_weight.go`:

```go
package configs

// WeightedSkill is a skill rank's contribution to any additive score
// (attack, defence, swing count, stealth, search, crafting difficulty, the
// progression roll): rank x SkillWeight up to SkillWeightKneeRank, then
// SkillWeightAboveKnee per rank above it (owner 2026-10-31, the endgame
// power leak). A knee of 0 is today's straight line. Every reader of
// SkillWeight goes through here; internal/guards keeps it that way.
func WeightedSkill(rank float64) float64 {
	if rank <= 0 {
		return 0
	}
	b := GetBalanceConfig()
	sw := float64(b.SkillWeight)
	knee := float64(b.SkillWeightKneeRank)
	if knee <= 0 || rank <= knee {
		return rank * sw
	}
	return knee*sw + (rank-knee)*float64(b.SkillWeightAboveKnee)
}
```

  Check `GetBalanceConfig()` is callable from inside `configs` as written (read its definition); adapt if it returns a value or needs another accessor. Run the test: green. Sabotage: make the above-knee branch return `rank * sw`, watch the knee cases fail, restore.

- [ ] **Step 5: Commit** `feat(configs): SkillWeight knee knobs and WeightedSkill helper`.

## Task 2: Crit damage rank cap

**Files:** `internal/combat/crit_damage.go:51-63`, its test file (find the existing `CritDamageMultiplier` test with `grep -rn "CritDamageMultiplier" internal/combat/*_test.go`).

- [ ] **Step 1: Failing test:** cap 0 / per 0.05: rank 89 -> 6.45 (today). Cap 65 / per 0.025: rank 10 -> 2.25, 65 -> 3.625, 89 -> 3.625, 200 -> 3.625; rank 0 -> 2.0. Red first (cap not applied).
- [ ] **Step 2: Implement:** after the `skillRank <= 0` return:

```go
	rank := float64(skillRank)
	if limit := float64(bal.CritDamageRankCap); limit > 0 && rank > limit {
		rank = limit
	}
	return base + float64(bal.CritDamagePerSkill)*rank
```

  Rewrite the doc comment's "deliberately does NOT clamp" paragraph: it now clamps at `CritDamageRankCap` (owner 2026-10-31, endgame power leak); 0 keeps the old unclamped line. Green; sabotage the clamp, watch the 89 case fail, restore.
- [ ] **Step 3: Commit** `feat(combat): crit damage stops growing at CritDamageRankCap`.

## Task 3: Route all 24 sites through `WeightedSkill`, and the guard

**Files:** the 24 sites in the spec's list.

- [ ] **Step 1: The guard first.** `internal/guards/skill_weight_reader_guard_test.go`, same package and shape as `ambient_line_key_guard_test.go`: walk non-test `.go` files under `internal/` and `modules/`, skip `internal/configs/`, parse each, and report every `*ast.SelectorExpr` whose `Sel.Name` is `SkillWeight` as `file:line: reads SkillWeight directly; use configs.WeightedSkill`. Fail if fewer than 100 files scanned. `TestSkillWeightGuardCanFail` feeds a probe with one direct read and one `configs.WeightedSkill(x)` call and expects exactly one report. Run: the main guard fails listing the 24 sites (that is the red; count them and confirm 24).
- [ ] **Step 2: Convert each site.** The rule: every `rank * SkillWeight` (or `SkillWeight * rank`, or a hoisted `w := float64(cfg.SkillWeight)` later multiplied by a rank) becomes `configs.WeightedSkill(rank)`, applied to the RANK alone, before any other factor. Examples:
  - `float64(stat) + float64(rank)*float64(bal.SkillWeight)` -> `float64(stat) + configs.WeightedSkill(float64(rank))`
  - swing count `(1 + rank*SW/50)` -> `(1 + configs.WeightedSkill(rank)/50)`
  - a site that hoists the weight and uses it for two ranks converts both uses and drops the hoisted variable.
  - a site that multiplies something that is NOT a skill rank by `SkillWeight` (read each one): stop and report it in your summary rather than guessing; do not convert it.
  Keep each site's comments true (several say "x SkillWeight"; reword to "weighted by configs.WeightedSkill" where they describe the formula). Add the `configs` import where missing.
- [ ] **Step 3: Behaviour is unchanged with the knee off.** Run the full touched packages: `go test ./internal/actions/ ./internal/characters/ ./internal/combat/ ./internal/crafting/ ./internal/hooks/ ./internal/configs/ -count=1` plus root and guards. All green with no test edits (Go defaults have no knee, so every score is identical). A test that needs editing means a conversion changed behaviour: fix the conversion, not the test.
- [ ] **Step 4: One knee-on test per family** (pin knee 65, SW 5, above 1.0): melee attack score (`calcAttackScore` or its nearest tested seam) for rank 89 drops by exactly 96 and for rank 50 is unchanged; one defence score likewise; `AttackSide.score` (defence_multiplier.go) likewise. Put each beside the existing tests of that function.
- [ ] **Step 5: Commit** `refactor: every SkillWeight reader goes through configs.WeightedSkill` (list the site count and any site you did not convert, with why).

## Task 4: Shipped values, docs

- [ ] **Step 1: `_datafiles/config.yaml`.** After `SkillWeight: 5.0` (line 1181), add a comment paragraph (knee rationale, owner 2026-10-31, grandmaster threshold, Meirok 445 -> 349 at weapon-combat 89) and:

```yaml
  SkillWeightKneeRank: 65
  SkillWeightAboveKnee: 1.0
```

  Change `CritDamagePerSkill: 0.05` to `0.025` and add `CritDamageRankCap: 65` after it, with a comment: max crit 3.625x; rank 25 crits 3.25x -> 2.63x (owner chose to move every player).
- [ ] **Step 2: Config test:** find the test that loads the shipped config for combat knobs (`grep -rln "config.yaml" internal/configs/*_test.go`) or add one on the `config_ambient_test.go` shape: shipped values read 5.0 / 65 / 1.0 / 0.025 / 65; absent keys load 0 / 0 / (SkillWeight) / 0.05 default / 0. Sabotage one shipped value, watch it fail, restore.
- [ ] **Step 3: Docs.** `internal/configs/context.md` (WeightedSkill, the three knobs, the guard), `internal/combat/context.md` (crit cap), `grep -rn "CritDamagePerSkill\|0.05 x rank\|SkillWeight" .claude/skills/` and fix any quoted formula, `docs/PATCH_NOTES.md` new dated entry in player voice per `dogmud-player-copy`, no numbers: skill still pays all the way to grandmaster, past it the returns flatten; critical hits land less hard at every level and stop growing at grandmaster.
- [ ] **Step 4: Commit** `balance: ship the skill weight knee at 65 and crit cap`.

## Task 5: Gate and PR (controller)

`gofmt -l internal/ modules/`; `go build ./...`; `go test ./... -count=1` (whole tree, since 24 sites span packages); `DOGMUD_BOOT_SMOKE=1 go test -run 'TestSmoke_' .`; `golangci-lint run --new-from-merge-base=origin/master`; isolated boot check per `dogmud-shipping`. Then mark PR #732 ready with a body (summary, spec link, model link, test plan) and wait for the owner's go before merging.
