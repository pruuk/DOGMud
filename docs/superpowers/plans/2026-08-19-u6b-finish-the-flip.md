# U6b Finish-the-Flip Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Tasks:** 21.

**Goal:** Every uncertain combat outcome resolves through one contest shape — one
contest that always runs, crit from that contest's margin against one bar, damage
from that margin, a counter tier on every channel gated on reach — and every
legacy per-channel parameter is deleted.

**Architecture:** The channel-defence seam (`internal/combat/defence_multiplier.go`)
grows an explicit attack side (`AttackSide`) and exposes attacker crit/fumble, so
spell, taunt, ranged and the 14 special moves all resolve through it — inheriting
its defence costing (U7/U8), skill-strip, progression (U9) and bonus tier for
free. The two-contest hit gates in spell and taunt are deleted. `DefenceSetFor`
becomes the single, equipment-gated defence-set source (melee's private builder
is deleted). Melee's dynamic crit bar is hoisted to all channels. The unowned
family (flee, grapple, drift, submission, throw, steal, sneak) converts to the
same weights and crit semantics. Three guards enumerate `RunContest` call sites
so no site can go unowned again.

**Tech Stack:** Go 1.x, standard `testing`. Narration YAML under
`_datafiles/world/dogmud/defense-messages/`. Balance values in
`_datafiles/config.yaml` (skip-worktree — see Standing Rules).

**Spec:** [`2026-08-19-u6b-finish-the-flip-design.md`](../specs/2026-08-19-u6b-finish-the-flip-design.md)
**Modelling gate (DISCHARGED, all five decisions made):** [`2026-08-19-u6b-modelling.md`](../specs/2026-08-19-u6b-modelling.md)

**Branch:** create `feature/u6b-finish-the-flip` off `feature/u9-progression-layer`
(U6b's code depends on U9's seam, and U9's PR #52 is unmerged). Cleanest is to
merge #52 first and branch off master; the owner decides. Do NOT commit U6b code
onto the U9 branch.

---

## Assumptions taken on the spec's open questions (owner: review these first)

| # | Assumption | Source |
|---|---|---|
| 1 | Fizzle becomes a partial-damage defence outcome; only the word is a copy question | Gate decision §5.2 (forced by the numbers) |
| 2 | `shoot` keeps Perception as its attack stat; only the weight unifies | Spec §8.2 |
| 3 | Melee's dynamic crit bar (Accuracy 1.5 / Blink 2.5 / skill-shift, floor 1.5) is HOISTED to all channels, not deleted | Spec §8.3 recommendation |
| 4 | `throw` gets the ranged defence set (dodge, block) and margin-scaled damage | Spec §4.5; modelled in appendix C |
| 5 | Counters are free (no cost), like riposte today; non-melee channels get a counter-SWING on any defensive crit; melee keeps its per-defence trio (riposte/auto-trip/auto-bash) | Spec §4.3 "riposte's mechanism" |
| 6 | Defy's crit counter-taunts INSTEAD of counter-swinging | Owner decision 2026-08-19 |
| 7 | The fumble-before-success ordering (fumble aborts even winning attacks, capping hit at 87.5%−fumble) is KEPT and documented, now uniformly | Modelling §6.3 "document or change, deliberately" |
| 8 | The crit-damage rank input is the RAW skill rank everywhere (taunt's ×5-weighted input was the outlier and is corrected — a named nerf to taunt crit damage) | Modelling §6.2; `CritOrMitigatedDamage` signature |
| 9 | Mob skill: nothing changes (accept the repriced gold dial) | Gate decision §5.1 |

## Standing rules (from the arc — violations are defects)

1. **No balance number inside `internal/`.** Every tuning value is a config knob.
   New knobs this plan adds: `CounterDamagePercent` (0.5), `GrappleAggressorDriftBonus`
   (value computed in Task 14), `GrappleProneAttackerMod` (0.3), `GrappleProneDefenderMod`
   (0.5). Deleted knobs: `SpellAttackSkillFactor`, `RangedShieldDefenseBonus`,
   `SubSkillWeight`, `StealSkillMultiplier`.
2. **`_datafiles/config.yaml` carries skip-worktree.** Build every config commit
   from the `git show HEAD:` blob via `git hash-object -w` + `git update-index
   --cacheinfo`, then RESTORE the skip-worktree flag (`git update-index
   --skip-worktree _datafiles/config.yaml`) — the cacheinfo write clears it.
   Never commit the disk copy; it holds dev overrides.
3. **Delete as you migrate.** A site's old resolution path goes in the same task
   that moves it.
4. **`export GOTMPDIR=C:/gotmp`** before any `go test`. The suite is clean, so a
   failure is real. **NEVER start, stop, or kill any server** — the user runs the
   local server; boot testing uses the isolated worktree recipe in Task 19.
5. `gofmt -l internal/ modules/` must print nothing before any commit. Do not
   use `parser.ParseDir` in new tests (deprecated, SA1019, the lint gate is
   only-new-issues — use `os.ReadDir` + `parser.ParseFile` as
   `internal/progression/seam_guard_test.go` does).
6. **This slice changes behaviour on purpose.** Every task lists its named
   behaviour changes; each goes in the PR individually. Existing tests that pin
   old behaviour get updated AND named. A test breaking over a MESSAGE you did
   not intend to change means you broke messaging — stop.
7. Line numbers in this plan drift. **Verify with the given grep before every
   edit.** Prefer codegraph MCP tools for symbol lookup.
8. Do not edit anything under `.worktrees/`.

## File structure (what changes where)

| File | Responsibility after U6b |
|---|---|
| `internal/combat/crit_bar.go` (new) | `CritBarFor` / `DefenseCritBar` — THE crit thresholds, hoisted from melee |
| `internal/combat/defence_sets.go` | `DefenceEntriesFor` — THE equipment-gated defence-entry builder, all channels including melee |
| `internal/combat/defence_multiplier.go` | `AttackSide`, `ResolveChannelAttack` — THE channel resolution seam; attacker crit/fumble exposed |
| `internal/combat/counter.go` (new) | The counter tier: reach gate, recursion bound, counter-swing |
| `internal/combat/skill_moves.go` | `ExecuteSkillMove` routed through the seam; gains Crit/Fumble |
| `internal/hooks/spell_resolution.go` | Hit gates deleted; five `!isCrit` skips deleted |
| `internal/actions/combat_taunt.go` | Gate deleted; defy contest always runs |
| `internal/actions/combat_fire.go` | `rangedDefenseScore` deleted; seam-routed |
| `internal/combat/flee.go`, `grapple.go`, `submission.go`, `hooks/Position_GrappleTick.go`, `usercommands/throw.go`, `actions/steal.go`, `actions/skill_helpers.go` | The family, converted |
| `internal/combat/contest_site_guard_test.go` (new) | The three §9 guards |
| `_datafiles/world/dogmud/defense-messages/` | Triads for newly-defendable attacks + counters |

---

## Task 1: Hoist the crit bar

**Files:**
- Create: `internal/combat/crit_bar.go`
- Modify: `internal/combat/combat_helpers.go` (~line 553 `calcCritThreshold`, ~line 966 the defensive 2.0)
- Test: `internal/combat/crit_bar_test.go` (create)

Melee's bar is dynamic (`calcCritThreshold`: base 2.0, Accuracy→1.5, Blink→2.5,
skill-difference shift, floor 1.5); every other channel uses the const
`ContestCritThreshold = 2.0`. Assumption 3: hoist the dynamic bar.

- [ ] **Step 1: Write the failing tests**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
)

