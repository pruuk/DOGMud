# U7b: Reservation Ceiling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cap total reservation on every pool at 66% of its max, refuse the breaching action rather than letting it succeed and clamp, and rebuild the companion power model so a pet's multiplier drives its pool, its price and its reservation together.

**Architecture:** A new `internal/characters/reservation.go` owns the cap: the cap value, a "would this addition breach it" predicate, a before/after overage snapshot that makes grandfathering fall out of the arithmetic, and the per-item reservation helpers the enforcement sites need. Every path that can raise reservation (equip, enchant pre-flight, enchant tier-up, summon, conjure, raise, charm, the two auto-spawn paths, the login backfill) consults it. Alongside that, `CalcCompanionPool` replaces the old base-pool-times-scale formula with `(charisma + manifestation x 5)` scaled by a per-pet multiplier applied AFTER the corpse average, so pet tiers stay proportionally separated at every corpse size.

**Tech Stack:** Go, `internal/characters` (pools, reservation, companions), `internal/hooks` (summon, charm, auto-spawn, round tick), `internal/usercommands` (craft, assess, status), `internal/spells` (SpellData), `internal/configs` (balance knobs), `internal/templates` (status rendering), `_datafiles/world/dogmud/spells/`, `_datafiles/world/dogmud/mobs/summons/`.

**Design authority:** `docs/superpowers/specs/2026-08-15-u7b-reservation-ceiling-design.md`. Decisions D1 to D16 in that document are settled and are not relitigated here. Where this plan extends the spec (three places, all flagged in "Decisions the spec left open" at the end), it says so explicitly.

---

## Numbers this plan ships

| Knob / value | Value | Why |
|---|---|---|
| `PoolReservationCapPct` | **0.66** | D1. New knob. Applies per pool to all three (D2). At 0.66 a fully geared companion still reads as having a third of its pool, which is the magnitude the raw-max sweep in Task 12 is sized against. |
| `CompanionReserveDefault` | 280 (unchanged) | Now the **base** the pet multiplier scales, not a flat tier. Charm keeps multiplier 1.0, so charm's price does not move. |
| Manifestation coefficient in `CalcCompanionPool` | **5** | `B = Charisma + manifestation x 5`. The spec names this as the lever to pull if newer summoners feel weak in playtest, ahead of the multipliers. |

### Pet multipliers, cast costs and derived reservation (D7, D8, D9)

Reservation is **derived**, never authored: `round(CompanionReserveDefault x petMultiplier)`.

| Family | Spell file | Mob | `summon_pet_multiplier` | `cost` (CP) | Derived reserve |
|---|---|---|---|---|---|
| Conjure | `conjure-magma.yaml` | 314 | 1.25 | 50 | 350 |
| Conjure | `conjure-earth.yaml` | 311 | 1.05 | 45 | 294 |
| Conjure | `conjure-fire.yaml` | 313 | 1.00 | 45 | 280 |
| Conjure | `conjure-air.yaml` | 312 | 0.90 | 40 | 252 |
| Conjure | `conjure-water.yaml` | 310 | 0.75 | 30 | 210 |
| Raise | `raise-golem.yaml` | 305 | 1.00 | 50 | 280 |
| Raise | `raise-vampire.yaml` | 304 | 0.83 | 45 | 232 |
| Raise | `raise-spectre.yaml` | 303 | 0.75 | 40 | 210 |
| Raise | `raise-zombie.yaml` | 301 | 0.67 | 35 | 188 |
| Raise | `raise-wraith.yaml` | 302 | 0.58 | 35 | 162 |
| Raise | `raise-skeleton.yaml` | 300 | 0.50 | 30 | 140 |
| Summon | `summon-steppe-spirit.yaml` | 243 | 0.75 | 35 | 210 |
| Summon | `summon-hive-swarm.yaml` | 111 | 0.30 | 30 | 84 |

### The formula, checked against the spec's own table

`B = Charisma + manifestation x 5`. Conjured: `round(B x mult)`. Raised: `round(((B + corpsePool) / 2) x mult)`.

The spec's expected-outcome table is internally consistent with **B = 406** and Go's `math.Round` (half away from zero):

| Corpse pool | golem (1.00) | skeleton (0.50) | Spec says |
|---|---|---|---|
| 100 | 253 | 253 x 0.5 = 126.5 -> **127** | 253 / 126 |
| 400 | 403 | 201.5 -> **202** | 403 / 202 |
| 1000 | 703 | 351.5 -> **352** | 703 / 352 |
| 2800 | 1603 | 801.5 -> **802** | 1603 / 802 |

Three of the four skeleton rows round **up**, so `math.Round` is the intended rounding and the spec's "126" is an arithmetic slip that should read 127. The magma crossover confirms it independently: `406 x 1.25 = 507.5 -> 508`, which is the spec's stated conjure-magma figure, and `(406 + 609) / 2 = 507.5` is the stated 609-pool crossover. **Use `math.Round`. Do not use truncation.**

### What the cap does to the only affected character

Meirok sits at 78.2% conviction reservation today (351 of 503). Under D9 his two golems rebase from a flat 352 to `280 x 1.00 = 280`, reserving 126 each instead of 158. With the Shadowweave ward that is roughly 292 of 503, about **58%**, under the 66% cap with both golems kept and before either inverse-skill rider is applied. **No migration pain and no forced dismissal.**

---

## File structure

| File | Responsibility |
|---|---|
| `internal/characters/reservation.go` (new) | The whole cap surface: `ReservationCap`, `WouldBreachReservationCap`, `ReservationOverages` / `ReservationSnapshot.Worsened`, `ItemReserveOnPool`, `EnchantReserveAt`, `ReserveShareBand`, `ReservationBandName`, `ReservationRefusal` |
| `internal/characters/reservation_cap_test.go` (new) | Cap arithmetic, grandfathering, band edges |
| `internal/characters/validate.go` | `GetPoolReservation` delegates per item to `ItemReserveOnPool`; the enchanting-rank rider lands there (D10 §4.2) |
| `internal/characters/companions.go` | `CalcCompanionPool` (new formula, D6); `CalcSpawnPoolFromBase` (renamed old formula, behaviour-tree adds only); `CalcCompanionReserve` gains the U7 rider (D10 §4.1); `CanAffordCompanion` deleted (D5) |
| `internal/characters/worn.go` | `Wear` refuses a breaching equip and reverts placement |
| `internal/spells/spells.go` | `SummonPetMultiplier` replaces `SummonBasePool`; `SummonScalingDivisor` and `SummonConvictionReserve` deleted (D12) |
| `internal/hooks/companion_summon.go` | Derived reserve, new pool formula, cap gate before component/corpse consumption |
| `internal/hooks/charm_spell.go` | Cap gate replaces `CanAffordCompanion` |
| `internal/hooks/manifester_companions.go` | Brood-mother auto-spawn gains the gate it never had |
| `internal/hooks/chrysifier_homunculus.go` | Homunculus auto-spawn gains the gate it never had |
| `internal/hooks/companion_reserve_backfill.go` | Backfill respects the cap; adds the D11 recompute |
| `internal/hooks/PlayerSpawn_HandleJoin.go` | Calls the recompute on login |
| `internal/hooks/NewRound_UserRoundTick.go` | Enchant tier-up skips when it would breach, and says why (D14) |
| `internal/usercommands/craft.go` | Enchant pre-flight cap gate |
| `internal/usercommands/assess.go` | `reserveShareBand` delegates to `characters.ReserveShareBand`; the raise disclosure drops `CanAffordCompanion` |
| `internal/usercommands/equip.go`, `internal/mobcommands/equip.go`, `internal/actions/sell.go` | Stop discarding the failure reason |
| `internal/behaviortree/actions_mob.go` | Calls the renamed `CalcSpawnPoolFromBase` |
| `internal/combat/ai.go`, `internal/behaviortree/actions_archer.go` | The four raw-max reads move to `EffectivePoolMax` |
| `internal/behaviortree/conditions_mob.go`, `internal/behaviortree/action_cast_best_in_category.go` | **Comment corrections only** |
| `internal/templates/templatesfunctions.go` | `reservationQuality` template function |
| `_datafiles/world/dogmud/templates/character/status.template` | The reservation readout row (D15) |
| `_datafiles/world/dogmud/spells/*.yaml` (13 files) | Pet multipliers, cast costs, dead-field removal |
| `_datafiles/world/dogmud/mobs/summons/*.yaml` (5 files) | Behaviour fixes (D13) |
| `_datafiles/world/dogmud/templates/help/*.template` | Companion, enchanting and conviction helpfiles reflect the cap |
| `docs/PATCH_NOTES.md`, `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md` | Paperwork |

> ⚠️ **`_datafiles/config.yaml` is NOT in this table and must not be edited by any task.** It carries `skip-worktree` and the owner's local overrides. Every knob this plan adds ships with a Go default and is **absent** from `config.yaml`, so it falls through to that default and the game behaves as designed without any config edit. Task 14 writes out exactly what the owner should paste in, and the **controller applies it**, not the implementer.

---

### Task 1: The reservation cap primitive

**This is the foundation and nothing else can be built without it.** The cap has to answer three different shapes of question: "what is the ceiling" (readout), "would adding N breach it" (summon, charm, tier-up, all of which know their delta up front), and "did that placement make things worse" (equip, which cannot know its delta until after the slot resolves and displacement is known).

The third shape is what makes grandfathering (D4) fall out for free. Comparing overage BEFORE against overage AFTER means a character already over the cap can still swap one reserving ring for an equally reserving ring, and is refused only when the overage genuinely grows. A naive "is total over the cap" check would refuse that swap and would effectively force the character to strip.

**Files:**
- Create: `internal/characters/reservation.go`
- Create: `internal/characters/reservation_cap_test.go`
- Modify: `internal/configs/config.balance.go` (add one field near the ENCHANTMENTS block at :452)
- Modify: `internal/configs/config.balance.spells.go` (default it, ENCHANTMENTS block at :46)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/reservation_cap_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Setup note (mirrors effective_pool_test.go): no config file is loaded in this
// package's tests, so every Balance multiplier feeding RecalculateStats' pool-max
// derivation is 0. RecalculateStats only writes .Mods, so setting .Base gives a
// deterministic post-Validate() pool max.

// The cap is a flat fraction of the pool's max, per pool.
func TestReservationCap_IsAFractionOfMax(t *testing.T) {
	c := New()
	c.StaminaMax.Base = 100
	c.HealthMax.Base = 400
	c.ConvictionMax.Base = 500
	c.Validate()

	for _, tc := range []struct {
		pool Pool
		want int
	}{
		{PoolStamina, 66},
		{PoolHealth, 264},
		{PoolConviction, 330},
	} {
		if got := c.ReservationCap(tc.pool); got != tc.want {
			t.Errorf("ReservationCap(%s) = %d, want %d", tc.pool, got, tc.want)
		}
	}
}

// An addition that would carry total reservation past the cap must be reported
// as a breach; one that lands inside it must not.
func TestWouldBreachReservationCap_Boundary(t *testing.T) {
	c := New()
	c.StaminaMax.Base = 100
	c.Validate()

	if c.WouldBreachReservationCap(PoolStamina, 66) {
		t.Errorf("adding exactly the cap must be allowed, not refused")
	}
	if !c.WouldBreachReservationCap(PoolStamina, 67) {
		t.Errorf("adding one past the cap must be refused")
	}
	// A zero or negative addition is never a breach: removing gear must never
	// be blocked by the ceiling, however far over the character already is.
	if c.WouldBreachReservationCap(PoolStamina, 0) {
		t.Errorf("a zero addition must never breach")
	}
	if c.WouldBreachReservationCap(PoolStamina, -50) {
		t.Errorf("a negative addition must never breach")
	}
}

// D4 grandfathering. A character ALREADY past the cap keeps everything they
// have, and is refused only ADDITIONS. Nothing here may force a removal.
func TestWouldBreachReservationCap_GrandfathersTheAlreadyOver(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999930: {ItemId: 999930, Name: "greedy collar", Type: items.Neck, ReserveStaminaPct: 0.80},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999930)
	c.Validate()

	if res := c.GetPoolReservation("stamina", 100); res != 80 {
		t.Fatalf("fixture: reservation = %d, want 80", res)
	}
	if !c.WouldBreachReservationCap(PoolStamina, 1) {
		t.Errorf("an over-cap character must be refused every addition")
	}
	if c.WouldBreachReservationCap(PoolStamina, 0) {
		t.Errorf("an over-cap character must NOT be forced to shed anything")
	}
}

// The overage snapshot is the seam the equip path uses. It must report zero for
// a pool inside the cap, the excess for one outside it, and Worsened must fire
// only when a pool's overage GREW -- which is what lets an over-cap character
// swap one reserving item for an equal one.
func TestReservationOverages_WorsenedOnlyOnGrowth(t *testing.T) {
	before := ReservationSnapshot{Health: 0, Stamina: 10, Conviction: 0}

	same := ReservationSnapshot{Health: 0, Stamina: 10, Conviction: 0}
	if _, worse := before.Worsened(same); worse {
		t.Errorf("an unchanged overage must not be reported as worse")
	}

	better := ReservationSnapshot{Health: 0, Stamina: 4, Conviction: 0}
	if _, worse := before.Worsened(better); worse {
		t.Errorf("a shrinking overage must not be reported as worse")
	}

	worseSnap := ReservationSnapshot{Health: 0, Stamina: 11, Conviction: 0}
	p, worse := before.Worsened(worseSnap)
	if !worse || p != PoolStamina {
		t.Errorf("a growing stamina overage must report (PoolStamina, true), got (%s, %v)", p, worse)
	}

	// A pool that goes from inside the cap to outside it is the ordinary case.
	fresh := ReservationSnapshot{}
	p, worse = fresh.Worsened(ReservationSnapshot{Health: 3})
	if !worse || p != PoolHealth {
		t.Errorf("a newly-breached health pool must report (PoolHealth, true), got (%s, %v)", p, worse)
	}
}

