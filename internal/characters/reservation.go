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
