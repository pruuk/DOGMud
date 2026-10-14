# Consider Fight Read Implementation Plan (#301)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `consider` stops comparing PowerScore ratios and reads a fight as a verdict plus two paces, predicted from combat's own math through shared helpers that PowerScore also uses, behind a Perception plus Search roll on the shared special-move cooldown.

**Architecture:** Layer 1 (pure, `internal/combat/fight_expect.go`) computes one side's expected melee and spell output and its defence from the very functions live combat calls (`buildAttackPlan`, `buildDamageParams`, `calcAttackScore`, a newly extracted `defenceEntryScore`, newly extracted spell helpers). Layer 2 (`internal/combat/fight_predict.go`) combines two sides into a `FightPrediction`. `internal/actions/fight_read.go` classifies it into a verdict and two paces and holds the copy; `actions.Consider` adds the roll, cooldown and training for players. PowerScore is rebuilt on layer 1. Four drift guards keep them from separating again.

**Tech Stack:** Go 1.25, the DOGMud engine (`internal/combat`, `internal/actions`, `internal/configs`), YAML config, Go tests.

**Spec:** `docs/superpowers/specs/completed/2026-10-13-consider-fight-read-design.md` (owner-approved 2026-10-13).

---

## Facts verified against source

Read from `origin/master` at `8267eb06f`, 2026-10-13, in addition to the spec's table.

| Fact | Where |
|---|---|
| `buildAttackPlan(src, tgt)` and `buildDamageParams(src, tgt, ws, statModBonus, srcType)` write no state; `buildDamageParams` already applies target mitigation, mob multiplier, statmod bonus, health penalty, prone, mutations, Warcry, crit multiplier and crit reduction | `internal/combat/combat_helpers.go:300`, `:492-561` |
| `calculateCombat` per swing: `calcAttackScore(...) * applyPositionHitModifiers(...)`, defence set from `DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), target, DefenceEntryOpts{})`, crit bar `calcCritThreshold`, statmod bonus `sourceChar.StatMod("damage")` | `internal/combat/combat.go:410-470` |
| The per-defence score is computed inline inside `runBestOfAllDefenseWithRunner` (quote, `GetDefenseScoreFor`, effectiveness, position, Rally, third party, sight ramp, incorporeal) and the function takes a `runner` | `combat_helpers.go:703-800` |
| A `combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}` scores with a sight multiplier of 1.0 (the parity test pins attack score 160 with it) | `internal/combat/melee_parity_stat_test.go:390-399` |
| Test fixtures to reuse: `pinParityBalance(t)`, `parityCombatant(t, name)`, the condition seed for `physical_mitigation`, and the `resolveCombatRound` driver loop | `melee_parity_stat_test.go:73`, `:109`, `:369-382`, `:430-456` |
| `combat.RunContest(atkScore, []contest.Entry) contest.Result` is the single opposed-contest entry point; `contest.Result` has `Success` and attack-positive `Margin` | `internal/combat/run_contest.go:23`; `internal/contest/contest.go:35-55`; `internal/actions/move.go:315` |
| `dice.OpposedRollStat` is deprecated with zero production callers: do not use it | `internal/dice/contest_floors.go:55-64` |
| Spell damage mean (stat, school skill, spell-power mutation, Dampened, weapon spell multiplier, conviction penalty) and mitigation channel live inline in `rollSpellDamage`; `combat` already imports `spells` | `internal/hooks/combat_shared_helpers.go:72-152`; `go list` |
| `spells.SeedSpellsForTest(map[string]*SpellData) func()` exists | `internal/spells/test_helpers.go:7` |
| In combat there is no base regen; only a `EffectRegenMult` (Regenerating) record heals: `HealthPerRound() * tox * regenMult * room` for players, `HealthPerRound() * regenMult * room` for mobs | `internal/hooks/NewRound_AutoHeal.go:217-230`, `:325-333`; `internal/characters/resources.go:379` |
| Tick records: `Condition.TickAmount` per trigger, `ConditionSpec.TickPool`, `RoundInterval`; the tick loop only fires records with `TriggersLeft > 0` and `RoundInterval >= 1` | `internal/conditions/conditions.go:15-31`, `:516-540`; `conditionspec.go:218-221` |
| Special-move refusal copy used at 15 call sites: `"You need a moment to recover before attempting another special move."` | e.g. `internal/usercommands/bash.go:53` |
| Balance knobs: field on `Balance` (`internal/configs/config.balance.go`), default in a `validate*()` method dispatched from `(*Balance).Validate` (`:1362`), value in `_datafiles/config.yaml` | `config.balance.go:1362-1372`; `config.balance.discovery.go` |
| Structural guard forbidding progression in `consider.go` | `internal/actions/consider_no_progression_test.go` |
| `fakeActor` in actions tests builds a bare `&characters.Character{}` (no Position); the new prediction needs a validated character | `internal/actions/consider_test.go:28-49` |
| AI companion: `considerWords(ratio)` (`actions.go:717`), its test (`aicompanion_test.go:504`), `.Ratio` reads at `actions.go:756`, `combat.go:204`, `combat.go:920`, flee check `combat.go:322` | as named |
| Mob ids: Straw Effigy 9614 (species 19, zero base damage). "Megalomania" is not in shipped mob data, so the hopeless case uses the owner's yardstick: species 99 Ascended (base 1000 in every stat), every skill at 100 (`skills.GetAllSkillNames()`), wielding the Blackrazor (item 40183); `Validate` hydrates species base stats | `_datafiles/world/dogmud/mobs/newcomer_antechamber/9614-straw_effigy.yaml`; `_datafiles/world/dogmud/species/99-ascended.yaml`; `_datafiles/world/dogmud/items/materials-40000/40183-the_blackrazor.yaml`; `internal/characters/validate.go:50-76`; `internal/skills/skills.go:144` |
| 19 mob files use the `lookout`, `thief` or `predator` archetype (the three PowerScore thresholds) | grep of `_datafiles/world/dogmud/mobs` |
| Root tests load mob templates with `fileloader.LoadAllFlatFiles[int, *mobs.Mob]` plus `mobs.SeedMobsForTest`, and spawn with `mobs.NewMobByIdFresh` | `shop_night_trade_guard_test.go:75-194` |

## File map

| File | Responsibility |
|---|---|
| `internal/configs/config.balance.go` (modify) | 10 new `Consider*` knobs on `Balance`; dispatch `validateConsider` |
| `internal/configs/config.balance.consider.go` (create) | defaults and ladder ordering for the knobs |
| `_datafiles/config.yaml` (modify) | shipped values with comments |
| `internal/combat/combat_helpers.go` (modify) | extract `defenceEntryScore` from the live defence loop |
| `internal/combat/spell_damage.go` (create) | `SpellRawDamage`, `SpellMitigation` extracted from `rollSpellDamage` |
| `internal/hooks/combat_shared_helpers.go` (modify) | `rollSpellDamage` calls the extracted helpers |
| `internal/characters/resources.go` (modify) | `CombatHealthRegenPerRound` |
| `internal/hooks/NewRound_AutoHeal.go` (modify) | in-combat heal reads `CombatHealthRegenPerRound` |
| `internal/conditions/effects.go` (modify) | `PoolTickPerRound` |
| `internal/combat/fight_expect.go` (create) | layer 1: expected swings, best defence, melee and spell damage per round, PowerScore terms |
| `internal/combat/fight_predict.go` (create) | layer 2: `FightPrediction`, `PredictFight` |
| `internal/combat/calculations.go` (modify) | PowerScore on layer 1 via `powerScoreParts` |
| `internal/actions/fight_read.go` (create) | verdicts, paces, read quality, classification, muddy slide, copy |
| `internal/actions/consider.go` (modify) | roll, cooldown, training, three lines |
| `modules/aicompanion/actions.go`, `combat.go` (modify) | read the shared fight read; flee on a lethal verdict |
| both `consider.template` files (modify) | new help |
| root `fight_fixtures_test.go`, `powerscore_table_test.go`, `consider_pinned_reads_test.go`, `fight_read_drift_guard_test.go` (create) | fixtures, before/after table, pinned verdicts, structural guard |
| `docs/superpowers/audits/2026-10-13-powerscore-before-after.md` (create) | the before/after table and the threshold decision |

## Ground rules for every task

- Work in the worktree from Task 0. Every text edit goes through the Edit tool (never a Python read-modify-write). Never `git add -A` or `git add .`: name paths.
- Go tests load Go config defaults, not `config.yaml` (see `dogmud-writing-tests`). Defaults in `validateConsider` therefore matter.
- Run a single package's tests with `go test ./internal/<pkg>/ -run <Name> -count=1`.
- No em dashes or en dashes in any comment, copy or doc you write.

---

### Task 0: Worktree and branch

**Files:** none

- [ ] **Step 1: Create the worktree from the spec branch**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/301-consider-fight-read C:/tmp/dogmud-301 docs/301-consider-spec
cd C:/tmp/dogmud-301
git rebase origin/master
go build ./...
```

Expected: build succeeds with no output. The branch carries the spec commits; the plan file is committed in this worktree too.

- [ ] **Step 2: Record the worktree in memory**

Per the global scratch rule, `C:/tmp/dogmud-301` must be named where it lives. Add one line to the session handoff or the plan's PR description: "worktree `C:/tmp/dogmud-301`, remove after merge".

---

### Task 1: Balance knobs

**Files:**
- Modify: `internal/configs/config.balance.go` (end of `Balance` struct, before the closing `}` near `:1360`; `Validate` at `:1362`)
- Create: `internal/configs/config.balance.consider.go`
- Create: `internal/configs/config.balance.consider_test.go`
- Modify: `_datafiles/config.yaml` (after `  SpecialMoveCooldown: 4`, `:784`)

- [ ] **Step 1: Write the failing test**

`internal/configs/config.balance.consider_test.go`:

```go
package configs

import "testing"

func TestValidateConsider_Defaults(t *testing.T) {
	b := Balance{}
	b.validateConsider()
	want := map[string][2]float64{
		"PaceMoments":      {float64(b.ConsiderPaceMoments), 2},
		"PaceShort":        {float64(b.ConsiderPaceShort), 6},
		"PaceLong":         {float64(b.ConsiderPaceLong), 20},
		"BandUntouched":    {float64(b.ConsiderBandUntouched), 0.05},
		"BandHopelessDent": {float64(b.ConsiderBandHopelessDent), 0.05},
		"BandScratches":    {float64(b.ConsiderBandScratches), 0.25},
		"BandHurt":         {float64(b.ConsiderBandHurt), 0.6},
		"BandBarely":       {float64(b.ConsiderBandBarely), 1.0},
		"BandLethal":       {float64(b.ConsiderBandLethal), 3.0},
		"ClearReadZ":       {float64(b.ConsiderClearReadZ), 1.0},
	}
	for name, gw := range want {
		if gw[0] != gw[1] {
			t.Errorf("Consider%s = %v, want default %v", name, gw[0], gw[1])
		}
	}
}

