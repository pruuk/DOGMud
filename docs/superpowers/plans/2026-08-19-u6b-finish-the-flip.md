# U6b Finish-the-Flip Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Tasks:** 21.

> **Revised 2026-08-19 after a blind adversarial review** (16 findings, 5 of
> them showstoppers). Where a step says "an earlier draft did X", that is the
> specific mistake the step exists to prevent repeating. The largest: the first
> draft hoisted melee's WHOLE dynamic crit bar to every channel, contradicting
> the modelling the owner's binding gate decisions were computed on — the
> spell/taunt numbers used the constant bar, and the skill-shift reads MELEE
> combat skill, so the full hoist would have let a swordsman's weapon skill
> lower his spell crit bar and collapsed royal crits at every gold price,
> silently un-deciding gate decision §5.1.

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
| 1 | Fizzle becomes a partial-damage defence outcome. **Copy ratified (owner, 2026-08-19):** defended casts speak the existing quell/dodge/block defence triads, exactly as melee narrates defences; the WORD "fizzle" survives only as flavor inside one or two quell defensive-crit (heavy-band) variants, where the spell truly is fully stopped; fumbles keep the backfire copy. "Fizzle" must never narrate a partial hit — that would lie about what happened. | Gate decision §5.2; owner copy decision |
| 2 | `shoot` keeps Perception as its attack stat; only the weight unifies | Spec §8.2 |
| 3 | **The crit bar hoists as a pure function of the CHANNEL's skill pair, with a shipped ceiling.** Owner decision, 2026-08-19: `CritBarFor(atkRank, defRank)` = `base 2.0 − slope×(atkRank − defRank)`, clamped to `[CritBarFloor, CritBarCeiling]`, with three new config knobs: `CritBarSkillSlope` **0.05**, `CritBarFloor` **1.5**, `CritBarCeiling` **3.0** (0 = uncapped). The attacker's rank is `AttackSide.SkillRank` (the channel's governing skill); the defender's rank is the WINNING defence's governing skill (via `DefenceSkillAndStat`). The blind review's finding-1 objection (a melee-skill hoist would couple weapon skill to spell crits and collapse the §5.1 royalty crits) is answered by BOTH halves: the per-channel pair removes the coupling, and the 3.0 ceiling keeps gold able to buy crits back against veterans on every channel — Queen@1000g crits Meirok ~47% (vs 82.5% at the old const bar, ~5% uncapped). **The ceiling changes live MELEE too** (a 1000g King goes from ~0.1% to ~28% melee crits vs Meirok — the old melee bar was uncapped); named in Task 1. **Accuracy and Blink are deleted**: their two bar reads are the only references in the codebase, no shipped content grants either flag, and the owner does not recognise them — upstream stowaways. | Owner directives 2026-08-19; review finding 1 |
| 4 | `throw` reuses `ChannelRanged`'s set — **ratified (owner, 2026-08-19)**: it is AoE resolved PER TARGET (one grenade, each target contests independently with their own margin and own crit-or-not); every target can dodge (dive clear), a SHIELDED target can block (hunker behind the shield), and the Task 2 equipment gate means shieldless targets get dodge only. No throw-specific set row. Margin-scaled damage per target. | Spec §4.5; modelled in appendix C; owner decision |
| 5 | Counters are free (no cost), like riposte today; non-melee channels get a counter-SWING on any defensive crit; melee keeps its per-defence trio (riposte/auto-trip/auto-bash) | Spec §4.3 "riposte's mechanism" |
| 6 | Defy's crit counter-taunts INSTEAD of counter-swinging | Owner decision 2026-08-19 |
| 7 | The fumble-before-success ordering (fumble aborts even winning attacks, capping hit at 87.5%−fumble) is KEPT and documented, now uniformly | Modelling §6.3 "document or change, deliberately" |
| 8 | The RAW skill rank is the ONE rank input everywhere — crit damage AND base damage. Taunt fed the ×5-weighted rank to BOTH (`CritOrMitigatedDamage` and `CalcRawDamage`'s `SkillMultiplier`); correcting both is a named taunt nerf: crit multiplier ×15.75→×4.6 at Meirok, and base damage for rhetoric ranks below the soft cap (raw 30: multiplier 3.0→2.55). One rank field, two consumers, no split. | Review finding 12; modelling §6.2 |
| 9 | Mob skill: nothing changes (accept the repriced gold dial) | Gate decision §5.1 |
| 10 | `CalcSneakScore` and the OPPOSED detection/shadow/plant/steal scores convert to linear ×SkillWeight; **`CalcSearchScore` keeps its `SkillMultiplier×25` shape for the three Category B consumers** (forage yield `forage.go:19`, `search.go` thresholds, `track.go:120`) which the spec walls off from this slice. A new linear function serves the contest sites. Converting `CalcSearchScore` in place would have silently changed forage yields and search/track rates in direct contradiction of spec §3.2. | Review finding 5 |
| 11 | Non-melee channels gain a FUMBLE abort they never had (specials and ranged had no fumble concept). Named per channel; uniform per Assumption 7. | Review finding E |

## Standing rules (from the arc — violations are defects)

1. **No balance number inside `internal/`.** Every tuning value is a config knob.
   New knobs this plan adds: `CritBarSkillSlope` (0.05), `CritBarFloor` (1.5),
   `CritBarCeiling` (3.0; 0 = uncapped), `CounterDamagePercent` (0.5),
   `GrappleAggressorDriftBonus` (value computed in Task 14),
   `GrappleProneAttackerMod` (0.5), `GrappleProneDefenderMod` (0.3).
   Deleted knobs: `SpellAttackSkillFactor`, `RangedShieldDefenseBonus`,
   `SubSkillWeight`, `StealSkillMultiplier`. Deleted flags: `buffs.Accuracy`,
   `buffs.Blink` (no shipped content grants either; upstream stowaways).
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

## Task 1: The crit bar — per-channel skill pair, knobbed, ceilinged

**Files:**
- Create: `internal/combat/crit_bar.go`
- Modify: `internal/combat/combat_helpers.go` (~line 553 `calcCritThreshold`, ~line 966 the defensive 2.0)
- Modify: `internal/configs/config.balance.go` + `config.balance.combat.go` (three new knobs)
- Modify: `internal/buffs/buffspec.go` (delete the `Accuracy` and `Blink` flag constants)
- Config: `_datafiles/config.yaml` (three knob lines, HEAD-blob method)
- Test: `internal/combat/crit_bar_test.go` (create)

Melee's bar today (`calcCritThreshold` at `combat_helpers.go:553-600`): base
2.0, Accuracy→1.5, Blink→2.5, then shifted by
`sourceChar.GetCombatSkillLevel() − targetChar.GetCombatSkillLevel()` at 0.05
per point, floored 1.5, NO ceiling. Every other channel uses the const
`ContestCritThreshold = 2.0`.

**The target (Assumption 3, owner decision 2026-08-19):**

```
bar = clamp(2.0 − CritBarSkillSlope × (atkRank − defRank),
            CritBarFloor, CritBarCeiling)      // ceiling 0 = uncapped
```

- **The ranks are the CHANNEL's, not melee's.** Attacker: the channel's
  governing skill (`AttackSide.SkillRank` at the seam; equipped combat skill
  for melee). Defender: the WINNING defence's governing skill — spellcasting
  behind quell, rhetoric behind defy, weapon/unarmed behind parry-block/dodge
  (`DefenceSkillAndStat` already maps it). Using melee skill for a spell crit
  bar was the review's finding 1: an incoherent coupling.
- **Three new config knobs**, because 0.05 and 1.5 are balance literals in
  `internal/` and this is the moment they are touched (standing rule 1):
  `CritBarSkillSlope: 0.05`, `CritBarFloor: 1.5`, `CritBarCeiling: 3.0`.
  Validation: slope `< 0` → 0.05; floor `<= 0` → 1.5; **ceiling `< 0` → 3.0
  and `0` is LEGAL, meaning uncapped** — document the off-switch beside it.
- **The ceiling is NEW to melee and is a named live-behaviour change**: the
  old melee bar was uncapped, so a stat-rich skill-1 mob attacker faced bar
  5.4 vs a veteran and effectively never crit. At ceiling 3.0 a 1000g King
  goes from ~0.1% to ~28% melee crits vs Meirok, and the same shape applies to
  every channel (Queen@1000g spell crits ~47% vs 82.5% at the old const bar,
  ~5% uncapped). Owner chose the ceiling explicitly with that table in hand;
  flipping it later is one config edit.
- **Accuracy and Blink are DELETED, not hoisted.** Their two bar reads are the
  only references in the entire codebase, no shipped buff grants either flag,
  and the owner does not recognise them — upstream stowaways. Delete the reads
  and the two constants in `buffspec.go`. Zero live behaviour change; name it
  in the commit and list both in the dead-code followup memory
  (`project-spell-proficiency-dead-code` is the same class).

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

// CritBarFor is pure arithmetic on the CHANNEL's skill pair. Inject the three
// knobs via the package's balance-config test idiom (grep -n "SkillWeight"
// internal/combat/*_test.go for the pattern); these cases assume shipped
// values slope 0.05, floor 1.5, ceiling 3.0.
func TestCritBarFor(t *testing.T) {
	cases := []struct {
		name             string
		atkRank, defRank int
		want             float64
	}{
		{"parity", 30, 30, 2.0},
		{"attacker out-skills: pins at floor", 69, 1, 1.5},
		{"defender out-skills: rises", 30, 40, 2.5},
		{"defender far out-skills: CEILING binds", 1, 69, 3.0},
		{"boss case: skill-1 mob vs spellcasting 52", 1, 52, 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CritBarFor(tc.atkRank, tc.defRank); got != tc.want {
				t.Errorf("CritBarFor(%d,%d)=%v want %v", tc.atkRank, tc.defRank, got, tc.want)
			}
		})
	}
}

// Ceiling 0 means UNCAPPED — the documented off-switch. A validator that
// "corrects" 0 back to 3.0 would make uncapping impossible; pin it.
func TestCritBarFor_ZeroCeilingIsUncapped(t *testing.T) {
	// inject ceiling 0, then:
	if got := CritBarFor(1, 69); got != 2.0+0.05*68 {
		t.Errorf("uncapped bar = %v, want %v", got, 2.0+0.05*68)
	}
}

// Melee routes through the same function on its combat-skill pair. Identical
// to the old bar at and below 3.0; ABOVE 3.0 the new ceiling binds — the named
// melee change (a stat-rich skill-1 mob vs a veteran now caps at 3.0 instead
// of 5.4). Accuracy/Blink are gone: no branch to test.
func TestCalcCritThreshold_MeleePair(t *testing.T) {
	cases := []struct {
		name               string
		atkSkill, defSkill int
		want               float64
	}{
		{"parity", 30, 30, 2.0},
		{"skill advantage pins at floor", 69, 1, 1.5},
		{"mob vs veteran: ceiling binds (was 5.4 uncapped)", 1, 69, 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			atk, def := barChar(t, tc.atkSkill), barChar(t, tc.defSkill)
			if got := calcCritThreshold(atk, def); got != tc.want {
				t.Errorf("calcCritThreshold=%v want %v", got, tc.want)
			}
		})
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

// CritBarFor is THE attacker-side crit threshold for every channel, as a pure
// function of the CHANNEL's skill pair (owner decision, 2026-08-19):
//
//	bar = clamp(base − slope×(atkRank − defRank), floor, ceiling)
//
// atkRank is the attack's governing skill rank (AttackSide.SkillRank at the
// seam; the equipped combat skill for melee). defRank is the WINNING defence's
// governing skill rank — spellcasting behind quell, rhetoric behind defy, the
// weapon/unarmed skills behind the physical three. Out-skill your target and
// the bar falls toward the floor; get out-skilled and it rises to the CEILING,
// which is what lets a gold-scaled, skill-poor boss still buy crits against a
// veteran (uncapped, a 1000g boss crits a veteran essentially never — the
// pre-U6b melee behaviour; the shipped 3.0 puts it near half its saturated
// rate instead). Ceiling 0 means uncapped, and is legal.
//
// All three values are config (CritBarSkillSlope 0.05, CritBarFloor 1.5,
// CritBarCeiling 3.0) — they were balance literals inside internal/ before
// U6b, which standing rule 1 forbids.
//
// Melee's old Accuracy/Blink adjustments do not survive: no shipped content
// ever granted either flag and both were deleted as upstream stowaways.
func CritBarFor(atkRank, defRank int) float64 {
	b := configs.GetBalanceConfig()
	bar := ContestCritThreshold - float64(b.CritBarSkillSlope)*float64(atkRank-defRank)
	if bar < float64(b.CritBarFloor) {
		bar = float64(b.CritBarFloor)
	}
	if c := float64(b.CritBarCeiling); c > 0 && bar > c {
		bar = c
	}
	return bar
}

// DefenseCritBar is the defender-side threshold. Melee shipped this as a
// hardcoded 2.0 separate from its own dynamic attack bar; U6b keeps it a
// single constant for every channel, ON PURPOSE — a defensive crit unlocks
// the counter tier, and skill already reaches the defensive-crit rate through
// the margin (a skilled defender out-rolls more often and by more). Shifting
// this bar by the same skill pair would triple-count skill on the defence
// side. Documented here so nobody "unifies" it into CritBarFor without
// deciding that.
func DefenseCritBar() float64 { return ContestCritThreshold }
```

Then in `combat_helpers.go`: `calcCritThreshold` (:553) becomes a thin melee
wrapper — `CritBarFor(sourceChar.GetCombatSkillLevel(),
targetChar.GetCombatSkillLevel())` — deleting its Accuracy/Blink branches and
its inline slope/floor arithmetic (the knobs carry them now). The separate
hardcoded defensive `2.0` (:966, inside `resolveDefenseOutcomeCore`) becomes
`DefenseCritBar()`. Delete the `Accuracy`/`Blink` constants from
`internal/buffs/buffspec.go` (their only references were the two branches just
removed) and add the three knobs (declaration + validation per the preamble +
config.yaml lines via the HEAD-blob method, each documented beside its value
including the ceiling's 0-means-uncapped off-switch).

- [ ] **Step 3b: Record the crit-column deltas the ceiling produces**

The §5.1 royalty table was executed at the const 2.0 bar; the owner chose the
per-channel pair + ceiling 3.0 WITH the delta table in hand (2026-08-19).
Record it in the commit so the numbers are on the PR:

| Attack vs Meirok | old const bar | shipped (pair + ceiling 3.0) |
|---|---|---|
| Queen spell 500g / 1000g / 2000g | 21.7% / 82.5% / 96.5% | 3.7% / 47.3% / 79.2% |
| King melee 1000g | ~0.1% (old melee bar was UNCAPPED 5.4) | ~28% — the named MELEE change |
| King bash 1000g | n/a (no crit tier existed) | ~39% |

Hit rates and non-crit damage are bar-independent and unchanged. Extend
`tools/balance/u6b_model_spell_taunt.py`'s `CRIT_T` to read the clamped-pair
bar so the modelling doc's crit columns can be regenerated under the shipped
values, and note the regeneration in the modelling doc.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/combat/ -run "TestCritBarFor|TestCalcCritThreshold" -v`
then the full package: `go test ./internal/combat/ -count=1`
Expected: the new tests PASS. Existing melee tests pass UNCHANGED except any
that pinned (a) the Accuracy/Blink branches — they pinned dead flags; or (b) a
bar above 3.0 — they pinned the uncapped behaviour the ceiling deliberately
changes. Update and name each.

- [ ] **Step 5: Commit**

```bash
git add internal/combat/ internal/buffs/ internal/configs/ _datafiles/
git commit -m "feat(u6b): one crit bar for every channel, knobbed and ceilinged

CritBarFor(atkRank, defRank) is pure arithmetic on the CHANNEL's skill
pair: base 2.0 minus CritBarSkillSlope per point of attacker skill
advantage, clamped to [CritBarFloor, CritBarCeiling]. Three new knobs
ship at 0.05 / 1.5 / 3.0; ceiling 0 is the documented uncapped
off-switch. Melee routes through it on its combat-skill pair.

Named melee change: the old melee bar was UNCAPPED, so a stat-rich
skill-1 mob vs a veteran faced bar 5.4 and effectively never crit; at
the shipped ceiling a 1000g boss melee-crits a veteran ~28% of the
time. Owner chose the ceiling with the delta table in hand.

Deletes the Accuracy and Blink buff flags and their two bar reads --
the only references in the codebase; no shipped content grants either;
upstream stowaways. Zero live change from that half."
```

---

## Task 2: One defence-set source, equipment-gated

**Files:**
- Modify: `internal/combat/defence_sets.go`
- Modify: `internal/combat/combat_helpers.go` (melee's `runBestOfAllDefense` set building)
- Modify: `internal/characters/combat.go` (`GetDefenseSequence` — deleted after migration)
- Test: `internal/combat/defence_entries_test.go` (create)

Today melee builds its set from `characters.GetDefenseSequence` (the consuming
call is at `internal/combat/combat.go:455`, not combat_helpers.go) and the
channel path builds from `DefenceSetFor` with NO equipment gate — a shieldless
bare-handed defender can roll block against a bolt or a physical spell.

**THE GATE IS SUBTLER THAN "parry needs a weapon, block needs a shield" —
read `characters/combat.go:271-307` and `worn.go:300,327` in full and copy the
checks EXACTLY.** The review caught three divergences a paraphrase would ship:

1. **Block requires a weapon AND `HasShield()`** — a shield-without-weapon
   defender gets dodge only today.
2. **`HasShield()` includes species NaturalBash** (`worn.go:300`) — earth
   elementals block with no shield ITEM. A `BestBlockRating() > 0` gate would
   strip them. Gate on `HasShield()`, not on the rating.
3. **`IsUnarmedStyle()` never parries** (`worn.go:327` — bare hands, Fist and
   Claws weapons) even when "armed". A plain wielded-weapon check would grant
   parry to a knuckle-fighter melee never gave it.

Any divergence from the copied checks CHANGES MELEE, and Task 19's parity cell
asserts melee is untouched — it will fire without explaining why.

**Seam boundary, stated so it cannot be half-implemented:**
`DefenceEntriesFor` returns **gated defence NAMES** (with dual-parry
duplication and opts filtering) — NOT scores. Melee's candidate loop
(`combat_helpers.go:678-800`) keeps ALL of its own scoring: clinch/grounded
penalties, Rally, `ConditionDefensePenalty`, darkness, third-party penalty,
Incorporeal, per-candidate quoting/skill-strip, and the
`DefenseAttempts`/`IncrementDefenseCount` bookkeeping. The channel seam keeps
its own scoring via `GetDefenseScoreFor` × `defenceEffectiveness` as today.
Returning scored entries would either lose or double-apply everything above.
Prone defence penalties apply inside each consumer's scoring (the channel seam
gains them; melee already has them) — NOT inside the name builder.

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
			got := DefenceEntriesFor(tc.channel, tc.def, DefenceEntryOpts{})
			assertSameSet(t, got, tc.want)
		})
	}
}

