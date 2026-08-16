package characters

import (
	"math"
	"strings"

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
//
// The 0.66 below is belt and braces, not a required fallback. An earlier
// version of this comment claimed the package loads no config in tests and so
// reads 0 here. That was measured and is FALSE: GetBalanceConfig returns fully
// defaulted values in this package's tests, PoolReservationCapPct among them.
//
// The guard stays anyway, for the same reason ContestFloor rejects zero: a cap
// of 0 or one out of range would not fail loudly, it would silently disable the
// ceiling everywhere, including in every test that thinks it is exercising it.
// A mechanism that quietly turns itself off is worse than a red build.
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
	return c.itemReserveOnPoolWithMax(itm, p, c.poolMax(p))
}

// itemReserveOnPoolWithMax is the single body both entry points share.
//
// GetPoolReservation needs the poolMax-taking form because RecalculateStats
// calls it mid-derivation, with a max it has just computed and not yet written
// back to the character. Reading c.poolMax(p) there would price the reservation
// against the PREVIOUS max. Keep exactly one body: an inlined second copy is
// what let the total and the per-item figure disagree silently before U7b.
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
	return c.enchantReserveAtWithMax(enchantType, tier, hands, c.poolMax(p))
}

// enchantReserveAtWithMax is the shared body, taking the pool max explicitly for
// the same mid-derivation reason itemReserveOnPoolWithMax does.
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

// reservationBand keys EVERY rung of its ladder to the CAP, not to the pool.
//
// It used to key only the top rung that way and measure the rest as a fraction
// of the full pool, which made the row useless for the one job it has. The cap
// sits at roughly two thirds of the pool, so `heavy` needed half the POOL,
// which is three quarters of the ceiling, and there was no rung at all between
// that and having literally reached the cap. A 2026-08-15 playtest watched the
// row read `notable` through three consecutive refusals: the character was two
// thirds of the way to their ceiling and the sheet was still describing them as
// comfortable, because two thirds of the ceiling is under half the pool.
//
// Measured against the cap, the ladder answers the question a player is
// actually asking, which is "how much room have I got left", and `near limit`
// gives them a rung of warning before the answer becomes none. `at limit` still
// means what it always meant: nothing further can be added.
//
// The cap is floor(maxPool * ~0.66), so with a positive pool it can only reach
// 0 on a pool of 1, where any reservation at all really has consumed the whole
// ceiling. `at limit` is the honest reading there, and it keeps the function
// total without a second fraction ladder to drift out of step with this one.
func reservationBand(reserve, maxPool, cap int) string {
	if maxPool <= 0 || reserve <= 0 {
		return `none`
	}
	if cap <= 0 {
		return `at limit`
	}
	frac := float64(reserve) / float64(cap)
	switch {
	case frac >= 1.0:
		return `at limit`
	case frac >= 0.75:
		return `near limit`
	case frac >= 0.55:
		return `heavy`
	case frac >= 0.35:
		return `notable`
	case frac >= 0.15:
		return `modest`
	default:
		return `slight`
	}
}

// ── The equip disclosure ─────────────────────────────────────────────────────

// ReservationTotals is a snapshot of the RESERVATION and the POOL MAX together,
// on all three pools.
//
// The pool max has to travel with the reserve figure, because equipping can
// move BOTH. A +Vitality item raises HealthMax, and every reserve_*_pct already
// worn is a percentage of that max, so the reserved POINTS climb even though
// the wearer has set no larger share of themselves aside. Comparing points
// alone would announce a reservation increase for a piece of gear that reserves
// nothing at all, which is precisely the "fires on every equip" failure the
// disclosure has to avoid.
//
// Distinct from ReservationSnapshot, which records overage PAST the cap and
// answers "did this get worse". This one records the raw position and answers
// "did more of me get set aside".
type ReservationTotals struct {
	Health        int
	Stamina       int
	Conviction    int
	HealthMax     int
	StaminaMax    int
	ConvictionMax int
}

// ReservationTotals snapshots the current reservation and pool max on all three
// pools. Take one BEFORE an equip attempt and hand it to
// ReservationIncreaseNotice afterwards.
func (c *Character) ReservationTotals() ReservationTotals {
	hMax := c.poolMax(PoolHealth)
	sMax := c.poolMax(PoolStamina)
	cMax := c.poolMax(PoolConviction)
	return ReservationTotals{
		Health:        c.GetPoolReservation(string(PoolHealth), hMax),
		Stamina:       c.GetPoolReservation(string(PoolStamina), sMax),
		Conviction:    c.GetPoolReservation(string(PoolConviction), cMax),
		HealthMax:     hMax,
		StaminaMax:    sMax,
		ConvictionMax: cMax,
	}
}