func barChar(t *testing.T, combatSkill int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Skills["weapon-combat"] = combatSkill
	return c
}

// The hoisted bar must be BIT-IDENTICAL to melee's private calcCritThreshold
// for every input — this task is a pure extraction for melee.
func TestCritBarFor_MatchesLegacyMeleeBar(t *testing.T) {
	cases := []struct {
		name          string
		atkBuff       buffs.Flag
		defBuff       buffs.Flag
		atkSkill, defSkill int
	}{
		{"parity", "", "", 30, 30},
		{"accuracy", buffs.Accuracy, "", 30, 30},
		{"blink", "", buffs.Blink, 30, 30},
		{"skill advantage pins at floor", "", "", 69, 1},
		{"skill disadvantage raises", "", "", 1, 69},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			atk, def := barChar(t, tc.atkSkill), barChar(t, tc.defSkill)
			if tc.atkBuff != "" {
				// use the package's existing test idiom for setting a buff flag;
				// check crit_floor_test.go / combat_helpers_test.go for the helper
			}
			got := CritBarFor(atk, def)
			want := calcCritThreshold(atk, def)
			if got != want {
				t.Errorf("CritBarFor=%v calcCritThreshold=%v", got, want)
			}
		})
	}
}

// nil actors fall back to the constant bar: the channel seam resolves contests
// where one side may be a static difficulty with no character.
func TestCritBarFor_NilFallsBackToConstant(t *testing.T) {
	if got := CritBarFor(nil, nil); got != ContestCritThreshold {
		t.Errorf("CritBarFor(nil,nil)=%v want %v", got, ContestCritThreshold)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/combat/ -run TestCritBarFor -v`
Expected: FAIL — `undefined: CritBarFor`.

- [ ] **Step 3: Implement**

`internal/combat/crit_bar.go`:

```go
package combat

import "github.com/GoMudEngine/GoMud/internal/characters"

// CritBarFor is THE attacker-side crit threshold, for every channel.
//
// It is melee's formerly-private dynamic bar (base 2.0, Accuracy 1.5, Blink
// 2.5, shifted by combat-skill difference with a 1.5 floor), hoisted in U6b so
// that a buff which changes crit odds changes them on every attack the buyer
// makes — before this, Accuracy buffed sword crits and not spell, bash or shot
// crits, and Blink protected against swords only.
//
// Nil on either side falls back to the constant: the seam also resolves
// contests against static difficulties that have no character behind them.
func CritBarFor(attacker, defender *characters.Character) float64 {
	if attacker == nil || defender == nil {
		return ContestCritThreshold
	}
	return calcCritThreshold(attacker, defender)
}

// DefenseCritBar is the defender-side threshold. Melee shipped this as a
// hardcoded 2.0 separate from its own dynamic attack bar; U6b keeps it a single
// constant for every channel, ON PURPOSE — a defensive crit unlocks the counter
// tier, and shifting that bar by the attacker's buffs would let an attacker's
// Accuracy make them EASIER to counter, which reads backwards. Documented here
// so nobody "unifies" it into CritBarFor without deciding that.
func DefenseCritBar() float64 { return ContestCritThreshold }
```

Then in `combat_helpers.go`: replace the two direct uses (find with
`grep -n "calcCritThreshold\|:= 2.0" internal/combat/combat_helpers.go`) so the
melee swing loop reads `CritBarFor(sourceChar, targetChar)` and the defensive
check reads `DefenseCritBar()`. `calcCritThreshold` itself stays (it is the
implementation `CritBarFor` wraps).

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/combat/ -run "TestCritBarFor|TestCalcCritThreshold" -v`
then the full package: `go test ./internal/combat/ -count=1`
Expected: PASS, zero existing-test changes — melee behaviour is bit-identical.

- [ ] **Step 5: Commit**

```bash
git add internal/combat/
git commit -m "refactor(u6b): hoist melee's dynamic crit bar into CritBarFor

Pure extraction: melee routes through it bit-identically. Later tasks
give every other channel the same bar, so Accuracy/Blink/skill-shift
stop being melee-only. DefenseCritBar stays a constant deliberately."
```

---

## Task 2: One defence-set source, equipment-gated

**Files:**
- Modify: `internal/combat/defence_sets.go`
- Modify: `internal/combat/combat_helpers.go` (melee's `runBestOfAllDefense` set building)
- Modify: `internal/characters/combat.go` (`GetDefenseSequence` — deleted after migration)
- Test: `internal/combat/defence_entries_test.go` (create)

Today melee builds its set from `characters.GetDefenseSequence`
(equipment-gated: parry needs a weapon, TWICE when dual-wielding, block needs a
shield; `filterDefensesForThirdParty` for grapples) and the channel path builds
from `DefenceSetFor` with NO equipment gate — a shieldless bare-handed defender
can roll block against a bolt or a physical spell.

- [ ] **Step 1: Write the failing tests**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// Build defenders with the package's existing equipment fixture idiom — find it
// first: grep -n "Equipment\." internal/combat/*_test.go | head. Wear needs a
// species fixture (HandsRequired has no nil guard).

func TestDefenceEntriesFor_EquipmentGate(t *testing.T) {
	bare := newDefenceTestCharacter(t)          // no weapon, no shield
	armed := newArmedDefenceTestCharacter(t)    // weapon, no shield
	shielded := newShieldedDefenceTestCharacter(t)

	cases := []struct {
		name    string
		channel AttackChannel
		def     *characters.Character
		want    []string
	}{
		{"bare vs melee: dodge only", ChannelMelee, bare, []string{characters.DefenseDodge}},
		{"armed vs melee: dodge+parry", ChannelMelee, armed, []string{characters.DefenseDodge, characters.DefenseParry}},
		{"shielded vs ranged: dodge+block", ChannelRanged, shielded, []string{characters.DefenseDodge, characters.DefenseBlock}},
		// THE new gate: no shield means no block, on ANY channel. Today the
		// channel path hands this defender a block roll against a bolt.
		{"bare vs ranged: dodge only", ChannelRanged, bare, []string{characters.DefenseDodge}},
		{"bare vs spell-physical: dodge only", ChannelSpellPhysical, bare, []string{characters.DefenseDodge}},
		{"mental: quell regardless of equipment", ChannelSpellMental, bare, []string{characters.DefenseQuell}},
		{"social: defy regardless of equipment", ChannelSocial, bare, []string{characters.DefenseDefy}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := namesOf(DefenceEntriesFor(tc.channel, tc.def, DefenceEntryOpts{}))
			assertSameSet(t, got, tc.want)
		})
	}
}

// Dual-wield double parry survives the migration: two parry entries.
func TestDefenceEntriesFor_DualWieldDoubleParry(t *testing.T) {
	dw := newDualWieldDefenceTestCharacter(t)
	got := namesOf(DefenceEntriesFor(ChannelMelee, dw, DefenceEntryOpts{}))
	if countOf(got, characters.DefenseParry) != 2 {
		t.Errorf("dual-wield parry entries = %d, want 2 (set: %v)", countOf(got, characters.DefenseParry), got)
	}
}

// Third-party grapple filtering survives via opts.
func TestDefenceEntriesFor_ThirdPartyGrappleFilter(t *testing.T) {
	// mirror filterDefensesForThirdParty's current contract — read it first
	// (grep -n "filterDefensesForThirdParty" internal/combat/combat_helpers.go)
	// and assert the same defences are removed via
	// DefenceEntryOpts{ThirdPartyVsGrappler: true}.
}
```

- [ ] **Step 2: Run to verify failure** — `undefined: DefenceEntriesFor`.

- [ ] **Step 3: Implement**

In `defence_sets.go`:

```go
// DefenceEntryOpts carries the situational filters the entry builder applies.
type DefenceEntryOpts struct {
	// ThirdPartyVsGrappler mirrors the melee-only filterDefensesForThirdParty
	// behaviour (a bystander swinging into a grapple faces a reduced set).
	ThirdPartyVsGrappler bool
}

// DefenceEntriesFor is THE defence-set builder for every channel, melee
// included. It merges DefenceSetFor's channel table with the equipment gate
// that previously lived only in characters.GetDefenseSequence:
//
//   - parry requires a wielded weapon, and appears TWICE when dual-wielding
//     (two blades, two chances — preserved from the melee builder verbatim)
//   - block requires a shield (BestBlockRating() > 0); without one, "block" is
//     not in the set on ANY channel. Before U6b the channel path had no gate
//     and a bare-handed defender could roll block against a bolt.
//   - dodge, quell and defy are always available on their channels.
//
// Entries come back scored via GetDefenseScoreFor x defenceEffectiveness, with
// the prone defence penalties applied — which before U6b hit melee only, so a
// prone defender dodged a bolt at full score while dodging a sword at penalty.
func DefenceEntriesFor(channel AttackChannel, defender *characters.Character, opts DefenceEntryOpts) []contest.Entry {
	// Build from DefenceSetFor(channel); for each defence apply:
	//   1. the equipment gate above (read the current GetDefenseSequence for
	//      the exact wielding checks and copy them here — then delete it)
	//   2. score := defender.GetDefenseScoreFor(d, includeSkill) — leave the
	//      includeSkill/cost quoting to the caller seam exactly as
	//      resolveChannelDefenceWithRunner does today
	//   3. prone penalties: if defender is prone, multiply by the existing
	//      ProneDodgePenalty / ProneParryPenalty / ProneBlockPenalty knobs
	//      (grep -rn "ProneDodgePenalty" internal/ for the exact read idiom)
	//   4. opts filters.
}
```

The exact wielding checks are copied out of `characters.GetDefenseSequence`
(read it in full first — `grep -n "func (c \*Character) GetDefenseSequence" -A40
internal/characters/combat.go`). Then:

- `runBestOfAllDefense` (melee) consumes `DefenceEntriesFor(ChannelMelee, ...)`.
- `resolveChannelDefenceWithRunner` consumes it for the channels (Task 3 wires
  the quoting around it).
- `GetDefenseSequence` is DELETED once nothing calls it (compiler is the sweep).
- Update `defence_sets.go`'s "MELEE DOES NOT" doc comment — it becomes false in
  this task, and a comment describing the removed model is worse than none.

- [ ] **Step 4: Run the tests + full combat/hooks suites.**

Expected: the new tests pass. Existing melee tests must pass UNCHANGED except
any that pinned a shieldless block on the channel path or an un-penalised prone
channel dodge — those pinned the defects. Name them.

**Named behaviour changes:** (1) shieldless/bare-handed defenders lose block
(and parry) on ranged/spell-physical channels; (2) prone defence penalties now
apply on every channel.

- [ ] **Step 5: Commit** (message names both changes and the deletion of
`GetDefenseSequence`).

---

## Task 3: The attack side — `AttackSide` + `ResolveChannelAttack`

**Files:**
- Modify: `internal/combat/defence_multiplier.go`
- Test: `internal/combat/resolve_channel_attack_test.go` (create)

The seam centerpiece. `ChannelAttackScore` has exactly ONE production caller
(`resolveChannelDefenceWithRunner:240`), so the attack side becomes explicit in
one move.

- [ ] **Step 1: Write the failing tests**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

func side(stat, rank int) AttackSide {
	return AttackSide{
		Stat: stat, StatName: "willpower",
		Skill: skills.Spellcasting, SkillRank: rank, Mult: 1.0,
	}
}

// Score = (Stat + Rank x SkillWeight) x Mult. SkillWeight comes from config;
// inject it via the balance-config test idiom the package already uses
// (grep -n "SkillWeight" internal/combat/*_test.go | head).
func TestAttackSide_Score(t *testing.T) {
	s := side(148, 52)
	if got, want := s.score(), 148.0+52*5; got != want {
		t.Errorf("score=%v want %v", got, want)
	}
	s.Mult = 0.75
	if got, want := s.score(), (148.0+52*5)*0.75; got != want {
		t.Errorf("mult score=%v want %v", got, want)
	}
}

// Attacker crit comes from THE contest's margin against CritBarFor, and a
// FLOORED outcome can never be a crit — the missing guard modelling found on
// the old spell/taunt call sites.
func TestResolveChannelAttack_FlooredNeverCrits(t *testing.T) {
	atk, def := newDefenceTestCharacter(t), newDefenceTestCharacter(t)
	runner := func(_ float64, entries []contest.Entry) contest.Result {
		return contest.Result{
			Contested: true, Winner: entries[0].Name,
			Floored: true, Success: true, Margin: 1, // floor-promoted "win"
		}
	}
	out := resolveChannelAttackWithRunner(ChannelSpellMental, side(148, 52), atk, def, runner)
	if out.AttackerCrit {
		t.Error("a floor-promoted win was promoted again to a crit")
	}
}

// Fumble aborts even a winning attack — Assumption 7, kept and documented.
// Fumble is self-relative: AttackRoll.ZScore <= -DefenseCritBar().
func TestResolveChannelAttack_FumblePreemptsSuccess(t *testing.T) {
	// runner returns Success=true but an attack roll with ZScore -2.5
	// -> out.AttackerFumble true, out.Defended semantics unchanged from the
	// runner, and the caller is expected to abort on Fumble first.
}

// The bonus-tier progression events must name the skill and stat the CALLER
// passed, not a per-channel hardcode — this deletes channelAttackSkillAndStat's
// drift risk that U9 had to comment around.
func TestResolveChannelAttack_ProgressionNamesTheCallersSkill(t *testing.T) {
	// Force a defensive crit via the runner; assert via
	// characters.ClaimedBonusThisRound that the attacker-side observed event
	// used the AttackSide's skill, with a side naming manifestation.
}
```

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement**

In `defence_multiplier.go`:

```go
// AttackSide is the attacker's half of a channel contest, made explicit.
//
// Before U6b the seam derived the attack score internally (ChannelAttackScore,
// hardcoded Willpower+Spellcasting / Charisma+Rhetoric), which meant a spell's
// primarystat (U9) could not reach the hit contest and the progression naming
// had to mirror the hardcode. Callers now say exactly what attacks:
//
//   Stat      fully-modified stat VALUE feeding the score
//   StatName  which stat that is — progression events carry it
//   Skill     governing skill — progression, cost and crit-rank input carry it
//   SkillRank RAW rank. It multiplies by SkillWeight for the score, and feeds
//             CritDamageMultiplier UNWEIGHTED (Assumption 8 — taunt used to
//             pass the weighted value, a x15.75-vs-x4.6 outlier, corrected).
//   Mult      situational multiplier on the whole score (1.0 default; taunt's
//             conviction-depletion factor and Task 17's shared modifiers land
//             here)
type AttackSide struct {
	Stat      int
	StatName  string
	Skill     skills.SkillTag
	SkillRank int
	Mult      float64
}

func (s AttackSide) score() float64 {
	m := s.Mult
	if m == 0 {
		m = 1.0
	}
	return (float64(s.Stat) + float64(s.SkillRank)*float64(configs.GetBalanceConfig().SkillWeight)) * m
}
```

Extend `ChannelDefenceResult` with:

```go
	// U6b: the attacker's half of the same contest. Crit is margin-derived
	// against CritBarFor and NEVER set on a Floored outcome; Fumble is
	// self-relative (AttackRoll.ZScore <= -DefenseCritBar()) and callers must
	// resolve it BEFORE success — a fumbled attack aborts even when the roll
	// won (Assumption 7, uniform across channels; it caps hit at the ceiling
	// minus the fumble rate, which is the pre-U6b spell/taunt behaviour made
	// universal and documented).
	AttackerCrit   bool
	AttackerFumble bool
```

Rename/extend the internal `resolveChannelDefenceWithRunner` into
`resolveChannelAttackWithRunner(channel, side AttackSide, attacker, defender,
runner)`:

1. `atkScore := side.score()` (replacing the `ChannelAttackScore` call).
2. Defence entries come from `DefenceEntriesFor(channel, defender, opts)`
   (Task 2), with the existing quote/affordability/strip logic wrapped around
   each entry exactly as today.
3. After the contest: `out.AttackerCrit = !res.Floored && AttackContestCrit(res.Margin, res.AttackRoll)`
   **with the bar**: `AttackContestCrit` uses the const — add a bar-parameterised
   variant `AttackContestCritAt(margin float64, roll dice.RollResult, bar float64) bool`
   in `crit_floor.go` (the existing function becomes a call with
   `ContestCritThreshold`) and use `CritBarFor(attacker, defender)`.
   `out.AttackerFumble = res.AttackRoll.ZScore <= -DefenseCritBar()`.
4. The U9 bonus tier (`awardChannelDefenceBonus`) now takes its attacker skill
   and stat FROM `side` — `channelAttackSkillAndStat` is deleted; its channel
   switch has no reason to exist once the caller states the names.
5. Public wrappers:

```go
// ResolveChannelAttack is THE channel resolution entry point.
func ResolveChannelAttack(channel AttackChannel, side AttackSide, attacker, defender *characters.Character) ChannelDefenceResult {
	return resolveChannelAttackWithRunner(channel, side, attacker, defender, RunContest)
}

// ResolveChannelDefence remains for the taunt/spell callers until Tasks 4-5
// migrate them, building the legacy default side. DELETED in Task 5 once the
// last caller moves; ChannelAttackScore goes with it.
```

- [ ] **Step 4: Run the tests + `go test ./internal/combat/... ./internal/hooks/... -count=1`.**

Expected: PASS; taunt/spell behaviour unchanged this task (they still enter via
the legacy wrapper).

- [ ] **Step 5: Commit.**

---

## Task 4: Collapse the spell channel

**Files:**
- Modify: `internal/hooks/spell_resolution.go` (both `resolveAgainstPlayer` and `resolveAgainstMob`, all five `!isCrit` skips)
- Modify: `internal/characters/cast_helpers.go` (`CalcSpellAttack` — deleted)
- Modify: `internal/configs/config.balance.go` + `config.balance.spells.go` (`SpellAttackSkillFactor` — deleted)
- Config: `_datafiles/config.yaml` (delete the knob line, HEAD-blob method)
- Test: `internal/hooks/spell_collapse_test.go` (create)

**Verify first — the five skips and both gates:**

```bash
grep -n "if !isCrit" internal/hooks/spell_resolution.go        # expect 5 hits
grep -n "runPlayerSpellContest\|runMobSpellContest\|spellDefenseValue\|CalcSpellAttack" internal/hooks/spell_resolution.go internal/characters/cast_helpers.go
```

- [ ] **Step 1: Write the failing tests**

```go
package hooks

import "testing"

// ONE contest. The old shape ran a hit gate (attacker skill x15 vs the
// defender's RAW Willpower, skill x0) and only consulted quell on non-crit
// hits. After the collapse, quell IS the contest.
func TestSpellResolution_OneContest_QuellAlwaysConsulted(t *testing.T) {
	// Drive resolveAgainstPlayer via the package's existing spell fixtures
	// (grep -rn "resolveAgainstPlayer\|castManifestationSpellForTest" internal/hooks/*_test.go)
	// with a deterministic runner seam if one exists; otherwise assert
	// structurally: after this task, grep-level truth is enforced by the
	// Task 18 guard, and this test asserts the OBSERVABLE: a defender with
	// enormous spellcasting and average Wil is hit far less than one with
	// the same Wil and no skill (impossible under the old gate, which read
	// only raw Wil).
}

// A spell crit must be beatable by quell: force the contest to a defensive
// win via high defender skill and assert no crit lands (the old code decided
// isCrit BEFORE any defence was contested).
func TestSpellCrit_FacesQuell(t *testing.T) { /* same fixture approach */ }

// Fizzle is now a partial-damage defence outcome (Assumption 1): a defended,
// non-crit cast deals reduced-but-nonzero damage per defenceDamageMultiplier.
func TestSpellDefendedCast_DealsPartialDamage(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Run to verify failure.**

- [ ] **Step 3: Implement the collapse, player path**

In `resolveAgainstPlayer` (verify location: `grep -n "func resolveAgainstPlayer"`):

```go
	castSkill := skills.Spellcasting
	if spellData.HasSchool(spells.SchoolManifestation) {
		castSkill = skills.Manifestation
	}
	side := combat.AttackSide{
		Stat:      spellData.CasterStatValue(user.Character.Stats), // U9 primarystat — the hit contest finally honours it
		StatName:  spellData.PrimaryStat,
		Skill:     castSkill,
		SkillRank: user.Character.GetSkillLevel(castSkill),
		Mult:      1.0,
	}
	out := combat.ResolveChannelAttack(spellAttackChannel(spellData), side, user.Character, target.Character)

	if out.AttackerFumble {
		// existing backfire block, verbatim — fumble still aborts first
	}
	if out.DefensiveCrit {
		// full negation; counter tier arrives in Task 10
	}
	// Damage: dmg = calcSpellDamageForCharacter(...) x out.DamageMultiplier —
	// the SAME multiplier semantics ExecuteSkillMove documents (1.0 attack win,
	// 0.0 defensive crit, 0.0-0.5 rolled defensive win, 0.5 floored save).
	// isCrit := out.AttackerCrit — from the ONE contest.
```

Deletions in the same step: `runPlayerSpellContest`, `runMobSpellContest`,
`spellDefenseValue`, ALL FIVE `if !isCrit` skips (each becomes "the contest
already ran"), and `characters.CalcSpellAttack` once nothing calls it. The mob
path (`resolveAgainstMob` and the mob-caster variants) gets the identical shape
with `mob.Character`.

**Copy decision:** the "fizzles" strings become the channel defence narration
(`sendSpellChannelDefenceMessages` already exists and renders quell triads).
Where a defended cast previously printed "fizzles", the quell/dodge/block triad
now speaks. Keep the fumble/backfire copy untouched.

- [ ] **Step 4: Delete the knob**

Remove `SpellAttackSkillFactor` from `config.balance.go`, its default from
`config.balance.spells.go`, and the `config.yaml` line via the HEAD-blob method
(Standing Rule 2). `grep -rn "SpellAttackSkillFactor" internal/ _datafiles/`
must return nothing afterwards.

- [ ] **Step 5: Run everything**

`go test ./internal/hooks/... ./internal/combat/... ./internal/characters/... -count=1`

Existing tests pinning the gate (fizzle rates, x15 scores, crit-skips-quell)
were pinning the defect — update and NAME each. U9's regression tests
(`TestSpellCast_TracksItsStatOnce`, the seam guard) must stay green.

**Named behaviour changes:** the modelling doc's numbers, quoted in the PR —
PvE ≈0.99x; caster-vs-caster 85.5%/84%-crit → 49.7%/2.5%; royalty repriced
(~500g buys today's 300g threat); fizzle becomes partial damage.

- [ ] **Step 6: Commit.**

---

## Task 5: Collapse taunt

**Files:**
- Modify: `internal/actions/combat_taunt.go` (gate at ~:164, crit at ~:225, skip at ~:246 — verify with `grep -n "runTauntContest\|if !isCrit" internal/actions/combat_taunt.go`)
- Test: `internal/actions/taunt_collapse_test.go` (create)

- [ ] **Step 1: Failing tests** — mirror Task 4's shape: one contest; the
defender's `Wil + rhetoric×5` contested ONCE (assert a fixed-score defender's
hit rate matches the single-contest closed form, not the squared double-contest
form); crit-damage rank input is the RAW rhetoric rank (Assumption 8).

- [ ] **Step 2: Verify failure.**

- [ ] **Step 3: Implement**

```go
	side := combat.AttackSide{
		Stat:      char.Stats.Charisma.ValueAdj,
		StatName:  "charisma",
		Skill:     skills.Rhetoric,
		SkillRank: char.GetSkillLevel(skills.Rhetoric),
		Mult:      convMult, // the conviction-depletion multiplier the OLD GATE
		                     // applied and the old defy leg omitted — the
		                     // surviving score keeps it (spec 4.1)
	}
	out := combat.ResolveChannelAttack(combat.ChannelSocial, side, char, target.Char)
```

Delete `runTauntContest` and the `if !isCrit` skip. Crit damage: pass
`side.SkillRank` (raw) to `CritOrMitigatedDamage` — **named nerf**: Meirok's
taunt crit multiplier drops ×15.75 → ×4.6.

This removes the LAST caller of the legacy `ResolveChannelDefence` wrapper
(Task 4 removed the spell ones): delete the wrapper and `ChannelAttackScore`.
`grep -rn "ResolveChannelDefence\b\|ChannelAttackScore" internal/` → only
`ResolveChannelAttack` remains.

- [ ] **Step 4: Suites green; taunt hit/crit rates unchanged per modelling
(bit-identical gate maths) — a taunt accuracy test that MOVES means the
implementation diverged from the modelled collapse; stop and reconcile.**

**Named behaviour changes:** defender no longer contested twice (E[mult] 0.338
→ 0.658 at parity — pure attacker buff); taunt crit damage rank input
corrected.

- [ ] **Step 5: Commit.**

---

## Task 6: `ExecuteSkillMove` onto the seam (+ bash, kick, trip)

**Files:**
- Modify: `internal/combat/skill_moves.go`
- Modify: `internal/actions/combat_bash.go`, `combat_kick.go`, `combat_trip.go`
- Test: `internal/combat/skill_moves_seam_test.go` (create)

- [ ] **Step 1: Failing tests**

```go
// SkillMoveResult gains Crit/Fumble; the contest routes through the seam so
// the defence is a SET, costed, strippable and progressed.
func TestExecuteSkillMove_RoutesThroughSeam(t *testing.T) {
	// bash a defender; assert (a) the defender's winning defence was CHARGED
	// (CostCommitResult non-NoCharge via the result), (b) defence progression
	// fired once (ClaimedBonusThisRound / use-count deltas), (c) a shieldless
	// defender never reports DefenceType "block".
}

// Crit exists now, from the margin, against CritBarFor, mitigation-bypassing
// via CritOrMitigatedDamage with the RAW rank.
func TestExecuteSkillMove_CritTier(t *testing.T) { /* forceable via runner seam */ }

// Counters must not recurse: a move executed AS a counter reports
// DefensiveCrit but triggers no further counter (consumed by Task 10; the
// flag exists from this task).
func TestExecuteSkillMove_IsCounterSuppressesCounterTier(t *testing.T) {}
```

- [ ] **Step 2: Verify failure.**

- [ ] **Step 3: Implement**

`SkillMoveParams` changes:

```go
type SkillMoveParams struct {
	Attacker, Defender *characters.Character
	Channel   AttackChannel   // NEW: ChannelMelee for the physical moves,
	                          // ChannelRanged for fire — decides the defence set
	Attack    AttackSide      // REPLACES AttackStat/AttackSkill/DefenseStat/
	                          // DefenseSkill: callers pass the raw rank; the
	                          // seam applies SkillWeight (x1 -> x5 flip lives here)
	IsCounter bool            // NEW: set when this move IS a counter; the
	                          // counter tier never fires from it (Task 10)
	// DamagePercent, KnockdownChance, SkillRank->Attack.SkillRank, DamageStat,
	// MitigationMultiplier, KnockdownToSupine etc. — keep, renaming SkillRank
	// away (Attack.SkillRank is the single rank input, damage AND crit)
}

type SkillMoveResult struct {
	Hit, Crit, Fumble bool   // Crit/Fumble NEW
	Damage            int
	StatusApplied     bool
	KnockedDown       bool
	TargetMaxHP       int
	Defence           ChannelDefenceResult // NEW: exposes DefenceType/DefensiveCrit
	                                       // for narration and the counter tier
}
```

Body: replace the direct `RunContest` + `defenceDamageMultiplier` with
`out := ResolveChannelAttack(p.Channel, p.Attack, p.Attacker, p.Defender)`;
`result.Hit = !out.Defended || out.DamageMultiplier > 0` per the existing
partial-damage doc; `result.Crit = out.AttackerCrit`; `result.Fumble =
out.AttackerFumble`; damage through `CritOrMitigatedDamage(rawDmg,
p.Attack.SkillRank, out.AttackerCrit, mitig, cap)` then `× out.DamageMultiplier`
on non-crits (a crit bypasses mitigation AND the defence multiplier is
irrelevant because a crit means the attack won the contest decisively).
StatusApplied stays binary on Hit, unchanged.

Convert the three callers in this task (bash shown; kick/trip identical shape):

```go
	result := combat.ExecuteSkillMove(combat.SkillMoveParams{
		Attacker: char, Defender: target.Char,
		Channel: combat.ChannelMelee,
		Attack: combat.AttackSide{
			Stat: char.Stats.Strength.ValueAdj, StatName: "strength",
			Skill: skills.WeaponCombat, SkillRank: char.GetSkillLevel(skills.WeaponCombat),
			Mult: 1.0,
		},
		DamagePercent: float64(cfg.BashDamagePercent),
		// ... unchanged fields
	})
```

- [ ] **Step 4: Suites. The two U9 melee regression tests stay green (autoattack
is untouched).** Compile errors in the 11 unconverted callers are EXPECTED at
this point mid-task — Task 6 must convert bash/kick/trip and leave the build
green by converting ALL callers mechanically if the signature change breaks
them; if that makes the task too large, keep a temporary variadic shim ONLY
until Task 7, marked `// DELETE(u6b-task7)`.

**Named behaviour changes:** ×1→×5 both sides on these three moves; their
defences now cost/strip/progress; crit tier exists (modelling: 2.9–5.0× vs
mobs, 0.60–0.77× vs skilled players).

- [ ] **Step 5: Commit.**

---

## Task 7: The remaining special-move callers

**Files:**
- Modify: `internal/actions/combat_gore.go`, `combat_hamstring.go`, `combat_maul.go`, `combat_pounce.go`, `combat_rake.go`, `combat_drain.go`, `combat_throttle.go`
- Modify: `internal/hooks/combat_shared_helpers.go` (riposte-trip and auto-bash callers — set `IsCounter: true`)
- Test: extend `internal/combat/skill_moves_seam_test.go`

Mechanical sweep of the same conversion. Find every caller first:
`grep -rn "ExecuteSkillMove(" --include=*.go internal/ | grep -v _test` — the
plan expects 14 total; report the real count. Beast moves pass
`skills.UnarmedCombat` and their existing stats. The two counter-effect callers
(dodge-crit auto-trip, block-crit auto-bash in `combat_shared_helpers.go`) set
`IsCounter: true`. Delete the Task 6 shim if one was used. Full suites green.
Commit.

---

## Task 8: Ranged onto the seam

**Files:**
- Modify: `internal/actions/combat_fire.go` (`rangedDefenseScore` deleted; `ExecuteFire` passes Channel + AttackSide)
- Modify: `internal/configs/` + config.yaml (`RangedShieldDefenseBonus` deleted, HEAD-blob method)
- Test: `internal/actions/combat_fire_seam_test.go` (create)

`ExecuteFire` currently folds the defender into `DefenseSkill:
int(rangedDefenseScore(defChar)), DefenseStat: 0`. It becomes:

```go
	Channel: combat.ChannelRanged,
	Attack: combat.AttackSide{
		Stat: attacker.Stats.Perception.ValueAdj, StatName: "perception", // Assumption 2
		Skill: skills.RangedCombat, SkillRank: attacker.GetSkillLevel(skills.RangedCombat),
		Mult: 1.0,
	},
```

Delete `rangedDefenseScore` and the knob. Tests: shielded defender gets a real
block entry (worth −41…−46% expected damage per modelling, vs −16…−27% from the
old flat 15); shieldless gets dodge only; a shot can now CRIT.

**Named behaviour changes:** ranged crit tier exists; shield reworked; shieldless
defenders take +20–34% more. Cross-room shots: resolution identical — the
counter exemption is Task 10's.

Commit.

---

## Task 9: Narration for newly-defendable attacks

**Files:**
- Create/extend triads under `_datafiles/world/dogmud/defense-messages/`
- Modify: the bash/kick/trip/beast/fire callers to render via `RenderChannelDefenceMessages` with the attack's name
- Test: a loader test asserting every newly-defendable attack name renders a non-empty triad for each defence in its channel's set

`RenderChannelDefenceMessages(out, identities, attack)` already exists and
takes the attack name. Check the data shape first
(`ls _datafiles/world/dogmud/defense-messages/` and read one file — quell.yaml
landed in U8 with weak/normal/heavy bands, 5 variants per band). Every
newly-defendable attack (16) must render: defender line, attacker line, room
line, per applicable defence, per band. **Player-copy rules: 80-char wrap, no
em dashes, no raw numbers, ESL-clear.** This is content — Task 21's adversarial
playtest gate covers it; do not skip variants ("shipping the mechanic without
the text repeats the gap U8 closed for quell and defy").

Commit.

---

## Task 10: The counter tier

**Files:**
- Create: `internal/combat/counter.go`
- Modify: `internal/hooks/spell_resolution.go`, `internal/actions/combat_taunt.go`, `internal/combat/skill_moves.go` callers, `internal/actions/combat_fire.go` (wire the tier at each channel's defensive-crit exit)
- Modify: `internal/configs/` + config.yaml (`CounterDamagePercent: 0.5`)
- Test: `internal/combat/counter_test.go` (create)

- [ ] **Step 1: Failing tests**

```go
// Reach gate: same room -> counter; the cross-room shot -> none (the ONE
// coherent exception; a wielding-dependent ranged counter was considered and
// declined by the owner).
func TestCounter_ReachGate(t *testing.T) {}

// Defy counter-taunts INSTEAD of counter-swinging (Assumption 6).
func TestCounter_DefyCounterTaunts(t *testing.T) {}

// Counters never earn counters.
func TestCounter_NoRecursion(t *testing.T) {}

// Melee's trio is untouched: parry->riposte, dodge->auto-trip, block->auto-bash
// still fire from the melee swing path and ONLY there.
func TestCounter_MeleeTrioUnchanged(t *testing.T) {}
```

- [ ] **Step 2: Verify failure.**

- [ ] **Step 3: Implement**

`internal/combat/counter.go`:

```go
// ExecuteCounter fires the counter tier for a defensive crit on a non-melee
// channel: one free counter-swing at CounterDamagePercent of weapon damage
// (riposte's mechanism, extracted from the parry-crit block in
// internal/hooks/combat_shared_helpers.go — read it and move the damage maths
// here; the hooks block then calls this too so the maths exists once).
//
// Rules, all owner decisions 2026-08-19:
//   - reach-gated: attacker and defender must share a room. The cross-room
//     shot is the one uncounterable attack, as a property of the weapon.
//   - defy crits COUNTER-TAUNT instead (the defender's taunt resolution runs
//     with IsCounter semantics), replacing the swing.
//   - a counter never earns a counter: everything this function triggers
//     carries IsCounter, and ExecuteCounter is never invoked for a result
//     produced under IsCounter.
//   - free: no cost, like riposte today.
//   - do NOT frame this as interrupting the attack — the attack has already
//     resolved. A defensive crit is a decisive defence that leaves an
//     opening; the counter is what you do with the opening.
func ExecuteCounter(defender, attacker *characters.Character, channel AttackChannel, sameRoom bool) CounterResult
```

Wire at each channel's defensive-crit exit (spell, taunt, specials, same-room
fire). Extract riposte's `0.5` literal to the `CounterDamagePercent` knob
(config declaration + validation `< 0` reset + config.yaml via HEAD blob) and
point the melee riposte block at the same knob — melee behaviour unchanged at
the shipped 0.5.

- [ ] **Step 4: Suites green. Named change:** counter tier on four channels
(modelling: 32–42 free damage/round down-tier — accepted for playtest; kiting
worth 36–100% of a shot vs 1.5–2× targets — accepted with cost known).

- [ ] **Step 5: Commit.**

---

## Task 11: Counter narration

**Files:** `_datafiles/world/dogmud/defense-messages/` counter triads per channel; loader test.

Each channel's counter must read channel-correct (the owner's hard requirement:
"as long as the messaging to the combatants and observers makes sense"): a
counter after a quell crit reads as putting a working down and stepping in; the
counter-taunt reads as turning the taunt back; never a generic riposte string
pasted under a spell. Same copy rules as Task 9. Commit.

---

## Task 12: Flee to uniform weights

**Files:** `internal/combat/flee.go` (~:83, ~:106 — verify `grep -n "\*25\|x 25\|25)" internal/combat/flee.go`); test extend `internal/combat/flee_test.go`.

Replace both hardcoded `×25` with `× SkillWeight` (config read). Modelling:
floor/ceiling-pinned near no-op (only novice-vs-trash moves, −8.9pp) — a flee
test that moves MORE than that is a wiring error. Delete the literals; the
Task 18 literal-guard pins their absence. Commit.

---

## Task 13: Grapple initiation + submission

**Files:** `internal/combat/grapple.go`, `internal/combat/submission.go`; config knobs `GrappleProneAttackerMod` (0.3) / `GrappleProneDefenderMod` (0.5); tests extend the packages' existing suites.

- Grapple: attack/defence gain `× SkillWeight` on their skill terms; crit moves
  from the self-relative `AttackZScore` to margin-vs-`CritBarFor` (named: the
  "Stage 8.4" semantics die); prone literals 0.3/0.5 → the two new knobs at
  identical shipped values.
- Submission: `SubSkillWeight` (1.5) deleted from config and code — both sides
  ×SkillWeight; the stun tier moves to margin crit (named: stun-crit vs trash
  2%→18–62% per modelling, accepted for playtest).
Commit.

---

## Task 14: Grapple drift — √2 fix, uniform weights, the aggressor knob

**Files:** `internal/hooks/Position_GrappleTick.go` (~:307-313, ~:680-682 and the `NOTE(U6)` — verify `grep -n "NOTE(U6)\|2.2\|2.0" internal/hooks/Position_GrappleTick.go`); config `GrappleAggressorDriftBonus`; test extend.

1. Fix the missing `√2` normalisation the roadmap assigned to U6 (drift z was
   inflated ~41%).
2. Coefficients 2.2/2.0 → `× SkillWeight` both sides.
3. **Restore the aggressor's edge as config** (gate decision §5.4): add
   `GrappleAggressorDriftBonus` as a multiplier on the aggressor's drift score.
   **Compute the shipped value, do not guess**: extend
   `tools/balance/u6b_model_counters_family_costs.py`'s drift section to solve
   for the multiplier that restores parity E[drift] ≈ +0.196 steps/round under
   the fixed+reweighted maths, and ship that value (expected order: 1.03–1.08).
   The script run and its output go in the PR.
4. Document (do not "fix") the pre-existing 12.5% floor-forced Holds.
Commit.

---

## Task 15: Throw and steal

**Files:** `internal/usercommands/throw.go` (~:267-305, ~:355-365), `internal/actions/steal.go` (~:98-101, ~:171, ~:349); config deletion `StealSkillMultiplier`; tests in both packages.

- **Throw** (Assumption 4): per-defender resolution routes through
  `ResolveChannelAttack(ChannelRanged, side, ...)` with
  `side = {Dex, "dexterity", Skullduggery, rank, 1.0}` — the CURRENT attack
  score is `Dex + skull×5` (already SkillWeight-coupled; the spec's §2.2 row
  understated it). The defender's stat-as-pseudo-skill (`Per × 2.5`) dies with
  the defence set. Damage gains the multiplier curve and the crit tier (named:
  12→165 expected vs trash — accepted for playtest).
- **Steal**: attacker `StealSkillMultiplier` → uniform `× SkillWeight` (knob
  deleted); defender raw Perception → `Perception + skullduggery×SkillWeight`
  (the defender's counter-craft is skullduggery — document the choice in the
  code). No crit tier: steal's outcomes are caught/unseen, not damage — state
  that as the documented reason per spec §4.5.
Commit.

---

## Task 16: Sneak and hidden detection

**Files:** `internal/actions/skill_helpers.go` (`CalcSneakScore`, `CalcSearchScore`), `internal/usercommands/go.go` hidden-detection callers; tests extend.

Replace `combat.SkillMultiplier(rank) × 25.0` in both score builders with
`rank × SkillWeight` (linear, uniform — the sqrt×25 shape was a sixth regime).
The light-conditional multipliers stay untouched. **Named change:** stealth and
detection curves change shape (sqrt→linear); at rank 50 the old term was
`3.0×25=75` and the new is `250` — model the crossover in the task (one table
in the commit message: ranks 5/25/50 old-vs-new for both sides; net detection
odds shift less than either side alone since both move). Commit.

---

## Task 17: The shared situational-modifier layer

**Files:**
- Create: `internal/combat/situational.go`
- Modify: the AttackSide call sites from Tasks 4–8 to compose `Mult` via it; `internal/hooks/NewRound_DoCombat.go` sleeping snapshot consumers
- Test: `internal/combat/situational_test.go`

```go
// SituationalAttackMult composes the attacker-side situational multipliers per
// the DECLARED table below. Every cell is deliberate; absence is deliberate.
//
//   modifier            melee  ranged  specials  spell  social
//   prone attacker        Y      Y        Y        N      N     (you cast/talk fine from the ground)
//   resource depletion    Y      Y        Y        Y*     Y*    (*already applied in damage; here it reaches ACCURACY on physical channels only — see spec 2.3)
//   encumbrance           N (cost-side only, U7's domain — not an accuracy term)
//
// Sleeping defenders: the auto-crit snapshot (forceCrit) now reaches EVERY
// channel via ChannelDefenceResult — CLAUDE.md always promised "the entire
// first round of attacks against them auto-crits" and only melee delivered.
func SituationalAttackMult(attacker *characters.Character, channel AttackChannel) float64
```

Implementation reads the EXISTING knobs (`ProneAttackMultiplier`,
resource-penalty family via `ResourceMultiplier`) — no new numbers. Sleeping:
thread a `ForceCrit bool` through `AttackSide`, honoured in
`resolveChannelAttackWithRunner` (sets AttackerCrit, skips the contest as the
melee path does — read `snapshotSleepingVictims`'s consumer first and mirror
its semantics exactly). **Each cell of the table that changes live behaviour is
a named change in the commit.** Commit.

---

## Task 18: The three guards

**Files:**
- Create: `internal/combat/contest_site_guard_test.go`
- Test-only task.

Per spec §9, and per the U6b lesson that a guard enumerating CHANNELS only
protects channels somebody remembered to name:

```go
// 1. Every combat.RunContest call site is behind a channel seam or on this
//    allowlist naming its owning slice. os.ReadDir + parser.ParseFile (NOT the
//    deprecated parser.ParseDir), scanning internal/... for selector calls to
//    RunContest; assert scanned-file count > 50 so the guard cannot silently
//    scan nothing (the U9 guard's vacuity lesson).
var contestSiteOwners = map[string]string{
	"internal/combat/defence_multiplier.go": "the seam itself",
	"internal/combat/flee.go":               "U6b task 12",
	"internal/hooks/Position_GrappleTick.go": "U6b task 14",
	// every other surviving site: an entry naming its owner, or the test fails
}
func TestEveryContestSiteIsOwned(t *testing.T)

// 2. Defence skill weight is x SkillWeight in every channel — asserted
//    behaviourally: for each channel, a defender's score with rank R equals
//    the rank-0 score + R x SkillWeight (drive GetDefenseScoreFor + the family
//    score builders directly).
func TestEveryChannelUsesUniformDefenceSkillWeight(t *testing.T)

// 3. No legacy skill-weight LITERAL survives: source-scan the named files for
//    the dead numbers (x25, x2.2, x2.0-as-drift-coef, SubSkillWeight,
//    StealSkillMultiplier, SpellAttackSkillFactor identifiers) — knob-greps
//    cannot see literals, which is how flee's x25 hid for the whole arc.
func TestNoLegacySkillWeightLiteralSurvives(t *testing.T)
```

First run: expect failures listing every site Tasks 4–16 have not yet converted
if run early — this task lands LAST among code tasks precisely so it passes
clean; run it early anyway once for the site inventory and record the list.
Commit.

---

## Task 19: Parity check, full suite, isolated boot

- [ ] `gofmt -l internal/ modules/` → nothing.
- [ ] `go test ./... -count=1` → zero failures.
- [ ] **Parity damage-per-swing ±10%** (arc completion criterion 5): extend
  `tools/balance/u6b_model_moves_ranged.py` with a before/after
  melee-autoattack expected-damage cell at light/mid/BIS mitigation and assert
  within ±10% — melee autoattack is UNTOUCHED by U6b, so any drift is a leak
  from Tasks 1–2 (bar extraction / entry builder). Run and record.
- [ ] Isolated boot: detached worktree + port overrides, per the standing
  recipe —

```bash
git worktree add --detach /c/tmp/dogmud-u6b-boot HEAD
cat > /c/tmp/dogmud-u6b-boot/boottest-overrides.yaml <<'EOF'
Network.TelnetPort: [33334]
Network.LocalPort: 9998
Network.HttpPort: 8091
Network.HttpsPort: 0
Network.AIPort: 0
Logging.LogToFile: false
EOF
cd /c/tmp/dogmud-u6b-boot && go build -o boottest.exe .
CONFIG_PATH=/c/tmp/dogmud-u6b-boot/boottest-overrides.yaml LOG_NOCOLOR=1 timeout 150 ./boottest.exe > boot.log 2>&1
# exit 124 = SUCCESS. Do not grep bare "panic" (MapConsistencyEnforce value).
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

  **Windows `timeout` does not reap the child**: after exit 124, find the PID
  (`Get-Process | Where-Object { $_.Path -like '*dogmud-u6b-boot*' }`) and
  `Stop-Process -Id <pid>` — by PID ONLY, never by image name. Then
  `rm -rf /c/tmp/dogmud-u6b-boot; git worktree prune`.
- [ ] The new narration data (Tasks 9, 11) loads clean: zero WARN/ERROR lines.
Commit anything the checks forced; otherwise no commit.

---

## Task 20: Documentation

**Files:** `internal/combat/context.md`, `internal/hooks/context.md`, `internal/actions/context.md`, `internal/characters/context.md`, `internal/configs` comments, `docs/PATCH_NOTES.md`, `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`, `_datafiles/config.yaml` knob docs (HEAD-blob).

- `context.md` for every touched package, symbols verified
  (`python tools/context_md_audit.py` → no new phantoms). The
  `defence_sets.go` "MELEE DOES NOT" note and every comment describing the
  two-contest model must be corrected — a comment describing the removed model
  is worse than none.
- `PATCH_NOTES.md`: player-facing, no numbers, no em dashes, 80-char wrap.
  Cover: skill now defends you against everything; special moves and shots can
  be dodged, parried, blocked — and can strike true (crits); decisive defences
  answer back in kind; buffs that sharpen or blur aim now do so for every kind
  of attack.
- Roadmap: U6b row → ✅ with what shipped; **flip the "Done when" criterion 2
  annotation from FALSE to met, citing the Task 18 guards**; U11 row keeps the
  ship-Done-when-as-tests obligation for the remaining criteria.
- Crib sheet: **the Elemental Queen entry changes meaning** — she was the
  designated quell-observation fight precisely because crit skipped quell;
  post-U6b quell answers every cast. Rewrite her checklist item: expect visible
  quell narration, far fewer royal crits at 300g, and note the gate decision
  that ~500g now buys the old 300g threat (if her fight feels flat, that is
  the repricing, tunable via instance multipliers in config). Add: bash/shot
  crits landing and being narrated; a counter firing after a decisive quell;
  boss-kiting from the adjacent room as a feel item.
Commit.

---

## Task 21: The adversarial playtest gate (REQUIRED)

Per the content SOP — this slice ships new player-facing narration (Tasks 9,
11) and changes how every attack resolves, so an in-game adversarial playtest
runs BEFORE handoff, not after:

```text
/playtest local --checkout <abs-path-to-branch-checkout> bug-finder 2026-08-03-prepush-sweep.yaml
```

(Harness is external at `../gomud-playtest-harness` — verify it exists first;
it has been deleted before. Local runs need `--checkout` and a goals file with
`ephemeral:`.) Drive real fights on every channel: spell duel, taunt, bash and
shoot both directions, a grapple, a flee, a throw; read every line of defence
and counter narration as a confused human would; report bluntly. Fix what it
finds, re-run if needed. Only then hand the branch to the owner — whose own
manual pass (crib sheet) is the final gate, per the arc's no-deploy rule.

---

## Execution notes

- **Task order is dependency order**: 1→2→3 are the foundation; 4–8 must follow
  3; 9 follows 6–8; 10 follows 4–8; 11 follows 10; 12–16 only need 1–3; 17
  follows 4–8; 18 after all code; 19–21 close. Tasks 12–16 can interleave with
  4–8 if parallelised, but never two tasks in the same file concurrently.
- **Expect existing tests to change.** Unlike U9, this slice is contracted to
  change behaviour. Every changed test is named in its task's commit with what
  it pinned. A message-level breakage you did not intend is a stop signal.
- **The modelling scripts are the oracle**: when a converted channel's measured
  rate disagrees with the modelling doc's table for the same cell, one of them
  is wrong — reconcile before proceeding, do not average.
- Two named nerfs land quietly if unwatched: taunt crit damage (Assumption 8)
  and shieldless channel defence (Task 2). Both are in their task's commit and
  must be in the PR body.