// A ladder out of order would make a band unreachable; it resets to the
// shipped ladder rather than guessing which value was meant.
func TestValidateConsider_OutOfOrderLaddersReset(t *testing.T) {
	b := Balance{ConsiderPaceMoments: 10, ConsiderPaceShort: 6, ConsiderPaceLong: 20,
		ConsiderBandScratches: 2.0, ConsiderBandHurt: 0.6}
	b.validateConsider()
	if b.ConsiderPaceMoments != 2 || b.ConsiderPaceShort != 6 || b.ConsiderPaceLong != 20 {
		t.Errorf("pace ladder = %v/%v/%v, want 2/6/20", b.ConsiderPaceMoments, b.ConsiderPaceShort, b.ConsiderPaceLong)
	}
	if b.ConsiderBandScratches != 0.25 || b.ConsiderBandHurt != 0.6 {
		t.Errorf("band ladder not reset: scratches %v hurt %v", b.ConsiderBandScratches, b.ConsiderBandHurt)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/configs/ -run TestValidateConsider -count=1`
Expected: FAIL to compile (`b.validateConsider undefined`, unknown fields).

- [ ] **Step 3: Add the fields**

In `internal/configs/config.balance.go`, immediately before the closing `}` of `type Balance struct` (after the `LightDarknessSpellDurationSkillDivisor` line), add:

```go

	// ── CONSIDER (#301) ─────────────────────────────────────────────────────
	// consider reads a fight as a verdict plus two paces; see
	// internal/actions/fight_read.go. Pace limits are rounds to kill. Band
	// limits are shares of health: cost is the share of yours the fight takes,
	// dent the share of theirs you take before dying.
	ConsiderPaceMoments      ConfigFloat `yaml:"ConsiderPaceMoments"`      // default 2
	ConsiderPaceShort        ConfigFloat `yaml:"ConsiderPaceShort"`        // default 6
	ConsiderPaceLong         ConfigFloat `yaml:"ConsiderPaceLong"`         // default 20
	ConsiderBandUntouched    ConfigFloat `yaml:"ConsiderBandUntouched"`    // default 0.05
	ConsiderBandHopelessDent ConfigFloat `yaml:"ConsiderBandHopelessDent"` // default 0.05
	ConsiderBandScratches    ConfigFloat `yaml:"ConsiderBandScratches"`    // default 0.25
	ConsiderBandHurt         ConfigFloat `yaml:"ConsiderBandHurt"`         // default 0.6
	ConsiderBandBarely       ConfigFloat `yaml:"ConsiderBandBarely"`       // default 1.0
	ConsiderBandLethal       ConfigFloat `yaml:"ConsiderBandLethal"`       // default 3.0
	ConsiderClearReadZ       ConfigFloat `yaml:"ConsiderClearReadZ"`       // default 1.0
```

In `func (b *Balance) Validate()`, add `b.validateConsider()` after `b.validateLighting()`.

- [ ] **Step 4: Add the validator**

`internal/configs/config.balance.consider.go`:

```go
package configs

// validateConsider sets defaults for the consider fight-read knobs (#301).
// Paces and bands are strict ascending ladders; a ladder out of order would
// make a step unreachable, so it resets to the shipped values whole.
func (b *Balance) validateConsider() {
	if b.ConsiderPaceMoments <= 0 {
		b.ConsiderPaceMoments = 2
	}
	if b.ConsiderPaceShort <= 0 {
		b.ConsiderPaceShort = 6
	}
	if b.ConsiderPaceLong <= 0 {
		b.ConsiderPaceLong = 20
	}
	if !(b.ConsiderPaceMoments < b.ConsiderPaceShort && b.ConsiderPaceShort < b.ConsiderPaceLong) {
		b.ConsiderPaceMoments, b.ConsiderPaceShort, b.ConsiderPaceLong = 2, 6, 20
	}

	if b.ConsiderBandUntouched <= 0 {
		b.ConsiderBandUntouched = 0.05
	}
	if b.ConsiderBandHopelessDent <= 0 {
		b.ConsiderBandHopelessDent = 0.05
	}
	if b.ConsiderBandScratches <= 0 {
		b.ConsiderBandScratches = 0.25
	}
	if b.ConsiderBandHurt <= 0 {
		b.ConsiderBandHurt = 0.6
	}
	if b.ConsiderBandBarely <= 0 {
		b.ConsiderBandBarely = 1.0
	}
	if b.ConsiderBandLethal <= 0 {
		b.ConsiderBandLethal = 3.0
	}
	if !(b.ConsiderBandUntouched < b.ConsiderBandScratches &&
		b.ConsiderBandScratches < b.ConsiderBandHurt &&
		b.ConsiderBandHurt < b.ConsiderBandBarely &&
		b.ConsiderBandBarely < b.ConsiderBandLethal) {
		b.ConsiderBandUntouched, b.ConsiderBandScratches, b.ConsiderBandHurt,
			b.ConsiderBandBarely, b.ConsiderBandLethal = 0.05, 0.25, 0.6, 1.0, 3.0
	}

	if b.ConsiderClearReadZ <= 0 {
		b.ConsiderClearReadZ = 1.0
	}
}
```

- [ ] **Step 5: Add the shipped values to config.yaml**

In `_datafiles/config.yaml`, with the Edit tool, replace the line `  SpecialMoveCooldown: 4` with:

```yaml
  SpecialMoveCooldown: 4
  #
  # Consider (#301): consider reads a fight as a verdict plus two paces.
  #   ConsiderPace*: rounds to kill at or under which a pace is "moments",
  #   "a short fight" or "a long fight"; above Long it is "an age".
  #   ConsiderBand*: limits on cost (the share of your health the fight takes)
  #   and, for HopelessDent, on dent (the share of theirs you take before
  #   dying). ConsiderClearReadZ: how far a won read roll must clear, in roll
  #   spreads, to read clearly. Each ladder must ascend or it resets whole.
  ConsiderPaceMoments: 2
  ConsiderPaceShort: 6
  ConsiderPaceLong: 20
  ConsiderBandUntouched: 0.05
  ConsiderBandHopelessDent: 0.05
  ConsiderBandScratches: 0.25
  ConsiderBandHurt: 0.6
  ConsiderBandBarely: 1.0
  ConsiderBandLethal: 3.0
  ConsiderClearReadZ: 1.0
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/configs/ -count=1`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.consider.go internal/configs/config.balance.consider_test.go _datafiles/config.yaml
git commit -m "feat(config): consider fight-read knobs (#301)"
```

---

### Task 2: Extract `defenceEntryScore` from the live defence loop

A pure refactor: the live loop and the prediction must score a defence with one function.

**Files:**
- Modify: `internal/combat/combat_helpers.go:703-800`
- Create: `internal/combat/defence_entry_score_test.go`

- [ ] **Step 1: Write the failing test**

`internal/combat/defence_entry_score_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// The live contest must score each defence with defenceEntryScore, the same
// function the fight prediction calls (#301). A capturing runner sees the
// entries the loop built; each must equal defenceEntryScore's score.
func TestDefenceEntryScore_IsWhatTheLiveContestRolls(t *testing.T) {
	pinParityBalance(t)
	attacker := parityCombatant(t, "attacker")
	defender := parityCombatant(t, "defender")
	ctx := combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}
	defSeq := DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), defender, DefenceEntryOpts{})
	if len(defSeq) == 0 {
		t.Fatal("fixture defender has no melee defence; the test proves nothing")
	}

	var seen []contest.Entry
	capture := func(_ float64, entries []contest.Entry) contest.Result {
		seen = append([]contest.Entry(nil), entries...)
		return contest.Result{}
	}
	res := AttackResult{}
	runBestOfAllDefenseWithRunner(&res, attacker, defender, defSeq, 100, false, ctx, capture)

	if len(seen) != len(defSeq) {
		t.Fatalf("runner saw %d entries, want %d", len(seen), len(defSeq))
	}
	for i, d := range defSeq {
		want, _, _ := defenceEntryScore(defender, d, false, ctx)
		if seen[i].Score != want {
			t.Errorf("%s: live entry score %v, defenceEntryScore %v", d, seen[i].Score, want)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/combat/ -run TestDefenceEntryScore -count=1`
Expected: FAIL to compile (`undefined: defenceEntryScore`).

- [ ] **Step 3: Extract the function**

In `combat_helpers.go`, add this function directly above `func runBestOfAllDefenseWithRunner`. Its body is the loop's scoring block moved verbatim, comments included (from `quote, quoted := targetChar.QuoteDefenseCost(defenseType)` through `defenseScore += mutations.GetPhysicalDefenseBonus(targetChar.Mutations)`):

```go
// defenceEntryScore is one defence's contest score against a melee swing, and
// the quote it was scored under. Pure: QuoteDefenseCost only quotes.
//
// The live contest loop and the fight prediction (fight_expect.go) both call
// it, so a defence can never be scored two ways (#301).
func defenceEntryScore(targetChar *characters.Character, defenseType combatvocab.Defence, isThirdParty bool, ctx combatContext) (float64, characters.CostQuote, bool) {
	bal := configs.GetBalanceConfig()

	// (moved verbatim from the loop: the U8 quote comment and the two lines)
	quote, quoted := targetChar.QuoteDefenseCost(defenseType)
	includeSkill := !quoted || quote.Affordable()
	defenseScore := targetChar.GetDefenseScoreFor(defenseType, includeSkill)

	// (moved verbatim: effectiveness, the position switch, Rally, third party,
	// sight ramp and incorporeal blocks with their comments)
	defenseScore *= defenceEffectiveness(defenseType)
	switch {
	case targetChar.IsProne() || targetChar.IsSupine():
		switch defenseType {
		case combatvocab.DefenceDodge:
			defenseScore *= float64(bal.ProneDodgePenalty)
		case combatvocab.DefenceParry:
			defenseScore *= float64(bal.ProneParryPenalty)
		case combatvocab.DefenceBlock:
			defenseScore *= float64(bal.ProneBlockPenalty)
		}
	case targetChar.IsStandingGrapple():
		switch defenseType {
		case combatvocab.DefenceDodge:
			defenseScore *= float64(bal.ClinchDodgePenalty)
		case combatvocab.DefenceParry:
			defenseScore *= float64(bal.ClinchParryPenalty)
		case combatvocab.DefenceBlock:
			defenseScore *= float64(bal.ClinchBlockPenalty)
		}
	case targetChar.IsGroundGrapple():
		switch defenseType {
		case combatvocab.DefenceDodge:
			defenseScore *= float64(bal.GroundedDodgePenalty)
		case combatvocab.DefenceParry:
			defenseScore *= float64(bal.GroundedParryPenalty)
		case combatvocab.DefenceBlock:
			defenseScore *= float64(bal.GroundedBlockPenalty)
		}
	}
	defenseScore *= targetChar.Conditions.Effect(conditions.EffectDefenseMult)
	if isThirdParty {
		defenseScore *= float64(bal.ThirdPartyGrapplePenalty)
	}
	defenseScore *= messaging.SightScoreMultiplier(ctx.targetDark, ctx.targetBright, bal)
	defenseScore += mutations.GetPhysicalDefenseBonus(targetChar.Mutations)

	return defenseScore, quote, quoted
}
```

Keep every original comment from those lines in place inside the new function; the abbreviated `(moved verbatim ...)` notes above stand for them and are not themselves added.

Then the loop in `runBestOfAllDefenseWithRunner` becomes (the U5b-2 "no affordability gate" comment stays in the loop, above the call):

```go
	for _, defenseType := range defSeq {
		// Track defense attempt
		result.DefenseAttempts = append(result.DefenseAttempts, defenseType)

		// Stage 9.4: Track defense for stance calculation
		targetChar.IncrementDefenseCount()

		// U5b-2: ... (the existing comment, unchanged)

		defenseScore, quote, quoted := defenceEntryScore(targetChar, defenseType, isThirdParty, ctx)

		entry := contest.Entry{Name: string(defenseType), Score: defenseScore}
		entries = append(entries, entry)
		candidates = append(candidates, quotedDefenceCandidate{
			entry:  entry,
			quote:  quote,
			quoted: quoted,
		})
	}
```

If `bal` is now unused at the top of `runBestOfAllDefenseWithRunner`, delete that line (the compiler will say).

- [ ] **Step 4: Run the new test and the existing combat suite**

Run: `go test ./internal/combat/ -count=1`
Expected: `ok` (includes `TestMeleeParityDamagePerSwing`, the statistical guard on this exact loop).

- [ ] **Step 5: Commit**

```bash
git add internal/combat/combat_helpers.go internal/combat/defence_entry_score_test.go
git commit -m "refactor(combat): extract defenceEntryScore from the live defence loop (#301)"
```

---

### Task 3: Extract the spell damage helpers

**Files:**
- Create: `internal/combat/spell_damage.go`
- Create: `internal/combat/spell_damage_test.go`
- Modify: `internal/hooks/combat_shared_helpers.go:72-152`

- [ ] **Step 1: Write the failing test**

`internal/combat/spell_damage_test.go`:

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

func spellFixtureCaster(t *testing.T) *characters.Character {
	t.Helper()
	c := parityCombatant(t, "caster")
	c.Stats.Willpower.Base = 120
	c.Stats.Willpower.Recalculate()
	c.Skills[string(skills.Spellcasting)] = 20
	c.ConvictionMax.Value = 1000
	c.Conviction = 1000
	return c
}

// SpellRawDamage is the pipeline mean rollSpellDamage rolls around (#301):
// with no mutations, no caster weapon and a full pool it is CalcRawDamage of
// the spell's primary stat and spellcasting rank.
func TestSpellRawDamage_IsThePipelineMean(t *testing.T) {
	pinParityBalance(t)
	caster := spellFixtureCaster(t)
	sd := &spells.SpellData{SpellId: "test-bolt", PrimaryStat: "willpower", DamageMultiplier: 1.5, DamageType: combatvocab.DamageMental}

	raw, rank, ok := SpellRawDamage(sd, caster)
	if !ok {
		t.Fatal("a damaging spell must report ok")
	}
	want := CalcRawDamage(caster.Stats.Willpower.ValueAdj, 20, 1.5, ChannelMagical)
	if rank != 20 || math.Abs(raw-want) > 1e-9 {
		t.Fatalf("SpellRawDamage = %v (rank %d), want %v (rank 20)", raw, rank, want)
	}
}

func TestSpellRawDamage_NotOkWithoutAMultiplier(t *testing.T) {
	pinParityBalance(t)
	if _, _, ok := SpellRawDamage(&spells.SpellData{SpellId: "legacy"}, spellFixtureCaster(t)); ok {
		t.Fatal("a spell with no damage_multiplier takes the legacy path and must not be ok")
	}
}

func TestSpellMitigation_ReadsTheDamageTypesChannel(t *testing.T) {
	pinParityBalance(t)
	target := parityCombatant(t, "target")
	sd := &spells.SpellData{SpellId: "test-bolt", DamageType: combatvocab.DamageMental}
	pct, curve, channel, ok := SpellMitigation(sd, target)
	if !ok || channel != "magical" || pct != target.GetMagicalMitigation() || curve != MitigationCurveFor(ChannelMagical) {
		t.Fatalf("SpellMitigation = %v %v %q %v, want the magical channel", pct, curve, channel, ok)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/combat/ -run 'TestSpell' -count=1`
Expected: FAIL to compile (`undefined: SpellRawDamage`).

- [ ] **Step 3: Create the helpers**

`internal/combat/spell_damage.go` (the bodies are `rollSpellDamage`'s lines moved verbatim; keep the U9, #22 and U7 comments with the lines they explain):

```go
package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// SpellRawDamage is a damaging spell's mean before mitigation, from caster:
// the spell's primary stat and school skill through CalcRawDamage, the
// spell-power mutations, crash-site dampening, the caster weapon's spell
// multiplier and the conviction penalty. ok is false for a spell with no
// damage_multiplier (the legacy magnitude path) or no caster. Pure: the dice
// stay in hooks.rollSpellDamage, which calls this, as does the fight
// prediction (#301).
func SpellRawDamage(spellData *spells.SpellData, caster *characters.Character) (rawDmg float64, skillLevel int, ok bool) {
	if spellData == nil || caster == nil || spellData.DamageMultiplier <= 0 {
		return 0, 0, false
	}

	castSkill := skills.Spellcasting
	if spellData.HasSchool(spells.SchoolManifestation) {
		castSkill = skills.Manifestation
	}
	skillLevel = caster.GetSkillLevel(castSkill)
	rawDmg = CalcRawDamage(spellData.CasterStatValue(caster.Stats), skillLevel, spellData.DamageMultiplier, ChannelMagical)

	if spBonus := mutations.GetSpellPowerMultiplier(caster.Mutations); spBonus != 0 {
		rawDmg *= 1.0 + spBonus
	}

	if caster.HasConditionFlag(conditions.Dampened) {
		factor := float64(configs.GetBalanceConfig().CrashSiteSuppressionFactor)
		rawDmg *= factor
		if rawDmg < 1 {
			rawDmg = 1
		}
	}

	if caster.Equipment.Weapon.ItemId > 0 {
		if sdm := caster.Equipment.Weapon.GetSpec().SpellDamageMultiplier; sdm > 0 {
			rawDmg *= sdm * mutations.GearEffectivenessMultiplier(caster.Mutations)
		}
	}

	cpPenalty := float64(configs.GetBalanceConfig().ConvictionPenaltyMax)
	rawDmg *= ResourceMultiplier(caster.Conviction,
		caster.EffectivePoolMax(characters.PoolConviction), cpPenalty)

	return rawDmg, skillLevel, true
}

// SpellMitigation is target's mitigation against spellData's damage type,
// keyed by MitigationChannelFor: physical harm meets physical mitigation,
// mental meets magical, social meets conviction. ok is false for a damage type
// with no channel (a non-harm spell); the curve is then the physical one, as
// rollSpellDamage has always fallen back to.
func SpellMitigation(spellData *spells.SpellData, target *characters.Character) (pct float64, curve MitigationCurve, channel string, ok bool) {
	ch, ok := MitigationChannelFor(spellData.DamageType)
	if !ok {
		return 0, MitigationCurveFor(ChannelPhysical), "none", false
	}
	switch ch {
	case ChannelPhysical:
		return target.GetPhysicalMitigation(), MitigationCurveFor(ch), "physical", true
	case ChannelMagical:
		return target.GetMagicalMitigation(), MitigationCurveFor(ch), "magical", true
	default:
		return target.GetConvictionMitigation(), MitigationCurveFor(ch), "conviction", true
	}
}
```

- [ ] **Step 4: Point `rollSpellDamage` at them**

In `internal/hooks/combat_shared_helpers.go`, replace the body of `rollSpellDamage` from `if spellData.DamageMultiplier > 0 && caster != nil {` through the closing `}` of that `if` block (just before `// Legacy fallback`) with:

```go
	if rawDmg, skillLevel, ok := combat.SpellRawDamage(spellData, caster); ok {
		mitigPct, curve, channelName, chOk := combat.SpellMitigation(spellData, target)
		if !chOk {
			mudlog.Error("calcSpellDamageForCharacter", "spell", spellData.SpellId, "error", "non-harm spell reached the damage pipeline")
		}

		// Chunk 5.11g: a crit bypasses mitigation up to the knee AND scales by the caster's
		// spellcasting rank. Before this, a crit against an unmitigated target
		// dealt exactly a normal hit's damage, so crit worth was set purely by
		// the defender's armour.
		return spellDamageRoll{
			rawDmg: rawDmg, rawMitig: mitigPct, effMitig: combat.EffectiveMitigation(mitigPct, curve),
			channel: channelName, isCrit: isCrit,
			dmg: combat.CritOrMitigatedDamage(rawDmg, skillLevel, isCrit, mitigPct, curve),
		}
	}
```

Remove imports the compiler reports unused (likely `skills`, `mutations`, possibly `conditions`, `configs`, `characters`; keep any still used elsewhere in the file).

- [ ] **Step 5: Run the tests**

Run: `go build ./... && go test ./internal/combat/ ./internal/hooks/ -count=1`
Expected: build clean; both `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/spell_damage.go internal/combat/spell_damage_test.go internal/hooks/combat_shared_helpers.go
git commit -m "refactor(combat): extract spell damage mean and mitigation from rollSpellDamage (#301)"
```

---

### Task 4: Health-per-round helpers

**Files:**
- Modify: `internal/characters/resources.go` (after `HealthPerRound`, `:379-400`)
- Modify: `internal/hooks/NewRound_AutoHeal.go:217-230`, `:325-333`
- Modify: `internal/conditions/effects.go` (after `HasEffect`, `:310`)
- Create: `internal/characters/combat_regen_test.go`
- Create: `internal/conditions/pool_tick_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/conditions/pool_tick_test.go`:

```go
package conditions

import "testing"

// PoolTickPerRound averages each live tick record's cached amount over its
// interval (#301): a heal of 6 every 3 rounds is 2 a round, a bleed of -4
// every round is -4, a spent record (TriggersLeft 0) and a record on another
// pool count nothing.
func TestPoolTickPerRound(t *testing.T) {
	t.Cleanup(SeedConditionsForTest(map[int]*ConditionSpec{
		9101: {ConditionId: 9101, Name: "test heal", TickPool: "health", RoundInterval: 3},
		9102: {ConditionId: 9102, Name: "test bleed", TickPool: "health", RoundInterval: 1},
		9103: {ConditionId: 9103, Name: "test stamina", TickPool: "stamina", RoundInterval: 1},
	}))
	cs := New()
	cs.List = append(cs.List,
		&Condition{ConditionId: 9101, TriggersLeft: 5, TickAmount: 6},
		&Condition{ConditionId: 9102, TriggersLeft: 5, TickAmount: -4},
		&Condition{ConditionId: 9102, TriggersLeft: 0, TickAmount: -100},
		&Condition{ConditionId: 9103, TriggersLeft: 5, TickAmount: 50},
	)
	if got := cs.PoolTickPerRound("health"); got != -2 {
		t.Fatalf("PoolTickPerRound(health) = %v, want -2 (2 heal, -4 bleed)", got)
	}
}
```

If `conditions.New()` returns a value rather than a pointer, take its address (`cs := New(); p := &cs`) and call through `p`; match the receiver of `HasEffect` (`func (bs *Conditions)`).

`internal/characters/combat_regen_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
)

// With no Regenerating record nothing heals in combat (NewRound_AutoHeal has
// no base regen in combat), and with one the record multiplies base regen.
func TestCombatHealthRegenPerRound(t *testing.T) {
	c := New()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.HealthMax.Value = 1000
	if got := c.CombatHealthRegenPerRound(); got != 0 {
		t.Fatalf("no record: got %v, want 0", got)
	}

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9201: {ConditionId: 9201, Name: "test regen", TriggerCount: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectRegenMult: conditions.EffectValue("3")}},
	}))
	c.Conditions.List = append(c.Conditions.List, &conditions.Condition{ConditionId: 9201, TriggersLeft: 1000})
	c.Conditions.Validate(true)

	want := float64(c.HealthPerRound()) * 3
	if got := c.CombatHealthRegenPerRound(); got != want {
		t.Fatalf("with record: got %v, want %v", got, want)
	}
}
```

`EffectValue` is "a number or the word magnitude" (`conditionspec.go:224-228`). If its Go type is not constructible from a string literal, read `internal/conditions/effects.go` for the type and build the value the way an existing test does (`grep -rn "EffectRegenMult" internal --include=*_test.go`).

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/conditions/ ./internal/characters/ -run 'TestPoolTickPerRound|TestCombatHealthRegenPerRound' -count=1`
Expected: FAIL to compile (both methods undefined).

- [ ] **Step 3: Implement**

In `internal/conditions/effects.go`, after `HasEffect`:

```go
// PoolTickPerRound is the per-round average of every live tick record on pool
// ("health", "stamina", "conviction"): each record's cached TickAmount spread
// over its RoundInterval. Positive heals, negative harms. Live means what the
// tick loop fires: TriggersLeft > 0 and RoundInterval >= 1. A record whose
// amount is not cached yet (TickAmount 0) counts nothing, and a stacking
// record counts once. Read-only; the fight prediction weighs heal-over-time
// and damage-over-time with it (#301).
func (bs *Conditions) PoolTickPerRound(pool string) float64 {
	total := 0.0
	for _, b := range bs.List {
		if b == nil || b.TriggersLeft <= 0 || b.TickAmount == 0 {
			continue
		}
		spec := GetConditionSpec(b.ConditionId)
		if spec == nil || spec.TickPool != pool || spec.RoundInterval < 1 {
			continue
		}
		total += float64(b.TickAmount) / float64(spec.RoundInterval)
	}
	return total
}
```

In `internal/characters/resources.go`, after `HealthPerRound`:

```go
// CombatHealthRegenPerRound is the health a heal spell's Regenerating record
// restores each combat round, before toxicity and the room multiplier: base
// regen times the record's multiplier, or 0 with no record. There is no other
// regeneration in combat. NewRound_AutoHeal's in-combat branches and the fight
// prediction both read it (#301).
func (c *Character) CombatHealthRegenPerRound() float64 {
	if !c.Conditions.HasEffect(conditions.EffectRegenMult) {
		return 0
	}
	return float64(c.HealthPerRound()) * c.Conditions.Effect(conditions.EffectRegenMult)
}
```

In `internal/hooks/NewRound_AutoHeal.go`, the player in-combat branch (`:217-230`) becomes:

```go
			} else {
				// In combat: no base regen, but a Regenerating record (heal spell) still applies
				if user.Character.Conditions.HasEffect(conditions.EffectRegenMult) {
					healAmt := int(math.Floor(user.Character.CombatHealthRegenPerRound() * toxRegenMult * regenMultiplier))
```

(the rest of the branch unchanged), and the mob in-combat branch (`:325-333`) becomes:

```go
		} else {
			// In combat: only the Regenerating record applies
			if mob.Character.Conditions.HasEffect(conditions.EffectRegenMult) {
				hpAmt := int(math.Floor(mob.Character.CombatHealthRegenPerRound() * mobRegenMult))
```

(the rest unchanged). The product is the same factors in a different order; a floating last-bit difference can at most move a floor at an exact integer boundary, which no test pins.

- [ ] **Step 4: Run the tests**

Run: `go build ./... && go test ./internal/conditions/ ./internal/characters/ ./internal/hooks/ -count=1`
Expected: all `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/conditions/effects.go internal/conditions/pool_tick_test.go internal/characters/resources.go internal/characters/combat_regen_test.go internal/hooks/NewRound_AutoHeal.go
git commit -m "refactor: shared in-combat regen and pool-tick-per-round helpers (#301)"
```

---

### Task 5: Root fixtures and the PowerScore BEFORE table

Captured now, before PowerScore changes, so Task 8 has a true baseline.

**Files:**
- Create: `fight_fixtures_test.go` (repo root, `package main`)
- Create: `powerscore_table_test.go` (repo root)
- Create: `docs/superpowers/audits/2026-10-13-powerscore-before-after.md`

- [ ] **Step 1: Write the shared root fixtures**

`fight_fixtures_test.go`:

```go
package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// loadFightWorld loads the shipped config and the data a fight read needs:
// conditions, items, species, spells and the mob templates, seeded so
// mobs.NewMobByIdFresh can spawn them. Shared by the PowerScore table and
// the pinned consider reads (#301).
func loadFightWorld(t *testing.T) map[int]*mobs.Mob {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(spells.SeedSpellsForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	species.LoadDataFiles()
	spells.LoadSpellFiles()

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", dataPath, err)
	}
	t.Cleanup(mobs.SeedMobsForTest(templates, map[int]*mobs.Mob{}))
	return templates
}

// fixtureAdventurer is a bare-handed player with every stat at stat and
// unarmed combat and search at rank, pools sized from stat. Not a shipped
// character: a fixed yardstick for tables and pinned reads.
func fixtureAdventurer(t *testing.T, name string, stat, rank int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Name = name
	c.RoomId = 1
	for _, s := range []*int{&c.Stats.Strength.Base, &c.Stats.Dexterity.Base, &c.Stats.Perception.Base,
		&c.Stats.Vitality.Base, &c.Stats.Willpower.Base, &c.Stats.Charisma.Base} {
		*s = stat
	}
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stats.Perception.Recalculate()
	c.Stats.Vitality.Recalculate()
	c.Stats.Willpower.Recalculate()
	c.Stats.Charisma.Recalculate()
	if err := c.Validate(); err != nil {
		t.Fatalf("validate %s: %v", name, err)
	}
	c.Skills[string(skills.UnarmedCombat)] = rank
	c.Skills[string(skills.Search)] = rank
	c.HealthMax.Value = 2 * stat
	c.Health = c.HealthMax.Value
	c.StaminaMax.Value = 2 * stat
	c.Stamina = c.StaminaMax.Value
	c.ConvictionMax.Value = stat
	c.Conviction = stat
	return c
}
```

If `SeedSpellsForTest(nil)` does not snapshot-and-restore the live registry the way the other three seeders do (read `internal/spells/test_helpers.go`), replace that line with a snapshot of `spells` the file's own helper supports, so this test cannot leak spells into others.

- [ ] **Step 2: Write the table test**

`powerscore_table_test.go`:

```go
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// TestPowerScoreTable writes PowerScore for every mob whose behavior acts on a
// PowerScore ratio, against three fixture players, with each archetype's own
// decision (#301). Run before and after PowerScore moves onto the shared
// helpers and compare. Opt-in: set DOGMUD_POWERSCORE_TABLE to the output path.
//
// Decisions, from the shipped behavior trees:
//   lookout  acts when self/target > 1.0   (lookout.yaml:49-50)
//   thief    acts when self/target > 1.5   (thief.yaml:50-51)
//   predator picks a target when target/self < 0.85 (predator.yaml:44)
func TestPowerScoreTable(t *testing.T) {
	out := os.Getenv("DOGMUD_POWERSCORE_TABLE")
	if out == "" {
		t.Skip("set DOGMUD_POWERSCORE_TABLE=<file> to write the PowerScore table (#301)")
	}
	templates := loadFightWorld(t)

	players := []struct {
		name       string
		stat, rank int
	}{{"novice", 60, 5}, {"journeyman", 100, 20}, {"veteran", 150, 40}}
	playerPower := make([]float64, len(players))
	for i, p := range players {
		playerPower[i] = combat.PowerScore(*fixtureAdventurer(t, p.name, p.stat, p.rank))
	}

	var ids []int
	for id, m := range templates {
		switch m.BehaviorArchetype {
		case "lookout", "thief", "predator":
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	var b strings.Builder
	b.WriteString("| mob | archetype | power |")
	for i, p := range players {
		fmt.Fprintf(&b, " vs %s (%.0f) |", p.name, playerPower[i])
	}
	b.WriteString("\n|---|---|---|---|---|---|\n")
	for _, id := range ids {
		inst := mobs.NewMobByIdFresh(mobs.MobId(id), 1)
		if inst == nil {
			t.Fatalf("mob %d failed to spawn", id)
		}
		arch := templates[id].BehaviorArchetype
		power := combat.PowerScore(inst.Character)
		fmt.Fprintf(&b, "| %d %s | %s | %.0f |", id, inst.Character.Name, arch, power)
		for i := range players {
			ratio := power / playerPower[i]
			acts := false
			switch arch {
			case "lookout":
				acts = ratio > 1.0
			case "thief":
				acts = ratio > 1.5
			case "predator":
				acts = 1/ratio < 0.85
			}
			mark := "-"
			if acts {
				mark = "acts"
			}
			fmt.Fprintf(&b, " %.2f %s |", ratio, mark)
		}
		b.WriteString("\n")
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d rows to %s", len(ids), out)
}
```

- [ ] **Step 3: Run it and check it skips by default**

Run: `go test . -run TestPowerScoreTable -count=1 -v`
Expected: `--- SKIP` with the message.

- [ ] **Step 4: Write the BEFORE table**

Run (Git Bash): `DOGMUD_POWERSCORE_TABLE="$TMP/powerscore-before.md" go test . -run TestPowerScoreTable -count=1 -v`
Expected: `wrote 19 rows` (the count is the number of lookout, thief and predator mobs; if it differs, record the real count, do not edit the test to hit 19).

- [ ] **Step 5: Start the audit doc with it**

Create `docs/superpowers/audits/2026-10-13-powerscore-before-after.md`:

```markdown
# PowerScore before and after the shared helpers (#301)

PowerScore moved its offense and avoidance terms onto the fight-read helpers
(plan `docs/superpowers/plans/completed/2026-10-13-consider-fight-read.md`, Task 8).
Its values change, so this table checks the three shipped behaviors that act
on a PowerScore ratio. Generated by `TestPowerScoreTable` (repo root) against
three fixture players (all stats 60/100/150, unarmed and search ranks
5/20/40). Each cell is the mob/player ratio and whether that mob's archetype
acts on it ("acts") or not ("-").

## Before (master, before Task 8)

<paste the contents of $TMP/powerscore-before.md here>

## After

(filled in Task 8)

## Decision

(filled in Task 8)
```

Paste the generated table under "Before" with the Edit tool.

- [ ] **Step 6: Commit**

```bash
git add fight_fixtures_test.go powerscore_table_test.go docs/superpowers/audits/2026-10-13-powerscore-before-after.md
git commit -m "test: PowerScore table and the before baseline (#301)"
```

---

### Task 6: Layer 1, one side's expected output

**Files:**
- Create: `internal/combat/fight_expect.go`
- Create: `internal/combat/fight_expect_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/combat/fight_expect_test.go`:

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// expectedSwings must be the live plan, swing for swing: the same weapons,
// swing counts, damage means and attack scores calculateCombat uses.
func TestExpectedSwings_IsTheLivePlan(t *testing.T) {
	pinParityBalance(t)
	a := parityCombatant(t, "attacker")
	d := parityCombatant(t, "defender")

	plan := buildAttackPlan(a, d)
	got := expectedSwings(a, d, User)
	if len(got) != len(plan.weapons) {
		t.Fatalf("%d expectations for %d planned weapons", len(got), len(plan.weapons))
	}
	for i, ws := range plan.weapons {
		sdp := buildDamageParams(a, d, ws, a.StatMod(`damage`), User)
		if got[i].swings != ws.swingCount || got[i].dmgMean != sdp.dmgMean {
			t.Errorf("weapon %d: swings %d dmgMean %v, live %d %v", i, got[i].swings, got[i].dmgMean, ws.swingCount, sdp.dmgMean)
		}
		if math.Abs(got[i].attackScore-160) > 1e-9 {
			t.Errorf("weapon %d attack score %v, want the parity 160", i, got[i].attackScore)
		}
	}
}

func TestCritShare_ParityIsTheTwoSigmaTail(t *testing.T) {
	// Equal scores, bar 2.0: P(Z >= 2) for a standard normal.
	if got := critShare(160, 160, 2.0); math.Abs(got-0.0227501) > 1e-6 {
		t.Fatalf("critShare(parity, bar 2) = %v, want 0.02275", got)
	}
}

// At parity a swing lands half the time; a defenceless target is always hit.
func TestMeleeDamagePerRound_HitChanceBrackets(t *testing.T) {
	pinParityBalance(t)
	a := parityCombatant(t, "attacker")
	d := parityCombatant(t, "defender")

	full := 0.0
	for _, sw := range expectedSwings(a, d, User) {
		full += float64(sw.swings) * sw.dmgMean
	}
	got := meleeDamagePerRound(a, d, User)
	if got <= 0.4*full || got >= 0.7*full {
		t.Fatalf("parity damage per round %v, want about half of every swing landing (%v)", got, full)
	}
	if bestMeleeDefenceScore(d) <= 0 {
		t.Fatal("parity defender must have a defence score")
	}
}

// PowerScore's terms are the same expectations against the neutral target.
func TestPowerScoreTerms_ReadTheSharedPlan(t *testing.T) {
	pinParityBalance(t)
	a := parityCombatant(t, "fighter")
	want := 0.0
	for _, sw := range expectedSwings(a, neutralTarget(), User) {
		want += float64(sw.swings) * sw.dmgMean
	}
	if got := meleeOffenseRaw(a, User); got != want {
		t.Fatalf("meleeOffenseRaw = %v, want %v", got, want)
	}
	dodge, _, _ := defenceEntryScore(a, combatvocab.DefenceDodge, false, neutralCtx())
	parry, _, _ := defenceEntryScore(a, combatvocab.DefenceParry, false, neutralCtx())
	block, _, _ := defenceEntryScore(a, combatvocab.DefenceBlock, false, neutralCtx())
	if got := meleeAvoidance(a); math.Abs(got-(dodge+parry+block)/3) > 1e-9 {
		t.Fatalf("meleeAvoidance = %v, want the mean of the three live scores", got)
	}
}

// A caster's best damaging spell, mitigated, spread over the shared cooldown.
func TestSpellDamagePerRound(t *testing.T) {
	pinParityBalance(t)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"test-bolt": {SpellId: "test-bolt", PrimaryStat: "willpower", DamageMultiplier: 1.5, DamageType: combatvocab.DamageMental},
	}))
	caster := spellFixtureCaster(t)
	caster.SpellBook = map[string]int{"test-bolt": 1}
	target := parityCombatant(t, "target")

	raw, _, _ := SpellRawDamage(spells.GetSpell("test-bolt"), caster)
	pct, curve, _, _ := SpellMitigation(spells.GetSpell("test-bolt"), target)
	// Casting shares the special-move cooldown, so one cast per cooldown.
	cooldown := float64(configs.GetBalanceConfig().SpecialMoveCooldown)
	if cooldown <= 0 {
		t.Fatalf("SpecialMoveCooldown = %v; the test needs a positive cooldown to mean anything", cooldown)
	}
	want := ApplyMitigation(raw, pct, curve) / cooldown
	if got := spellDamagePerRound(caster, target); math.Abs(got-want) > 1e-9 {
		t.Fatalf("spellDamagePerRound = %v, want %v", got, want)
	}
	if got := spellDamagePerRound(parityCombatant(t, "no spells"), target); got != 0 {
		t.Fatalf("a character with no spells: %v, want 0", got)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/combat/ -run 'TestExpectedSwings|TestCritShare|TestMeleeDamagePerRound|TestPowerScoreTerms|TestSpellDamagePerRound' -count=1`
Expected: FAIL to compile (undefined layer 1 functions).

- [ ] **Step 3: Implement layer 1**

`internal/combat/fight_expect.go`:

```go
package combat

import (
	"math"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// Layer 1 of the fight read (#301): one side's expected output and defence,
// built ONLY from the functions live combat calls. Nothing here may carry its
// own copy of a swing, hit, damage or defence formula; the drift guards in
// fight_read_drift_guard_test.go (repo root) and fight_expect_stat_test.go
// hold that line. Everything is pure: no dice, no state written.

// neutralCtx is a prediction's combat context: full sight both ways and no
// light penalty, so the sight ramp scores 1.0.
func neutralCtx() combatContext {
	return combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}
}

var (
	neutralOnce sync.Once
	neutralChar *characters.Character
)

// neutralTarget is an opponent with no armour and no species: what PowerScore
// measures offense against, so that term has no opponent in it. Built once;
// nothing may mutate it.
func neutralTarget() *characters.Character {
	neutralOnce.Do(func() {
		c := characters.New()
		c.Name = "neutral target"
		_ = c.Validate()
		neutralChar = c
	})
	return neutralChar
}

// sourceTypeOf is the SourceTarget combat uses for c.
func sourceTypeOf(c *characters.Character) SourceTarget {
	if c.IsMob {
		return Mob
	}
	return User
}

// swingExpectation is one weapon's planned round: how many swings, the attack
// score each swing rolls, a normal hit's and a crit's mean damage, and the
// crit bar.
type swingExpectation struct {
	swings      int
	attackScore float64
	dmgMean     float64
	critMean    float64
	critBar     float64
}

// expectedSwings is src's planned round against tgt, weapon by weapon, from
// the calls calculateCombat makes: buildAttackPlan, buildDamageParams,
// calcAttackScore times applyPositionHitModifiers, and calcCritThreshold.
func expectedSwings(src, tgt *characters.Character, srcType SourceTarget) []swingExpectation {
	plan := buildAttackPlan(src, tgt)
	statModBonus := src.StatMod(`damage`)
	ctx := neutralCtx()
	bar := calcCritThreshold(src, tgt)
	out := make([]swingExpectation, 0, len(plan.weapons))
	for _, ws := range plan.weapons {
		sdp := buildDamageParams(src, tgt, ws, statModBonus, srcType)
		out = append(out, swingExpectation{
			swings:      ws.swingCount,
			attackScore: calcAttackScore(src, tgt, ws.weapon, ws.penalty, ctx) * applyPositionHitModifiers(src, tgt),
			dmgMean:     sdp.dmgMean,
			critMean:    sdp.rawDmgForCrit * sdp.critDmgMult * (1 - sdp.critReduction),
			critBar:     bar,
		})
	}
	return out
}

// bestMeleeDefenceScore is defender's strongest melee defence, scored by
// defenceEntryScore as the live contest scores it; 0 with none.
func bestMeleeDefenceScore(defender *characters.Character) float64 {
	best := 0.0
	ctx := neutralCtx()
	for _, d := range DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), defender, DefenceEntryOpts{}) {
		if s, _, _ := defenceEntryScore(defender, d, false, ctx); s > best {
			best = s
		}
	}
	return best
}

// critShare is the chance a swing crits: the attack-positive margin,
// normalised as ContestCritAt normalises it (roll spread times root two),
// reaching bar.
func critShare(atk, def, bar float64) float64 {
	sigma := dice.StdDevFor(atk)
	if sigma <= 0 {
		return 0
	}
	z := bar - (atk-def)/(sigma*math.Sqrt2)
	return 0.5 * math.Erfc(z/math.Sqrt2)
}

// meleeDamagePerRound is src's expected melee damage to tgt in one round.
// It approximates the live contest on purpose: one defence (the best), no
// contest floor, gap compression, fumbles or partial deflection. The read is
// rough by design (spec), and fight_expect_stat_test.go measures how rough.
func meleeDamagePerRound(src, tgt *characters.Character, srcType SourceTarget) float64 {
	def := bestMeleeDefenceScore(tgt)
	total := 0.0
	for _, sw := range expectedSwings(src, tgt, srcType) {
		hit := 1.0
		if def > 0 {
			hit = dice.OpposedSuccessChance(sw.attackScore, def, dice.StdDevFor(sw.attackScore))
		}
		crit := math.Min(critShare(sw.attackScore, def, sw.critBar), hit)
		total += float64(sw.swings) * ((hit-crit)*sw.dmgMean + crit*sw.critMean)
	}
	return total
}

// spellDamagePerRound is src's best damaging spell against tgt, mitigated,
// spread over the shared special-move cooldown that casting claims.
func spellDamagePerRound(src, tgt *characters.Character) float64 {
	best := 0.0
	for spellId := range src.SpellBook {
		if !src.HasSpell(spellId) {
			continue
		}
		sd := spells.GetSpell(spellId)
		raw, _, ok := SpellRawDamage(sd, src)
		if !ok {
			continue
		}
		pct, curve, _, chOk := SpellMitigation(sd, tgt)
		if !chOk {
			continue
		}
		if d := ApplyMitigation(raw, pct, curve); d > best {
			best = d
		}
	}
	cd := float64(configs.GetBalanceConfig().SpecialMoveCooldown)
	if cd < 1 {
		cd = 1
	}
	return best / cd
}

// meleeOffenseRaw is PowerScore's melee offense: every planned swing's mean
// damage against the neutral target, before hit chance.
func meleeOffenseRaw(char *characters.Character, srcType SourceTarget) float64 {
	total := 0.0
	for _, sw := range expectedSwings(char, neutralTarget(), srcType) {
		total += float64(sw.swings) * sw.dmgMean
	}
	return total
}

// meleeAvoidance is PowerScore's avoidance: the mean of char's dodge, parry
// and block as the live contest scores them.
func meleeAvoidance(char *characters.Character) float64 {
	ctx := neutralCtx()
	sum := 0.0
	for _, d := range []combatvocab.Defence{combatvocab.DefenceDodge, combatvocab.DefenceParry, combatvocab.DefenceBlock} {
		s, _, _ := defenceEntryScore(char, d, false, ctx)
		sum += s
	}
	return sum / 3.0
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/combat/ -count=1`
Expected: `ok`. If `neutralTarget()` panics inside `buildWeaponSetup` or `buildDamageParams`, the zero character lacks something those read (a nil `Position`, for instance); fix it in `neutralTarget` by setting that field the way `parityCombatant` does, never by guarding the shared function.

- [ ] **Step 5: Commit**

```bash
git add internal/combat/fight_expect.go internal/combat/fight_expect_test.go
git commit -m "feat(combat): layer 1 of the fight read on combat's own functions (#301)"
```

---

### Task 7: Layer 2, `PredictFight`

**Files:**
- Create: `internal/combat/fight_predict.go`
- Create: `internal/combat/fight_predict_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/combat/fight_predict_test.go`:

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)

func TestPredictFight_StrongerSideCostsLess(t *testing.T) {
	pinParityBalance(t)
	strong := parityCombatant(t, "strong")
	strong.Stats.Strength.Base, strong.Stats.Dexterity.Base = 160, 160
	strong.Stats.Strength.Recalculate()
	strong.Stats.Dexterity.Recalculate()
	weak := parityCombatant(t, "weak")
	strong.Health, weak.Health = 300, 300

	p := PredictFight(strong, weak)
	q := PredictFight(weak, strong)
	if !(p.Cost < q.Cost && p.Dent > q.Dent) {
		t.Fatalf("strong reads cost %v dent %v; weak reads cost %v dent %v", p.Cost, p.Dent, q.Cost, q.Dent)
	}
	if p.RoundsToKillThem >= q.RoundsToKillThem {
		t.Fatalf("strong needs %v rounds, weak %v", p.RoundsToKillThem, q.RoundsToKillThem)
	}
}

// Zero damage is infinity, explicitly: no division by zero, no NaN.
func TestPredictFight_ZeroDamageIsInfinite(t *testing.T) {
	p := FightPrediction{}
	finish(&p, 100, 100)
	if !math.IsInf(p.RoundsToKillThem, 1) || !math.IsInf(p.RoundsToKillYou, 1) || p.Cost != 0 || p.Dent != 0 {
		t.Fatalf("zero both ways = %+v", p)
	}

	// You can hurt them, they cannot hurt you: you take nothing, and you would
	// deal them everything given forever.
	q := FightPrediction{YourDamagePerRound: 5}
	finish(&q, 100, 100)
	if q.Cost != 0 || !math.IsInf(q.Dent, 1) || q.RoundsToKillThem != 20 {
		t.Fatalf("one-way damage = %+v", q)
	}
}

// Lifesteal on your weapon comes off the damage you take.
func TestPredictFight_LifestealReducesIncoming(t *testing.T) {
	pinParityBalance(t)
	you := parityCombatant(t, "you")
	them := parityCombatant(t, "them")
	you.Health, them.Health = 300, 300
	before := PredictFight(you, them).TheirDamagePerRound

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9301: {ConditionId: 9301, Name: "test lifesteal", TriggerCount: 1, StatMods: statmods.StatMods{"lifesteal_pct": 50}},
	}))
	you.Conditions.List = append(you.Conditions.List, &conditions.Condition{ConditionId: 9301, TriggersLeft: 1000})
	you.Conditions.Validate(true)

	if after := PredictFight(you, them).TheirDamagePerRound; after >= before {
		t.Fatalf("lifesteal left incoming at %v (was %v)", after, before)
	}
}
```

The zero-damage case calls the internal `finish` directly so it does not depend on building two characters that deal exactly nothing.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/combat/ -run TestPredictFight -count=1`
Expected: FAIL to compile.

- [ ] **Step 3: Implement**

`internal/combat/fight_predict.go`:

```go
package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// FightPrediction is one character's read of a one-on-one fight against
// another as both stand now (#301, layer 2). Damage rates are net: each
// side's output less the other side's heals, never negative.
type FightPrediction struct {
	YourDamagePerRound  float64 // net damage you deal them each round
	TheirDamagePerRound float64 // net damage they deal you each round
	RoundsToKillThem    float64 // math.Inf(1) when YourDamagePerRound is 0
	RoundsToKillYou     float64 // math.Inf(1) when TheirDamagePerRound is 0
	Cost                float64 // share of your current health the fight takes
	Dent                float64 // share of their current health you take before you drop
}

// PredictFight reads you against them from layer 1: melee and spell output
// each way, then each side's lifesteal (on its melee), Regenerating record and
// health ticks (heal-over-time less damage-over-time).
func PredictFight(you, them *characters.Character) FightPrediction {
	yourMelee := meleeDamagePerRound(you, them, sourceTypeOf(you))
	theirMelee := meleeDamagePerRound(them, you, sourceTypeOf(them))
	yourOut := yourMelee + spellDamagePerRound(you, them)
	theirOut := theirMelee + spellDamagePerRound(them, you)

	p := FightPrediction{
		YourDamagePerRound:  math.Max(0, yourOut-healPerRound(them, theirMelee)),
		TheirDamagePerRound: math.Max(0, theirOut-healPerRound(you, yourMelee)),
	}
	finish(&p, float64(max(you.Health, 1)), float64(max(them.Health, 1)))
	return p
}

// healPerRound is what c regains each combat round: lifesteal on its own
// melee damage, a heal spell's Regenerating record, and health ticks
// (negative when damage-over-time outweighs heal-over-time).
func healPerRound(c *characters.Character, meleeOut float64) float64 {
	h := meleeOut * float64(c.StatMod("lifesteal_pct")) / 100.0
	h += c.CombatHealthRegenPerRound()
	h += c.Conditions.PoolTickPerRound("health")
	return h
}

// finish derives rounds, cost and dent from the two net rates. Zero damage is
// infinite rounds, handled here rather than by a division by zero.
func finish(p *FightPrediction, yourHealth, theirHealth float64) {
	p.RoundsToKillThem = roundsToKill(theirHealth, p.YourDamagePerRound)
	p.RoundsToKillYou = roundsToKill(yourHealth, p.TheirDamagePerRound)

	switch {
	case p.TheirDamagePerRound == 0:
		p.Cost = 0
	case math.IsInf(p.RoundsToKillThem, 1):
		p.Cost = math.Inf(1)
	default:
		p.Cost = p.RoundsToKillThem * p.TheirDamagePerRound / yourHealth
	}

	switch {
	case p.YourDamagePerRound == 0:
		p.Dent = 0
	case math.IsInf(p.RoundsToKillYou, 1):
		p.Dent = math.Inf(1)
	default:
		p.Dent = p.RoundsToKillYou * p.YourDamagePerRound / theirHealth
	}
}

func roundsToKill(health, perRound float64) float64 {
	if perRound <= 0 {
		return math.Inf(1)
	}
	return health / perRound
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/combat/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/combat/fight_predict.go internal/combat/fight_predict_test.go
git commit -m "feat(combat): PredictFight, layer 2 of the fight read (#301)"
```

---

### Task 8: PowerScore on the shared helpers, and the AFTER table

**Files:**
- Modify: `internal/combat/calculations.go:17-142`
- Create: `internal/combat/powerscore_parts_test.go`
- Modify: `docs/superpowers/audits/2026-10-13-powerscore-before-after.md`

- [ ] **Step 1: Write the failing test (drift guard 2, same inputs)**

`internal/combat/powerscore_parts_test.go`:

```go
package combat

import "testing"

// Drift guard 2 (#301): PowerScore's melee offense and avoidance ARE the
// fight read's layer 1 terms, and PowerScore is the sum of its parts.
func TestPowerScoreParts_AreTheSharedTerms(t *testing.T) {
	pinParityBalance(t)
	c := parityCombatant(t, "fighter")

	parts := powerScoreParts(*c)
	if parts.meleeOffense != meleeOffenseRaw(c, User) {
		t.Errorf("meleeOffense %v, layer 1 %v", parts.meleeOffense, meleeOffenseRaw(c, User))
	}
	if parts.avoidance != meleeAvoidance(c) {
		t.Errorf("avoidance %v, layer 1 %v", parts.avoidance, meleeAvoidance(c))
	}
	if got, want := PowerScore(*c), parts.total(); got != want {
		t.Errorf("PowerScore %v, sum of parts %v", got, want)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/combat/ -run TestPowerScoreParts -count=1`
Expected: FAIL to compile (`undefined: powerScoreParts`).

- [ ] **Step 3: Rebuild PowerScore**

In `internal/combat/calculations.go`, replace `PowerScore` (from its doc comment at `:17` through its closing brace before `func ChanceToTame`) with:

```go
// powerScoreTerms are PowerScore's components, kept apart so drift guard 2
// can hold the shared ones to the fight read's layer 1 (#301).
type powerScoreTerms struct {
	meleeOffense float64 // layer 1: every planned swing's mean against the neutral target
	spellOffense float64 // magical and conviction channel raw damage, half weight each
	mitigation   float64 // effective mitigation, averaged over the three channels
	avoidance    float64 // layer 1: mean of dodge, parry and block as the contest scores them
	naturalArmor float64
	durability   float64
	skills       float64
	mutations    float64
	kd           float64
}

func (p powerScoreTerms) total() float64 {
	return p.meleeOffense + p.spellOffense + p.mitigation*300 + p.avoidance*2 + p.naturalArmor +
		p.durability + p.skills + p.mutations + p.kd
}

// powerScoreParts computes PowerScore's terms. Melee offense and avoidance
// come from the fight read's shared helpers (fight_expect.go), so PowerScore,
// consider and combat cannot drift apart (#301).
func powerScoreParts(char characters.Character) powerScoreTerms {
	c := &char
	p := powerScoreTerms{}

	// ── OFFENSE ──────────────────────────────────────────────────────────
	p.meleeOffense = meleeOffenseRaw(c, sourceTypeOf(c))

	spellcastingRank := char.GetSkillLevel(skills.Spellcasting)
	spellMult := 1.0
	if char.Equipment.Weapon.ItemId > 0 {
		if sdm := char.Equipment.Weapon.GetSpec().SpellDamageMultiplier; sdm > 0 {
			spellMult = sdm * mutations.GearEffectivenessMultiplier(char.Mutations)
		}
	}
	magAtk := CalcRawDamage(char.Stats.Willpower.ValueAdj, spellcastingRank, spellMult, ChannelMagical)
	convAtk := CalcRawDamage(char.Stats.Charisma.ValueAdj, char.GetSkillLevel(skills.Rhetoric), 0.5, ChannelConviction)
	p.spellOffense = magAtk*0.5 + convAtk*0.5

	// ── DEFENSE ──────────────────────────────────────────────────────────
	// Effective, not raw (#465).
	p.mitigation = (EffectiveMitigationFor(c, ChannelPhysical) +
		EffectiveMitigationFor(c, ChannelMagical) +
		EffectiveMitigationFor(c, ChannelConviction)) / 3.0
	p.avoidance = meleeAvoidance(c)
	p.naturalArmor = float64(mutations.GetNaturalArmor(char.Mutations))

	// ── DURABILITY ───────────────────────────────────────────────────────
	p.durability = float64(char.HealthMax.Value) +
		float64(char.StaminaMax.Value)*0.5 +
		float64(char.ConvictionMax.Value)*0.5

	// ── SKILLS ───────────────────────────────────────────────────────────
	totalRanks := 0
	for _, rank := range char.GetAllSkillRanks() {
		totalRanks += rank
	}
	p.skills = math.Sqrt(float64(totalRanks)) * 25.0

	// ── MUTATIONS ────────────────────────────────────────────────────────
	totalMutLevels := 0
	for _, level := range char.Mutations {
		if level > 0 {
			totalMutLevels += level
		}
	}
	p.mutations = float64(totalMutLevels) * 20.0

	// ── KD RATIO ─────────────────────────────────────────────────────────
	deaths := char.KD.TotalDeaths
	if deaths < 1 {
		deaths = 1
	}
	p.kd = math.Min(float64(char.KD.TotalKills)/float64(deaths)*10.0, 50.0)

	return p
}

// PowerScore computes an absolute power score for a character: one number for
// the behavior trees, targeting and the leaderboard. Its melee offense and
// avoidance are the fight read's shared terms (powerScoreParts).
func PowerScore(char characters.Character) float64 {
	return powerScoreParts(char).total()
}
```

The old melee block multiplied by `1 + mutations.GetDamageMultiplier` itself; `buildDamageParams` already applies it inside `meleeOffenseRaw`, so it is gone on purpose. Remove imports the compiler reports unused (`items`, `combatvocab` maybe; `configs` stays if `ChanceToTame` or others use it).

- [ ] **Step 4: Run the combat suite**

Run: `go build ./... && go test ./internal/combat/ ./internal/behaviortree/ ./internal/targeting/ ./modules/leaderboards/ -count=1`
Expected: all `ok`. A test pinning an exact old PowerScore value will fail; read it, and if it pins the number rather than a property, update it to the new value with a comment citing #301 and this task.

- [ ] **Step 5: Write the AFTER table and compare**

Run (Git Bash): `DOGMUD_POWERSCORE_TABLE="$TMP/powerscore-after.md" go test . -run TestPowerScoreTable -count=1 -v`
Then: `diff <(grep -o 'acts\|-' "$TMP/powerscore-before.md") <(grep -o 'acts\|-' "$TMP/powerscore-after.md")` is not precise enough; compare the two files row by row by eye or with `diff "$TMP/powerscore-before.md" "$TMP/powerscore-after.md"`, listing every cell whose `acts`/`-` mark changed.

- [ ] **Step 6: Record the decision**

Paste the AFTER table into the audit doc under "After". Under "Decision", write exactly one of:

- "No archetype decision flips for any mob against any fixture player. Thresholds unchanged." (if no mark changed), or
- a list of each flipped cell (mob, player, before ratio and mark, after ratio and mark).

If any mark flipped: **stop here and report the flipped cells to the owner** with the before and after ratios. Do not retune `lookout.yaml`, `thief.yaml` or `predator.yaml` without the owner's ruling; record the ruling in the audit doc when it comes.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/calculations.go internal/combat/powerscore_parts_test.go docs/superpowers/audits/2026-10-13-powerscore-before-after.md
git commit -m "refactor(combat): PowerScore on the fight read's shared terms (#301)"
```

---

### Task 9: Verdicts, paces, read quality and copy

**Files:**
- Create: `internal/actions/fight_read.go`
- Create: `internal/actions/fight_read_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/actions/fight_read_test.go`:

```go
package actions

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

var inf = math.Inf(1)

func TestPaceFor(t *testing.T) {
	bal := configs.GetBalanceConfig() // Go defaults: 2 / 6 / 20
	cases := []struct {
		rounds float64
		want   Pace
	}{{1, PaceMoments}, {2, PaceMoments}, {2.01, PaceShort}, {6, PaceShort}, {20, PaceLong}, {20.5, PaceAge}, {inf, PaceNever}}
	for _, c := range cases {
		if got := paceFor(c.rounds, bal); got != c.want {
			t.Errorf("paceFor(%v) = %v, want %v", c.rounds, got, c.want)
		}
	}
}

func TestClassifyFight(t *testing.T) {
	bal := configs.GetBalanceConfig()
	p := func(you, them, cost, dent float64) combat.FightPrediction {
		return combat.FightPrediction{TheirDamagePerRound: 1, RoundsToKillThem: you, RoundsToKillYou: them, Cost: cost, Dent: dent}
	}
	cases := []struct {
		name string
		in   combat.FightPrediction
		want Verdict
	}{
		{"no incoming damage", combat.FightPrediction{TheirDamagePerRound: 0, RoundsToKillThem: 3, RoundsToKillYou: inf}, VerdictUntouched},
		{"tiny cost", p(3, 60, 0.04, 20), VerdictUntouched},
		// The effigy: both sides an age; dent is tiny only because it is slow.
		{"effigy stalemate before hopeless", p(5000, 400, 12, 0.01), VerdictStalemate},
		{"megalomania", p(inf, 1, inf, 0.001), VerdictHopeless},
		{"scratches", p(3, 30, 0.1, 10), VerdictScratches},
		{"hurt", p(4, 10, 0.4, 2.5), VerdictHurt},
		{"barely", p(8, 9, 0.9, 1.1), VerdictBarely},
		{"likely lose", p(10, 5, 2, 0.5), VerdictLikelyLose},
		{"lose badly", p(12, 2, 6, 0.2), VerdictLoseBadly},
	}
	for _, c := range cases {
		if got := classifyFight(c.in, bal).Verdict; got != c.want {
			t.Errorf("%s: verdict %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMuddySlide(t *testing.T) {
	want := map[Verdict]Verdict{
		VerdictUntouched: VerdictUntouched, VerdictScratches: VerdictHurt, VerdictHurt: VerdictBarely,
		VerdictBarely: VerdictBarely, VerdictLikelyLose: VerdictBarely, VerdictLoseBadly: VerdictLikelyLose,
		VerdictHopeless: VerdictHopeless, VerdictStalemate: VerdictStalemate,
	}
	for in, out := range want {
		if got := muddySlide(in); got != out {
			t.Errorf("muddySlide(%v) = %v, want %v", in, got, out)
		}
	}
}

func TestReadQualityFor(t *testing.T) {
	bal := configs.GetBalanceConfig()
	sd := dice.StdDevFor(100)
	if got := readQualityFor(contest.Result{Success: false}, 100, bal); got != ReadMuddy {
		t.Errorf("lost roll: %v, want muddy", got)
	}
	if got := readQualityFor(contest.Result{Success: true, Margin: 0.5 * sd}, 100, bal); got != ReadFair {
		t.Errorf("narrow win: %v, want fair", got)
	}
	if got := readQualityFor(contest.Result{Success: true, Margin: 1.5 * sd}, 100, bal); got != ReadClear {
		t.Errorf("wide win: %v, want clear", got)
	}
}

// Player copy: every line a read can produce fits 80 columns with a long name,
// and carries no digits.
func TestFightReadCopy_FitsAndHasNoNumbers(t *testing.T) {
	name := "Captain of the Watch Halix" // 26 characters
	var lines []string
	for _, q := range []ReadQuality{ReadClear, ReadFair, ReadMuddy} {
		lines = append(lines, openerText(q, name))
	}
	for v := VerdictUntouched; v <= VerdictStalemate; v++ {
		lines = append(lines, verdictText[v])
	}
	for yp := PaceMoments; yp <= PaceNever; yp++ {
		for tp := PaceMoments; tp <= PaceNever; tp++ {
			lines = append(lines, paceText(FightRead{YourPace: yp, TheirPace: tp}))
		}
	}
	for _, l := range lines {
		if len(l) > 80 {
			t.Errorf("%d columns: %q", len(l), l)
		}
		if strings.ContainsAny(l, "0123456789") {
			t.Errorf("a number in player copy: %q", l)
		}
		if l == "" {
			t.Error("an empty line in the copy tables")
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/actions/ -run 'TestPaceFor|TestClassifyFight|TestMuddySlide|TestReadQualityFor|TestFightReadCopy' -count=1`
Expected: FAIL to compile.

- [ ] **Step 3: Implement**

`internal/actions/fight_read.go`:

```go
package actions

import (
	"fmt"
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// Verdict is who comes out ahead in a read fight, and how cleanly (#301).
// It never speaks of pace; Pace does.
type Verdict int

const (
	VerdictUntouched  Verdict = iota // they won't lay a finger on you
	VerdictScratches                 // a few scratches at most
	VerdictHurt                      // you'll win, but you'll feel it
	VerdictBarely                    // you'd win, barely, if at all
	VerdictLikelyLose                // you'd likely lose
	VerdictLoseBadly                 // you'd lose badly
	VerdictHopeless                  // zero chance
	VerdictStalemate                 // neither of you is ending this soon
)

// Lethal reports a verdict that predicts the reader loses.
func (v Verdict) Lethal() bool {
	return v == VerdictLikelyLose || v == VerdictLoseBadly || v == VerdictHopeless
}

// Danger orders verdicts from safest to deadliest; a stalemate sits with
// barely.
func (v Verdict) Danger() int {
	switch v {
	case VerdictUntouched:
		return 0
	case VerdictScratches:
		return 1
	case VerdictHurt:
		return 2
	case VerdictBarely, VerdictStalemate:
		return 3
	case VerdictLikelyLose:
		return 4
	case VerdictLoseBadly:
		return 5
	}
	return 6
}

// Pace is how long one side needs to bring the other down.
type Pace int

const (
	PaceMoments Pace = iota
	PaceShort
	PaceLong
	PaceAge
	PaceNever
)

// ReadQuality is how clearly a player read the fight: their consider roll.
// ReadNone is a read with no roll (a mob, the AI companion).
type ReadQuality int

const (
	ReadNone ReadQuality = iota
	ReadClear
	ReadFair
	ReadMuddy
)

// FightRead is a classified prediction: the verdict and both paces.
type FightRead struct {
	Verdict    Verdict
	YourPace   Pace // how long you need to bring them down
	TheirPace  Pace // how long they need to bring you down
	Prediction combat.FightPrediction
}

// ReadFight is you's read of them with no roll: the prediction, classified.
// Mobs, the AI companion and the pinned tests use it; a player's consider
// adds the roll on top.
func ReadFight(you, them *characters.Character) FightRead {
	return classifyFight(combat.PredictFight(you, them), configs.GetBalanceConfig())
}

func paceFor(rounds float64, bal configs.Balance) Pace {
	switch {
	case math.IsInf(rounds, 1):
		return PaceNever
	case rounds <= float64(bal.ConsiderPaceMoments):
		return PaceMoments
	case rounds <= float64(bal.ConsiderPaceShort):
		return PaceShort
	case rounds <= float64(bal.ConsiderPaceLong):
		return PaceLong
	}
	return PaceAge
}

// classifyFight applies the spec's band order: untouched first (a target that
// cannot hurt you is never hopeless), stalemate before hopeless (a slow,
// pillow-hitting target is not zero chance), then the cost ladder.
func classifyFight(p combat.FightPrediction, bal configs.Balance) FightRead {
	r := FightRead{
		YourPace:   paceFor(p.RoundsToKillThem, bal),
		TheirPace:  paceFor(p.RoundsToKillYou, bal),
		Prediction: p,
	}
	switch {
	case p.TheirDamagePerRound <= 0 || p.Cost < float64(bal.ConsiderBandUntouched):
		r.Verdict = VerdictUntouched
	case r.YourPace == PaceAge && r.TheirPace == PaceAge:
		r.Verdict = VerdictStalemate
	case p.Dent < float64(bal.ConsiderBandHopelessDent):
		r.Verdict = VerdictHopeless
	case p.Cost < float64(bal.ConsiderBandScratches):
		r.Verdict = VerdictScratches
	case p.Cost < float64(bal.ConsiderBandHurt):
		r.Verdict = VerdictHurt
	case p.Cost < float64(bal.ConsiderBandBarely):
		r.Verdict = VerdictBarely
	case p.Cost < float64(bal.ConsiderBandLethal):
		r.Verdict = VerdictLikelyLose
	default:
		r.Verdict = VerdictLoseBadly
	}
	return r
}

// muddySlide moves a muddy read's verdict one step toward barely. Untouched,
// barely, hopeless and stalemate never slide (owner ruling 2026-10-13: no
// extra slide on a critical failure either).
func muddySlide(v Verdict) Verdict {
	switch v {
	case VerdictScratches:
		return VerdictHurt
	case VerdictHurt, VerdictLikelyLose:
		return VerdictBarely
	case VerdictLoseBadly:
		return VerdictLikelyLose
	}
	return v
}

func readQualityFor(res contest.Result, readScore float64, bal configs.Balance) ReadQuality {
	if !res.Success {
		return ReadMuddy
	}
	if sd := dice.StdDevFor(readScore); sd > 0 && res.Margin/sd >= float64(bal.ConsiderClearReadZ) {
		return ReadClear
	}
	return ReadFair
}

// ── Copy (player-copy rules: no numbers, 80 columns, plain words) ──────────

var verdictText = map[Verdict]string{
	VerdictUntouched:  `They won't lay a finger on you.`,
	VerdictScratches:  `A few scratches at most.`,
	VerdictHurt:       `You'll win, but you'll feel it.`,
	VerdictBarely:     `You'd win, barely, if at all.`,
	VerdictLikelyLose: `You'd likely lose.`,
	VerdictLoseBadly:  `You'd lose badly.`,
	VerdictHopeless:   `What could you possibly be thinking? You have zero chance.`,
	VerdictStalemate:  `Neither of you is ending this soon.`,
}

var verdictColor = map[Verdict]string{
	VerdictUntouched: `blue-bold`, VerdictScratches: `green`, VerdictHurt: `green`,
	VerdictBarely: `yellow`, VerdictStalemate: `yellow`,
	VerdictLikelyLose: `red-bold`, VerdictLoseBadly: `red-bold`, VerdictHopeless: `red-bold`,
}

var yourPaceText = map[Pace]string{
	PaceMoments: `you'd beat them in moments`,
	PaceShort:   `you'd beat them in a short fight`,
	PaceLong:    `beating them would be a long fight`,
	PaceAge:     `beating them would take an age`,
	PaceNever:   `you could never beat them`,
}

var theirPaceText = map[Pace]string{
	PaceMoments: `they could beat you in moments`,
	PaceShort:   `they could beat you in a short fight`,
	PaceLong:    `they'd need a long fight to beat you`,
	PaceAge:     `beating you would take them an age`,
	PaceNever:   `they could never beat you`,
}

// openerText is the read's first line; the muddy opener is the hedge.
func openerText(q ReadQuality, targetName string) string {
	switch q {
	case ReadClear:
		return fmt.Sprintf(`You size up %s with a practised eye.`, targetName)
	case ReadMuddy:
		return fmt.Sprintf(`You study %s, unsure of what you see.`, targetName)
	}
	return fmt.Sprintf(`You take a fresh look at %s, weighing your chances.`, targetName)
}

// paceText joins the two paces into one sentence.
func paceText(r FightRead) string {
	s := yourPaceText[r.YourPace] + `; ` + theirPaceText[r.TheirPace] + `.`
	return strings.ToUpper(s[:1]) + s[1:]
}

// verdictLine is the verdict in its colour, for a player.
func verdictLine(r FightRead) string {
	return fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, verdictColor[r.Verdict], verdictText[r.Verdict])
}

// FightReadPlain is the read as untagged sentences, verdict then pace: the AI
// companion's words for it.
func FightReadPlain(r FightRead) string {
	return verdictText[r.Verdict] + ` ` + paceText(r)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/actions/ -run 'TestPaceFor|TestClassifyFight|TestMuddySlide|TestReadQualityFor|TestFightReadCopy' -count=1`
Expected: `ok`. If the copy test flags a line, shorten the copy (keep the meaning), never raise the limit.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/fight_read.go internal/actions/fight_read_test.go
git commit -m "feat(actions): verdicts, paces, read quality and copy for the fight read (#301)"
```

---

### Task 10: `consider` with the roll, the cooldown and training

**Files:**
- Modify: `internal/actions/consider.go` (whole file)
- Modify: `internal/actions/consider_test.go` (whole file)
- Modify: `internal/actions/consider_no_progression_test.go` (`TestConsider_AwardsNoProgression` only)

- [ ] **Step 1: Rewrite the tests first**

Replace `internal/actions/consider_test.go` with:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// fakeActor is a minimal Actor for unit tests. It records SendText lines and
// OnStatUse/OnSkillUse calls.
type fakeActor struct {
	awardRecorder // records Actor.AwardResolved calls
	char          *characters.Character
	name          string
	isPlayer      bool
	sent          []string
	statUses      map[string]int
}

// newFakeActor builds a validated character (the fight prediction reads its
// Position, pools and plan) with every stat's ValueAdj at statAdj and pools
// at healthMax. Setting ValueAdj after Validate skips the Training pipeline,
// as character_test.go does for combat math.
func newFakeActor(name string, statAdj, healthMax int, isPlayer bool) *fakeActor {
	c := characters.New()
	c.Name = name
	if err := c.Validate(); err != nil {
		panic(err)
	}
	c.Stats.Strength.ValueAdj = statAdj
	c.Stats.Dexterity.ValueAdj = statAdj
	c.Stats.Perception.ValueAdj = statAdj
	c.Stats.Vitality.ValueAdj = statAdj
	c.Stats.Willpower.ValueAdj = statAdj
	c.Stats.Charisma.ValueAdj = statAdj
	c.HealthMax.Value, c.Health = healthMax, healthMax
	c.StaminaMax.Value, c.Stamina = healthMax, healthMax
	c.ConvictionMax.Value, c.Conviction = healthMax/2, healthMax/2
	return &fakeActor{char: c, name: name, isPlayer: isPlayer, statUses: map[string]int{}}
}

func (a *fakeActor) GetCharacter() *characters.Character         { return a.char }
func (a *fakeActor) GetRoom() *rooms.Room                        { return nil }
func (a *fakeActor) SendText(_ messaging.Category, msg string)   { a.sent = append(a.sent, msg) }
func (a *fakeActor) GetName() string                             { return a.name }
func (a *fakeActor) IsPlayer() bool                              { return a.isPlayer }
func (a *fakeActor) GetUserId() int                              { return 0 }
func (a *fakeActor) GetMobInstanceId() int                       { return 0 }
func (a *fakeActor) AddCondition(conditionId int, source string) {}
func (a *fakeActor) OnSkillUse(skillName string) bool {
	a.statUses[skillName]++
	return false
}
func (a *fakeActor) OnStatUse(statName string) bool {
	a.statUses[statName]++
	return false
}

// fixRoll pins the read roll for one test.
func fixRoll(t *testing.T, res contest.Result) {
	t.Helper()
	old := considerContest
	considerContest = func(float64, []contest.Entry) contest.Result { return res }
	t.Cleanup(func() { considerContest = old })
}

func TestConsider_PlayerReadSendsThreeLinesAndTrainsSearch(t *testing.T) {
	fixRoll(t, contest.Result{Success: true, Margin: 1})
	self := newFakeActor("hero", 100, 100, true)
	target := newFakeActor("orc", 100, 100, false)

	r := Consider(self, target)
	if len(self.sent) != 3 {
		t.Fatalf("got %d lines, want opener, verdict and pace: %q", len(self.sent), self.sent)
	}
	if !strings.Contains(self.sent[0], "orc") {
		t.Errorf("opener should name the target: %q", self.sent[0])
	}
	if r.Quality != ReadFair {
		t.Errorf("quality %v, want fair for a narrow win", r.Quality)
	}
	if self.statUses["search"] != 1 || len(self.statUses) != 1 {
		t.Errorf("trained %v, want search once and nothing else", self.statUses)
	}
}

// The cooldown is the gate: a refused read neither rolls nor trains (#301).
func TestConsider_CooldownRefusalNeitherRollsNorTrains(t *testing.T) {
	rolled := false
	old := considerContest
	considerContest = func(float64, []contest.Entry) contest.Result { rolled = true; return contest.Result{} }
	t.Cleanup(func() { considerContest = old })

	self := newFakeActor("hero", 100, 100, true)
	if !ClaimSpecialMove(self.char) {
		t.Fatal("fixture could not claim the cooldown")
	}
	r := Consider(self, newFakeActor("orc", 100, 100, false))
	if !r.OnCooldown || rolled || len(self.statUses) != 0 {
		t.Fatalf("refusal: OnCooldown %v rolled %v trained %v", r.OnCooldown, rolled, self.statUses)
	}
	if len(self.sent) != 1 || !strings.Contains(self.sent[0], "moment to recover") {
		t.Fatalf("refusal text: %q", self.sent)
	}
}

func TestConsider_MuddyReadSlidesTheVerdict(t *testing.T) {
	fixRoll(t, contest.Result{Success: false})
	self := newFakeActor("hero", 100, 100, true)
	target := newFakeActor("orc", 100, 100, false)
	clean := ReadFight(self.char, target.char).Verdict

	r := Consider(self, target)
	if r.Quality != ReadMuddy || r.Read.Verdict != muddySlide(clean) {
		t.Fatalf("muddy read: quality %v verdict %v, want muddy %v", r.Quality, r.Read.Verdict, muddySlide(clean))
	}
}

// A mob's consider (and the AI companion's) has no roll, no cooldown, no
// training and no text.
func TestConsider_MobReadIsSilentAndFree(t *testing.T) {
	self := newFakeActor("goblin", 100, 100, false)
	r := Consider(self, newFakeActor("hero", 100, 100, true))
	if r.Quality != ReadNone || r.OnCooldown || len(self.statUses) != 0 || len(self.sent) != 0 {
		t.Fatalf("mob read: %+v trained %v sent %q", r, self.statUses, self.sent)
	}
	if !SpecialMoveReady(self.char) {
		t.Fatal("a mob's consider must not take the special-move cooldown")
	}
}
```

In `internal/actions/consider_no_progression_test.go`, replace `TestConsider_AwardsNoProgression` (keep `sourceDir`, `readSource` and the `look` test) with:

```go
// Consider trains, but only behind the shared special-move cooldown (#301).
//
// look and consider were once the only stat faucets with no cooldown and no
// gate: ~3,600 perception uses an hour against forage's 150 (U10b-0 Phase D).
// consider now competes with special moves and spells for the cooldown, which
// is the gate. This guard keeps the training call after the claim, keeps it on
// skill use only, and keeps stat and direct progression calls out.
func TestConsider_TrainsOnlyBehindTheCooldown(t *testing.T) {
	src := readSource(t, "consider.go")
	claim := strings.Index(src, "ClaimSpecialMove(")
	train := strings.Index(src, "OnSkillUse(")
	if claim < 0 || train < 0 || train < claim {
		t.Fatalf("consider.go must claim the special-move cooldown before it trains (claim at %d, train at %d)", claim, train)
	}
	for _, banned := range []string{"OnStatUse", "CheckStatProgression", "CheckSkillProgression"} {
		if strings.Contains(src, banned) {
			t.Errorf("consider.go calls %s; it trains only through OnSkillUse behind the cooldown", banned)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/actions/ -run 'TestConsider' -count=1`
Expected: FAIL to compile (`undefined: considerContest`, `r.Quality`, `r.OnCooldown`).

- [ ] **Step 3: Rewrite consider.go**

Replace `internal/actions/consider.go` with:

```go
package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// ConsiderResult is the outcome of a consider (#301).
type ConsiderResult struct {
	Read           FightRead
	Quality        ReadQuality // ReadNone for a mob or the companion, or a refused read
	OnCooldown     bool        // a player's read refused: the special-move cooldown is running
	TargetName     string
	TargetIsPlayer bool
}

// considerContest rolls a player's read. Tests pin it.
var considerContest = combat.RunContest

// considerCooldownText is the special-move refusal every special move sends.
const considerCooldownText = "You need a moment to recover before attempting another special move."

// Consider reads a fight from actor's side (#301): a verdict and two paces
// predicted from combat's own math (ReadFight).
//
// A player's consider is a skill read. It claims the shared special-move
// cooldown, so it competes with special moves and spells; then rolls
// Perception plus Search against the target's Charisma plus Rhetoric through
// combat.RunContest. The roll sets how clear the read is, a lost roll slides
// a middle verdict toward barely, and every roll trains Search (and through
// it Perception). The cooldown is the gate on that training: consider was an
// ungated perception faucet until U10b-0 Phase D removed its progression, and
// consider_no_progression_test.go keeps the training behind the claim.
//
// A mob's consider (and the AI companion's) is the bare prediction: no roll,
// no cooldown, no training, no text.
func Consider(actor Actor, target Actor) ConsiderResult {
	selfChar := actor.GetCharacter()
	targetChar := target.GetCharacter()
	result := ConsiderResult{TargetName: target.GetName(), TargetIsPlayer: target.IsPlayer()}

	if !actor.IsPlayer() {
		result.Read = ReadFight(selfChar, targetChar)
		return result
	}

	if !ClaimSpecialMove(selfChar) {
		result.OnCooldown = true
		actor.SendText(messaging.CategorySystem, considerCooldownText)
		return result
	}

	bal := configs.GetBalanceConfig()
	readScore := float64(selfChar.Stats.Perception.ValueAdj) +
		float64(selfChar.GetSkillLevel(skills.Search))*float64(bal.SkillWeight)
	targetScore := float64(targetChar.Stats.Charisma.ValueAdj) +
		float64(targetChar.GetSkillLevel(skills.Rhetoric))*float64(bal.SkillWeight)
	roll := considerContest(readScore, []contest.Entry{{Name: string(skills.Rhetoric), Score: targetScore}})
	result.Quality = readQualityFor(roll, readScore, bal)
	actor.OnSkillUse(string(skills.Search))

	result.Read = ReadFight(selfChar, targetChar)
	if result.Quality == ReadMuddy {
		result.Read.Verdict = muddySlide(result.Read.Verdict)
	}

	considerType := "mob"
	if result.TargetIsPlayer {
		considerType = "user"
	}
	name := fmt.Sprintf(`<ansi fg="%sname">%s</ansi>`, considerType, result.TargetName)
	actor.SendText(messaging.CategorySystem, openerText(result.Quality, name))
	actor.SendText(messaging.CategorySystem, verdictLine(result.Read))
	actor.SendText(messaging.CategorySystem, paceText(result.Read))
	return result
}
```

`internal/usercommands/consider.go` and `internal/mobcommands/consider.go` need no change: they call `actions.Consider(actor, target)` and ignore the result.

- [ ] **Step 4: Run the tests**

Run: `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ -count=1`
Expected: build fails only in `modules/aicompanion` (it still reads `.Ratio`; Task 11 fixes it). Run the three packages anyway: `ok`. If `go build ./...` blocks the test run, run `go vet ./internal/actions/` and the three `go test` commands.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/consider.go internal/actions/consider_test.go internal/actions/consider_no_progression_test.go
git commit -m "feat(consider): a skill read on the special-move cooldown (#301)"
```

(Task 11 lands before any push, so the broken companion build never leaves this branch.)

---

### Task 11: The AI companion reads the shared fight read

**Files:**
- Modify: `modules/aicompanion/actions.go:716-760`
- Modify: `modules/aicompanion/combat.go:204`, `:320-325`, `:909-925`
- Modify: `modules/aicompanion/aicompanion_test.go:504-506`

- [ ] **Step 1: Update the test first**

In `aicompanion_test.go`, replace

```go
	if considerWords(5) != `they pose no threat to you` || considerWords(0.7) != `they have the upper hand` {
		t.Fatal("consider words drifted from the engine's bands")
	}
```

with

```go
	// The companion speaks the engine's own fight read (#301): no private copy.
	if got := actions.FightReadPlain(actions.FightRead{Verdict: actions.VerdictUntouched}); !strings.HasPrefix(got, `They won't lay a finger on you.`) {
		t.Fatalf("companion consider words: %q", got)
	}
```

and add `"github.com/GoMudEngine/GoMud/internal/actions"` to the test's imports if absent.

- [ ] **Step 2: Replace the companion's copy and ratio reads**

In `actions.go`, delete `considerWords` (the whole function and its comment) and in `consider` replace

```go
	result := fmt.Sprintf(`You sized up %s: %s.`, t.Name, considerWords(res.Ratio))
```

with

```go
	result := fmt.Sprintf(`You sized up %s. %s`, t.Name, actions.FightReadPlain(res.Read))
```

In `combat.go:204`, replace

```go
		odds := considerWords(actions.Consider(self, actions.NewMobActorInRoom(m, room)).Ratio)
```

with

```go
		odds := actions.FightReadPlain(actions.Consider(self, actions.NewMobActorInRoom(m, room)).Read)
```

Replace `worstOdds` (`:909-925`, with its comment) with:

```go
// worstRead is the most dangerous fight among the enemies here, the same read
// a player gets from consider, without the roll. ok is false with no enemy in
// the room.
func worstRead(room *rooms.Room, mob *mobs.Mob, f *fightState) (actions.FightRead, bool) {
	self := actions.NewMobActorInRoom(mob, room)
	var worst actions.FightRead
	found := false
	for id := range f.Enemies {
		e := mobs.GetInstance(id)
		if e == nil || e.Character.RoomId != room.RoomId {
			continue
		}
		r := actions.Consider(self, actions.NewMobActorInRoom(e, room)).Read
		if !found || r.Verdict.Danger() > worst.Verdict.Danger() {
			worst, found = r, true
		}
	}
	return worst, found
}
```

and the flee check at `:322` becomes:

```go
		if r, ok := worstRead(room, mob, f); ok && r.Verdict.Lethal() && f.FleeAt == `about_to_die` {
```

(The old rule fled readier at a PowerScore ratio below 0.4; the new one at a verdict of likely lose or worse, which is what that ratio was trying to say.)

- [ ] **Step 3: Build and test**

Run: `go build ./... && go test ./modules/aicompanion/ ./internal/actions/ -count=1`
Expected: build clean; `ok`.

- [ ] **Step 4: Commit**

```bash
git add modules/aicompanion/actions.go modules/aicompanion/combat.go modules/aicompanion/aicompanion_test.go
git commit -m "feat(aicompanion): read fights through the shared fight read (#301)"
```

---

### Task 12: Drift guards 1, 3 and 4

**Files:**
- Create: `fight_read_drift_guard_test.go` (repo root, guard 1)
- Create: `internal/combat/fight_expect_stat_test.go` (guard 3)
- Create: `consider_pinned_reads_test.go` (repo root, guard 4)

- [ ] **Step 1: Guard 1, no second copy**

`fight_read_drift_guard_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoFile reads a file relative to this test's directory (the repo root),
// independent of the test binary's CWD.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(here), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// Drift guard 1 (#301): PowerScore, consider, the AI companion and live combat
// share one set of helpers. A second copy of a swing, hit, damage or defence
// formula in any of them fails here.
func TestFightReadUsesTheSharedHelpers(t *testing.T) {
	type rule struct {
		file          string
		must, mustNot []string
	}
	rules := []rule{
		{"internal/combat/calculations.go",
			[]string{"meleeOffenseRaw(", "meleeAvoidance("},
			[]string{"swings :=", "addWeaponDPS", "GetDefenseScore(", "CalcRawDamage(char.Stats.Strength"}},
		{"internal/combat/fight_expect.go",
			[]string{"buildAttackPlan(", "buildDamageParams(", "calcAttackScore(", "applyPositionHitModifiers(",
				"calcCritThreshold(", "defenceEntryScore(", "SpellRawDamage(", "SpellMitigation("},
			[]string{"swings :=", "GetDefenseScoreFor("}},
		{"internal/combat/combat_helpers.go",
			[]string{"defenceEntryScore(targetChar, defenseType, isThirdParty, ctx)"}, nil},
		{"internal/hooks/combat_shared_helpers.go",
			[]string{"combat.SpellRawDamage(", "combat.SpellMitigation("},
			[]string{"spellData.CasterStatValue("}},
		{"internal/actions/consider.go",
			[]string{"ReadFight("},
			[]string{"PowerScore", "predictionFor"}},
		{"modules/aicompanion/actions.go", []string{"FightReadPlain("}, []string{"considerWords", ".Ratio"}},
		{"modules/aicompanion/combat.go", []string{"worstRead("}, []string{"considerWords", ".Ratio", "worstOdds"}},
	}
	for _, r := range rules {
		src := repoFile(t, r.file)
		for _, s := range r.must {
			if !strings.Contains(src, s) {
				t.Errorf("%s no longer calls %s: the fight read and combat must share it", r.file, s)
			}
		}
		for _, s := range r.mustNot {
			if strings.Contains(src, s) {
				t.Errorf("%s contains %q: a second copy of a shared formula (#301)", r.file, s)
			}
		}
	}
}
```

Prove it can fail: temporarily add the comment `// swings := 1` to `calculations.go`, run `go test . -run TestFightReadUsesTheSharedHelpers -count=1`, see it FAIL, remove the comment, see it PASS.

- [ ] **Step 2: Guard 3, prediction matches combat**

`internal/combat/fight_expect_stat_test.go`:

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/statmods"
)

// meleePredictionTolerance bounds how far the fight read's melee damage per
// round may sit from what live combat measures. The prediction skips the
// contest floor, gap compression, fumbles and partial deflection by design,
// so it is loose; a combat change that bypasses the shared helpers moves the
// measurement far past it. Do not widen it to make a red run pass: report the
// deltas to the owner instead (the spec's bands assume this roughness).
const meleePredictionTolerance = 0.30

const meleePredictionRounds = 40000

func statCombatant(t *testing.T, name string, stat, rank int) *characters.Character {
	t.Helper()
	c := parityCombatant(t, name)
	c.Stats.Strength.Base, c.Stats.Dexterity.Base, c.Stats.Vitality.Base = stat, stat, stat
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	c.Stats.Vitality.Recalculate()
	c.Skills[string(skills.UnarmedCombat)] = rank
	return c
}

// Drift guard 3 (#301): the predicted melee damage per round lands within
// tolerance of the real seam, resolveCombatRound, over many rounds.
func TestMeleePredictionMatchesCombat(t *testing.T) {
	if testing.Short() {
		t.Skip("statistical guard; skipped in -short")
	}
	pinParityBalance(t)

	cases := []struct {
		name                       string
		atkStat, atkRank           int
		defStat, defRank, defMitPc int
	}{
		{"even pair", 100, 30, 100, 30, 0},
		{"outclassed attacker", 80, 10, 140, 40, 0},
		{"armoured defender", 120, 30, 100, 20, 40},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			attacker := statCombatant(t, "attacker", c.atkStat, c.atkRank)
			attacker.SetAggro(0, 1, characters.DefaultAttack)
			defender := statCombatant(t, "defender", c.defStat, c.defRank)
			if c.defMitPc > 0 {
				t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
					9401: {ConditionId: 9401, Name: "test mitigation", TriggerCount: 1,
						StatMods: statmods.StatMods{"physical_mitigation": c.defMitPc}},
				}))
				defender.Conditions.List = append(defender.Conditions.List,
					&conditions.Condition{ConditionId: 9401, TriggersLeft: 1000000000})
				defender.Conditions.Validate(true)
			}

			want := meleeDamagePerRound(attacker, defender, User)
			ctx := neutralCtx()
			total := 0.0
			for i := 0; i < meleePredictionRounds; i++ {
				attacker.Stamina, attacker.Health = attacker.StaminaMax.Value, attacker.HealthMax.Value
				defender.Stamina, defender.Health = defender.StaminaMax.Value, defender.HealthMax.Value
				if !attacker.Position.IsStanding() {
					attacker.Position = position.NewMachine()
				}
				if !defender.Position.IsStanding() {
					defender.Position = position.NewMachine()
				}
				result, _ := resolveCombatRound(attacker, defender, User, User, ctx)
				for _, ev := range result.SwingEvents {
					total += float64(ev.Damage)
				}
			}
			got := total / meleePredictionRounds
			delta := (want - got) / got
			t.Logf("%s: predicted %.3f measured %.3f per round (delta %+.1f%%)", c.name, want, got, delta*100)
			if math.Abs(delta) > meleePredictionTolerance {
				t.Errorf("prediction %.3f is %+.1f%% from measured %.3f (tolerance %.0f%%)",
					want, delta*100, got, meleePredictionTolerance*100)
			}
		})
	}
}
```

Run: `go test ./internal/combat/ -run TestMeleePredictionMatchesCombat -count=1 -v`
Expected: PASS, with three logged deltas. **Copy the three logged lines into the PR description.** If any case exceeds 30%, do not widen the tolerance: stop and report the deltas to the owner.

Prove it can fail: temporarily multiply the return of `meleeDamagePerRound` by 2, run, see FAIL, revert, see PASS.

- [ ] **Step 3: Guard 4, pinned reads**

`consider_pinned_reads_test.go`:

```go
package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Drift guard 4 (#301): reads the owner named, pinned.

// The Straw Effigy has vast health and pillow hits (species 19, zero base
// damage). It must never read as a quick kill either way: your pace is an
// age, and the verdict is "won't lay a finger" or a stalemate, never "zero
// chance" and never a loss.
func TestPinnedRead_StrawEffigy(t *testing.T) {
	loadFightWorld(t)
	effigy := mobs.NewMobByIdFresh(mobs.MobId(9614), 1)
	if effigy == nil {
		t.Fatal("Straw Effigy (9614) failed to spawn")
	}
	novice := fixtureAdventurer(t, "Novice", 100, 10)

	r := actions.ReadFight(novice, &effigy.Character)
	if r.Verdict != actions.VerdictUntouched && r.Verdict != actions.VerdictStalemate {
		t.Errorf("effigy verdict %v, want untouched or stalemate (prediction %+v)", r.Verdict, r.Prediction)
	}
	if r.YourPace != actions.PaceAge {
		t.Errorf("effigy: your pace %v, want an age", r.YourPace)
	}
}

// Deviation (Task 12, 2026-10-13): the shipped fixture zeroes the six base stats before Validate, since characters.New rolls stats and Validate hydrates only a zero base.
// fixtureAscended is the owner's "zero chance" yardstick: species 99
// (Ascended, base 1000 in every stat, hydrated by Validate), every skill at
// 100, wielding the Blackrazor (item 40183).
func fixtureAscended(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Name = "Ascended"
	c.RoomId = 1
	c.SpeciesId = 99
	if err := c.Validate(); err != nil {
		t.Fatalf("validate Ascended: %v", err)
	}
	if c.Stats.Strength.ValueAdj < 1000 {
		t.Fatalf("Ascended strength %d: species stats did not hydrate", c.Stats.Strength.ValueAdj)
	}
	for _, s := range skills.GetAllSkillNames() {
		c.Skills[string(s)] = 100
	}
	c.Equipment.Weapon = items.New(40183)
	if c.Equipment.Weapon.ItemId != 40183 {
		t.Fatal("the Blackrazor (40183) did not load")
	}
	c.Health = c.HealthMax.Value
	c.Stamina = c.StaminaMax.Value
	c.Conviction = c.ConvictionMax.Value
	return c
}

// An Ascended with every skill at 100 and the Blackrazor against a novice:
// zero chance for the novice, and moments for the Ascended.
func TestPinnedRead_Hopeless(t *testing.T) {
	loadFightWorld(t)
	novice := fixtureAdventurer(t, "Novice", 60, 1)
	pinnacle := fixtureAscended(t)

	if r := actions.ReadFight(novice, pinnacle); r.Verdict != actions.VerdictHopeless {
		t.Errorf("novice vs pinnacle: %v, want hopeless (prediction %+v)", r.Verdict, r.Prediction)
	}
	if r := actions.ReadFight(pinnacle, novice); r.Verdict != actions.VerdictUntouched || r.YourPace != actions.PaceMoments {
		t.Errorf("pinnacle vs novice: %v / %v, want untouched in moments (prediction %+v)", r.Verdict, r.YourPace, r.Prediction)
	}
}
```

Run: `go test . -run 'TestPinnedRead' -count=1 -v`
Expected: PASS. If the effigy reads anything else, the prediction or a band limit is wrong for the very case that motivated the spec: stop and report the logged prediction to the owner; do not loosen the assertion.

- [ ] **Step 4: Commit**

```bash
git add fight_read_drift_guard_test.go internal/combat/fight_expect_stat_test.go consider_pinned_reads_test.go
git commit -m "test: drift guards for the shared fight-read helpers (#301)"
```

---

### Task 13: Help, patch notes and package docs

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/consider.template` (whole file)
- Modify: `_datafiles/world/default/templates/help/consider.template` (whole file)
- Modify: `docs/PATCH_NOTES.md` (top `## Unreleased` section)
- Modify: `internal/combat/context.md`, `internal/actions/context.md`, `internal/characters/context.md`, `internal/conditions/context.md`, `internal/configs/context.md`, `modules/aicompanion/context.md`
- Modify: `docs/README.md`

- [ ] **Step 1: Help (both worlds, identical)**

Replace each `consider.template` with:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">consider</ansi>

The <ansi fg="command">consider</ansi> command sizes up a target: what a fight
would cost you, and how long each of you would need to beat the other.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">consider goblin</ansi>

It reads both of you as you stand right now. Your spells, potions, wards and
enchantments count, and so do theirs.

Sizing someone up is a skill. Your <ansi fg="stat">Perception</ansi> and
<ansi fg="skill">search</ansi> skill work against their <ansi fg="stat">Charisma</ansi>
and <ansi fg="skill">rhetoric</ansi>, so a good read is clear and a poor one
is muddy and may be off. Results vary, and each read trains search.

Considering uses your special-move recovery, like <ansi fg="command">bash</ansi>
or a spell, so you can't consider and use a special move in the same breath.

<ansi fg="yellow">What you might hear:</ansi>

  <ansi fg="blue-bold">They won't lay a finger on you.</ansi>
  <ansi fg="green">A few scratches at most.</ansi>
  <ansi fg="green">You'll win, but you'll feel it.</ansi>
  <ansi fg="yellow">You'd win, barely, if at all.</ansi>
  <ansi fg="yellow">Neither of you is ending this soon.</ansi>
  <ansi fg="red-bold">You'd likely lose.</ansi>
  <ansi fg="red-bold">You'd lose badly.</ansi>
  <ansi fg="red-bold">What could you possibly be thinking? You have zero chance.</ansi>

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help cooldowns</ansi>, <ansi fg="command">help search</ansi>
```

Every line must stay within 80 visible columns once tags are stripped; check the two longest by eye.

- [ ] **Step 2: Patch note**

Add to the top `## Unreleased` section of `docs/PATCH_NOTES.md`:

```markdown
- `consider` now tells you what a fight would really cost you, from "they
  won't lay a finger on you" to "you have zero chance", and how long each of
  you would need to beat the other. It weighs both of you as you stand right
  now, buffs, wards and enchantments included. Sizing someone up is now a
  skill: Perception and search against their Charisma and rhetoric, so a
  poor read is muddier. It uses your special-move recovery, and it trains
  search.
```

- [ ] **Step 3: Package docs**

For each `context.md` below, add a short section naming the new or changed symbols (verify each with `codegraph_search` or grep before writing it, per CLAUDE.md):

- `internal/combat/context.md`: the fight read layers (`fight_expect.go`: `expectedSwings`, `bestMeleeDefenceScore`, `meleeDamagePerRound`, `spellDamagePerRound`, `meleeOffenseRaw`, `meleeAvoidance`, `neutralTarget`; `fight_predict.go`: `FightPrediction`, `PredictFight`), `defenceEntryScore`, `SpellRawDamage`, `SpellMitigation`, `powerScoreParts`, and the rule that none of them may carry their own formula (drift guards 1 to 3).
- `internal/actions/context.md`: `Consider` (roll, cooldown, training; `considerContest` test seam), `fight_read.go` (`Verdict`, `Pace`, `ReadQuality`, `FightRead`, `ReadFight`, `FightReadPlain`, the band order).
- `internal/characters/context.md`: `CombatHealthRegenPerRound`.
- `internal/conditions/context.md`: `PoolTickPerRound`.
- `internal/configs/context.md`: the ten `Consider*` knobs and `validateConsider`.
- `modules/aicompanion/context.md`: consider words come from `actions.FightReadPlain`; flee readiness from `worstRead` and `Verdict.Lethal`.

Then run `python tools/context_md_audit.py` and fix any phantom symbol it reports.

- [ ] **Step 4: README rows**

Add rows to `docs/README.md` beside the spec's row for this plan and for `docs/superpowers/audits/2026-10-13-powerscore-before-after.md`, one sentence each.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/consider.template _datafiles/world/default/templates/help/consider.template docs/PATCH_NOTES.md internal/combat/context.md internal/actions/context.md internal/characters/context.md internal/conditions/context.md internal/configs/context.md modules/aicompanion/context.md docs/README.md
git commit -m "docs: consider help, patch note and package docs for the fight read (#301)"
```

---

### Task 14: Pre-push gate, boot check and playtest

Follow `dogmud-shipping` exactly; the summary below does not replace it.

- [ ] **Step 1: Local gates**

```bash
gofmt -l internal/ modules/            # want no output
go build ./...
go test ./... -count=1 2>&1 | tail -40 # every package ok
~/go/bin/golangci-lint run --new-from-merge-base=origin/master   # want 0 issues
```

- [ ] **Step 2: Boot check** in an isolated detached worktree per `dogmud-shipping` (fixed-path `boot-check.exe`, scratch `CONFIG_PATH` override moving every port, exit 124 is success, zero panic lines, one `Server Ready`). Remove the worktree afterwards.

- [ ] **Step 3: Playtest gate**

Invoke the `dogmud-playtesting` skill and run an ephemeral local session against this branch (`--checkout`). Goals:

1. A fresh character considers the Straw Effigy in the antechamber: expect a stalemate or "won't lay a finger", with "beating them would take an age".
2. The same character considers a rat or other starter creature: expect "won't lay a finger" or "a few scratches", paced in moments or a short fight.
3. Consider a mid-level mob bare, then again after a buff (a potion or ward): the read should improve or hold, never worsen.
4. Consider twice in a row: the second is refused with "You need a moment to recover..." and does not train.
5. Consider repeatedly across cooldowns: the opener varies (clear, fair, muddy), and a muddy read's verdict is at most one step off.
6. Check `help consider` renders within 80 columns.

File any finding as a GitHub issue (`--repo pruuk/DOGMud`); fix findings in this branch before the PR when they are this work's defects.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feat/301-consider-fight-read
gh pr create --repo pruuk/DOGMud --base master --head feat/301-consider-fight-read \
  --title "feat(consider): read fights from combat's own math (#301)" --body-file <prepared body>
```

The body states: what changed; the PowerScore before/after decision (link the audit doc); the three guard-3 deltas; the playtest result; "Fixes #301". End with the Claude Code attribution line. Read the URL `gh` prints and confirm it says `pruuk/DOGMud`.

- [ ] **Step 5: Merge when CI is green** with `gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch`, then remove the worktree `C:/tmp/dogmud-301` and the spec worktree `C:/tmp/dogmud-301-spec`.

---

## Success criteria

The work is done when every one of these holds, each checked by the named evidence:

1. **#301 is fixed.** A player who would win in one round never reads "even": the old ratio bands are gone (`predictionFor` deleted, guard 1), and a strong-versus-weak read is `VerdictUntouched` in moments (`TestPinnedRead_Hopeless`).
2. **The effigy reads true.** The Straw Effigy reads "won't lay a finger" or a stalemate with your pace "an age", never hopeless or a loss (`TestPinnedRead_StrawEffigy`, playtest goal 1).
3. **Zero chance exists.** A novice against an Ascended with every skill at 100 and the Blackrazor reads hopeless (`TestPinnedRead_Hopeless`).
4. **Verdict and pace are separate.** Every read sends an opener, a verdict that never mentions pace, and one pace line (`TestConsider_PlayerReadSendsThreeLinesAndTrainsSearch`, `fight_read.go` copy tables).
5. **Buffs count.** Lifesteal lowers incoming damage (`TestPredictFight_LifestealReducesIncoming`); mitigation from wards and potions feeds through `buildDamageParams` (`TestMeleePredictionMatchesCombat`, "armoured defender"); playtest goal 3 shows a buffed read is no worse.
6. **The roll works as ruled.** Perception plus Search against Charisma plus Rhetoric through `combat.RunContest`; clear, fair and muddy tiers (`TestReadQualityFor`); a muddy read slides only bands 2, 3, 5 and 6, one step toward barely, with no extra slide on a critical failure (`TestMuddySlide`, `TestConsider_MuddyReadSlidesTheVerdict`).
7. **The cooldown is the gate.** `consider` claims the special-move cooldown; a refused read neither rolls nor trains; training happens only after the claim (`TestConsider_CooldownRefusalNeitherRollsNorTrains`, `TestConsider_TrainsOnlyBehindTheCooldown`, playtest goal 4).
8. **Mobs and the companion read silently.** No roll, cooldown, training or text for a mob (`TestConsider_MobReadIsSilentAndFree`); the companion speaks `FightReadPlain` and flees readier on a lethal verdict.
9. **One set of helpers.** The live defence loop, `rollSpellDamage`, PowerScore, consider and the companion call the shared functions, and a second copy fails the build's tests (guard 1, with its failing probe proven in Task 12; `TestDefenceEntryScore_IsWhatTheLiveContestRolls`; `TestPowerScoreParts_AreTheSharedTerms`).
10. **The prediction tracks combat.** Predicted melee damage per round is within 30% of measured for all three guard-3 matchups, with the deltas recorded in the PR (`TestMeleePredictionMatchesCombat`, failing probe proven).
11. **PowerScore's consumers are checked.** The before/after table is committed with a decision; any flipped lookout, thief or predator decision was ruled on by the owner before merge (audit doc).
12. **Copy follows the rules.** Every read line and both help files fit 80 columns with no numbers (`TestFightReadCopy_FitsAndHasNoNumbers`, Task 13 Step 1).
13. **Shipped clean.** gofmt empty, `go build ./...`, `go test ./...` green, lint 0 issues, boot check `Server Ready`, playtest goals 1 to 6 pass or have filed issues, CI green on the PR, merged with `--merge`.