// The short band vocabulary used by the status sheet. Every label must fit the
// 13-character column the template reserves for it, and the top band must key
// off the CAP rather than a fixed fraction, so a player can see when they have
// no room left to add.
func TestReservationBandName_ShortVocabulary(t *testing.T) {
	c := New()
	c.StaminaMax.Base = 100
	c.Validate()

	if got := c.ReservationBandName("stamina"); got != "none" {
		t.Errorf("an unreserved pool = %q, want \"none\"", got)
	}

	for _, tc := range []struct {
		reserve int
		want    string
	}{
		{0, "none"},
		{10, "slight"},
		{20, "modest"},
		{40, "notable"},
		{60, "heavy"},
		{66, "at limit"},
		{90, "at limit"},
	} {
		if got := reservationBand(tc.reserve, 100, 66); got != tc.want {
			t.Errorf("reservationBand(%d, 100, 66) = %q, want %q", tc.reserve, got, tc.want)
		}
		if len(tc.want) > 13 {
			t.Errorf("band %q is %d chars and will break the status box", tc.want, len(tc.want))
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run 'TestReservationCap|TestWouldBreach|TestReservationOverages|TestReservationBandName' -v`
Expected: FAIL to build, `undefined: ReservationSnapshot`, `undefined: reservationBand`, and `c.ReservationCap undefined (type *Character has no field or method ReservationCap)`.

- [ ] **Step 3: Add the config knob**

In `internal/configs/config.balance.go`, immediately ABOVE the `// ── ENCHANTMENTS ──` comment at line 452, add:

```go
	// ── POOL RESERVATION ─────────────────────────────────────────────────────
	PoolReservationCapPct ConfigFloat `yaml:"PoolReservationCapPct"` // Ceiling on TOTAL reservation per pool, as a fraction of that pool's max (default 0.66)
```

In `internal/configs/config.balance.spells.go`, immediately ABOVE the `// ── ENCHANTMENTS ──` block at line 46, add:

```go
	// ── POOL RESERVATION ─────────────────────────────────────────────────────
	if b.PoolReservationCapPct <= 0 || b.PoolReservationCapPct > 1 {
		// 0.66 (U7b). Both guards matter: <= 0 is "absent from config.yaml", and
		// > 1 would make the cap unreachable and silently disable the ceiling,
		// which is the one failure mode nobody would notice until U8 shipped.
		b.PoolReservationCapPct = 0.66
	}
```

- [ ] **Step 4: Implement the reservation surface**

Create `internal/characters/reservation.go`:

```go
package characters

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/enchantments"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// ── The ceiling ──────────────────────────────────────────────────────────────
//
// Total reservation on a pool is capped at PoolReservationCapPct of that pool's
// max, per pool, for players and mobs alike (GetPoolReservation has never had an
// IsMob gate, and companions carrying enchanted gear reserve on prod today).
//
// The cap REFUSES the breaching action rather than letting it succeed and clamp
// (D3), and it never forces a removal (D4): a character already past the cap
// keeps everything they have and is refused only additions.
//
// This exists because U8 introduces a skill-strip on insufficient resource,
// which turns an over-reserved pool from a cosmetic oddity into a permanent
// crippling penalty. 96% health reservation is reachable with shipped gear and
// no mutation today; 120% with one rank of Extra Arms.

// ReservationCap returns the maximum total reservation permitted on a pool.
func (c *Character) ReservationCap(p Pool) int {
	pct := float64(configs.GetBalanceConfig().PoolReservationCapPct)
	if pct <= 0 || pct > 1 {
		pct = 0.66
	}
	return int(math.Floor(float64(c.poolMax(p)) * pct))
}

// WouldBreachReservationCap reports whether adding `added` points of reservation
// to pool p would carry the total past the cap.
//
// Grandfathering falls out of the arithmetic rather than needing a branch: an
// already-over character fails `current + added > cap` for every positive
// addition and passes it for every zero or negative one, so they can shed gear
// freely and add nothing. Do not "simplify" this into a `current > cap` check.
func (c *Character) WouldBreachReservationCap(p Pool, added int) bool {
	if added <= 0 {
		return false
	}
	max := c.poolMax(p)
	return c.GetPoolReservation(string(p), max)+added > c.ReservationCap(p)
}

// ReservationSnapshot records how far PAST the cap each pool sits. A pool inside
// the cap records 0, never a negative headroom figure: the only question any
// caller asks of it is "did this get worse", and signed headroom would make an
// unrelated pool's improvement mask another pool's breach.
type ReservationSnapshot struct {
	Health     int
	Stamina    int
	Conviction int
}

// ReservationOverages snapshots the current overage on all three pools.
func (c *Character) ReservationOverages() ReservationSnapshot {
	return ReservationSnapshot{
		Health:     c.reservationOverage(PoolHealth),
		Stamina:    c.reservationOverage(PoolStamina),
		Conviction: c.reservationOverage(PoolConviction),
	}
}

func (c *Character) reservationOverage(p Pool) int {
	max := c.poolMax(p)
	if over := c.GetPoolReservation(string(p), max) - c.ReservationCap(p); over > 0 {
		return over
	}
	return 0
}

// Worsened reports the first pool whose overage GREW between two snapshots.
//
// This, and not a plain "is it over the cap" test, is what the equip path must
// use. Equipping displaces, so the delta is only knowable after the slot
// resolves; and D4 grandfathering means a character already over must still be
// able to swap one reserving ring for an equally reserving one. A cap test would
// refuse that swap and effectively force them to strip.
func (before ReservationSnapshot) Worsened(after ReservationSnapshot) (Pool, bool) {
	if after.Health > before.Health {
		return PoolHealth, true
	}
	if after.Stamina > before.Stamina {
		return PoolStamina, true
	}
	if after.Conviction > before.Conviction {
		return PoolConviction, true
	}
	return "", false
}

// ── Per-item reservation ─────────────────────────────────────────────────────

// ItemReserveOnPool returns what a single equipped item contributes to the
// reservation on `p`. GetPoolReservation is the sum of this over the equipment
// set; the enforcement sites need the per-item figure so they can price a swap
// or a tier-up without re-totalling.
//
// A single item can contribute through BOTH mechanisms at once (a Chrysalis
// enchantment on an item whose spec ALSO carries a reserve_*_pct) and the
// contributions intentionally stack. That is by design, not a leftover.
func (c *Character) ItemReserveOnPool(itm items.Item, p Pool) int {
	pool := string(p)
	poolMax := c.poolMax(p)
	spec := itm.GetSpec()
	total := 0

	if itm.HasChrysalisEnchantment() && itm.ReservePool == pool {
		total += c.EnchantReserveAt(itm.EnchantType, itm.EnchantTier, spec.Hands, p)
	}

	var itemPct float64
	switch pool {
	case "health":
		itemPct = spec.ReserveHealthPct
	case "stamina":
		itemPct = spec.ReserveStaminaPct
	case "conviction":
		itemPct = spec.ReserveConvictionPct
	}
	if itemPct > 0 {
		total += int(math.Floor(float64(poolMax) * itemPct))
	}
	return total
}

// EnchantReserveAt returns what a Chrysalis enchantment of the given type and
// tier reserves on pool p for THIS character, with the wearer's enchanting-rank
// rider applied (D10 §4.2).
//
// The rider is costs.SkillCostMultiplier, the same inverse-skill band U7 built.
// Its penalty half applies deliberately: a tier-4 8% enchant costs 8.8% at
// enchanting 0, 8.0% at 25, 6.1% at 54 and 3.2% at 100. The rider is applied to
// the PERCENTAGE before the floor, not to the floored points, so it cannot be
// rounded away on a small pool.
//
// It scales the ENCHANTMENT contribution only. Pinnacle-item reserve_*_pct is
// deliberately left flat: that reservation is the item's price, not a piece of
// craft the wearer's skill has any purchase on.
func (c *Character) EnchantReserveAt(enchantType string, tier int, hands int, p Pool) int {
	pct := enchantments.GetTierReservePct(enchantType, tier, hands)
	if pct <= 0 {
		return 0
	}
	pct *= costs.SkillCostMultiplier(c.GetSkillLevel(skills.Enchanting))
	return int(math.Floor(float64(c.poolMax(p)) * pct))
}

// ── Player-facing bands ──────────────────────────────────────────────────────
//
// TWO vocabularies, deliberately. ReserveShareBand is a PROSE fragment that has
// to read inside a sentence ("your gear is holding a heavy share of your
// stamina"). ReservationBandName is a single SHORT word for the status sheet's
// 13-character column, and its top band keys off the CAP so a player can see
// when they have no room left to add. Do not merge them: the prose phrases do
// not fit the column, and the column words do not read as prose.

// ReserveShareBand names what SHARE of a pool a reservation holds. Player-facing
// text never shows the raw number.
func ReserveShareBand(reserve, maxPool int) string {
	if maxPool <= 0 || reserve >= maxPool {
		return `all`
	}
	frac := float64(reserve) / float64(maxPool)
	switch {
	case frac < 0.15:
		return `a small part`
	case frac < 0.30:
		return `a modest share`
	case frac < 0.50:
		return `a significant portion`
	case frac < 0.75:
		return `a heavy share`
	default:
		return `nearly all`
	}
}

// ReservationBandName returns the short status-sheet word for a pool's current
// reservation. `pool` is a plain string so the template can call it directly.
func (c *Character) ReservationBandName(pool string) string {
	p := Pool(pool)
	max := c.poolMax(p)
	return reservationBand(c.GetPoolReservation(pool, max), max, c.ReservationCap(p))
}

func reservationBand(reserve, maxPool, cap int) string {
	if maxPool <= 0 || reserve <= 0 {
		return `none`
	}
	if cap > 0 && reserve >= cap {
		return `at limit`
	}
	frac := float64(reserve) / float64(maxPool)
	switch {
	case frac < 0.15:
		return `slight`
	case frac < 0.30:
		return `modest`
	case frac < 0.50:
		return `notable`
	default:
		return `heavy`
	}
}

// ReservationRefusal is the message every refusing path sends. It names
// RESERVATION as the cause rather than exhaustion or a generic failure, because
// a player has no other way to learn that their own gear is the obstacle and
// resting will never fix it. Bands only, never a number, matching the disclosure
// style `stand` and `assess` already use.
func (c *Character) ReservationRefusal(p Pool) string {
	max := c.poolMax(p)
	share := ReserveShareBand(c.GetPoolReservation(string(p), max), max)
	return `Your gear already holds ` + share + ` of your ` + poolDisplayName(p) +
		` in reserve. You cannot take on more until you set something else aside.`
}

func poolDisplayName(p Pool) string {
	switch p {
	case PoolHealth:
		return `health`
	case PoolStamina:
		return `stamina`
	case PoolConviction:
		return `conviction`
	}
	return `strength`
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/characters/ -run 'TestReservationCap|TestWouldBreach|TestReservationOverages|TestReservationBandName' -v`
Expected: PASS, all five tests.

- [ ] **Step 6: Confirm no import cycle and the package still builds**

Run: `go build ./internal/characters/ && go vet ./internal/characters/`
Expected: no output. `internal/costs` imports only `internal/configs` and `internal/skills`, neither of which imports `internal/characters`, so `characters -> costs` is a legal new edge. If the build reports a cycle, stop and report it rather than moving the helper.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/reservation.go internal/characters/reservation_cap_test.go internal/configs/config.balance.go internal/configs/config.balance.spells.go
git commit -m "feat(characters): the pool reservation ceiling

Total reservation on a pool is now capped at a configurable fraction of
that pool's max, per pool, for players and companions alike. The breaching
action is refused rather than allowed to succeed and clamp, and a character
already past the cap is grandfathered: they keep everything they have and
are refused only additions.

Grandfathering falls out of the arithmetic rather than needing a branch.
WouldBreachReservationCap tests current + added > cap, which an over-cap
character fails for every positive addition and passes for every removal.
The equip path needs a different shape again, because displacement means
its delta is only knowable after the slot resolves, so ReservationSnapshot
compares overage before against overage after and refuses only genuine
growth. A plain over-the-cap test there would refuse an over-cap character
swapping one reserving ring for an equally reserving one, which is exactly
the forced strip D4 rules out.

Enchantment reservation now takes the wearer's enchanting rank through the
U7 inverse-skill band, applied to the percentage before the floor so it
cannot be rounded away on a small pool.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `summon_pet_multiplier` replaces the base pool

**Files:**
- Modify: `internal/spells/spells.go` (fields at :45-53, validation at :253-258)
- Test: `internal/spells/summon_fields_test.go` (create)

Three fields go and one arrives. `summon_scaling_divisor` has **never been read** by any production code: it is declared in the struct and present in all thirteen summon YAMLs and has zero readers (spec trap 5). `summon_conviction_reserve` goes because D9 derives reservation from the pet multiplier, and leaving an authorable override beside a derived value is a second source of truth that will drift. `summon_base_pool` is replaced outright by D12.

- [ ] **Step 1: Write the failing test**

Create `internal/spells/summon_fields_test.go`:

```go
package spells

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// The pet multiplier is the single dial: it drives the companion's pool, and
// (via CompanionReserveDefault) its ongoing reservation. summon_base_pool,
// summon_scaling_divisor and summon_conviction_reserve are gone -- the first
// replaced, the second never read by anything, the third now derived.
func TestSpellData_SummonPetMultiplierParses(t *testing.T) {
	var sd SpellData
	in := "spellid: test-summon\nsummon_mob_id: 300\nsummon_pet_multiplier: 0.5\n"
	if err := yaml.Unmarshal([]byte(in), &sd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sd.SummonPetMultiplier != 0.5 {
		t.Fatalf("SummonPetMultiplier = %v, want 0.5", sd.SummonPetMultiplier)
	}
}

// A summon spell with no multiplier is an authoring error the loader must warn
// about, exactly as it used to warn about a missing base pool.
func TestSpellData_ValidateWarnsOnMissingPetMultiplier(t *testing.T) {
	sd := SpellData{SpellId: "test-summon", SummonMobId: 300}
	if err := sd.Validate(); err != nil {
		t.Fatalf("Validate returned an error, want nil: %v", err)
	}
	// Validate warns rather than failing; this test pins that it does not panic
	// and does not start returning an error, which would take the server down at
	// boot on an authoring slip that has always been a warning.
}

// The retired fields must be gone from the struct. A YAML file still carrying
// them parses (the loader is non-strict) but must not populate anything.
func TestSpellData_RetiredSummonFieldsAreGone(t *testing.T) {
	src := yamlTagsOf(SpellData{})
	for _, dead := range []string{"summon_base_pool", "summon_scaling_divisor", "summon_conviction_reserve"} {
		if strings.Contains(src, dead) {
			t.Errorf("SpellData still declares %q; it must be deleted, not left unread", dead)
		}
	}
}
```

Add the reflection helper at the bottom of the same file:

```go
func yamlTagsOf(v any) string {
	t := reflect.TypeOf(v)
	var b strings.Builder
	for i := 0; i < t.NumField(); i++ {
		b.WriteString(string(t.Field(i).Tag))
		b.WriteString("\n")
	}
	return b.String()
}
```

and add `"reflect"` to the file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/spells/ -run TestSpellData_Summon -v`
Expected: FAIL, `sd.SummonPetMultiplier undefined (type SpellData has no field or method SummonPetMultiplier)`.

- [ ] **Step 3: Swap the fields**

In `internal/spells/spells.go`, replace lines 44-53 with:

```go
	// Companion summoning fields: replaces JS onMagic for summon spells
	SummonMobId int `yaml:"summon_mob_id,omitempty"`
	// SummonPetMultiplier is the single dial for this pet's tier. It scales the
	// caster's own power into the companion's stat pool (see
	// characters.CalcCompanionPool), and multiplies CompanionReserveDefault to
	// give the ongoing Conviction the companion reserves.
	//
	// It replaced summon_base_pool in U7b. Under the old shape the pet's base
	// pool MULTIPLIED the caster's power and the corpse was averaged in
	// afterwards, so the corpse's share grew until it swamped the pet choice: at
	// a 1000-pool corpse a skeleton fielded 587 and a golem 675, meaning five
	// times the price bought 15% more pet. Applying the multiplier AFTER the
	// average keeps every tier proportionally separated at every corpse size.
	SummonPetMultiplier  float64 `yaml:"summon_pet_multiplier,omitempty"`
	SummonComponentId    int     `yaml:"summon_component_id,omitempty"`
	SummonRequiresCorpse bool    `yaml:"summon_requires_corpse,omitempty"`
	SummonMinCorpsePool  int     `yaml:"summon_min_corpse_pool,omitempty"`
```

Note what is NOT there any more: `SummonBasePool`, `SummonScalingDivisor` (never read by anything in production) and `SummonConvictionReserve` (now derived from the multiplier, so an authorable value beside it would be a second source of truth).

- [ ] **Step 4: Update the loader validation**

In the same file, replace the summon validation at lines 252-255 with:

```go
	// Validate summon fields
	if s.SummonMobId > 0 && s.SummonPetMultiplier <= 0 {
		mudlog.Warn("Spell.Validate", "spellId", s.SpellId, "warning", "summon_mob_id set but summon_pet_multiplier is 0 or missing")
	}
```

Leave the `SummonRequiresCorpse` / `SummonMinCorpsePool` check at :256-258 exactly as it is.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/spells/ -run TestSpellData_Summon -v`
Expected: PASS, all three.

- [ ] **Step 6: Confirm the compiler names every stale reader**

Run: `go build ./... 2>&1 | head -30`
Expected: errors in exactly three files, which the next tasks fix:
- `internal/hooks/companion_summon.go` (`spellData.SummonBasePool`, `spellData.SummonConvictionReserve`)
- `internal/hooks/companion_reserve_backfill.go` (`sp.SummonConvictionReserve`)
- `internal/usercommands/assess.go` (`sp.SummonConvictionReserve`)

If the compiler names a fourth file, **stop and report it** rather than patching it blind: the field inventory in this plan was taken on 2026-08-15 and a fourth reader means something landed since.

- [ ] **Step 7: Commit**

This commit leaves the tree red on purpose is NOT acceptable, so fold it into Task 4's commit instead. Mark this step done without committing, and carry `internal/spells/` into Task 4's `git add`.

> **Why Tasks 2, 3 and 4 share one commit.** Deleting a struct field breaks its readers by construction, and the readers are the very functions Tasks 3 and 4 rewrite. Splitting them produces a red intermediate commit, which this plan's "no commit is red" rule forbids. The three tasks stay separate for reviewability and each has its own tests; only the commit is shared.

---

### Task 3: The companion power formula

**Files:**
- Modify: `internal/characters/companions.go` (:147-165)
- Modify: `internal/behaviortree/actions_mob.go` (:78)
- Test: `internal/characters/companions_test.go` (replace `TestCalcCompanionStatPool` and `TestCalcRaisedStatPool`)

- [ ] **Step 1: Write the failing test**

In `internal/characters/companions_test.go`, DELETE `TestCalcCompanionStatPool` (lines 181-231) and `TestCalcRaisedStatPool` (lines 285-309) and add in their place:

```go
// ─── CalcCompanionPool ───────────────────────────────────────────────────────

// The numbers here are the spec's own expected-outcome table, which is
// internally consistent with B = 406 (Charisma 166 + manifestation 48 x 5).
//
// Rounding is math.Round, half away from zero. Three of the four skeleton rows
// in the spec's table land on a .5 and round UP, which is what pins it; the
// magma crossover confirms it independently (406 x 1.25 = 507.5 -> 508, the
// spec's stated conjure-magma figure). The spec's "126" for the 100-pool
// skeleton is an arithmetic slip and should read 127.
func TestCalcCompanionPool(t *testing.T) {
	const cha, manifest = 166, 48 // B = 166 + 240 = 406

	tests := []struct {
		name       string
		charisma   int
		manifest   int
		multiplier float64
		corpsePool int
		want       int
	}{
		// Conjures: no corpse, so the multiplier applies to B directly.
		{"conjure magma", cha, manifest, 1.25, 0, 508},
		{"conjure earth", cha, manifest, 1.05, 0, 426},
		{"conjure fire", cha, manifest, 1.00, 0, 406},
		{"conjure water", cha, manifest, 0.75, 0, 305}, // 304.5 -> 305
		{"hive swarm", cha, manifest, 0.30, 0, 122},    // 121.8 -> 122

		// Raises: the multiplier applies AFTER the corpse average, which is the
		// whole point of the reshape. Every tier stays proportionally separated
		// at every corpse size.
		{"golem on a trash corpse", cha, manifest, 1.00, 100, 253},
		{"skeleton on a trash corpse", cha, manifest, 0.50, 100, 127},
		{"golem on a boss corpse", cha, manifest, 1.00, 400, 403},
		{"skeleton on a boss corpse", cha, manifest, 0.50, 400, 202},
		{"golem on a rich corpse", cha, manifest, 1.00, 1000, 703},
		{"skeleton on a rich corpse", cha, manifest, 0.50, 1000, 352},
		{"golem on the Core Guardian", cha, manifest, 1.00, 2800, 1603},
		{"skeleton on the Core Guardian", cha, manifest, 0.50, 2800, 802},

		// A fresh summoner: no manifestation investment at all.
		{"novice conjures fire", 100, 0, 1.00, 0, 100},
		{"novice raises a skeleton", 100, 0, 0.50, 60, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcCompanionPool(tt.charisma, tt.manifest, tt.multiplier, tt.corpsePool)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Every pet tier must stay proportionally separated however big the corpse is.
// This is the property the reshape exists to deliver, and it is the one a future
// "simplification" back to a pre-average multiplier would silently destroy.
func TestCalcCompanionPool_TiersStaySeparatedAtEveryCorpseSize(t *testing.T) {
	const cha, manifest = 166, 48
	for _, corpse := range []int{0, 100, 500, 1000, 2800, 10000} {
		golem := CalcCompanionPool(cha, manifest, 1.00, corpse)
		skeleton := CalcCompanionPool(cha, manifest, 0.50, corpse)
		ratio := float64(golem) / float64(skeleton)
		if ratio < 1.99 || ratio > 2.01 {
			t.Errorf("corpse %d: golem/skeleton = %.4f, want 2.0 at every corpse size "+
				"(the multiplier must be applied AFTER the average, not before)", corpse, ratio)
		}
	}
}

// A zero or negative multiplier must not silently field a pool-zero companion.
func TestCalcCompanionPool_FloorsAtOne(t *testing.T) {
	if got := CalcCompanionPool(100, 0, 0, 0); got != 1 {
		t.Errorf("a zero multiplier = %d, want 1 (never a pool-zero companion)", got)
	}
	if got := CalcCompanionPool(0, 0, 1.0, 0); got != 1 {
		t.Errorf("a zero-charisma novice = %d, want 1", got)
	}
}

// ─── CalcSpawnPoolFromBase ───────────────────────────────────────────────────

// The behaviour-tree add scaler keeps the OLD shape and is renamed to say so.
// It is not the player companion formula and must not be confused for one: its
// callers are authored boss encounters whose base_pool values (50 for the Core
// Guardian's repair frames, 300 for the Sentinel) were tuned against exactly
// this curve.
func TestCalcSpawnPoolFromBase(t *testing.T) {
	// Config defaults apply when no config is loaded:
	//   ManifestStatScaleChaFactor   = 150
	//   ManifestStatScaleSkillFactor = 0.02
	// scale = 1.0 + 100/150 + 0*0.02 = 1.667  ->  50 * 1.667 = 83
	assert.Equal(t, 83, CalcSpawnPoolFromBase(50, 100, 0))
	// scale = 1.667  ->  300 * 1.667 = 500
	assert.Equal(t, 500, CalcSpawnPoolFromBase(300, 100, 0))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run 'TestCalcCompanionPool|TestCalcSpawnPoolFromBase' -v`
Expected: FAIL, `undefined: CalcCompanionPool` and `undefined: CalcSpawnPoolFromBase`.

- [ ] **Step 3: Implement**

In `internal/characters/companions.go`, replace the whole of `CalcCompanionStatPool` (lines 147-165) with:

```go
// CalcCompanionPool computes the stat pool for a player-summoned companion.
//
//	B    = charisma + manifestationSkill * ManifestationPoolCoefficient
//	pool = round(B * petMultiplier)                      (conjured, no corpse)
//	pool = round(((B + corpsePool) / 2) * petMultiplier) (raised, from a corpse)
//
// The multiplier is applied AFTER the corpse average, and that ordering is the
// whole point. Under the old shape the pet's base pool multiplied the caster's
// power and the corpse was averaged in afterwards, so the corpse's share grew
// until it swamped the pet choice: at a 1000-pool corpse a skeleton fielded 587
// and a golem 675, meaning five times the price bought 15% more pet. Applying
// the multiplier last keeps every tier proportionally separated at every corpse
// size, which is pinned by
// TestCalcCompanionPool_TiersStaySeparatedAtEveryCorpseSize. Do not "simplify"
// this by folding the multiplier into B.
//
// Known consequence, accepted: mid-level summoners lose roughly 20%, because
// manifestation x 5 is flat where the old term multiplied a base pool.
// High-skill summoners gain slightly. If newer summoners feel too weak in
// playtest, the lever is ManifestationPoolCoefficient or a flat constant added
// to B, NOT the per-pet multipliers -- moving those would flatten the tiers this
// function exists to separate.
//
// This is NOT CalcSpawnPoolFromBase, which is the behaviour-tree add scaler and
// keeps the old curve for authored boss encounters.
func CalcCompanionPool(charisma int, manifestationSkill int, petMultiplier float64, corpsePool int) int {
	base := float64(charisma + manifestationSkill*ManifestationPoolCoefficient)
	if corpsePool > 0 {
		base = (base + float64(corpsePool)) / 2.0
	}
	pool := int(math.Round(base * petMultiplier))
	if pool < 1 {
		// Never field a pool-zero companion: NewMobByIdFresh divides by the pool
		// when distributing stats, and every downstream ratio reads a zero pool
		// as "no penalty at all".
		return 1
	}
	return pool
}

// ManifestationPoolCoefficient is how much one rank of manifestation adds to a
// companion's power base. Deliberately a constant and not a config knob: it is
// the shape of the formula rather than a tuning dial, and the spec names it as
// the FIRST lever to reach for if the playtest says newer summoners are weak, at
// which point it earns a knob.
const ManifestationPoolCoefficient = 5

// CalcSpawnPoolFromBase scales an AUTHORED base pool by the spawner's Charisma
// and manifestation skill.
//
//	scale  = 1.0 + charisma/chaFactor + manifestationSkill*skillFactor
//	result = round(baseStatPool * scale)
//
// This is the BEHAVIOUR-TREE add scaler and its only caller is
// behaviortree.actSummonCompanion. It is NOT the player companion formula --
// that is CalcCompanionPool, which U7b reshaped.
//
// It deliberately keeps the old curve. Its callers are authored boss encounters
// whose base_pool values were tuned against exactly this shape: the Core
// Guardian and Warden Prime summon repair frames at base_pool 50, Old Edrin at
// 60, and the Sentinel at 300. Putting them on the companion formula would nerf
// the Sentinel's adds roughly fivefold and buff the Core Guardian's by about a
// fifth, neither of which U7b intends.
//
// Config knobs: ManifestStatScaleChaFactor (default 150, NOT 200 -- the old doc
// comment said 200 and the 200 in the fallback below is unreachable),
// ManifestStatScaleSkillFactor (default 0.02).
func CalcSpawnPoolFromBase(baseStatPool int, charisma int, manifestationSkill int) int {
	cfg := configs.GetBalanceConfig()
	chaFactor := float64(cfg.ManifestStatScaleChaFactor)
	skillFactor := float64(cfg.ManifestStatScaleSkillFactor)
	if chaFactor <= 0 {
		chaFactor = 150
	}
	scale := 1.0 + float64(charisma)/chaFactor + float64(manifestationSkill)*skillFactor
	return int(math.Round(float64(baseStatPool) * scale))
}
```

- [ ] **Step 4: Point the behaviour-tree caller at the renamed function**

In `internal/behaviortree/actions_mob.go` line 78, change:

```go
	pool := characters.CalcCompanionStatPool(basePool, charisma, manifestSkill)
```

to:

```go
	// CalcSpawnPoolFromBase, not CalcCompanionPool: this path spawns authored
	// boss adds whose base_pool values were tuned against the old curve, and it
	// is not a player companion (it reserves nothing and has no pet multiplier).
	pool := characters.CalcSpawnPoolFromBase(basePool, charisma, manifestSkill)
```

- [ ] **Step 5: Run the tests**

Run: `go test -count=1 ./internal/characters/ -run 'TestCalcCompanionPool|TestCalcSpawnPoolFromBase' -v`
Expected: PASS, all four.

Then run: `go build ./internal/behaviortree/`
Expected: no output.

- [ ] **Step 6: Fix the stale calibration test**

Run: `go test ./internal/characters/ 2>&1 | head -20`

`internal/characters/companion_calibration_test.go` references `CalcCompanionStatPool` and will fail to build. Read it, and rewrite each assertion against `CalcCompanionPool` with the new expected values computed from `B = charisma + manifestation x 5`. **Do not delete the file** and do not weaken an assertion to make it pass: if a case cannot be expressed under the new formula, say so in your report rather than dropping it.

> **Correction, found during Task 3:** this is wrong.
> `companion_calibration_test.go` does **not** reference `CalcCompanionStatPool`
> at all. It exercises the reserve-and-budget path only
> (`CalcCompanionReserve` plus `CanAffordCompanion`), so Task 3 leaves it
> building and passing untouched. It **will** break in Task 4, when
> `CanAffordCompanion` is deleted per D5, and that is where its rewrite belongs.
> Its `fieldN` helper is the only thing to change: point it at the new cap
> predicate instead of `CanAffordCompanion`. The `base` costs it passes (350,
> 440, 735) are per-type base reserve costs, which Task 4 rebases onto
> `CompanionReserveDefault x petMultiplier`, so the expected companion counts
> must be recomputed there rather than preserved.
>
> Two further Task 3 notes: deleting `TestCalcRaisedStatPool` orphans the
> `math` import in `companions_test.go`, which must be removed or the package
> will not build. And the doc comment the plan supplies for
> `CalcSpawnPoolFromBase` says "the 200 in the fallback below is unreachable"
> while the code it supplies sets that fallback to 150; the shipped comment was
> reworded to match the code.

Step 6 has no separate commit; Task 4 commits the three tasks together.

---

### Task 4: Companion reserve tracks the pet multiplier

**Files:**
- Modify: `internal/characters/companions.go` (`CalcCompanionReserve` at :170-178; delete `CanAffordCompanion` at :180-192)
- Test: `internal/characters/companions_test.go` (`TestCalcCompanionReserve_Calibration` at :150-177)
- Delete: `internal/characters/companion_afford_test.go`

Two changes land together because they are the same decision seen from two sides. D9 makes the ongoing budget track pet power instead of a flat companion count, and D5 removes `CanAffordCompanion` because the cap subsumes it. Keeping both would leave two different ceilings on the same pool: `CanAffordCompanion` is a 100% conviction-only cap (`CompanionCastingFloorPct` defaults to 0.0 and is absent from `config.yaml`, so it reduces to "must not exceed the max") sitting beside a 66% three-pool one, and the weaker of the two would never fire.

- [x] **Step 1: Write the failing test**

In `internal/characters/companions_test.go`, replace `TestCalcCompanionReserve_Calibration` (lines 150-177) with:

```go
// ─── CalcCompanionReserve ─────────────────────────────────────────────────────

// The base a companion reserves is now CompanionReserveDefault scaled by the
// pet's multiplier (D9), so the ongoing budget tracks pet POWER rather than
// being a flat two-tier charge shared across both families.
func TestCompanionReserveBase_TracksThePetMultiplier(t *testing.T) {
	for _, tt := range []struct {
		name       string
		multiplier float64
		want       int
	}{
		{"magma", 1.25, 350},
		{"earth", 1.05, 294},
		{"fire / golem", 1.00, 280},
		{"vampire", 0.83, 232},
		{"water / spectre / steppe spirit", 0.75, 210},
		{"zombie", 0.67, 188},
		{"wraith", 0.58, 162},
		{"skeleton", 0.50, 140},
		{"hive swarm", 0.30, 84},
		// Charm has no authored pet, so it reserves the unscaled default. Its
		// price therefore does not move in U7b.
		{"charm (no pet)", 1.00, 280},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CompanionReserveBase(tt.multiplier))
		})
	}
}

// CalcCompanionReserve composes the U7 inverse-skill band ON TOP OF the existing
// manifestation reduction (D10 §4.1). Compose, never replace: the U7 curve
// bottoms at 0.40 while the existing reduction already reaches 0.45 at
// manifestation 55 and 0.21 with the Manifester mutation, so a replacement would
// make companions DEARER for everyone, the exact opposite of intent.
func TestCalcCompanionReserve_ComposesTheInverseSkillRider(t *testing.T) {
	seedMut := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"broodmaster": {
			MutationId: "broodmaster", Name: "Broodmaster", Rarity: 5, Pole: "belief",
			Pros: []mutations.MutationEffect{{Type: "companion_reserve_reduction"}},
		},
	})
	defer seedMut()

	// Rank 0: the rider's PENALTY half applies, deliberately and consistently
	// with the item side. A rank-0 summoner pays 1.10x, so 280 -> 308.
	novice := New()
	novice.Skills[string(skills.Manifestation)] = 0
	assert.Equal(t, 308, novice.CalcCompanionReserve(280))

	// Rank 25 is the rider's neutral point (1.00x), and the existing skill
	// reduction is 0.25, so 280 * 0.75 * 1.00 = 210.
	mid := New()
	mid.Skills[string(skills.Manifestation)] = 25
	assert.Equal(t, 210, mid.CalcCompanionReserve(280))

	// The composed curve must never be worse than the existing reduction alone
	// at any rank past the rider's neutral point -- that is the property that
	// makes composition safe.
	for rank := 25; rank <= 100; rank++ {
		c := New()
		c.Skills[string(skills.Manifestation)] = rank
		composed := c.CalcCompanionReserve(280)
		if composed > 210 {
			t.Fatalf("manifestation %d: composed reserve %d exceeds the rank-25 figure 210; "+
				"the curve must be monotonically non-increasing past neutral", rank, composed)
		}
	}
}

// The reserve must never round to zero: a free companion is an unbounded one.
func TestCalcCompanionReserve_FloorsAtOne(t *testing.T) {
	c := New()
	c.Skills[string(skills.Manifestation)] = 100
	if got := c.CalcCompanionReserve(1); got < 1 {
		t.Fatalf("CalcCompanionReserve(1) = %d, want at least 1", got)
	}
}

// D5: the cap subsumes CanAffordCompanion, which is removed rather than kept
// alongside. Two ceilings on the same pool means the weaker one never fires.
func TestCanAffordCompanionIsGone(t *testing.T) {
	// This test exists only as a tombstone. If someone reintroduces the method
	// the compiler will not complain, so state the intent in prose: companion
	// affordability is now WouldBreachReservationCap(PoolConviction, reserve)
	// plus the GetMaxCompanions count backstop, checked at the call site.
	t.Skip("tombstone: see WouldBreachReservationCap and GetMaxCompanions")
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run 'TestCompanionReserveBase|TestCalcCompanionReserve' -v`
Expected: FAIL, `undefined: CompanionReserveBase`.

- [x] **Step 3: Implement the derived base and the rider**

In `internal/characters/companions.go`, replace `CalcCompanionReserve` (lines 167-178) and DELETE `CanAffordCompanion` (lines 180-192) entirely, leaving:

```go
// CompanionReserveBase returns the PRE-reduction Conviction a companion of the
// given pet multiplier reserves: CompanionReserveDefault scaled by the
// multiplier (D9).
//
// This replaced two flat tiers (280 and 352) shared across both families. The
// ongoing budget now tracks pet POWER, which is what makes the reservation
// ceiling a real choice rather than a disguised companion count. Cast cost is a
// one-time toll on a companion that persists across logout and reboot with full
// state, so it is the wrong place to carry differentiation; reservation is.
//
// A multiplier of 0 (charm, the brood floor, the homunculus -- none of which is
// an authored summon with a pet tier) means "unscaled", so those paths keep
// their own bases untouched.
func CompanionReserveBase(petMultiplier float64) int {
	base := int(configs.GetBalanceConfig().CompanionReserveDefault)
	if petMultiplier <= 0 {
		return base
	}
	if r := int(math.Round(float64(base) * petMultiplier)); r > 0 {
		return r
	}
	return 1
}

// CalcCompanionReserve returns the Conviction a companion of the given base cost
// reserves for THIS summoner, after the manifestation-skill and Manifester-
// mutation reductions and the U7 inverse-skill rider.
//
//	reservation = round(base * (1 - reduction) * costs.SkillCostMultiplier(manif))
//
// The rider COMPOSES onto the existing reduction; it does not replace it
// (D10 §4.1). Replacing would be strictly worse at every rank: the U7 curve
// bottoms at 0.40 while the existing reduction already reaches 0.45 at
// manifestation 55 and 0.21 with the Manifester mutation, so a replacement would
// make companions dearer for everyone, the opposite of intent.
//
// Known consequence, accepted: composed, the curve double-counts manifestation
// below rank 55 and is a 10% PENALTY at rank 0, only becoming a discount past
// rank 25. That matches the settled decision on the item side and is deliberate.
func (c *Character) CalcCompanionReserve(baseCost int) int {
	cfg := configs.GetBalanceConfig()
	manif := c.GetSkillLevel(skills.Manifestation)
	manifRed := math.Min(float64(cfg.CompanionReserveSkillCap), float64(manif)*float64(cfg.CompanionReserveSkillPct))
	mutRank := mutations.GetCompanionReserveRank(c.Mutations)
	mutRed := math.Min(float64(cfg.CompanionReserveMutCap), float64(mutRank)*float64(cfg.CompanionReserveMutPctPerRank))
	reduction := math.Min(float64(cfg.CompanionReserveTotalCap), manifRed+mutRed)

	reserve := float64(baseCost) * (1.0 - reduction) * costs.SkillCostMultiplier(manif)
	if r := int(math.Round(reserve)); r > 0 {
		return r
	}
	// A companion that reserves nothing is an unbounded one, and the login
	// backfill uses 0 as its "legacy record" marker besides.
	return 1
}
```

Add `"github.com/GoMudEngine/GoMud/internal/costs"` to the file's imports.

- [x] **Step 4: Delete the obsolete affordability test**

Run: `git rm internal/characters/companion_afford_test.go`

That file tests `CanAffordCompanion` end to end and has no meaning once the method is gone. The tombstone in Step 1 records why.

- [x] **Step 5: Update the four compile-broken readers**

Deleting `SummonBasePool`, `SummonConvictionReserve` and `CanAffordCompanion` breaks four files by construction. All four are fixed here so this commit is green; Task 9 then does the auto-spawn and login work that has no compile pressure behind it.

In `internal/hooks/companion_summon.go`, replace lines 41-58 with:

```go
	// ── 1. Reservation + budget gate ────────────────────────────────────
	// Reservation is DERIVED from the pet multiplier (D9), never authored.
	// Fail early, before consuming any component or corpse.
	baseReserve := characters.CompanionReserveBase(spellData.SummonPetMultiplier)
	// Count cap and conviction budget are separate refusals, reported
	// separately so a player at their companion limit isn't wrongly told they
	// lack conviction.
	if len(ch.Companions) >= ch.GetMaxCompanions() {
		user.SendText(messaging.CategorySpellManifestation,
			"You are already sustaining as many companions as your will can hold.")
		return false
	}
	reserve := ch.CalcCompanionReserve(baseReserve)
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		user.SendText(messaging.CategorySpellManifestation,
			ch.ReservationRefusal(characters.PoolConviction))
		return false
	}
```

and replace line 99 with:

```go
	pool := characters.CalcCompanionPool(charisma, manifestSkill, spellData.SummonPetMultiplier, corpsePool)
```

then DELETE lines 101-104 (the `if corpseConsumed { pool = (pool + corpsePool) / 2 }` block). The average now lives inside `CalcCompanionPool`, and leaving both would average twice.

`corpsePool` is 0 unless a corpse was consumed, and `CalcCompanionPool` treats a non-positive corpse pool as "conjured", so the `corpseConsumed` bool becomes unused. Delete its declaration and its assignment; `go build` will name it if you miss one.

In `internal/hooks/companion_reserve_backfill.go`, replace lines 21-26 with:

```go
	for _, sp := range spells.GetAllSpells() {
		if sp.SummonMobId == mobId && sp.SummonPetMultiplier > 0 {
			return characters.CompanionReserveBase(sp.SummonPetMultiplier)
		}
	}
	return int(configs.GetBalanceConfig().CompanionReserveDefault)
```

and add `"github.com/GoMudEngine/GoMud/internal/characters"` to its imports if the compiler asks (it is already imported for the `*characters.Character` parameter).

In `internal/usercommands/assess.go`, replace lines 95-99 with:

```go
		reserve := user.Character.CalcCompanionReserve(
			characters.CompanionReserveBase(sp.SummonPetMultiplier))
```

and replace line 127 with:

```go
			if user.Character.WouldBreachReservationCap(characters.PoolConviction, g.maxReserve) {
```

Add `"github.com/GoMudEngine/GoMud/internal/characters"` to `assess.go`'s imports if it is not already there, and drop the now-unused `configs` import if the compiler says so.

In `internal/hooks/charm_spell.go`, replace lines 36-45 with:

```go
	// ── 2. Reservation + budget gate ───────────────────────────────────
	// A charmed creature isn't an authored summon type, so it has no pet
	// multiplier and reserves the unscaled default (reduced by the caster's
	// manifestation and mutation). Its price therefore does not move in U7b.
	reserve := ch.CalcCompanionReserve(characters.CompanionReserveBase(0))
	if len(ch.Companions) >= ch.GetMaxCompanions() {
		user.SendText(messaging.CategorySystem,
			`You are already sustaining as many companions as your will can hold.`)
		return false
	}
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		user.SendText(messaging.CategorySystem, ch.ReservationRefusal(characters.PoolConviction))
		return false
	}
```

The count backstop is now reported separately here as well. It never was before: `CanAffordCompanion` folded the count cap and the conviction budget into one bool, so a player at their companion limit was told they lacked conviction. Drop the now-unused `configs` import if the compiler says so.

- [x] **Step 6: Delete the two knobs the removal orphaned**

`CompanionCastingFloorPct` had exactly one reader, `CanAffordCompanion`, and is now dead. It is absent from `config.yaml`, so deleting it changes nothing at runtime, but leaving it reads as a live tuning dial and is not one.

In `internal/configs/config.balance.go`, delete the `CompanionCastingFloorPct` field (line 653). In `internal/configs/config.balance.mobs.go`, delete the comment at line 156 that explains its intentional zero default. In `internal/configs/config.balance.companions_test.go`, delete the assertion at lines 31-33.

Leave `ManifestStatScaleChaFactor` and `ManifestStatScaleSkillFactor` alone: `CalcSpawnPoolFromBase` still reads both, and they are shipped in `config.yaml` at lines 1268-1269.

- [x] **Step 7: Run the full affected set**

```bash
gofmt -l internal/
go build ./...
go test -count=1 ./internal/characters/ ./internal/spells/ ./internal/hooks/ ./internal/usercommands/ ./internal/behaviortree/
```
Expected: `gofmt` prints nothing, the build is clean, and every package passes. `internal/relationships` is quarantined by Windows Defender and is a known pre-existing failure; it is not in this list for that reason.

If `internal/hooks/companion_reserve_backfill_test.go` or `internal/usercommands/assess_disclosure_test.go` fails, read it before changing it: both encode real behaviour and their expectations move because the reserve figures move, not because the behaviour is wrong. Update the expected numbers, never the assertion's shape.

- [x] **Step 8: Commit Tasks 2, 3 and 4 together**

**Before staging, fix the two TEST files that read the retired fields.** They
are invisible to `go build` and will fail `go test` compilation:

- `internal/hooks/companion_reserve_backfill_test.go:23` sets
  `SummonConvictionReserve`.
- `internal/usercommands/assess_disclosure_test.go:27,:32` set
  `SummonConvictionReserve: 350` and `: 440`.

Convert both to `SummonPetMultiplier` with values that preserve each test's
intent, deriving the reserve through the same path production now uses.

```bash
git add internal/spells/spells.go internal/spells/summon_fields_test.go \
        internal/spells/test_main_test.go \
        internal/characters/companions.go internal/characters/companions_test.go \
        internal/characters/companion_calibration_test.go \
        internal/characters/companion_afford_test.go \
        internal/behaviortree/actions_mob.go \
        internal/hooks/companion_summon.go internal/hooks/companion_reserve_backfill.go \
        internal/hooks/companion_reserve_backfill_test.go \
        internal/hooks/charm_spell.go internal/usercommands/assess.go \
        internal/usercommands/assess_disclosure_test.go \
        internal/configs/config.balance.go internal/configs/config.balance.mobs.go \
        internal/configs/config.balance.companions_test.go
git commit -m "feat(companions): pet multipliers replace base pools, and price the reservation

One dial now drives a companion's power and its ongoing price.
summon_pet_multiplier replaces summon_base_pool, and the multiplier is
applied AFTER the corpse average rather than before it. Under the old
shape the pet's base pool multiplied the caster and the corpse was
averaged in afterwards, so the corpse's share grew until it swamped the
pet choice: at a 1000-pool corpse a skeleton fielded 587 and a golem 675,
meaning five times the price bought fifteen percent more pet. Every tier
now stays proportionally separated at every corpse size.

Reservation is derived from the same multiplier instead of two flat tiers
shared across both families, so the ongoing budget tracks pet power. Cast
cost is a one-time toll on something that persists across logout and
reboot, which makes it the wrong place to carry differentiation.

summon_scaling_divisor is deleted: declared in the struct, present in all
thirteen summon YAMLs, and never read by anything.
summon_conviction_reserve is deleted because it is now derived, and an
authorable value beside a derived one is a second source of truth.
CanAffordCompanion is deleted because the reservation ceiling subsumes it:
it was a hundred-percent conviction-only cap, since CompanionCastingFloorPct
defaults to zero and is absent from config.yaml, and two ceilings on one
pool means the weaker never fires.

The behaviour-tree add scaler keeps the old curve and is renamed
CalcSpawnPoolFromBase to say so. Its callers are authored boss encounters
tuned against that shape, and moving them onto the companion formula would
nerf the Sentinel's adds roughly fivefold.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: The thirteen summon spell YAMLs

**One task, not thirteen.** Each file takes the same three-line edit plus one `cost:` change, there is no per-file reasoning beyond reading the multiplier off the table, and the whole set is verified by one grep plus one boot. Splitting it would produce twelve commits that each leave the loader warning "summon_mob_id set but summon_pet_multiplier is 0 or missing" for the files not yet touched, which is a red interval by any useful definition. It is a single mechanical sweep and belongs in a single commit.

**Files (all under `_datafiles/world/dogmud/spells/`):**
- Modify: `conjure-magma.yaml`, `conjure-earth.yaml`, `conjure-fire.yaml`, `conjure-air.yaml`, `conjure-water.yaml`
- Modify: `raise-golem.yaml`, `raise-vampire.yaml`, `raise-spectre.yaml`, `raise-zombie.yaml`, `raise-wraith.yaml`, `raise-skeleton.yaml`
- Modify: `summon-steppe-spirit.yaml`, `summon-hive-swarm.yaml`
- Create: `internal/spells/summon_calibration_test.go`

- [ ] **Step 1: Record the before state**

```bash
grep -n "^cost:\|^summon_" _datafiles/world/dogmud/spells/conjure-*.yaml \
     _datafiles/world/dogmud/spells/raise-*.yaml \
     _datafiles/world/dogmud/spells/summon-hive-swarm.yaml \
     _datafiles/world/dogmud/spells/summon-steppe-spirit.yaml
```
Expected: thirteen files, each showing `cost:`, `summon_mob_id:`, `summon_base_pool:`, `summon_conviction_reserve:`, `summon_scaling_divisor:`, and (for the six raises) `summon_requires_corpse:` / `summon_min_corpse_pool:`. Paste this into your report as the before state.

- [ ] **Step 2: Apply the edit to each file**

In every one of the thirteen files, make exactly these changes with `Edit` (never a script, and never a Python read-modify-write, which has destroyed files in this repo twice):

1. Replace the `summon_base_pool: <n>` line with `summon_pet_multiplier: <m>` from the table below.
2. Delete the `summon_conviction_reserve: <n>` line.
3. Delete the `summon_scaling_divisor: <n>` line.
4. Set `cost: <c>` from the table below.

Leave `summon_mob_id`, `summon_component_id`, `summon_requires_corpse` and `summon_min_corpse_pool` untouched.

| File | `cost:` | `summon_pet_multiplier:` |
|---|---|---|
| `conjure-magma.yaml` | `50` (was 450) | `1.25` |
| `conjure-earth.yaml` | `45` (was 200) | `1.05` |
| `conjure-fire.yaml` | `45` (was 350) | `1.00` |
| `conjure-air.yaml` | `40` (was 280) | `0.90` |
| `conjure-water.yaml` | `30` (was 150) | `0.75` |
| `raise-golem.yaml` | `50` (was 100) | `1.00` |
| `raise-vampire.yaml` | `45` (was 80) | `0.83` |
| `raise-spectre.yaml` | `40` (was 60) | `0.75` |
| `raise-zombie.yaml` | `35` (was 30) | `0.67` |
| `raise-wraith.yaml` | `35` (was 45) | `0.58` |
| `raise-skeleton.yaml` | `30` (was 20) | `0.50` |
| `summon-steppe-spirit.yaml` | `35` (unchanged) | `0.75` |
| `summon-hive-swarm.yaml` | `30` (was 50) | `0.30` |

Worked example. `raise-skeleton.yaml` lines 11 and 29-34 go from:

```yaml
cost: 20
...
summon_mob_id: 300
summon_base_pool: 60
summon_conviction_reserve: 280
summon_scaling_divisor: 500
summon_requires_corpse: true
summon_min_corpse_pool: 30
```

to:

```yaml
cost: 30
...
summon_mob_id: 300
summon_pet_multiplier: 0.50
summon_requires_corpse: true
summon_min_corpse_pool: 30
```

**Two of the raises get DEARER to cast** (skeleton 20 to 30, zombie 30 to 35) and everything else gets much cheaper. That is not an error in the table. Cast cost is now a low, flat entry gate across the whole set, so the cheapest raises rise to meet it while the conjures fall out of a band that made `conjure-magma` self-excluding: at 450 it was 89% of a maxed summoner's whole conviction pool, uncastable outright for a mid-level one, and impossible for anyone already fielding companions, because their reservation had already dropped usable conviction below the cast price.

- [ ] **Step 3: Verify the sweep is complete and consistent**

```bash
grep -rn "summon_base_pool\|summon_scaling_divisor\|summon_conviction_reserve" _datafiles/
```
Expected: **no output.** Any hit is a file the sweep missed.

```bash
grep -c "summon_pet_multiplier" _datafiles/world/dogmud/spells/*.yaml | grep -v ":0"
```
Expected: exactly thirteen lines, each ending `:1`.

- [ ] **Step 4: Verify every file still parses**

```bash
python -c "import yaml,glob; [yaml.safe_load(open(f)) for f in glob.glob('_datafiles/world/dogmud/spells/*.yaml')]" && echo YAML_OK
```
Expected: `YAML_OK`.

- [ ] **Step 5: Pin the shipped multipliers**

Create `internal/spells/summon_calibration_test.go`:

```go
package spells

import "testing"

// The shipped multipliers, pinned. This is the table the design settled on and
// the one every reservation figure derives from, so a typo in a YAML file fails
// here rather than surfacing as a mysteriously cheap companion.
//
// It reads the LOADED spells, so it only asserts when the data files are
// present in this test binary.
func TestShippedSummonPetMultipliers(t *testing.T) {
	want := map[string]float64{
		"conjure-magma": 1.25, "conjure-earth": 1.05, "conjure-fire": 1.00,
		"conjure-air": 0.90, "conjure-water": 0.75,
		"raise-golem": 1.00, "raise-vampire": 0.83, "raise-spectre": 0.75,
		"raise-zombie": 0.67, "raise-wraith": 0.58, "raise-skeleton": 0.50,
		"summon-steppe-spirit": 0.75, "summon-hive-swarm": 0.30,
	}
	if len(GetAllSpells()) == 0 {
		t.Skip("spell data not loaded in this test binary")
	}
	for id, mult := range want {
		sp := GetSpell(id)
		if sp == nil {
			t.Errorf("%s: spell not found", id)
			continue
		}
		if sp.SummonPetMultiplier != mult {
			t.Errorf("%s: summon_pet_multiplier = %v, want %v", id, sp.SummonPetMultiplier, mult)
		}
	}
}
```

Run: `go test ./internal/spells/ -run TestShippedSummonPetMultipliers -v`
Expected: PASS, or SKIP if the package's test binary does not load data files. If it skips, say so in your report; the boot test in Task 15 is what then exercises the loaded values.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/world/dogmud/spells/ internal/spells/summon_calibration_test.go
git commit -m "content(spells): pet multipliers and a flat cast-cost band for the thirteen summons

Every summon spell now carries a single pet multiplier instead of a base
pool, a hand-authored reservation, and a scaling divisor nothing ever read.
Cast costs collapse into a narrow band, because a companion persists across
logout and reboot with full state, so the cast price is amortised over its
entire life and is the wrong place to carry differentiation. Reservation
carries the real cost instead.

This removes a self-excluding trap. Conjuring a magma elemental cost almost
the whole conviction pool of a maxed summoner, was uncastable outright for a
mid-level one, and was impossible for anyone already fielding companions,
because their existing reservation had already dropped usable conviction
below the cast price.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: The enchanting-rank rider on item reservation

Task 1 built `EnchantReserveAt` with the rider inside it. This task routes `GetPoolReservation` through the per-item helper, so the rider goes live and the per-item figure the enforcement sites price a swap with can never disagree with the total they test it against.

**Files:**
- Modify: `internal/characters/validate.go` (`GetPoolReservation` at :245-281)
- Modify: `internal/characters/reservation.go` (split the helpers so both entry points share one body)
- Test: `internal/characters/reservation_rider_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/reservation_rider_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/enchantments"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// D10 section 4.2: an enchantment's reservation scales by the WEARER's
// enchanting rank through the U7 inverse-skill band. The penalty half applies,
// consistently with the companion side: a rank-0 wearer pays 1.10x.
func TestGetPoolReservation_ScalesEnchantmentsByEnchantingRank(t *testing.T) {
	defer enchantments.SeedEnchantmentsForTest(map[string]*enchantments.EnchantmentDef{
		"test-drain": {
			EnchantId:   "test-drain",
			ReservePool: "stamina",
			Tiers:       []enchantments.TierDef{{Tier: 0, ReservePct: 0.10}},
		},
	})()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999940: {ItemId: 999940, Name: "drained cloak", Type: items.Neck},
	})()

	newWearer := func(enchantingRank int) *Character {
		c := New()
		c.StaminaMax.Base = 1000
		itm := items.New(999940)
		itm.EnchantType = "test-drain"
		itm.EnchantTier = 0
		itm.ReservePool = "stamina"
		c.Equipment.Neck = itm
		c.Skills[string(skills.Enchanting)] = enchantingRank
		c.Validate()
		return c
	}

	// rank 0   -> 0.10 * 1.10 = 0.110 -> 110
	// rank 25  -> 0.10 * 1.00 = 0.100 -> 100
	// rank 100 -> 0.10 * 0.40 = 0.040 ->  40
	for _, tc := range []struct {
		rank int
		want int
	}{
		{0, 110},
		{25, 100},
		{100, 40},
	} {
		c := newWearer(tc.rank)
		if got := c.GetPoolReservation("stamina", 1000); got != tc.want {
			t.Errorf("enchanting %d: reservation = %d, want %d", tc.rank, got, tc.want)
		}
	}
}

// Pinnacle-item reserve_*_pct is deliberately NOT scaled. That reservation is
// the item's price, not a piece of craft the wearer's skill has any purchase on,
// and scaling it would hand every enchanter a discount on gear they never made.
func TestGetPoolReservation_DoesNotScalePinnacleItemReserve(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999941: {ItemId: 999941, Name: "pinnacle band", Type: items.Ring, ReserveStaminaPct: 0.10},
	})()

	for _, rank := range []int{0, 25, 100} {
		c := New()
		c.StaminaMax.Base = 1000
		c.Equipment.Ring = items.New(999941)
		c.Skills[string(skills.Enchanting)] = rank
		c.Validate()
		if got := c.GetPoolReservation("stamina", 1000); got != 100 {
			t.Errorf("enchanting %d: pinnacle reservation = %d, want a flat 100", rank, got)
		}
	}
}

// The per-item helper and the total must agree by construction: the enforcement
// sites price a swap with the former and test it against the latter.
func TestItemReserveOnPool_SumsToGetPoolReservation(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999942: {ItemId: 999942, Name: "band a", Type: items.Ring, ReserveStaminaPct: 0.07},
		999943: {ItemId: 999943, Name: "band b", Type: items.Ring, ReserveStaminaPct: 0.11},
	})()

	c := New()
	c.StaminaMax.Base = 1000
	c.Equipment.Ring = items.New(999942)
	c.Equipment.Ring2 = items.New(999943)
	c.Validate()

	sum := 0
	for _, itm := range c.Equipment.GetAllItems() {
		sum += c.ItemReserveOnPool(itm, PoolStamina)
	}
	if total := c.GetPoolReservation("stamina", 1000); sum != total {
		t.Fatalf("per-item sum %d != GetPoolReservation %d; the two must not be able to drift", sum, total)
	}
}
```

Verify `enchantments.SeedEnchantmentsForTest` and the `TierDef` field names (`Tier`, `ReservePct`) against `internal/enchantments/enchantments.go` before writing, and correct the fixture rather than the production code if they differ.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run 'TestGetPoolReservation_|TestItemReserveOnPool_' -v`
Expected: FAIL on the rider test, reporting reservation 100 where 110 is wanted at rank 0. The rider was written in Task 1 but nothing calls it yet.

- [ ] **Step 3: Route the total through the per-item helper**

In `internal/characters/validate.go`, replace the body of `GetPoolReservation` (lines 245-281) with:

```go
func (c *Character) GetPoolReservation(pool string, poolMax int) int {
	total := 0

	// Per item, through the same helper the enforcement sites use to price a
	// single swap, so the total and the per-item figure cannot drift apart. The
	// helper carries the note about one item contributing through BOTH a
	// Chrysalis enchantment and a spec reserve_*_pct at once, which stacks by
	// design.
	//
	// poolMax is passed down rather than read fresh: RecalculateStats calls this
	// mid-derivation with the value it has just computed, before that value has
	// been written back to the character.
	for _, itm := range c.Equipment.GetAllItems() {
		total += c.itemReserveOnPoolWithMax(itm, Pool(pool), poolMax)
	}

	// Companions reserve Conviction while fielded (snapshotted at summon time).
	if pool == "conviction" {
		for i := range c.Companions {
			total += c.Companions[i].ConvictionReserve
		}
	}

	return total
}
```

- [ ] **Step 4: Split the reservation helpers so both entry points share one body**

In `internal/characters/reservation.go`, replace `ItemReserveOnPool` and `EnchantReserveAt` with the following four functions, keeping the doc comments Task 1 wrote (they move onto the exported wrappers):

```go
func (c *Character) ItemReserveOnPool(itm items.Item, p Pool) int {
	return c.itemReserveOnPoolWithMax(itm, p, c.poolMax(p))
}

func (c *Character) itemReserveOnPoolWithMax(itm items.Item, p Pool, poolMax int) int {
	pool := string(p)
	spec := itm.GetSpec()
	total := 0

	if itm.HasChrysalisEnchantment() && itm.ReservePool == pool {
		total += c.enchantReserveAtWithMax(itm.EnchantType, itm.EnchantTier, spec.Hands, poolMax)
	}

	var itemPct float64
	switch pool {
	case "health":
		itemPct = spec.ReserveHealthPct
	case "stamina":
		itemPct = spec.ReserveStaminaPct
	case "conviction":
		itemPct = spec.ReserveConvictionPct
	}
	if itemPct > 0 {
		total += int(math.Floor(float64(poolMax) * itemPct))
	}
	return total
}

func (c *Character) EnchantReserveAt(enchantType string, tier int, hands int, p Pool) int {
	return c.enchantReserveAtWithMax(enchantType, tier, hands, c.poolMax(p))
}

func (c *Character) enchantReserveAtWithMax(enchantType string, tier int, hands int, poolMax int) int {
	pct := enchantments.GetTierReservePct(enchantType, tier, hands)
	if pct <= 0 {
		return 0
	}
	// The rider scales the PERCENTAGE, before the floor, so it cannot be rounded
	// away on a small pool.
	pct *= costs.SkillCostMultiplier(c.GetSkillLevel(skills.Enchanting))
	return int(math.Floor(float64(poolMax) * pct))
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -count=1 ./internal/characters/ 2>&1 | tail -30`
Expected: PASS for the whole package.

`TestRecalculateStats_PoolReservationClamping` in `godfunc_refactor_test.go` exercises the same function. If it fails, read it first: its fixture likely leaves enchanting at rank 0, so its expected reservation rises by a tenth. Update the number with a comment saying why. **Do not set the fixture's enchanting rank to 25 to dodge the change**: rank 0 is a real state and the test should assert what a rank-0 wearer really pays.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/validate.go internal/characters/reservation.go internal/characters/reservation_rider_test.go internal/characters/godfunc_refactor_test.go
git commit -m "feat(characters): enchantment reservation scales with the wearer's enchanting rank

An enchantment's reserved share now runs through the U7 inverse-skill band
on the wearer's enchanting skill. A tier-four eight percent enchant holds
8.8 percent at rank zero, eight at twenty-five, 6.1 at fifty-four, and 3.2
at a hundred. The penalty half applies, consistently with the companion
side.

Pinnacle-item reserve percentages are deliberately left flat. That
reservation is the item's price rather than a piece of craft the wearer's
skill has any purchase on, and scaling it would discount gear the wearer
never made.

The total is now the sum of a per-item helper rather than an inlined loop,
so the figure the enforcement sites price a swap with and the figure they
test it against cannot drift.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Enforcement at the equip seam

`Character.Wear` is the seam, not `actions.EquipItem`. `internal/actions/sell.go:339` calls `mob.Character.Wear` directly for the gear-upgrade path and would walk straight past a gate that lived one level up.

**The check runs AFTER placement, and reverts.** Equipping displaces, and the displaced item's own reservation counts, so the delta is not knowable until the slot resolves. `wearWeaponOrShield` and `wearArmorSlot` mutate only `c.Equipment` (every write goes through a slot field or a pointer into one), with exactly one exception: the `ComponentBag` branch also calls `c.SortComponentItems()`, which moves items between `c.Items` and `c.ComponentItems`. That call has to move out of the placement helper and run only after the check passes, or a reverted equip would leave the component sort half-applied.

**Files:**
- Modify: `internal/characters/worn.go` (`Wear` at :572-605, `wearArmorSlot` ComponentBag branch at :544-547)
- Modify: `internal/mobcommands/equip.go` (report the refusal)
- Modify: `internal/actions/sell.go` (:339, stop discarding the reason)
- Test: `internal/characters/wear_reservation_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/wear_reservation_test.go`:

```go
package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// An equip that would carry reservation past the cap is REFUSED (D3), and the
// character is left exactly as they were: the item is not on the body, and
// nothing was displaced.
func TestWear_RefusesABreachingEquip(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999950: {ItemId: 999950, Name: "hungry collar", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.60},
		999951: {ItemId: 999951, Name: "hungry belt", Type: items.Belt, Subtype: items.Wearable, ReserveStaminaPct: 0.30},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999950) // 60 against a 66 cap
	c.Validate()

	returned, worn, reason := c.Wear(items.New(999951)) // +30 would make 90
	if worn {
		t.Fatalf("a breaching equip must be refused, not worn")
	}
	if len(returned) != 0 {
		t.Errorf("a refused equip must displace nothing, got %d items back", len(returned))
	}
	if c.Equipment.Belt.ItemId != 0 {
		t.Errorf("a refused equip must leave the slot empty, found item %d", c.Equipment.Belt.ItemId)
	}
	if c.Equipment.Neck.ItemId != 999950 {
		t.Errorf("a refused equip must leave existing gear untouched")
	}
	if !strings.Contains(strings.ToLower(reason), "reserve") {
		t.Errorf("the refusal must name reservation as the cause, got %q", reason)
	}
	if strings.ContainsAny(reason, "0123456789") {
		t.Errorf("a player-facing refusal must carry no raw numbers, got %q", reason)
	}
}

// D4 grandfathering at the equip seam. A character ALREADY past the cap must
// still be able to swap one reserving item for an equally reserving one; a plain
// over-the-cap test would refuse that and force them to strip.
func TestWear_GrandfatheredCharacterCanStillSidegrade(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999952: {ItemId: 999952, Name: "old yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.80},
		999953: {ItemId: 999953, Name: "new yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.80},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999952) // 80, already past the 66 cap
	c.Validate()

	_, worn, reason := c.Wear(items.New(999953))
	if !worn {
		t.Fatalf("an equal-for-equal swap must be allowed even past the cap, refused with %q", reason)
	}
	if c.Equipment.Neck.ItemId != 999953 {
		t.Errorf("the swap did not take: neck holds %d", c.Equipment.Neck.ItemId)
	}
}

// An equip that REDUCES reservation must always be allowed, however far over the
// character already is. Nothing here may ever force or block a removal.
func TestWear_ADowngradeIsAlwaysAllowed(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999954: {ItemId: 999954, Name: "heavy yoke", Type: items.Neck, Subtype: items.Wearable, ReserveStaminaPct: 0.80},
		999955: {ItemId: 999955, Name: "light scarf", Type: items.Neck, Subtype: items.Wearable},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999954)
	c.Validate()

	if _, worn, reason := c.Wear(items.New(999955)); !worn {
		t.Fatalf("swapping to unreserved gear must always be allowed, refused with %q", reason)
	}
}

// An ordinary equip by an unreserved character must be completely unaffected.
// The overwhelming majority of equips are this case and not one number may move.
func TestWear_OrdinaryEquipIsUnchanged(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999956: {ItemId: 999956, Name: "plain cap", Type: items.Head, Subtype: items.Wearable},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Validate()

	if _, worn, reason := c.Wear(items.New(999956)); !worn {
		t.Fatalf("an ordinary equip must succeed, refused with %q", reason)
	}
	if c.Equipment.Head.ItemId != 999956 {
		t.Errorf("the cap is not on the head")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run TestWear_ -v`
Expected: `TestWear_RefusesABreachingEquip` FAILS with "a breaching equip must be refused, not worn". The other three PASS today and stand as guards against the fix over-reaching.

- [ ] **Step 3: Move the component sort out of the placement helper**

In `internal/characters/worn.go`, in `wearArmorSlot`'s `items.ComponentBag` branch (lines 544-547), delete the `c.SortComponentItems()` call so the branch reads:

```go
	case items.ComponentBag:
		returnItems = append(returnItems, c.Equipment.ComponentBag)
		c.Equipment.ComponentBag = i
```

The sort moves into `Wear`, after the reservation check. It has to: the check reverts by restoring the whole `Worn` value, which cannot undo a component sort, so a refused component-bag equip would otherwise leave materials filed into a bag nobody is wearing.

- [ ] **Step 4: Gate `Wear`**

Replace `Wear` (lines 572-605) with:

```go
func (c *Character) Wear(i items.Item) (returnItems []items.Item, newItemWorn bool, failureReason string) {

	i.Validate()

	spec := i.GetSpec()

	if spec.Type != items.Weapon && spec.Subtype != items.Wearable {
		return returnItems, false, `That item cannot be equipped.`
	}

	// Min-Strength wield gate: heavy bows and arbalests require a minimum
	// Strength to operate. Checked before HandsRequired so the rejection is
	// immediate and consistent for all callers.
	if spec.MinStrength > 0 && c.Stats.Strength.ValueAdj < spec.MinStrength {
		return returnItems, false, `You aren't strong enough to handle ` + i.DisplayName() + `.`
	}

	iHandsRequired := c.HandsRequired(i)
	if iHandsRequired > 2 {
		return returnItems, false, `That requires too many hands.`
	}

	// U7b reservation ceiling. The check runs AFTER placement and reverts,
	// because equipping DISPLACES and the displaced item's own reservation
	// counts, so the delta is not knowable until the slot resolves. Comparing
	// overage before against overage after is also what delivers D4
	// grandfathering: a character already past the ceiling can still swap one
	// reserving ring for an equally reserving one, where a plain
	// over-the-ceiling test would refuse that and force them to strip.
	//
	// Restoring the whole Worn value is a sound revert because both placement
	// helpers write ONLY into c.Equipment (wearWeaponOrShield through pointers
	// into it, wearArmorSlot by assigning slot fields). The single call that
	// touched other state, SortComponentItems, was moved out of wearArmorSlot
	// and runs below, after this check passes.
	beforeReserve := c.ReservationOverages()
	savedEquipment := c.Equipment

	if spec.Type == items.Weapon || spec.Type == items.Offhand {
		returnItems, newItemWorn, failureReason = c.wearWeaponOrShield(i, spec, iHandsRequired, c.CanDualWield())
	} else {
		returnItems, newItemWorn, failureReason = c.wearArmorSlot(i, spec)
	}

	if !newItemWorn {
		return returnItems, newItemWorn, failureReason
	}

	if pool, worse := beforeReserve.Worsened(c.ReservationOverages()); worse {
		c.Equipment = savedEquipment
		return nil, false, c.ReservationRefusal(pool)
	}

	if spec.Type == items.ComponentBag {
		c.SortComponentItems()
	}
	if spec.Type != items.Weapon && spec.Type != items.Offhand {
		// Preserved from the pre-U7b shape: permabuffs are reapplied on the
		// armour path only, and only on success.
		c.reapplyPermabuffs(returnItems...)
	}
	return returnItems, newItemWorn, failureReason
}
```

- [ ] **Step 5: Run the tests**

Run: `go test -count=1 ./internal/characters/ -run TestWear_ -v`
Expected: PASS, all four.

Run: `go test -count=1 ./internal/characters/`
Expected: PASS. If a `Worn` or component-bag test fails on the moved sort, read it: it may be asserting that `wearArmorSlot` sorts, which is now `Wear`'s job. Move the assertion, do not delete it.

- [ ] **Step 6: Stop the mob paths from swallowing the refusal**

Two sites discard `failureReason`, which is why a mob-side refusal is currently invisible on every mob path. With the ceiling live they would each become a silent no-op that reads as a bug to whoever handed over the item.

In `internal/mobcommands/equip.go`, immediately after `result := actions.EquipItem(actor, matchItem.Name())`, add:

```go
	if !result.Equipped && result.FailureReason != "" {
		// gearup drives this command with `wear !<id>` and infers success by
		// diffing the equipment set, so without this a refusal is invisible on
		// every mob path: nothing worn, nothing said, and the giver with no way
		// to learn why. Speaking it makes a companion declining a gift read as a
		// decision rather than a bug.
		room.SendTextVisual(messaging.CategoryEquipment,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> turns the <ansi fg="item">%s</ansi> over, then sets it aside.`,
				mob.Character.Name, matchItem.DisplayName()))
	}
```

In `internal/actions/sell.go` at line 339, change:

```go
				returnedItems, wore, _ := mob.Character.Wear(newItem)
```

to:

```go
				returnedItems, wore, wearFailure := mob.Character.Wear(newItem)
```

and in the `} else {` branch immediately below it (the one calling `shopInv.AddStockAtRound(item.ItemId, 1, ...)`), insert before that existing line:

```go
					if wearFailure != "" {
						room.SendTextVisual(messaging.CategoryLoot,
							fmt.Sprintf(`<ansi fg="mobname">%s</ansi> considers the <ansi fg="itemname">%s</ansi>, then shelves it instead.`,
								mob.Character.Name, newItem.DisplayName()))
					}
```

`internal/usercommands/equip.go` already surfaces refusals to the player. Read the `if result.Equipped {` block at :229 and its `else` to confirm, and say so in your report rather than changing it.

- [ ] **Step 7: Build and run the affected packages**

```bash
gofmt -l internal/
go build ./...
go test -count=1 ./internal/characters/ ./internal/actions/ ./internal/mobcommands/ ./internal/usercommands/
```
Expected: no gofmt output, clean build, all tests pass. `internal/mobcommands/gearup_test.go` exercises the diff-based success inference and must still pass.

- [ ] **Step 8: Commit**

```bash
git add internal/characters/worn.go internal/characters/wear_reservation_test.go internal/mobcommands/equip.go internal/actions/sell.go
git commit -m "feat(characters): equipping refuses when it would breach the reservation ceiling

The gate sits in Wear rather than in actions.EquipItem, because the mob
gear-upgrade path in actions/sell.go calls Wear directly and would walk
straight past a gate that lived one level up.

It checks after placement and reverts, because equipping displaces and the
displaced item's own reservation counts, so the delta is not knowable until
the slot resolves. Comparing overage before against overage after is also
what delivers grandfathering: a character already past the ceiling can
still swap one reserving ring for an equally reserving one, where a plain
over-the-ceiling test would refuse that and force them to strip.

SortComponentItems moves out of the placement helper and runs after the
check, because restoring the equipment set cannot undo a component sort and
a refused bag would otherwise leave materials filed into a bag nobody is
wearing.

Two mob paths stopped discarding the failure reason. Without that a refusal
was invisible on every mob path: gearup infers success by diffing the
equipment set, so a declined gift produced no item worn and no word said.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Enforcement in enchanting, and the tier-up skip

Two paths, one commit, because they are the same reservation source seen at its two entry points: the deliberate one a player starts, and the passive one that fires in combat with no action to refuse.

The tier-up is the more important half and was named nowhere in the original spec. `EnchantTierUpBaseChance` rolls every combat round on every Chrysalis-enchanted equipped item, and a tier-up **doubles** the reserve fraction at low tiers, so a character sitting at 64% can cross the ceiling mid-fight having done nothing at all.

**Files:**
- Modify: `internal/usercommands/craft.go` (after `resolveEnchantSlot` at :220-229)
- Modify: `internal/hooks/NewRound_UserRoundTick.go` (tier-up at :527-540)
- Test: `internal/hooks/enchant_tierup_cap_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/hooks/enchant_tierup_cap_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/enchantments"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// D14. Tier-up is a PASSIVE breach: it rolls in combat and doubles the reserved
// fraction at low tiers, so a character just under the ceiling can cross it
// having taken no action at all. It must skip rather than be allowed.
func TestEnchantTierUpSkipsWhenItWouldBreach(t *testing.T) {
	defer enchantments.SeedEnchantmentsForTest(map[string]*enchantments.EnchantmentDef{
		"test-greed": {
			EnchantId:   "test-greed",
			ReservePool: "stamina",
			Tiers: []enchantments.TierDef{
				{Tier: 0, ReservePct: 0.30},
				{Tier: 1, ReservePct: 0.60},
			},
		},
	})()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999960: {ItemId: 999960, Name: "greedy band", Type: items.Ring},
		999961: {ItemId: 999961, Name: "plain yoke", Type: items.Neck, ReserveStaminaPct: 0.20},
	})()

	c := characters.New()
	c.StaminaMax.Base = 100
	itm := items.New(999960)
	itm.EnchantType = "test-greed"
	itm.EnchantTier = 0
	itm.ReservePool = "stamina"
	c.Equipment.Ring = itm
	c.Skills["enchanting"] = 25 // the rider's neutral rank, so percentages are exact
	c.Validate()

	// Tier 0 holds 30 against a 66 cap. Tier 1 would hold 60, an addition of 30,
	// landing at exactly 60 and INSIDE the cap. This one must be allowed.
	if enchantTierUpWouldBreach(c, &c.Equipment.Ring) {
		t.Errorf("a tier-up landing inside the cap must be allowed")
	}

	// Add a second reserving item. Now 30 + 20 = 50 today, and the tier-up's
	// extra 30 would make 80 against the same 66 cap.
	c.Equipment.Neck = items.New(999961)
	c.Validate()
	if !enchantTierUpWouldBreach(c, &c.Equipment.Ring) {
		t.Errorf("a tier-up crossing the cap must be skipped")
	}
}

// The skip must never fire for an enchantment that reserves nothing, or every
// non-reserving enchantment in the game would silently stop advancing.
func TestEnchantTierUpAllowsNonReservingEnchantments(t *testing.T) {
	defer enchantments.SeedEnchantmentsForTest(map[string]*enchantments.EnchantmentDef{
		"test-edge": {
			EnchantId: "test-edge",
			Tiers:     []enchantments.TierDef{{Tier: 0}, {Tier: 1}},
		},
	})()
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999962: {ItemId: 999962, Name: "keen blade", Type: items.Weapon},
	})()

	c := characters.New()
	c.StaminaMax.Base = 100
	itm := items.New(999962)
	itm.EnchantType = "test-edge"
	itm.EnchantTier = 0
	c.Equipment.Weapon = itm
	c.Validate()

	if enchantTierUpWouldBreach(c, &c.Equipment.Weapon) {
		t.Errorf("an enchantment that reserves nothing must never be blocked from advancing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hooks/ -run TestEnchantTierUp -v`
Expected: FAIL, `undefined: enchantTierUpWouldBreach`.

- [ ] **Step 3: Implement the tier-up skip**

In `internal/hooks/NewRound_UserRoundTick.go`, add after the imports and before the hook function:

```go
// enchantTierUpWouldBreach reports whether advancing this item one enchantment
// tier would carry the wearer's total reservation past the ceiling.
//
// Tier-up is a PASSIVE breach with no action to refuse: it rolls every combat
// round on every Chrysalis-enchanted equipped item, and it DOUBLES the reserved
// fraction at low tiers, so a character sitting just under the ceiling can cross
// it mid-fight having done nothing. Grandfathering means it can never force a
// dismissal; it simply must not make things worse.
func enchantTierUpWouldBreach(ch *characters.Character, itm *items.Item) bool {
	if itm.ReservePool == "" {
		return false
	}
	pool := characters.Pool(itm.ReservePool)
	hands := itm.GetSpec().Hands
	added := ch.EnchantReserveAt(itm.EnchantType, itm.EnchantTier+1, hands, pool) -
		ch.EnchantReserveAt(itm.EnchantType, itm.EnchantTier, hands, pool)
	return ch.WouldBreachReservationCap(pool, added)
}
```

Then replace the tier-up block at lines 529-540 with:

```go
						if float64(itemPtr.EnchantUses) >= threshold {
							if util.Rand(100) < int(float64(bal.EnchantTierUpBaseChance)*100) {
								if enchantTierUpWouldBreach(user.Character, itemPtr) {
									// Say why, but not every round. EnchantUses is
									// deliberately NOT reset: the item stays ready
									// to advance the moment its wearer makes room,
									// rather than losing the progress it earned.
									if user.Character.GetCooldown("enchant-tierup-blocked") == 0 {
										user.Character.TryCooldown("enchant-tierup-blocked", "200 rounds")
										user.SendText(messaging.CategorySkillProgress,
											`<ansi fg="yellow">Your `+itemPtr.DisplayName()+
												` strains to deepen, but you have nothing left to feed it. `+
												`Set some other burden aside and it will grow.</ansi>`)
									}
									continue
								}
								itemPtr.EnchantTier++
								itemPtr.EnchantUses = 0
								enchantments.ApplyTier(itemPtr, eDef, itemPtr.EnchantTier)

								newTier := itemPtr.EnchantTier
								if newTier < len(eDef.Tiers) && eDef.Tiers[newTier].TierUpMessage != "" {
									user.SendText(messaging.CategorySkillProgress, fmt.Sprintf(`<ansi fg="magenta">%s</ansi>`, eDef.Tiers[newTier].TierUpMessage))
								}
							}
						}
```

Verify `GetCooldown` and `TryCooldown` against `internal/characters/` before writing: `manifester_companions.go:71,75` uses both in exactly this shape. If `TryCooldown` returns a value, either use it as the condition or assign it to `_` explicitly. **Never leave a returned error unchecked**: CI runs `errcheck`.

Add `"github.com/GoMudEngine/GoMud/internal/items"` to the file's imports if it is not already present.

- [ ] **Step 4: Add the enchant pre-flight gate**

`resolveEnchantSlot` already returns an error message before the multi-round activity starts, which makes it the preferred gate: refusing here costs the player nothing, where refusing at completion can only refund materials after the rounds are already spent.

In `internal/usercommands/craft.go`, immediately after the `if targetItem == nil` block (lines 226-229), add:

```go
	// U7b: refuse a breaching enchant BEFORE the multi-round activity starts.
	// Subtracting what the target item already reserves is what makes
	// re-enchanting work: the old enchantment is replaced rather than stacked,
	// so only the difference is new.
	if def := enchantments.GetEnchantment(recipe.EnchantType); def != nil && def.ReservePool != "" {
		pool := characters.Pool(def.ReservePool)
		added := user.Character.EnchantReserveAt(recipe.EnchantType, 0, targetItem.GetSpec().Hands, pool) -
			user.Character.ItemReserveOnPool(*targetItem, pool)
		if user.Character.WouldBreachReservationCap(pool, added) {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="red">%s</ansi>`,
				user.Character.ReservationRefusal(pool)))
			return true, nil
		}
	}
```

Add `"github.com/GoMudEngine/GoMud/internal/enchantments"` and `"github.com/GoMudEngine/GoMud/internal/characters"` to `craft.go`'s imports if they are not already there.

- [ ] **Step 5: Run the tests**

```bash
gofmt -l internal/
go build ./...
go test -count=1 ./internal/hooks/ ./internal/usercommands/
```
Expected: no gofmt output, clean build, all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/NewRound_UserRoundTick.go internal/hooks/enchant_tierup_cap_test.go internal/usercommands/craft.go
git commit -m "feat(enchanting): the ceiling is enforced on enchanting, tier-up included

Enchanting is refused before the work begins rather than after, because
refusing at the start costs the player nothing while refusing at completion
can only refund materials once the rounds are already spent. Re-enchanting
an item subtracts what that item already holds, so replacing one
enchantment with another prices only the difference.

Tier-up is the harder half and was named nowhere in the original design.
It rolls every combat round on every enchanted item worn, and it doubles
the reserved share at low tiers, so a character sitting just under the
ceiling could cross it mid-fight having done nothing whatever. It now skips
and says so. The item's accumulated uses are deliberately kept rather than
reset, so it advances the moment its wearer makes room instead of losing
the progress it earned.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: The two auto-spawn paths, and the login refresh

Three sites that the compiler cannot point at, all of which write a reservation with no budget check whatsoever:

- **Brood-mother floor** (`tickBroodMotherFloor` / `spawnBroodFloor`). Fires every round for a Brood Mother apex owner with no live companion. It has never called an affordability check of any kind.
- **Chrysifier homunculus** (`tickHomunculus` / `spawnHomunculus`). Same, and its own docstring says so explicitly: "There is NO affordability gate."
- **Login backfill** (`backfillCompanionReserves`). Stamps a reserve onto legacy zero-reserve companions with no budget check.

The backfill also carries D11. `ConvictionReserve` is frozen at summon time, so existing companions would otherwise keep their pre-U7b numbers indefinitely, and returning veterans would never see the rebase that is the whole reason no migration hurts. Recomputing every companion's reserve at login, not only the zero ones, subsumes the backfill: a legacy zero and a stale 352 are the same problem, and one function fixes both.

**Files:**
- Modify: `internal/hooks/manifester_companions.go` (`spawnBroodFloor` at :81-111)
- Modify: `internal/hooks/chrysifier_homunculus.go` (`tickHomunculus` docstring at :75-80, `spawnHomunculus` at :108-157)
- Modify: `internal/hooks/companion_reserve_backfill.go` (rename and widen)
- Modify: `internal/hooks/PlayerSpawn_HandleJoin.go` (:65-69)
- Test: `internal/hooks/companion_reserve_backfill_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `internal/hooks/companion_reserve_backfill_test.go`:

```go
// D11. Reserves are RECOMPUTED at login, not merely backfilled when zero.
// ConvictionReserve is frozen at summon time, so without this a returning
// veteran keeps their pre-U7b figures forever and never sees the rebase that is
// the entire reason no migration hurts.
func TestRefreshCompanionReserves_RecomputesAStaleSnapshot(t *testing.T) {
	ch := characters.New()
	ch.ConvictionMax.Base = 500
	ch.Validate()
	ch.Companions = []characters.CompanionInfo{
		// A golem carrying the old flat 352-derived figure.
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised, ConvictionReserve: 158},
		// A legacy record that never had one at all.
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised, ConvictionReserve: 0},
	}

	if !refreshCompanionReserves(ch) {
		t.Fatalf("a stale snapshot must be reported as changed")
	}
	if ch.Companions[0].ConvictionReserve == 158 {
		t.Errorf("the stale 158 was not recomputed")
	}
	if ch.Companions[1].ConvictionReserve == 0 {
		t.Errorf("the legacy zero was not stamped")
	}
	if ch.Companions[0].ConvictionReserve != ch.Companions[1].ConvictionReserve {
		t.Errorf("two identical companions must recompute to the same reserve, got %d and %d",
			ch.Companions[0].ConvictionReserve, ch.Companions[1].ConvictionReserve)
	}
}

// Recomputing must be idempotent: a second login must not move the number
// again, or every login would drift a returning player's budget.
func TestRefreshCompanionReserves_IsIdempotent(t *testing.T) {
	ch := characters.New()
	ch.ConvictionMax.Base = 500
	ch.Validate()
	ch.Companions = []characters.CompanionInfo{
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
	}

	refreshCompanionReserves(ch)
	first := ch.Companions[0].ConvictionReserve

	if refreshCompanionReserves(ch) {
		t.Errorf("a second refresh must report no change")
	}
	if ch.Companions[0].ConvictionReserve != first {
		t.Errorf("a second refresh moved the reserve from %d to %d", first, ch.Companions[0].ConvictionReserve)
	}
}

// D4 grandfathering at login. Recomputing must NEVER dismiss a companion, even
// if the recomputed total sits past the ceiling. Refuse additions, never force a
// removal.
func TestRefreshCompanionReserves_NeverDismisses(t *testing.T) {
	ch := characters.New()
	ch.ConvictionMax.Base = 100 // tiny pool: any companion breaches
	ch.Validate()
	ch.Companions = []characters.CompanionInfo{
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
	}

	refreshCompanionReserves(ch)
	if len(ch.Companions) != 2 {
		t.Fatalf("login recompute dismissed a companion: %d left, want 2", len(ch.Companions))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hooks/ -run TestRefreshCompanionReserves -v`
Expected: FAIL, `undefined: refreshCompanionReserves`.

- [ ] **Step 3: Widen the backfill into a refresh**

In `internal/hooks/companion_reserve_backfill.go`, replace `backfillCompanionReserves` (lines 29-46) with:

```go
// refreshCompanionReserves recomputes ConvictionReserve on EVERY companion from
// the reserve that companion's mob id would be charged today (D11).
//
// It replaced a backfill that only stamped records loading as 0. Widening it was
// the point: ConvictionReserve is frozen at summon time so it cannot drift while
// a companion is fielded, which is right within a session and wrong across the
// U7b rebase. A legacy zero and a stale pre-rebase figure are the same problem,
// and recomputing at login fixes both. Meirok's two golems rebase from 158 each
// to 126 each on his next login with no action from him.
//
// It NEVER dismisses a companion, whatever the recomputed total comes to
// (D4 grandfathering). Reservation is refused on ADDITION only.
//
// Returns true if any reserve moved, so the caller knows whether to recalculate.
func refreshCompanionReserves(ch *characters.Character) bool {
	changed := false
	for i := range ch.Companions {
		want := ch.CalcCompanionReserve(legacyCompanionBaseReserve(ch.Companions[i].MobId))
		if ch.Companions[i].ConvictionReserve != want {
			ch.Companions[i].ConvictionReserve = want
			changed = true
		}
	}
	return changed
}
```

Rename `legacyCompanionBaseReserve` to `companionBaseReserveFor` in the same file and update its doc comment's first line to "companionBaseReserveFor resolves the base (pre-reduction) Conviction cost a companion of this mob id would be charged if created today", since it is no longer only about legacy records. Update the call above to match.

- [ ] **Step 4: Call the refresh at login**

In `internal/hooks/PlayerSpawn_HandleJoin.go`, replace lines 65-69 with:

```go
	// D11: recompute every companion's reserve from what it would cost today.
	// The snapshot is deliberately frozen at summon time so it cannot drift
	// mid-life, which makes login the only place a rebase can reach a returning
	// veteran. This never dismisses anyone; reservation is refused on addition
	// only.
	if refreshCompanionReserves(user.Character) {
		user.Character.RecalculateStats()
	}
```

- [ ] **Step 5: Gate the brood-mother floor**

In `internal/hooks/manifester_companions.go`, in `spawnBroodFloor`, move the reserve calculation ABOVE the spawn and gate on it. Replace lines 81-111 with:

```go
func spawnBroodFloor(user *users.UserRecord, room *rooms.Room) *mobs.Mob {
	ch := user.Character

	// U7b: check the budget BEFORE spawning. This path had no affordability
	// check of any kind, so it wrote a reservation into a pool that might have
	// had no room for it, every round, forever. Failing here returns nil, which
	// the caller already handles by backing off for ten rounds.
	reserve := ch.CalcCompanionReserve(broodFloorReserve)
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		return nil
	}

	mob := mobs.NewMobByIdFresh(mobs.MobId(broodSpawnMobId), room.RoomId, 120)
	if mob == nil {
		return nil
	}
	room.AddMob(mob.InstanceId)
	mob.Character.Charm(user.UserId, 99999, "")
	mob.Character.EndAggro()
	ch.TrackCharmed(mob.InstanceId, true)

	info := characters.CompanionInfo{
		MobId:             broodSpawnMobId,
		InstanceId:        mob.InstanceId,
		SourceType:        characters.CompanionSummoned,
		Name:              mob.Character.Name,
		BaseName:          mob.Character.Name,
		AutoAssist:        true,
		ConvictionReserve: reserve,
	}
	if !ch.AddCompanion(info) {
		ch.TrackCharmed(mob.InstanceId, false)
		room.RemoveMob(mob.InstanceId)
		mobs.DestroyInstance(mob.InstanceId)
		return nil
	}
	ch.RecalculateStats()
	return mob
}
```

- [ ] **Step 6: Gate the homunculus, and say why it refused**

In `internal/hooks/chrysifier_homunculus.go`, replace the last paragraph of `tickHomunculus`'s doc comment (lines 76-80, the sentence beginning "There is NO affordability gate") with:

```go
// U7b added the reservation gate this path never had. Its old docstring said
// "There is NO affordability gate -- the homunculus is the owner's apex identity
// and always manifests", which was true and is no longer: an ungated write into
// a capped pool is exactly what the ceiling exists to stop.
//
// The homunculus is a CRAFTING apex whose owner has no particular reason to
// have invested in manifestation. At the old base of 1000 the cap would have
// made it unfieldable by exactly the character it is built for, while leaving
// it fieldable by a summoner who does not need it. Owner decision 2026-08-15:
// the base drops to 300. Only one homunculus can exist at a time regardless,
// which hasLiveHomunculus already enforces.
//
// STILL WATCH THIS IN PLAYTEST. 300 fits a 66% cap only from roughly 455
// Conviction max upward, and nearer 500 once the rank-0 rider penalty applies,
// so a low-Conviction crafter can still be refused. The refusal is spoken
// rather than silent precisely so that shows up as a report instead of a
// mystery; the lever, if it bites, is HomunculusConvictionReserve.
```

- [ ] **Step 6a: Lower the homunculus base reserve to 300 (owner, 2026-08-15)**

In `internal/configs/config.balance.mobs.go`, change the default:

```go
	if b.HomunculusConvictionReserve < 1 {
		b.HomunculusConvictionReserve = 300
	}
```

And in `internal/configs/config.balance.go`, correct the field comment so it no
longer advertises the old number:

```go
	HomunculusConvictionReserve   ConfigInt   `yaml:"HomunculusConvictionReserve"`   // Chrysifier: base Conviction the homunculus reserves before reduction (default 300; was 1000 before the U7b cap made that unfieldable by its own crafter)
```

Update the assertion in `internal/configs/config.balance.chrysifier_test.go`
from `!= 1000` / `want 1000` to `!= 300` / `want 300`.

**The controller also adds the key to `_datafiles/config.yaml`** so the value is
documented where it lives. Do not edit that file yourself.

**Do NOT add a count cap.** `hasLiveHomunculus`
(`internal/hooks/chrysifier_homunculus.go:65`) already returns early at
`:89` when a live homunculus exists, so the max of one the owner asked for is
present today. Adding a second gate would be two mechanisms for one rule.

Then replace `spawnHomunculus`'s reserve section. Move the reserve above the spawn and gate on it: insert immediately after `cfg := configs.GetBalanceConfig()` at line 110:

```go
	reserve := ch.CalcCompanionReserve(int(cfg.HomunculusConvictionReserve))
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		// Spoken, not silent. The caller backs off for ten rounds on nil, so a
		// silent refusal here would look to the player exactly like the apex
		// being broken.
		user.SendText(messaging.CategorySpellManifestation,
			`Your homunculus stirs and will not hold its shape. `+
				ch.ReservationRefusal(characters.PoolConviction))
		return nil
	}
```

and delete the now-duplicated `reserve := ch.CalcCompanionReserve(int(cfg.HomunculusConvictionReserve))` at line 137.

Add `"github.com/GoMudEngine/GoMud/internal/messaging"` to the file's imports.

- [ ] **Step 7: Run the tests**

```bash
gofmt -l internal/
go build ./...
go test -count=1 ./internal/hooks/
```
Expected: no gofmt output, clean build, all tests pass. Any existing test naming `backfillCompanionReserves` must be renamed to `refreshCompanionReserves`; read it first, because `TestBackfillCompanionReserves_UnknownMobUsesDefault` asserts real fallback behaviour that must survive the rename intact.

- [ ] **Step 8: Commit**

```bash
git add internal/hooks/manifester_companions.go internal/hooks/chrysifier_homunculus.go \
        internal/hooks/companion_reserve_backfill.go internal/hooks/companion_reserve_backfill_test.go \
        internal/hooks/PlayerSpawn_HandleJoin.go
git commit -m "fix(companions): the two auto-spawn paths and the login refresh respect the ceiling

The brood-mother floor and the Chrysifier homunculus both wrote a
reservation into the owner's pool with no affordability check of any kind,
every round, forever. The homunculus said so in its own docstring. Both now
check before they spawn, and both already had a back-off the caller applies
on failure. The homunculus speaks its refusal rather than backing off
silently, because it is a crafting apex whose owner has no reason to have
invested in manifestation and it carries the heaviest base reserve in the
game, so if that combination bites it should arrive as a report rather than
a mystery.

The login backfill becomes a refresh. Reserves are frozen at summon time,
which is right within a session and wrong across a rebase, so a returning
veteran would otherwise keep pre-U7b figures indefinitely. Recomputing
every companion rather than only the zero-reserve ones subsumes the old
backfill: a legacy zero and a stale figure are the same problem. It never
dismisses anyone, whatever the recomputed total comes to.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: The behaviour fixes

These are not polish. Each one makes a pet multiplier a lie until it is fixed: a water elemental priced at 0.75 of a magma elemental that silently discards two rounds in five is not 0.75 of a magma elemental.

**Files (all under `_datafiles/world/dogmud/mobs/summons/`):**
- Modify: `313-fire_elemental.yaml`, `310-water_elemental.yaml`, `300-skeleton.yaml`, `111-hive_swarm.yaml`, `312-air_elemental.yaml`

- [ ] **Step 1: Fix mob 313, the fire elemental**

It carries `behavior_archetype: melee_self_buff`, whose first branch looks for a `self_offense` spell, and its spellbook contains none. So it casts one ward at the start of a fight and returns Failure forever after.

Give it the vampire's setup (mob 304 is the working reference for this archetype). In `313-fire_elemental.yaml`, change the `spellbook:` block from:

```yaml
  spellbook:
    conviction-armor: 3
    conviction-ward: 3
```

to:

```yaml
  spellbook:
    conviction-armor: 3
    conviction-ward: 3
    conviction-surge: 3
  skills:
    spellcasting: 1
```

`conviction-surge` is the game's only `self_offense` spell, which is what the archetype's first branch needs to match. The `skills:` block is the second half of Step 4 and is folded in here so the file is edited once.

- [ ] **Step 2: Fix mobs 310, 300 and 111, the archetype-less three**

None of the three has a `behavior_archetype` at all, so each falls through to the legacy AI, where an empty `combatcommands` entry returns "I acted" and consumes the round. Water and skeleton discard roughly 40% of their rounds; the hive swarm roughly 50%.

In each file, add `behavior_archetype: generic_fighter` on the line immediately after `archetype: fighting`, matching the placement mob 312 and 313 already use:

- `310-water_elemental.yaml`: after line 3 (`archetype: fighting`)
- `300-skeleton.yaml`: after line 3 (`archetype: fighting`)
- `111-hive_swarm.yaml`: after line 3 (`archetype: fighting`)

`generic_fighter` is the bandit DPS archetype and is the right fit for all three: none has a spellbook, none tanks, and all three are meant to simply hit things.

- [ ] **Step 3: Leave the flavour emotes alone**

Mob 300's `combatcommands` includes two pure-flavour emotes that also consume rounds. **Do not delete them.** With `behavior_archetype` set, the behaviour tree takes the round before the legacy `combatcommands` list is consulted, so the emotes stop costing anything and remain available as flavour. Deleting them would strip character from the mob to fix a problem the archetype already fixed. Note this in your report so a reviewer does not file it as a miss.

- [ ] **Step 4: Give mob 312 its spellcasting skill**

Mobs 312 (air) and 313 (fire) carry spellbooks but no `spellcasting` skill entry, unlike 302, 303 and 304, which all set `spellcasting: 1`. They cast at skill 0.

313 was handled in Step 1. In `312-air_elemental.yaml`, add a `skills:` block immediately before the existing `stats:` block:

```yaml
  skills:
    spellcasting: 1
```

Match the indentation of the sibling `spellbook:` and `stats:` keys (two spaces, inside `character:`), and check `304-vampire.yaml` for the exact shape before writing.

- [ ] **Step 5: Verify the files parse and the fields took**

```bash
python -c "import yaml,glob; [yaml.safe_load(open(f)) for f in glob.glob('_datafiles/world/dogmud/mobs/summons/*.yaml')]" && echo YAML_OK
grep -n "behavior_archetype\|spellcasting" _datafiles/world/dogmud/mobs/summons/*.yaml
```
Expected: `YAML_OK`, then `behavior_archetype` on 111, 300, 310, 312, 313 and the existing 304, and `spellcasting: 1` on 302, 303, 304, 312 and 313.

**A mob YAML with a bad field or a name/filename mismatch panics at STARTUP, not at parse time.** The python check proves the YAML is well-formed and nothing more; Task 14's boot test is what proves the engine accepts it.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/world/dogmud/mobs/summons/
git commit -m "fix(summons): four summoned creatures stop throwing away their rounds

A pet's price is a lie while the pet cannot act. The water elemental, the
skeleton and the hive swarm had no behaviour archetype at all, so each fell
through to the legacy AI where an empty combat-command entry reports that
it acted and consumes the round: roughly two rounds in five for the first
two and one in two for the swarm. All three become generic fighters, which
is what they always were.

The fire elemental had an archetype whose first branch looks for a
self-offence spell and a spellbook containing none, so it warded itself
once at the start of a fight and then did nothing at all for the rest of
it. It gains the same self-offence spell the vampire already uses.

The air and fire elementals also carried spellbooks with no spellcasting
skill behind them, unlike every other summon that casts, so they were
casting untrained. Both now start at one, matching their siblings.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: The four raw-max reads, and two comment corrections

**FOUR reads move, not six.** The U7 review rejected all six on the premise that mobs cannot carry a reservation. That premise is false for companions: `GetPoolReservation` has no `IsMob` gate, and Meirok's two golems wear enchanted gear reserving their own health and conviction on the live save.

But two of the six are genuinely unaffected, and for a reason that has nothing to do with the original premise: `internal/behaviortree/conditions_mob.go` and `internal/behaviortree/action_cast_best_in_category.go` both read through `mobs.FindPackmatesInRoom`, which **skips charmed mobs** (`internal/mobs/packmates.go:42`), and every companion is charmed. Those two get their comments corrected, because the comments currently give the false reason, and a future reader acting on that reason would change the wrong thing.

Magnitude scales with the ceiling: at 66% a fully geared companion reads as permanently at 34% health to every scorer that divides by the raw max.

**Files:**
- Modify: `internal/combat/ai.go` (:632 `ScoreGrapple`, :687 `ScoreDrain`, :722 `preferredSpell`)
- Modify: `internal/behaviortree/actions_archer.go` (`hpPercent` at :185-189)
- Modify: `internal/behaviortree/conditions_mob.go` (:132-136, comment only)
- Modify: `internal/behaviortree/action_cast_best_in_category.go` (:121-122, comment only)
- Test: `internal/combat/companion_raw_max_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/companion_raw_max_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// Reservation is not woundedness. A companion at its FULL reachable health
// reads as badly hurt to every scorer that divides by the raw max, which skews
// its own tactical choices toward panic behaviour it has no reason for.
//
// This is the same defect U7 fixed player-side. It was left in place at these
// sites on the premise that mobs cannot carry a reservation, which is false for
// companions: GetPoolReservation has no IsMob gate, and companions wearing
// enchanted gear reserve on prod today.
func TestCompanionAtFullReachableHealthIsNotScoredAsWounded(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999970: {ItemId: 999970, Name: "burdened collar", Type: items.Neck, ReserveHealthPct: 0.60},
	})()

	c := characters.New()
	c.HealthMax.Base = 100
	c.Equipment.Neck = items.New(999970)
	c.Validate()

	// The companion is at its ceiling: RecalculateStats has clamped current
	// health to max - reservation, which IS full for this character.
	c.Health = 999
	c.Validate()

	if eff := c.EffectivePoolMax(characters.PoolHealth); c.Health != eff {
		t.Fatalf("fixture: health %d is not the effective max %d", c.Health, eff)
	}

	pct := float64(c.Health) * 100.0 / float64(c.EffectivePoolMax(characters.PoolHealth))
	if pct < 99 {
		t.Fatalf("a companion at its ceiling must score as full health, got %.1f%%", pct)
	}

	raw := float64(c.Health) * 100.0 / float64(c.HealthMax.Value)
	if raw >= 99 {
		t.Fatalf("fixture is not exercising the defect: the raw reading is %.1f%%, "+
			"which means the reservation did not take", raw)
	}
}
```

This test pins the *property* rather than driving each scorer, because `ScoreGrapple`, `ScoreDrain` and `preferredSpell` all need a live mob instance and a room. Verifying the three call sites is Step 3's grep.

- [ ] **Step 2: Run test to verify it passes as a fixture check**

Run: `go test ./internal/combat/ -run TestCompanionAtFullReachableHealth -v`
Expected: PASS. It documents the property and proves the fixture is real; the behaviour change is proven by Step 3's diff and Step 5's grep.

- [ ] **Step 3: Move the three reads in `ai.go`**

In `internal/combat/ai.go`, replace the comment and read at lines 629-632 with:

```go
	// SELF-side, and EffectivePoolMax deliberately. The old comment here said
	// "reservation comes from equipped reserve_*_pct items, Chrysalis
	// enchantments and fielded companions, none of which mobs carry, so
	// EffectivePoolMax would be a no-op that implies mobs do." That is FALSE for
	// companions: GetPoolReservation has no IsMob gate, and companions wearing
	// enchanted gear reserve on prod today. Against the raw max a geared
	// companion reads as permanently wounded and scores its own tactics for a
	// crisis it is not in. At the U7b ceiling that misreading tops out at
	// "permanently at 34% health".
	mobHealthPercent := float64(mob.Character.Health) * 100.0 / float64(mob.Character.EffectivePoolMax(characters.PoolHealth))
```

At lines 686-687:

```go
	// SELF-side, EffectivePoolMax -- see ScoreGrapple. Reservation is not
	// woundedness, and companions DO reserve.
	hpPct := float64(mob.Character.Health) * 100 / float64(mob.Character.EffectivePoolMax(characters.PoolHealth))
```

At lines 721-722:

```go
	// SELF-side, EffectivePoolMax -- see ScoreGrapple. A reserved companion must
	// not panic-heal at a full pool.
	selfPct := float64(mob.Character.Health) * 100 / float64(mob.Character.EffectivePoolMax(characters.PoolHealth))
```

`EffectivePoolMax` floors at 1, so none of these can divide by zero. Add the `characters` import if `ai.go` does not already have it (it almost certainly does; check before adding).

- [ ] **Step 4: Move `hpPercent`**

In `internal/behaviortree/actions_archer.go`, replace lines 183-190 with:

```go
// hpPercent returns the character's current health as a percentage of the max it
// can ACTUALLY reach (0..100). A non-positive max is treated as full health.
//
// EffectivePoolMax, not HealthMax.Value. Every caller is self-side, and a
// companion carrying reserving gear would otherwise read as permanently wounded
// at a completely full pool: at the U7b ceiling, permanently at 34%. That would
// make a reserved archer refuse to kite forever, since the kite branch bails
// when it judges itself too hurt to disengage safely.
func hpPercent(char *characters.Character) float64 {
	max := char.EffectivePoolMax(characters.PoolHealth)
	if max <= 0 {
		return 100
	}
	return float64(char.Health) * 100 / float64(max)
}
```

**Before writing this, confirm every caller is self-side.** Run `grep -n "hpPercent(" internal/behaviortree/*.go` and check each. If any call passes a character that is not the acting mob itself, stop and report it: a target-side read has different correctness (you want to know how hurt the enemy really is), and that call would need to stay on the raw max with its own comment.

- [ ] **Step 5: Correct the two comments, and only the comments**

In `internal/behaviortree/conditions_mob.go`, replace the comment at lines 132-136 with:

```go
		// Raw max on purpose, NOT EffectivePoolMax -- but not for the reason this
		// comment used to give. It said mobs carry no pool reservation, which is
		// false: GetPoolReservation has no IsMob gate and companions wearing
		// enchanted gear reserve on prod today. The real reason is narrower and
		// more durable: FindPackmatesInRoom SKIPS charmed mobs
		// (internal/mobs/packmates.go:42) and every companion is charmed, so no
		// reserving character can reach this loop. If that filter ever changes,
		// this read must change with it.
```

In `internal/behaviortree/action_cast_best_in_category.go`, replace lines 121-122 with:

```go
		// Raw max on purpose -- see condPackmateBelowHpRatio. Not because mobs
		// cannot reserve (they can, and companions do) but because
		// FindPackmatesInRoom skips charmed mobs, so no companion reaches here.
```

**Change nothing else in either file.** These two sites are correct as they stand; only the stated reason was wrong.

- [ ] **Step 6: Verify the sweep**

```bash
grep -n "HealthMax.Value\|StaminaMax.Value\|ConvictionMax.Value" internal/combat/ai.go internal/behaviortree/actions_archer.go
```
Expected: no output from either file.

```bash
gofmt -l internal/
go build ./...
go test -count=1 ./internal/combat/ ./internal/behaviortree/ ./internal/hooks/
```
Expected: no gofmt output, clean build, all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/ai.go internal/combat/companion_raw_max_test.go \
        internal/behaviortree/actions_archer.go internal/behaviortree/conditions_mob.go \
        internal/behaviortree/action_cast_best_in_category.go
git commit -m "fix(ai): a reserved companion is not a wounded one

Four self-side reads divided current health by the raw maximum while the
current value had already been clamped to maximum minus reservation, so a
companion sitting at its ceiling scored itself as badly hurt and chose its
tactics for a crisis it was not in. At the U7b ceiling that misreading tops
out at reading a completely full companion as permanently at a third
health. This is the same defect U7 fixed player-side.

They were left alone during the U7 review on the premise that mobs cannot
carry a reservation. That premise is false: the reservation total has no
mob gate whatsoever, and companions wearing enchanted gear reserve on
production today.

Two further sites keep the raw maximum and get their comments corrected
instead. They are right, but for a reason nobody had written down: both
read through the packmate finder, which skips charmed creatures, and every
companion is charmed. Recording the real reason matters because the reason
that was written down would have led the next reader to change them.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: The player-facing surface

D15: a reservation readout, in descriptive bands only, no numbers anywhere. Three pieces: a template function, a row on the `status` sheet, and the helpfiles.

**The status sheet needs its own short vocabulary.** `ReserveShareBand`'s phrases are prose fragments ("a significant portion" is 21 characters) and the sheet's columns are 13 wide, so the long band would break the box outright. Task 1 already shipped `reservationBand` for this, whose top label keys off the **cap** rather than a fixed fraction, so a player can see when they have no room left to add without ever being shown a number.

**Files:**
- Modify: `internal/templates/templatesfunctions.go` (add `reservationQuality` beside `encumbranceQuality` at :304-317)
- Modify: `_datafiles/world/dogmud/templates/character/status.template`
- Modify: `internal/usercommands/assess.go` (delegate `reserveShareBand`)
- Create: `_datafiles/world/dogmud/templates/help/reservation.template`
- Modify: `_datafiles/world/dogmud/keywords.yaml`
- Modify: `_datafiles/world/dogmud/templates/help/companion.template`, `enchanting.template`, `conviction.template`, `manifestation.template`
- Test: `internal/templates/reservation_quality_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/templates/reservation_quality_test.go`, modelled on the existing `encumbrance_quality_test.go` (read that file first and match its harness exactly):

```go
package templates

import (
	"strings"
	"testing"
)

// The status sheet reserves a 13-character column. Every band label must fit
// inside it or the box border breaks, and no label may contain a digit.
func TestReservationQualityFitsTheStatusColumn(t *testing.T) {
	for _, band := range []string{"none", "slight", "modest", "notable", "heavy", "at limit"} {
		if len(band) > 13 {
			t.Errorf("band %q is %d characters and will break the status box", band, len(band))
		}
		if strings.ContainsAny(band, "0123456789") {
			t.Errorf("band %q contains a digit; reservation is disclosed in words only", band)
		}
	}
}

// Rendering pads to the requested width and colours by severity, matching the
// vitalQuality / toxicityQuality / encumbranceQuality convention.
func TestReservationQualityRenders(t *testing.T) {
	out, err := Process(`{{ reservationQuality "at limit" 13 }}`, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(out, "at limit") {
		t.Errorf("rendered output %q does not contain the band", out)
	}
	if !strings.Contains(out, "red-bold") {
		t.Errorf("the at-limit band must render in the most severe colour, got %q", out)
	}
}
```

Replace `Process(...)` with whatever entry point `encumbrance_quality_test.go` uses; do not invent a template API.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/templates/ -run TestReservationQuality -v`
Expected: FAIL, `function "reservationQuality" not defined`.

- [ ] **Step 3: Add the template function**

In `internal/templates/templatesfunctions.go`, immediately after the `encumbranceQuality` entry (which ends at line 317), add:

```go
		// reservationQuality(bandName[, padTo]) returns a colored word for how
		// much of a pool is held in reserve, from Character.ReservationBandName.
		// Optionally padded to padTo visual characters, following the
		// vitalQuality / toxicityQuality / encumbranceQuality convention.
		//
		// Words only, never a number. "at limit" is not a fraction: it means the
		// reservation ceiling has been reached and nothing further can be added,
		// which is the one thing a player actually needs to act on.
		"reservationQuality": func(bandName string, padTo ...int) string {
			var color string
			switch bandName {
			case "notable":
				color = "yellow"
			case "heavy":
				color = "red"
			case "at limit":
				color = "red-bold"
			default: // "none", "slight", "modest"
				color = "green"
			}
			result := `<ansi fg="` + color + `">` + bandName + `</ansi>`
			if len(padTo) > 0 && padTo[0] > len(bandName) {
				result += strings.Repeat(" ", padTo[0]-len(bandName))
			}
			return result
		},
```

- [ ] **Step 4: Add the row to the status sheet**

In `_datafiles/world/dogmud/templates/character/status.template`, insert a new line immediately after line 8 (the Health / Stamina / Conviction row), pushing the Toxicity row down:

```
 │ <ansi fg="yellow">Reserved:   </ansi>{{ reservationQuality (.Character.ReservationBandName "health") 13 }}            {{ reservationQuality (.Character.ReservationBandName "stamina") 13 }}            {{ reservationQuality (.Character.ReservationBandName "conviction") 13 }} │
```

The alignment is deliberate and must be exact. The row above is ` │ ` (3) + label (12) + value (13), three times, + ` │` (2) = 80. This row uses the same shape but replaces the second and third labels with 12 spaces each, so each band sits directly beneath the pool it belongs to. That placement is the whole design: nobody has to be told which pool "heavy" refers to.

Count the characters after writing. A miscount by one breaks the box border on every `status` for every player.

- [ ] **Step 5: Delegate `reserveShareBand` so there is one set of edges**

In `internal/usercommands/assess.go`, replace `reserveShareBand` (lines 150-171) with:

```go
// reserveShareBand delegates to characters.ReserveShareBand, which is where the
// edges now live because `Wear` and the auto-spawn refusals need them too and
// cannot import usercommands. Kept as a package-local name so assess.go and
// stand.go read unchanged.
func reserveShareBand(reserve, maxPool int) string {
	return characters.ReserveShareBand(reserve, maxPool)
}
```

- [ ] **Step 6: Write the reservation helpfile**

Create `_datafiles/world/dogmud/templates/help/reservation.template`. Hard-wrap at 78 characters. No en dashes and no em dashes; use `--` the way the existing helpfiles do. No numbers of any kind, including percentages. ESL-clear.

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">reservation</ansi>

Some things you carry do not simply help you -- they hold part of you
back. A Chrysalis enchantment feeds on the wearer. A companion draws on
your will for as long as it walks beside you. What they take is called
reservation, and it is set aside from one of your three pools for as long
as you keep them.

Reserved strength is not lost and it is not damage. Rest will not bring it
back, because nothing took it from you -- it is simply spoken for. What it
does mean is that your true ceiling is lower than your listed one, and
everything that measures you against a full pool measures you against the
part you can still reach.

<ansi fg="yellow">━━━ The Limit ━━━</ansi>

You cannot set aside more than a certain share of any one pool. When you
reach that limit, the game will refuse the thing that would push you past
it: wearing another draining item, beginning another enchantment, or
binding another companion. It refuses cleanly and costs you nothing.

It will never take anything away from you. If you are already past the
limit, everything you have stays exactly where it is. You will only be
told no when you try to add.

<ansi fg="yellow">━━━ Reading It ━━━</ansi>

Type <ansi fg="command">status</ansi> and look at the Reserved line under your pools. Each pool
gets a word of its own:

  <ansi fg="green">none</ansi> or <ansi fg="green">slight</ansi>    Nothing worth worrying about.
  <ansi fg="green">modest</ansi>            A real cost, comfortably borne.
  <ansi fg="yellow">notable</ansi>           You are giving up something to keep this.
  <ansi fg="red">heavy</ansi>             Close to the limit. Choose the next one carefully.
  <ansi fg="red-bold">at limit</ansi>          You cannot add anything more to this pool.

<ansi fg="yellow">━━━ Making Room ━━━</ansi>

Remove a draining item, disenchant one, or dismiss a companion, and the
share comes straight back.

Skill also buys room. A practised enchanter binds magic that takes less
from its wearer, and a practised summoner holds a companion on less of
their will. Both are worth raising if you mean to live this way.

<ansi fg="yellow">See Also:</ansi>

  <ansi fg="command">help status</ansi>, <ansi fg="command">help companion</ansi>, <ansi fg="command">help enchanting</ansi>, <ansi fg="command">help conviction</ansi>
```

- [ ] **Step 7: Register the topic**

In `_datafiles/world/dogmud/keywords.yaml`:

1. Add `      - reservation` to the `character:` list under `help: command:`, in the alphabetical position the list already uses.
2. Add an alias line beside the others in the alias block, matching their column alignment:

```yaml
  reservation:      [reserved, reserve, set aside]
```

- [ ] **Step 8: Cross-reference the four existing helpfiles**

Each needs a short paragraph and a `See Also` entry. Do not rewrite these files; add to them.

- `companion.template`: add a paragraph saying a companion holds part of your will for as long as it walks with you, that a stronger companion holds more, and that there is a limit to how much you can set aside at once. Add `help reservation` to its See Also.
- `enchanting.template`: add a paragraph saying a Chrysalis enchantment feeds on its wearer, that a deeper tier feeds more, that an enchantment will not deepen further when you have nothing left to give it, and that a skilled enchanter binds magic which takes less. Add `help reservation` to its See Also.
- `conviction.template`: add a sentence to the "Running Low" section noting that companions and some gear hold part of the pool aside permanently, and that `status` shows how much. Add `help reservation` to its See Also.
- `manifestation.template`: add a sentence saying that skill in manifestation lowers what a companion holds, so a practised summoner keeps more companions on the same will. Add `help reservation` to its See Also.

80-character wrap, no en or em dashes, no numbers, ESL-clear throughout.

- [ ] **Step 9: Verify**

```bash
gofmt -l internal/
go build ./...
go test -count=1 ./internal/templates/ ./internal/usercommands/ ./internal/devtools/
```
Expected: no gofmt output, clean build, all tests pass. `helpfile_completeness_test.go` exists in both `internal/usercommands/` and `internal/devtools/` and is what catches a topic registered with no template, or a template with no topic; if either fails, the registration in Step 7 is wrong.

```bash
awk '{ gsub(/<[^>]*>/, ""); if (length($0) > 80) print FILENAME": "FNR" ("length($0)")" }' \
  _datafiles/world/dogmud/templates/help/reservation.template \
  _datafiles/world/dogmud/templates/help/companion.template \
  _datafiles/world/dogmud/templates/help/enchanting.template \
  _datafiles/world/dogmud/templates/help/conviction.template \
  _datafiles/world/dogmud/templates/help/manifestation.template
```
Expected: no output. This strips the ANSI tags before measuring, since they are markup and not visible width.

```bash
grep -n "[–—]" _datafiles/world/dogmud/templates/help/*.template
```
Expected: no output. En and em dashes are banned in player copy.

- [ ] **Step 10: Commit**

```bash
git add internal/templates/templatesfunctions.go internal/templates/reservation_quality_test.go \
        internal/usercommands/assess.go \
        _datafiles/world/dogmud/templates/character/status.template \
        _datafiles/world/dogmud/templates/help/ \
        _datafiles/world/dogmud/keywords.yaml
git commit -m "feat(status): reservation is visible, in words

The status sheet gains a Reserved line sitting directly beneath the three
pools, so each pool's reserved share appears under the pool it belongs to
and nobody has to be told which is which. It is words only: none, slight,
modest, notable, heavy, at limit. The last of those is not a fraction. It
means the ceiling has been reached and nothing further can be added, which
is the one thing a player can actually act on.

The sheet needs its own short vocabulary because the prose phrases used in
sentences do not fit a status column. Both sets of edges now live in one
place, since the refusal messages need them too and cannot reach into the
command package.

A new help topic explains what reservation is, that rest cannot return it
because nothing took it, that reaching the limit refuses cleanly and costs
nothing, that nothing will ever be taken away, and that skill in enchanting
or manifestation buys room back. Four existing helpfiles now point at it.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Config, package docs and the paperwork

**Files:**
- Modify: `internal/characters/context.md`, `internal/spells/context.md`, `internal/enchantments/context.md`, `internal/hooks/context.md`
- Modify: **`docs/schemas/spell.md`** (added after Task 2: it documents all three retired fields, `summon_base_pool` and `summon_scaling_divisor` and `summon_conviction_reserve`, in its field table at :49-50 and in two worked examples at :113-114 and :131-132. A schema doc describing fields the loader no longer reads is worse than none, because content authors code against it.)
- Modify: `docs/PATCH_NOTES.md`
- Modify: `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`
- **Do NOT modify:** `_datafiles/config.yaml`

- [ ] **Step 1: Write the config note for the controller, do not apply it**

`_datafiles/config.yaml` carries `skip-worktree` and the owner's local overrides, and staging it has leaked local dev config to master before. **The implementer must not touch it.** Instead, put the following block verbatim in your task report so the controller can hand it to the owner:

> **Config changes the owner should apply to `_datafiles/config.yaml`.**
>
> Nothing here is required for correct behaviour: every new knob is absent from
> the file and falls through to its Go default, which is the shipped value. These
> are documentation and cleanup.
>
> **1. Add the new ceiling knob** to the `Balance` block, near the enchantment
> knobs:
>
> ```yaml
>   # ── POOL RESERVATION ────────────────────────────────────────────────────────
>   # Ceiling on TOTAL reservation per pool, as a fraction of that pool's max.
>   # Applies to health, stamina and conviction alike, and to companions as well
>   # as players. The breaching action is REFUSED rather than allowed through and
>   # clamped, and a character already past the ceiling keeps everything they
>   # have: only additions are refused.
>   #
>   # Raising this toward 1.0 walks back toward the pre-U7b state, where 96%
>   # health reservation was reachable with shipped gear. Lowering it makes
>   # multi-companion summoners and deep enchant stacks choose sooner. It is read
>   # by every enforcement site, so a change takes effect at once with no
>   # restart of anything but the server.
>   PoolReservationCapPct: 0.66
> ```
>
> **2. Delete two orphaned keys.** `ManifestStatScaleChaFactor` (line 1268) and
> `ManifestStatScaleSkillFactor` (line 1269) are still read, by
> `CalcSpawnPoolFromBase`, so **leave them**. Nothing else needs removing:
> `CompanionCastingFloorPct` was already absent, and the retired summon YAML keys
> never lived in `config.yaml`.
>
> So in practice there is exactly one optional addition and no deletions.

- [ ] **Step 2: Update `internal/characters/context.md`**

Add a section documenting the reservation surface. Every symbol named must exist; verify with:

```bash
grep -nE '^(func|type|const|var)\s' internal/characters/reservation.go
```

Cover, with verified signatures: `ReservationCap`, `WouldBreachReservationCap`, `ReservationOverages`, `ReservationSnapshot`, `Worsened`, `ItemReserveOnPool`, `EnchantReserveAt`, `ReserveShareBand`, `ReservationBandName`, `ReservationRefusal`, `CalcCompanionPool`, `CompanionReserveBase`, `CalcSpawnPoolFromBase`, `ManifestationPoolCoefficient`.

The Gotchas section must record, in this order:

1. **`GetPoolReservation` has no `IsMob` gate.** Companions reserve. Any code that assumes reservation is player-only is wrong.
2. **`Worsened`, not a cap test, at the equip seam.** A cap test refuses an over-cap character an equal-for-equal swap and thereby forces them to strip, which D4 rules out.
3. **`Wear` reverts by restoring the whole `Worn` value**, which is only sound because the placement helpers touch nothing else. `SortComponentItems` was moved out of `wearArmorSlot` for exactly this reason. Anything new added to a placement helper that mutates state outside `c.Equipment` breaks the revert silently.
4. **`CalcCompanionPool` applies the multiplier AFTER the corpse average.** Folding it into `B` collapses the pet tiers as corpses grow.
5. **`CalcSpawnPoolFromBase` is NOT the companion formula.** It is the behaviour-tree add scaler, its callers are authored boss encounters tuned against its exact curve, and moving them would nerf the Sentinel's adds roughly fivefold.
6. **`CalcCompanionReserve` composes the U7 rider onto the existing reduction, never replaces it.** Replacing is worse at every rank.
7. **Two band vocabularies on purpose.** `ReserveShareBand` is prose for sentences; `ReservationBandName` is a short column word whose top label keys off the cap. Merging them breaks the status box.

- [ ] **Step 3: Update the other three `context.md` files**

- `internal/spells/context.md`: `SummonPetMultiplier` replaces `SummonBasePool`; `SummonScalingDivisor` and `SummonConvictionReserve` are gone; reservation is derived, never authored.
- `internal/enchantments/context.md`: `GetTierReservePct` returns the RAW tier percentage; the wearer's enchanting-rank rider is applied by `characters.EnchantReserveAt`, not here, so calling `GetTierReservePct` directly gives an unrideed figure.
- `internal/hooks/context.md`: `refreshCompanionReserves` recomputes every companion's reserve on login and replaces the old zero-only backfill; both auto-spawn paths now gate on the ceiling; enchant tier-up skips rather than breaching.

If any of these four files does not exist, create it following the project convention (Purpose / Files / core types / Public API / Gotchas / Dependencies / Consumers) and say so in your report.

- [ ] **Step 4: Add the patch note**

Prepend a dated entry to `docs/PATCH_NOTES.md`, under `# DOGMud Patch Notes` and above the existing `## 2026-08-15` entries. Player-facing framing, no raw numbers, no en or em dashes. Cover:

- There is now a limit to how much of yourself you can promise away, and the game refuses cleanly rather than letting you cripple yourself.
- Nothing is ever taken from you. If you are already past the limit, everything you have stays.
- `status` now shows what each pool has set aside, in words.
- Summoned and raised creatures were rebuilt. What you conjure is far cheaper to call and costs more to keep, and the difference between a weak servant and a strong one is now real at every level of corpse.
- Four summoned creatures had been throwing away a large share of their turns and now fight properly.
- Skill pays twice: a practised enchanter binds magic that takes less from its wearer, and a practised summoner holds more on the same will.

Draft opening, matching the voice of the entry above it:

```markdown
## 2026-08-15: What you promise away, and what you keep

Some of what you carry does not simply help you. A Chrysalis enchantment
feeds on its wearer, and a companion draws on your will for as long as it
walks beside you. Until now there was no limit at all on how much of
yourself you could promise away, and it was entirely possible to give away
so much that almost nothing was left to fight with. There is a limit now,
and the game refuses cleanly at it: the item is not worn, the enchantment is
not begun, the creature is not called, and you have lost nothing by asking.

Nothing will ever be taken from you. If you are already past the limit,
everything you have stays exactly where it is, and you will only be told no
when you try to add. Type status to see what each of your pools has set
aside, in plain words.
```

Continue in the same register for the companion rebuild and the four fixed creatures.

- [ ] **Step 5: Add a crib-sheet section**

In `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md` section 10 (Reserved for U7-U12), add `### 10b. U7b: the reservation ceiling` immediately after the existing `### 10a. U7` subsection, with these checks:

1. **`status` on Meirok before anything else.** His conviction should now read `notable` or `heavy`, not `at limit`: his two golems rebase on login and he should land comfortably under the ceiling with both kept. If it says `at limit`, the login refresh did not run.
2. **Try to breach it deliberately.** Stack reserving gear until `status` says `at limit`, then try to wear one more. Confirm the refusal names reservation, carries no numbers, and leaves the previous item still worn.
3. **Sidegrade while at the limit.** With one pool at `at limit`, swap one reserving item for another of the same weight. It must be ALLOWED. If it is refused, grandfathering is broken and the character has been forced to strip.
4. **Conjure a magma elemental as a mid-level summoner.** It used to be uncastable. Confirm the cast lands, then confirm `status` shows the reservation it now costs to keep.
5. **Raise a skeleton and a golem off the same corpse, one after the other.** The golem must be visibly the stronger of the two at every corpse size, not merely marginally so at large ones.
6. **Fight alongside a water elemental, a skeleton and a hive swarm.** Each should act on nearly every round. If any of the three regularly stands there doing nothing, its archetype fix did not take.
7. **A Chrysifier with the homunculus apex.** Confirm the homunculus still manifests. **If it refuses and speaks the reservation message, report it immediately** -- the homunculus is a crafting apex whose owner has no reason to have invested in manifestation, and it carries the heaviest base reserve in the game, so this is the interaction most likely to bite.
8. **Fight long enough for an enchant tier-up while near the limit.** Confirm the skip message arrives, and confirm the item still tiers up later once you remove something.

- [ ] **Step 6: Mark U7b done on the roadmap**

In `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, prefix the U7b row in the Plans table (line 179) with `✅ **DONE.**` following the pattern U3 and U4 use, and add a short outcome paragraph to the `### U7b` section recording what actually shipped, including the three places this plan extended the spec (see the closing section of this plan).

- [ ] **Step 7: Commit**

```bash
git add internal/characters/context.md internal/spells/context.md \
        internal/enchantments/context.md internal/hooks/context.md \
        docs/PATCH_NOTES.md docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md \
        docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md
git commit -m "docs(u7b): package docs, patch note and the playtest checks

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Note the explicit paths.** Never `git add -A` or `git add .` in this repository: three untracked items (`.agents/`, `.codex/`, `AGENTS.md`) live permanently in this working tree and would be swept in.

---

### Task 14: Verify and boot

- [ ] **Step 1: Formatting, build, full tests**

```bash
gofmt -l internal/ modules/     # must print nothing
go build ./...
go test -count=1 ./...
```

`internal/relationships` is quarantined by Windows Defender and is a known pre-existing failure, not yours. Anything else that fails is yours.

- [ ] **Step 2: Confirm the retired symbols are genuinely gone**

```bash
grep -rn "CalcCompanionStatPool\|CanAffordCompanion\|SummonBasePool\|SummonScalingDivisor\|SummonConvictionReserve\|CompanionCastingFloorPct\|backfillCompanionReserves" internal/ modules/ --include=*.go
grep -rn "summon_base_pool\|summon_scaling_divisor\|summon_conviction_reserve" _datafiles/
```
Expected: no output from either. A hit in `docs/` is fine and expected (completed plans reference the old names); a hit in `internal/`, `modules/` or `_datafiles/` is a miss.

- [ ] **Step 3: Boot test in an isolated detached worktree**

YAML data files panic at **startup**, not at parse time, on a filename or name-field mismatch, an ID collision or an unresolved reference. Nothing but a real boot catches these, and this slice edits eighteen data files.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
```

Then edit **the copy** (never the original) to use non-default ports and disable file logging: `TelnetPort 43333`, `LocalPort 19999`, `HttpPort 18090`, `AIPort 15555`, `Logging.LogToFile false`. The owner is running a server on 33333/44444/9999/8090/55555 and this must not collide with it.

```bash
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
```

**Exit code 124 is the SUCCESS case.** It means the timeout fired because the server stayed up. Build to `boot-check.exe` at a fixed path and never `go run .`: `go run` links into a randomly named temp directory every time, and Windows Firewall keys its rules on the executable path, so every run pops a security dialog at whoever is at the keyboard.

```bash
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
grep -n "Spell.Validate" boot.log                                        # want no summon warnings
```

**Do not grep for the bare word `panic`.** The config key `GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic` and produces false hits.

The third grep is this slice's specific check: a summon spell whose `summon_pet_multiplier` failed to land logs `summon_mob_id set but summon_pet_multiplier is 0 or missing` and would otherwise sail through as a warning nobody read.

- [ ] **Step 4: Clean up the worktree**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-boot-check
```
If Windows holds a lock, `rm -rf C:/tmp/dogmud-boot-check` then `git worktree prune`. Diff any tracked file before forcing the removal, so nothing written inside the worktree is lost.

- [ ] **Step 5: Report, do not commit**

This task produces no commit. Report the three grep counts and the exit code verbatim.

---

### Task 15: Adversarial playtest, then the PR

**This is the content gate and it is required, not optional.** This slice changes player-facing content (thirteen spell files, five mob files, six helpfiles, the status sheet) and live balance. Boot-clean and "YAML parses" verify the *system* and never the *experience*. Refusal messages that confuse, a status row that misaligns, a summon that is now too weak to be worth casting, a companion that stands there doing nothing: none of that is visible to a boot test or to reading the code.

- [ ] **Step 1: Author the goals file**

Create a goals file under `tools/playtest/goals/` named `2026-08-15-u7b-reservation.yaml`, with `ephemeral:` set (local runs require it). Drive these objectives, in order:

1. Read `status` and describe the Reserved row in your own words. **Does the box border line up? Is it obvious which word belongs to which pool?**
2. Acquire and wear reserving gear until one pool reads `at limit`. Report every message along the way.
3. Try to wear one more reserving item. **Is the refusal comprehensible? Does it tell you what to do about it? Does it leak any number?**
4. Swap one reserving item for another of the same weight while at the limit. It must be allowed.
5. `help reservation`, then `help companion` and `help enchanting`. **Does the help explain what just happened to you, or does it read as though written by someone who already knew?**
6. Cast every summon you can reach. Report which felt worth the price and which did not.
7. Raise a skeleton and then a golem from comparable corpses. **Is the golem obviously the better creature?**
8. Fight alongside a water elemental, a skeleton and a hive swarm and watch them for ten rounds each. **Does each one act on nearly every round?**
9. Try to enchant while at the limit.

- [ ] **Step 2: Run it with an explicitly adversarial mandate**

```text
/playtest local --checkout <abs-path-to-this-checkout> bug-finder 2026-08-15-u7b-reservation.yaml
```

Read every line of in-game output. Report every usability problem bluntly. Fix what it finds, re-run if the fix touched player-facing text, and only then hand over.

Two setup notes. Playtest profiles ship carrying three to five pounds, which sits at the bottom of the encumbrance curve, so re-kit if any objective depends on load. And to reach `at limit` quickly, an admin-granted reserving item beats grinding for one; say in your report which route you took.

- [ ] **Step 3: Wipe instance saves before any manual smoke test**

```bash
rm -rf _datafiles/world/dogmud/mobs.instances/* _datafiles/world/dogmud/rooms.instances/*
```

Stale instance saves silently shadow template edits, and this slice edits five mob templates. **Do NOT wipe `shops/`, `guilds/` or `moderation/`** -- those are persistent living state, not instance overrides.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feature/u7b-reservation-ceiling
gh pr create --repo pruuk/DOGMud --base master --head feature/u7b-reservation-ceiling --fill
```

**`--repo pruuk/DOGMud` on every single `gh` command.** This repository is a fork of `GoMudEngine/GoMud` and `gh` defaults to the parent; a bare `gh pr create` has opened a PR against upstream before and had to be closed immediately.

```bash
gh pr checks <n> --repo pruuk/DOGMud --watch
gh run view <id> --repo pruuk/DOGMud --log-failed
```

A green check is not proof. `gh pr checks --watch` can return before every workflow has re-run, and the lint gate is configured `only-new-issues`. Confirm which runs actually executed before concluding anything.

- [ ] **Step 5: Do NOT merge**

Stop at a green, reviewed PR and hand back. Merging to master **is** shipping in this project, the arc's standing rule is no deploy until the whole arc is done and playtested, and the balance riders in this slice need the owner's sign-off at the mandatory pre-deploy playtest.

---

## Spec coverage

Every decision and every section of `2026-08-15-u7b-reservation-ceiling-design.md`, and the task that implements it.

| Spec item | Task |
|---|---|
| D1 cap at 66% | 1 (`ReservationCap`, `PoolReservationCapPct`) |
| D2 per pool, all three | 1 (`ReservationOverages` covers health, stamina, conviction) |
| D3 refuse, do not clamp | 7 (equip), 8 (enchant), 4 step 5 (summon, charm), 9 (auto-spawn) |
| D4 grandfathering | 1 (`Worsened` + the `added <= 0` guard), 7 (sidegrade test), 9 (`NeverDismisses` test) |
| D5 `CanAffordCompanion` removed | 4 |
| D6 new companion formula | 3 (`CalcCompanionPool`) |
| D7 pet multipliers | 5 (the thirteen YAMLs), 3 (test table) |
| D8 cast costs 30-50 | 5 |
| D9 reservation scales with the multiplier | 4 (`CompanionReserveBase`), 5 |
| D10 both inverse-skill riders | 4 (companion side), 1 + 6 (item side) |
| D11 recompute on login | 9 (`refreshCompanionReserves`) |
| D12 field swap and deletion | 2 |
| D13 behaviour fixes | 10 |
| D14 tier-up skips | 8 |
| D15 reservation readout, bands only | 12 |
| D16 dismissal destroys gear, accepted | none; no code change, recorded here for completeness |
| §3.1 formula and its rounding | 3 |
| §3.2 multiplier table | 5 |
| §3.3 cast-cost table | 5 |
| §3.4 `280 x petMultiplier` | 4 |
| §4.1 compose, do not replace | 4 |
| §4.2 `GetTierReservePct` x enchanting rider | 1 (helper), 6 (wired) |
| §5 mob 313 `conviction-surge` | 10 step 1 |
| §5 mobs 310 / 300 / 111 `generic_fighter` | 10 step 2 |
| §5 mobs 312 / 313 `spellcasting: 1` | 10 steps 1 and 4 |
| §6.1 equip / wield | 7 |
| §6.1 enchant pre-flight | 8 |
| §6.1 enchant completion (already re-checks) | none needed; verified as already correct, recorded in 8 |
| §6.1 summon / conjure / raise | 4 step 5 |
| §6.1 charm | 4 step 5 |
| §6.1 brood-mother auto-spawn | 9 step 5 |
| §6.1 homunculus auto-spawn | 9 step 6 |
| §6.1 login backfill | 9 steps 3-4 |
| §6.1 message-quality gaps (`sell.go`, `mobcommands/equip.go`, `gearup`) | 7 step 6 |
| §6.2 tier-up | 8 |
| §6.2 `ConditionEnchantWithdrawal`, `BodyConvictionScale`, `MigrateEnchantments` | 1; all three are clamps on effect with no action to refuse, and D4 means they can never make an over-cap character worse. No further code needed, because `WouldBreachReservationCap` is consulted on ADDITION only and these three add nothing. Recorded here so a reviewer does not read it as a gap. |
| §7 migration, no forced dismissals | 9 |
| §8 readout, refusal wording, helpfiles | 12 |
| §9 out of scope | honoured; none of the five named items is touched |
| §10 trap 1 no `IsMob` gate | 11, and `context.md` in 13 |
| §10 trap 2 four reads not six | 11 |
| §10 trap 3 `ChaFactor` is 150 | 3 (fallback corrected to 150 in `CalcSpawnPoolFromBase`) |
| §10 trap 4 `CanAffordCompanion` is a 100% cap | 4 |
| §10 trap 5 `summon_scaling_divisor` never read | 2 |
| §10 trap 6 corpse pools are small | 3 (the test table spans 100 to 2800) |
| §10 trap 7 `return_damage` uncapped | out of scope per §9; no task |
| §10 trap 8 reflect on the species record | informational; no task |
| §10 trap 9 read `config.yaml` | honoured throughout; shipped values used, never Go defaults |
| §10 expected outcomes table | 3 (test), 15 (playtest objectives 6 and 7) |

## Deliberately not in this slice

Carried from the spec's §9, unchanged: U8's skill-strip; the poisoned `EnchantBaseline` on Meirok's arena tower shield; companion area spells hitting non-owner party members; `symbiotic-bond`'s decorative `companion_empowerment` value; retuning `CompanionSoftCap`.

Added by this plan: converting `actSummonCompanion` to the companion formula (see below).

## Decisions the spec left genuinely open

Three, all flagged where they occur and repeated here so the controller can overrule any of them before work starts.

1. **`summon_conviction_reserve` is deleted, not kept as an override.** D12 names only `summon_base_pool` and `summon_scaling_divisor`, but D9 derives reservation from the pet multiplier, which makes an authorable value beside it a second source of truth that will drift on the first retune. Deleted. If the owner wants per-spell reservation overrides, that is a field to add back deliberately, with a documented precedence rule.

2. **`CalcCompanionStatPool` is renamed and KEPT, not deleted.** The spec says the new formula "replaces" it, and it does for every player companion. But its other caller is `behaviortree.actSummonCompanion`, which spawns authored boss adds: the Core Guardian and Warden Prime at `base_pool: 50`, Old Edrin at 60, the Sentinel at 300. Putting those on the companion formula would nerf the Sentinel's adds roughly fivefold and buff the Core Guardian's by about a fifth, neither of which U7b intends and neither of which the spec discusses. It is renamed `CalcSpawnPoolFromBase` so nobody mistakes it for the companion formula again, and `ManifestStatScaleChaFactor` / `ManifestStatScaleSkillFactor` stay alive to serve it.

3. **`CompanionReserveDefault` keeps its name.** The spec calls this knob `CompanionReserveBase`. No such field exists; the shipped one is `CompanionReserveDefault` (Go default 280, absent from `config.yaml`). Renaming would be free, since the key is absent, but the existing name is what every other reference in the codebase uses and the churn buys nothing. The new *function* is named `CompanionReserveBase`, which is where the spec's word lands.

## Concerns

Two, both proceeding as the spec directs.

1. **The homunculus may become unfieldable by its own owner.** §6.1 requires the Chrysifier auto-spawn to respect the ceiling, and Task 9 implements that. But the homunculus is a **crafting** apex, `HomunculusConvictionReserve` defaults to 1000 (the heaviest in the game, absent from `config.yaml`), and the composed rider makes that 1100 at manifestation 0. A crafter with a conviction pool around 425 has a ceiling near 280, so the apex simply cannot manifest for the character it was designed for. The spec does not discuss this, and it is the one place where correct enforcement plausibly breaks working content.

   Proceeding as written, with two mitigations that cost nothing: the refusal is **spoken** rather than silent, so it arrives in playtest as a report instead of a mystery, and crib-sheet check 10b.7 targets it specifically. If it bites, the lever is `HomunculusConvictionReserve`, and lowering it is a Go-default edit needing no config change.

2. **The composed rider is a penalty at low rank on both sides at once.** §4.1 accepts a 10% penalty at rank 0 for companions, and §4.2 applies the same shape to items. A brand-new character who is both a novice enchanter and a novice summoner pays the penalty twice over, on two different pools, at exactly the point in the game where they have the least of both. Each half is individually settled and deliberate; the compounding is not discussed anywhere. It is almost certainly small in absolute terms, since a novice has little reserving gear and few companions, but it is worth a specific look during the playtest rather than an assumption. No code change proposed.