// Dual-wield double parry survives the migration: two parry entries.
func TestDefenceEntriesFor_DualWieldDoubleParry(t *testing.T) {
	dw := newDualWieldDefenceTestCharacter(t)
	got := DefenceEntriesFor(ChannelMelee, dw, DefenceEntryOpts{})
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

// DefenceEntriesFor is THE defence-set NAME builder for every channel, melee
// included. It intersects DefenceSetFor's channel table with the equipment
// gate copied VERBATIM from characters.GetDefenseSequence (which this task
// then deletes):
//
//   - parry: wielded weapon AND !IsUnarmedStyle() — knuckle/claw fighters
//     never parry; appears TWICE when dual-wielding (two blades, two chances)
//   - block: wielded weapon AND HasShield() — which includes species
//     NaturalBash, so an earth elemental blocks with no shield item; do NOT
//     gate on BestBlockRating()
//   - dodge, quell and defy: always available on their channels
//
// It returns NAMES ONLY. Scoring stays with the consumer: melee's candidate
// loop keeps its situational penalties/quoting/bookkeeping; the channel seam
// keeps GetDefenseScoreFor x defenceEffectiveness, and gains the prone
// penalties there (before U6b a prone defender dodged a bolt at full score
// while dodging a sword at penalty).
func DefenceEntriesFor(channel AttackChannel, defender *characters.Character, opts DefenceEntryOpts) []string
```

The exact wielding checks are copied out of `characters.GetDefenseSequence`
(read it in full first — `grep -n "func (c \*Character) GetDefenseSequence" -A40
internal/characters/combat.go` — and the two helpers it leans on at
`worn.go:300` and `worn.go:327`). Then:

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
3. After the contest: `out.AttackerCrit = !res.Floored && AttackContestCritAt(res.Margin, res.AttackRoll, bar)`
   where the bar is the CHANNEL's skill pair:
   `bar := CritBarFor(side.SkillRank, defenderRankOf(res.Winner))` — the
   defender rank is the WINNING defence's governing skill rank, resolved via
   `DefenceSkillAndStat(res.Winner)` + `defender.GetSkillLevel(...)`
   (uncontested/static outcomes use defRank 0). Add the bar-parameterised
   `AttackContestCritAt(margin float64, roll dice.RollResult, bar float64) bool`
   in `crit_floor.go`; the existing `AttackContestCrit` becomes a call with
   `ContestCritThreshold`.
   `out.AttackerFumble = res.AttackRoll.ZScore <= -DefenseCritBar()`.
4. The U9 bonus tier (`awardChannelDefenceBonus`) now takes its attacker skill
   and stat FROM `side` — `channelAttackSkillAndStat` is deleted; its channel
   switch has no reason to exist once the caller states the names.
   **And it consumes the seam's ALREADY-DERIVED crit/fumble verdicts** —
   today it re-derives `attackCrit` itself via the const-bar
   `AttackContestCrit` (`defence_multiplier.go:376-380`), under a comment
   forbidding exactly that duplication. Left as-is, a skill-advantaged
   attacker would crit at the floored bar for narration and damage while the
   progression bonus still demanded 2.0 — two verdicts for one contest. Pass
   `out.AttackerCrit`/`out.AttackerFumble` in; delete the re-derivation.
5. **`channelDamageChannel` gains melee → `"physical"` and ranged →
   `"physical"` rows.** Its default arm returns `""` on the stated premise
   that those channels never reach this path — false after Tasks 6–8, and
   `ToughenStatFor("")` would silently make a bash crit toughen the
   defender's DEXTERITY instead of vitality. Delete the stale premise comment
   with it.
6. Public wrappers:

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

**Verify first — the five skips, both gates, and the FUNCTION TOPOLOGY:**

```bash
grep -n "if !isCrit" internal/hooks/spell_resolution.go        # expect 5 hits
grep -n "runPlayerSpellContest\|runMobSpellContest\|runPlayerSpellDefence\|spellDefenseValue\|CalcSpellAttack" internal/hooks/spell_resolution.go internal/characters/cast_helpers.go
```

**The topology is the hard part, not the deletion.** The gates live in FOUR
resolvers (`resolveAgainstMob` ~:293, `resolveAgainstPlayer` ~:796, and the two
mob-caster variants ~:1379/:1399), but the defence contest, its narration and
the DefensiveCrit early-return live inside the EFFECT APPLIERS
(`applyMobEffect_damage`, `applyMobEffect_knockdown`, `applyPlayerEffect`, and
the two mob-caster damage/DoT arms — the five `!isCrit` sites), reached through
the **`runPlayerSpellDefence` seam variable (~:275-278)**. One-contest-per-cast
means: the resolver runs `ResolveChannelAttack` ONCE and **threads the
`ChannelDefenceResult` through the applier signatures**; the appliers consume
it (damage scaling, narration, DefensiveCrit return) instead of rolling their
own. An earlier draft sketched everything inline in `resolveAgainstPlayer` and
said nothing about the threading — following it would have left the appliers
rolling a second contest. Delete the `runPlayerSpellDefence` seam variable
once nothing calls it.

**Two follow-on deletions in the same task:**

- The U9-era direct crit-received toughening blocks
  (`spell_resolution.go` ~:824-842 and ~:1466-1480) become DUPLICATES once the
  seam's bonus tier sees the crit — the once-per-round dedupe would mask the
  double-fire, not prevent it. Delete them; the seam covers it. (Taunt's
  equivalent goes in Task 5.)
- `internal/spells/manifestation_channel_guard_test.go` forbids manifestation
  spells from declaring a contested `target_defense_type` BECAUSE
  `ChannelAttackScore` hardcoded Willpower. This task makes that content
  legitimate (`AttackSide` carries the spell's primarystat), and the test
  references the dead symbol only in comments, so nothing will force the
  update. **Retire it, replacing it with its inversion**: a loader test that a
  contesting manifestation spell resolves with its declared stat.

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

**Copy decision (ratified — Assumption 1):** the "fizzles" strings are deleted;
defended casts speak the channel defence triads
(`sendSpellChannelDefenceMessages` already exists and renders them). The word
"fizzle" survives ONLY as flavor inside one or two quell defensive-crit
(heavy-band) variants — Task 9 adds those variants — where the spell truly is
fully stopped. It must never narrate a partial hit. Fumble/backfire copy
untouched.

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

Delete `runTauntContest` and the `if !isCrit` skip, plus taunt's direct
crit-received toughening block (~:267 — a Task 4-class duplicate once the
seam's bonus tier runs).

**The rank input, BOTH consumers (Assumption 8):** taunt feeds the ×5-weighted
rhetoric rank to `CritOrMitigatedDamage` AND to `CalcRawDamage`'s
`SkillMultiplier` (`combat_taunt.go` ~:205-209). Pass `side.SkillRank` (raw) to
both. Two named nerfs, not one: crit multiplier ×15.75 → ×4.6 at Meirok, and
base damage for any rhetoric rank below the soft cap (raw 30: SkillMultiplier
3.0 → 2.55; at 50+ the clamp makes it a wash). The modelling's E[mult]
0.338→0.658 table did not include the base-damage half — say so in the PR
rather than implying the modelling blessed it.

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
`out := ResolveChannelAttack(p.Channel, p.Attack, p.Attacker, p.Defender)`.

**`result.Hit = !out.Defended` — the contest WIN, exactly as today.** An
earlier draft wrote `!out.Defended || out.DamageMultiplier > 0`, which — since
floored saves are 0.5 and rolled defensive wins are 0–0.5 — would have made
nearly every DEFENDED outcome a "Hit", so knockdown rolls started firing on
defended bashes and every caller's hit messaging flipped, while the same task
claimed StatusApplied semantics were unchanged. Defended-partial damage lands
with `Hit == false`, per the current `skill_moves.go` doc; StatusApplied and
KnockedDown stay gated on the contest win.

`result.Crit = out.AttackerCrit`; `result.Fumble = out.AttackerFumble` (a NEW
abort for these moves — Assumption 11, named); damage through
`CritOrMitigatedDamage(rawDmg, p.Attack.SkillRank, out.AttackerCrit, mitig,
cap)` then `× out.DamageMultiplier` on non-crits (a crit means the attack won
decisively; the multiplier is 1.0 there by construction).

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
plan expects 14 total (verified 2026-08-19; note **`combat_drain.go` has TWO
call sites, :112 and :268** — converting one and not the other compiles and
half-converts drain); report the real count.

**Also in this task**: the defender-side economy change every converted move
carries, named ONCE here for the whole family — routing through the seam means
the defender's winning defence is now CHARGED (U7/U8) and the defender is
PROGRESSED **win-or-lose** (the channel path's convention, documented as
divergent from melee's defence-used gate — U10b's question). Today's
scalar-defence specials cost defenders nothing and taught them nothing. That is
an intended consequence of unification, but it is a rate change on the
DEFENDER's side that the per-task "named changes" lists would otherwise miss. Beast moves pass
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
line, per applicable defence, per band. **Also in this task (Assumption 1):
add one or two "fizzle"-flavored variants to quell's defensive-crit / heavy
band** ("the working fizzles against your quelling will" class) — the word's
only surviving home, used exclusively where the spell is fully stopped. **Player-copy rules: 80-char wrap, no
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
//   - defy crits COUNTER-TAUNT instead, replacing the swing. NOTE THE
//     PLACEMENT: taunt resolution lives in internal/actions, which IMPORTS
//     internal/combat — this package can never call it. The counter-taunt is
//     wired AT THE TAUNT CALL SITE in internal/actions (the defy-crit exit),
//     via a dedicated cost-free entry point; see the carve-out below. Its
//     test lives in internal/actions for the same reason.
//   - a counter never earns a counter: everything this function triggers
//     carries IsCounter, and ExecuteCounter is never invoked for a result
//     produced under IsCounter.
//   - free: no cost, like riposte today.
//   - do NOT frame this as interrupting the attack — the attack has already
//     resolved. A defensive crit is a decisive defence that leaves an
//     opening; the counter is what you do with the opening.
func ExecuteCounter(defender, attacker *characters.Character, channel AttackChannel, sameRoom bool) CounterResult
```

Wire at each channel's defensive-crit exit, **enumerated — the four quadrants
exist on the spell channel and both directions everywhere**: spell
player-caster path, spell mob-caster path (a PLAYER defender counters a mob
caster; a MOB defender counters a player caster), taunt (both directions),
every `ExecuteSkillMove` consumer via `result.Defence.DefensiveCrit`, and
same-room `ExecuteFire`. A wiring list that only covers the player-attacker
direction hands mobs a counter immunity nobody decided.

**The defy counter-taunt carve-out, explicit:** `ExecuteTaunt` burns the
`special-move` cooldown (`combat_taunt.go` ~:130), pays U8 admission cost, and
mutates aggro (~:189) — ALL wrong for a free counter. The counter-taunt entry
point (in `internal/actions`, per the import direction above) must bypass
cooldown, cost and aggro mutation, carry `IsCounter` so it can never earn a
counter, and reuse only the CONTEST + narration of taunt resolution. Assert
each bypass in the actions-package test.

Extract riposte's `0.5` literal (`hooks/combat_shared_helpers.go:244`) to the
`CounterDamagePercent` knob (config declaration + validation `< 0` reset +
config.yaml via HEAD blob) and point the melee riposte block at the same knob —
melee behaviour unchanged at the shipped 0.5.

**Countered-party economy, named:** a counter-swing routed through the seam
means the ORIGINAL ATTACKER, now defending the counter, is charged for their
defence and progressed by it — today's auto-trip/auto-bash cost and taught the
countered party nothing. "Counters are free" is true only for the counterer.
This applies to melee's existing auto-trip/auto-bash too the moment Task 7
routes them through the seam — a melee-side change Tasks 6–7 inherit and this
task's PR text must carry.

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

**Files:** `internal/combat/flee.go`; test extend `internal/combat/flee_test.go`.

**THREE ×25 literals, not two** — verify with `grep -nE '\* *25' internal/combat/flee.go`
(the pattern MUST tolerate spaces around `*`):

| Site | Side |
|---|---|
| `flee.go:83` | blocker score, `...UnarmedCombat)*25` |
| `flee.go:106` | blocker score (user variant) |
| **`flee.go:126`, `fleeContestScore`** | **the FLEER's score — `GetSkillLevel(skills.Skullduggery) * 25`, spaced, invisible to a `\*25` grep** |

An earlier draft named only the two blocker sites; converting those alone drops
blockers to ×5 while the fleer keeps ×25, making flee near-unblockable for
anyone with skullduggery — the exact inverse of the modelled near-no-op, and
the same declared-converted-while-unconverted failure this arc keeps repeating.

Replace all three with `× SkillWeight` (config read). Modelling: floor/ceiling
pinned near no-op (only novice-vs-trash moves, −8.9pp) — a flee test that moves
MORE than that is a wiring error. Task 18's literal-guard regex must also
tolerate the spaced form. Commit.

---

## Task 13: Grapple initiation + submission

**Files:** `internal/combat/grapple.go`, `internal/combat/submission.go`,
**`internal/combat/grapple_move.go`** (the crit-band consumer an earlier draft
omitted); config knobs `GrappleProneAttackerMod` (**0.5**) /
`GrappleProneDefenderMod` (**0.3**); tests extend the packages' existing suites.

**The knob values, from the CODE — an earlier draft had them SWAPPED:**
`grapple.go:74` is the DEFENDER prone → `DefenseScore *= 0.3`; `grapple.go:79`
is the ATTACKER prone → `AttackScore *= 0.5`. So `GrappleProneAttackerMod: 0.5`
and `GrappleProneDefenderMod: 0.3`. Copy the literal each site currently uses
into the knob that replaces IT; shipping the swapped values is a silent double
flip (prone attackers worse, prone defenders better).

- Grapple: attack/defence gain `× SkillWeight` on their skill terms.
- **Crit semantics live in `grapple_move.go:41-56`, not grapple.go**: a
  three-band ladder on `GrappleResult.AttackZScore` (>2.0 crit, <0.5 weak,
  <−2.0 fumble). Convert the CRIT band to normalized-margin-vs-`CritBarFor`
  and the WEAK band to the normalized margin (same quantity, same 0.5 line);
  the fumble band is already self-relative and stays (Assumption 7). Thread
  the margin through `GrappleResult` — converting grapple.go's roll while the
  ladder still reads the old z-score is a half-conversion that compiles.
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

## Task 15: Throw, steal — and plant, which an earlier draft missed entirely

**Files:** `internal/usercommands/throw.go` (~:267-305, ~:355-365),
`internal/actions/steal.go` (~:98-101, ~:171, ~:349, **and :509**),
**`internal/actions/plant.go`** (~:108-109 and its FOUR `RunContest` sites at
~:157, :276, :325, :414 — verify all with grep); config deletion
`StealSkillMultiplier`; tests in the three packages.

- **Throw** (Assumption 4): per-defender resolution routes through
  `ResolveChannelAttack(ChannelRanged, side, ...)` with
  `side = {Dex, "dexterity", Skullduggery, rank, 1.0}` — the CURRENT attack
  score is `Dex + skull×SkillWeight` (already knob-coupled; the spec's §2.2 row
  understated it). The defender's stat-as-pseudo-skill (`Per × SkillWeight×0.5`)
  dies with the defence set. Damage gains the multiplier curve and the crit
  tier (named: 12→165 expected vs trash — accepted for playtest).
- **Steal — the REAL attacker shape, so the sqrt term is not missed:** today
  it is `(Dex + combat.SkillMultiplier(rank)×25.0) × StealSkillMultiplier`
  (`steal.go:99-101`) — a sqrt-curve regime TIMES a global knob, not a linear
  weight. It becomes `Dex + rank×SkillWeight`, both the sqrt×25 term and the
  knob deleted. Defender: raw Perception → `Perception +
  skullduggery×SkillWeight` (the counter-craft is skullduggery — document the
  choice). **A FOURTH steal contest exists at `steal.go:509`** — the
  container-theft observer pass, scored on raw highest-observer Perception,
  same ×0-defender class; convert it with the rest. No crit tier: steal's
  outcomes are caught/unseen, not damage — the documented reason per §4.5.
- **Plant shares steal's engine and an earlier draft omitted it**:
  `plant.go:108-109` builds `SkillMultiplier(rank)×25.0` then multiplies by
  `cfg.StealSkillMultiplier` — deleting the knob without converting plant
  BREAKS THE BUILD, and plant holds four more `RunContest` sites in the same
  unowned class. Convert plant to the same linear shape as steal in this task;
  its four sites get Task 18 owner entries.
Commit.

---

## Task 16: Sneak and hidden detection

**Files:** `internal/actions/skill_helpers.go`, `internal/usercommands/go.go`,
`internal/actions/shadow.go` (~:176), `internal/actions/sneak.go`,
`internal/usercommands/skill.skullduggery.shadow.go` (~:136); tests extend.

**`CalcSearchScore` does NOT convert in place — an earlier draft would have
silently changed the Category B sites the spec walls off.** Its consumers split
two ways:

| Consumer | Class | Fate |
|---|---|---|
| `go.go` hidden detection, `sneak.go` (×2), `shadow.go:176`, `skill.skullduggery.shadow.go:136`, `steal.go:402`, `plant.go:323` | OPPOSED contests | move to a NEW linear function |
| **`forage.go:19` (forage YIELD — not even a contest), `search.go:63` (flat thresholds ×6), `track.go:120`** | **Category B — spec §3.2: "U6b does not silently absorb them"** | **keep `CalcSearchScore` untouched** |

Add `CalcDetectionScore(c) = Perception + rank(search)×SkillWeight` for the
opposed sites; `CalcSneakScore` converts in place (all its consumers are
opposed), keeping the light-conditional multipliers untouched;
`CalcSearchScore` keeps its `SkillMultiplier×25` shape with a comment naming
the Category B sites as its remaining, deliberately-unconverted consumers.

**Named change:** stealth/detection curves change shape (sqrt→linear) on the
OPPOSED sites only; at rank 50 the old term was `3.0×25=75`, the new is `250`;
at rank 5 it DROPS (~41→25) — one table in the commit (ranks 5/25/50,
old-vs-new, both sides; net detection odds move less than either side alone
since both move). Forage yields and search/track rates move ZERO — assert that
with a regression test on `CalcSearchScore`'s output. Commit.

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
// Keys are FILE:FUNC (spec §9), not files — a file-level allowlist hides new
// unowned sites inside allowlisted files, and steal.go/plant.go each hold four
// contests. Derive func names from the enclosing FuncDecl during the AST walk.
var contestSiteOwners = map[string]string{
	"internal/combat/defence_multiplier.go:resolveChannelAttackWithRunner": "the seam itself",
	"internal/combat/flee.go:ResolveFleeBlockers":                          "U6b task 12",
	"internal/combat/flee.go:fleeContestScore":                             "U6b task 12",
	"internal/hooks/Position_GrappleTick.go:processGrapplePair":            "U6b task 14",
	// Pre-assigned owners for the sites the review found OUTSIDE the family of
	// seven — the implementer must not invent owners mid-task:
	"internal/actions/defuse.go:...":                 "deliberate: trap-difficulty contest, converted U4",
	"internal/hooks/NewRound_MobRoundTick.go:...":    "U10c (charm-refresh; redesigned wholesale there)",
	"internal/actions/plant.go:...":                  "U6b task 15 (four sites)",
	"internal/actions/steal.go:...":                  "U6b task 15 (four sites incl. :509 observer pass)",
	"internal/actions/shadow.go:...":                 "U6b task 16",
	"internal/actions/sneak.go:...":                  "U6b task 16 (two sites)",
	"internal/usercommands/skill.skullduggery.shadow.go:...": "U6b task 16",
	// fill the real func names from the first guard run's failure output
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
//    The regex MUST tolerate spaces around the operator (`\* *25`): flee's
//    third literal is written `* 25` and a tight pattern misses it, which is
//    exactly how the plan's own first draft missed it.
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
- [ ] **Parity damage-per-swing ±10%** (arc completion criterion 5), in GO,
  not only in the model: a Python model extension can only reproduce the model
  and can never detect a Go-side leak from Tasks 1–2. Write a Go statistical
  test in `internal/combat`: N=200,000 simulated melee swings at a fixed
  matchup (light/mid/BIS mitigation cells), assert the empirical mean damage is
  within tolerance of the analytic value computed inside the test from the
  same shipped formula. A Task 1 bar leak or a Task 2 gate divergence moves
  the empirical mean; the analytic anchor does not. Keep the model
  cross-check as a second opinion, not the check.
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
- Named nerfs/changes that land quietly if unwatched, all in their task's
  commit AND the PR body: taunt crit AND base damage (Assumption 8), shieldless
  channel defence (Task 2), the specials fumble abort (Assumption 11), the
  defender-side cost/progression economy on specials and counters (Tasks 7 and
  10), and the opposed-stealth curve reshape (Task 16).
- **This plan was corrected by a blind adversarial review before execution**
  (16 findings; the header note names the largest). Where a task says "an
  earlier draft did X", that is a trap that was already sprung once — do not
  "simplify" the guard back out.