// shareGrew reports whether the reserved SHARE of a pool went up, comparing
// before/after as a cross-multiplication so there is no float epsilon to tune.
// An unchanged max reduces it to a plain point comparison.
func shareGrew(beforeRes, beforeMax, afterRes, afterMax int) bool {
	if afterRes <= 0 {
		return false
	}
	if beforeMax <= 0 || afterMax <= 0 {
		return beforeRes <= 0
	}
	return afterRes*beforeMax > beforeRes*afterMax
}

// ReservationIncreaseNotice returns the line to show a player who has just put
// on something that sets more of them aside, or the EMPTY STRING when nothing
// did. Callers must treat empty as "say nothing".
//
// It fires on a larger SHARE, not on more points -- see ReservationTotals for
// why those differ. It also stays silent on a swap that trades one reserving
// item for a heavier-on-one-pool, lighter-on-another set only where the share
// genuinely fell; any pool whose share rose is reported.
//
// Bands only, never a number, matching ReservationRefusal and the status
// sheet's Reserved row. The band quoted is the pool's NEW total share, not the
// item's own contribution: the player's question after equipping is "where does
// this leave me", and the answer that lets them predict the next refusal is the
// total.
func (c *Character) ReservationIncreaseNotice(before ReservationTotals) string {
	after := c.ReservationTotals()

	var parts []string
	for _, p := range []struct {
		pool                                 Pool
		beforeRes, beforeMax, afterRes, aMax int
	}{
		{PoolHealth, before.Health, before.HealthMax, after.Health, after.HealthMax},
		{PoolStamina, before.Stamina, before.StaminaMax, after.Stamina, after.StaminaMax},
		{PoolConviction, before.Conviction, before.ConvictionMax, after.Conviction, after.ConvictionMax},
	} {
		if !shareGrew(p.beforeRes, p.beforeMax, p.afterRes, p.aMax) {
			continue
		}
		parts = append(parts, ReserveShareBand(p.afterRes, p.aMax)+` of your `+poolDisplayName(p.pool))
	}

	if len(parts) == 0 {
		return ``
	}

	return `Putting that on sets part of you aside. Your gear and bonds now hold ` +
		joinWithAnd(parts) + ` in reserve.`
}

// joinWithAnd renders a list as plain English: "a", "a and b", "a, b and c".
func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ``
	case 1:
		return parts[0]
	case 2:
		return parts[0] + ` and ` + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], `, `) + ` and ` + parts[len(parts)-1]
}

// ReservationRefusal is the message every refusing path sends. It names
// RESERVATION as the cause rather than exhaustion or a generic failure, because
// a player has no other way to learn that their own gear is the obstacle and
// resting will never fix it. Bands only, never a number, matching the disclosure
// style `stand` and `assess` already use.
//
// `added` is the reservation the refused action would ADD, and it is required
// because there are two entirely different refusals hiding behind one cap test.
//
//	added > cap   The single thing on its own demands more than the ceiling
//	              allows. What the character already holds is irrelevant, and
//	              putting things down cannot help. This is the case an
//	              adversarial playtest hit first: a magma elemental refused as
//	              a FIRST companion, at zero reservation, told it "already
//	              held a small part" of a pool it held none of. Never send the
//	              holding wording here: at zero reservation it is simply false.
//	added <= cap  What is already held plus this addition crosses the ceiling.
//	              Here the holding wording is both true and actionable, and
//	              the arithmetic guarantees the character really is holding
//	              something (current + added > cap >= added implies current > 0).
//
// The holding case says "gear and bonds", not "gear". This branch is shared by
// equip, enchant, tier-up, charm, summon, raise and the homunculus. On the
// companion paths the reservation actually blocking the player is very often
// another companion, so blaming gear alone would send them stripping armour
// when what they need to do is release a pet.
//
// The remedy wording is lifted from help reservation's "Making Room" section on
// purpose. The pre-fix text said "set something else aside", which that helpfile
// defines as RESERVING -- so the refusal was telling the player to reserve more
// when it meant the exact opposite.
func (c *Character) ReservationRefusal(p Pool, added int) string {
	pool := poolDisplayName(p)

	if added > c.ReservationCap(p) {
		return `That alone would set aside more of your ` + pool +
			` than you are able to hold in reserve. Nothing you put down will ` +
			`make room for it. You need a deeper well to draw on, or something ` +
			`that asks less of you.`
	}

	max := c.poolMax(p)
	share := ReserveShareBand(c.GetPoolReservation(string(p), max), max)
	return `Your gear and bonds already hold ` + share + ` of your ` + pool +
		` in reserve. You cannot take on more until you make room: remove a ` +
		`draining item, disenchant one, or dismiss a companion.`
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
