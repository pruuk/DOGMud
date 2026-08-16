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
	return reservationCapFor(c.poolMax(p))
}

// reservationCapFor is ReservationCap keyed on a pool max rather than on a
// character. ReserveShareBand needs the ceiling and is a package function with
// no character to ask, and the two must never compute it differently.
func reservationCapFor(maxPool int) int {
	pct := float64(configs.GetBalanceConfig().PoolReservationCapPct)
	if pct <= 0 || pct > 1 {
		pct = 0.66
	}
	return int(math.Floor(float64(maxPool) * pct))
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

// ── Who is actually holding it ───────────────────────────────────────────────
//
// Every reservation message used to open with the fixed phrase "Your gear and
// bonds", whatever the player actually had. A 2026-08-16 playtest held exactly
// ONE COMPANION and wore nothing, and was told its gear was holding a share of
// its conviction. It had no gear. Naming a source the player does not have
// sends them hunting for it, which is the same defect as the refusal that
// claimed a holding when there was none.
//
// The two sources are separable at the point of truth: item reservation comes
// from the equipment walk in GetPoolReservation, companion reservation is the
// c.Companions sum in the same function. So the message can name what is really
// there, and offer only the remedies that could really help.

// reserveSources records which kinds of thing hold part of a given pool. Each
// one maps to a different remedy, which is why the two gear kinds are kept
// apart: disenchanting cannot help a pinnacle item whose own spec sets part of
// you aside, and taking off gear cannot help someone whose whole reservation is
// a companion.
type reserveSources struct {
	drainingItem bool // an item whose own spec reserves (reserve_*_pct)
	enchantment  bool // a Chrysalis enchantment on something worn
	companion    bool // a fielded companion, conviction only
}

func (s reserveSources) gear() bool { return s.drainingItem || s.enchantment }

func (s reserveSources) any() bool { return s.gear() || s.companion }

// reserveSourcesOn walks the same two places GetPoolReservation totals, so the
// subject of a sentence can never name a source that contributed nothing to the
// number in it.
func (c *Character) reserveSourcesOn(p Pool) reserveSources {
	poolMax := c.poolMax(p)
	pool := string(p)

	var s reserveSources
	for _, itm := range c.Equipment.GetAllItems() {
		if itm.HasChrysalisEnchantment() && itm.ReservePool == pool &&
			c.enchantReserveAtWithMax(itm.EnchantType, itm.EnchantTier, itm.GetSpec().Hands, poolMax) > 0 {
			s.enchantment = true
		}

		spec := itm.GetSpec()
		var itemPct float64
		switch p {
		case PoolHealth:
			itemPct = spec.ReserveHealthPct
		case PoolStamina:
			itemPct = spec.ReserveStaminaPct
		case PoolConviction:
			itemPct = spec.ReserveConvictionPct
		}
		if itemPct > 0 && int(math.Floor(float64(poolMax)*itemPct)) > 0 {
			s.drainingItem = true
		}
	}

	if p == PoolConviction {
		for i := range c.Companions {
			if c.Companions[i].ConvictionReserve > 0 {
				s.companion = true
				break
			}
		}
	}
	return s
}

// merge unions two source sets, for a message that names more than one pool.
func (s reserveSources) merge(o reserveSources) reserveSources {
	return reserveSources{
		drainingItem: s.drainingItem || o.drainingItem,
		enchantment:  s.enchantment || o.enchantment,
		companion:    s.companion || o.companion,
	}
}

// subject is the noun phrase that opens a reservation sentence, naming only
// what is really holding something.
//
// The fallback is deliberately vague rather than "Your gear and bonds": a
// sentence built from a set with nothing in it has no business asserting either
// source. In practice every caller has already established that the reservation
// is positive, so the fallback is unreachable; it exists so that a future caller
// that has not cannot reintroduce the falsehood.
func (s reserveSources) subject() string {
	switch {
	case s.gear() && s.companion:
		return `Your gear and bonds`
	case s.gear():
		return `Your gear`
	case s.companion:
		return `Your bonds`
	}
	return `What you carry`
}

// verb is the present-tense form of "hold" that agrees with subject(). Kept
// beside it because a variable subject with a fixed verb is how "Your gear now
// hold a heavy share" gets shipped.
func (s reserveSources) verb() string {
	if s.subject() == `Your gear` || s.subject() == `What you carry` {
		return `holds`
	}
	return `hold`
}

// remedies lists the ways this particular player could make room, in the same
// words help reservation's "Making Room" section uses.
//
// Telling someone to remove a draining item when they wear none is the mirror
// of naming gear they do not have. The full list is the fallback for the same
// unreachable case subject() guards.
func (s reserveSources) remedies() string {
	var parts []string
	if s.drainingItem || !s.any() {
		parts = append(parts, `remove a draining item`)
	}
	if s.enchantment || !s.any() {
		parts = append(parts, `disenchant something you wear`)
	}
	if s.companion || !s.any() {
		parts = append(parts, `dismiss a companion`)
	}
	return joinWithOr(parts)
}

// ReservationHolders returns the sentence subject naming what is really holding
// part of pool p ("Your gear", "Your bonds", "Your gear and bonds"), and the
// present-tense "hold"/"holds" that agrees with it.
//
// Exported for the login rebase notice in internal/hooks, which had the same
// fixed-phrase problem from the other direction: it credited the whole
// reservation to companions even when half of it was gear.
func (c *Character) ReservationHolders(p Pool) (subject, verb string) {
	s := c.reserveSourcesOn(p)
	return s.subject(), s.verb()
}

// ── Player-facing bands ──────────────────────────────────────────────────────
//
// ONE ladder, TWO vocabularies. Every rung below has a short word for the
// status sheet's 13-character column AND a prose fragment that has to read
// inside a sentence ("your gear is holding a heavy share of your stamina").
// They are two spellings of the SAME rung and MUST be changed together: if you
// edit a row, edit both halves of it, and if you add or move an edge, move it
// once in reserveRungOf where every caller sees it.
//
// They were keyed differently until 2026-08-16 and the result was three
// vocabularies contradicting each other at the same instant. With health at
// roughly two fifths of the pool and conviction at roughly two thirds, the
// equip line said "a significant portion" of health and "a heavy share" of
// conviction while the status sheet, one line away, said `heavy` and
// `near limit`. So `heavy` named two different fill levels at once, conviction
// was simultaneously heavy and near its limit, and the same prose phrase
// covered two states the sheet called different things. The cause was that the
// prose measured the POOL and the sheet measured the CEILING.
//
// Everything is measured against the CEILING now, because that is the question
// a player is actually asking: not "how much of me is spoken for" but "how much
// room have I got left". The row used to key only its top rung that way, which
// made it useless for its one job. The cap sits at roughly two thirds of the
// pool, so `heavy` needed half the POOL, which is three quarters of the
// ceiling, and there was no rung at all between that and having literally
// reached the cap. A 2026-08-15 playtest watched the row read `notable` through
// three consecutive refusals: the character was two thirds of the way to their
// ceiling and the sheet was still describing them as comfortable.

// reserveRung is a position on the shared severity ladder. Ordered, so a
// greater rung always means less room left.
type reserveRung int

const (
	rungNone reserveRung = iota
	rungSlight
	rungModest
	rungNotable
	rungHeavy
	rungNearLimit
	rungAtLimit
)

// reserveLadder holds both spellings of every rung side by side, so the two
// vocabularies physically cannot drift apart the way they did before: there is
// one row per rung and you cannot edit one half without seeing the other.
//
// The `short` column feeds the status sheet and help reservation's legend, and
// must stay inside 13 visible characters. The `prose` fragment completes the
// sentence "... hold <prose> of your <pool> in reserve", so it has to be a noun
// phrase that reads after "hold" and before "of your".
var reserveLadder = [...]struct {
	short string
	prose string
}{
	rungNone:      {`none`, `no part`},
	rungSlight:    {`slight`, `a slight part`},
	rungModest:    {`modest`, `a modest share`},
	rungNotable:   {`notable`, `a notable share`},
	rungHeavy:     {`heavy`, `a heavy share`},
	rungNearLimit: {`near limit`, `nearly all you can set aside`},
	rungAtLimit:   {`at limit`, `all you can set aside`},
}

// reserveRungOf is the ONLY place the edges live. Both vocabularies read it, so
// there is no second fraction ladder to fall out of step with this one.
//
// The cap is floor(maxPool * ~0.66), so with a positive pool it can only reach
// 0 on a pool of 1, where any reservation at all really has consumed the whole
// ceiling. `at limit` is the honest reading there, and it keeps the function
// total.
func reserveRungOf(reserve, maxPool, cap int) reserveRung {
	if maxPool <= 0 || reserve <= 0 {
		return rungNone
	}
	if cap <= 0 {
		return rungAtLimit
	}
	frac := float64(reserve) / float64(cap)
	switch {
	case frac >= 1.0:
		return rungAtLimit
	case frac >= 0.75:
		return rungNearLimit
	case frac >= 0.55:
		return rungHeavy
	case frac >= 0.35:
		return rungNotable
	case frac >= 0.15:
		return rungModest
	default:
		return rungSlight
	}
}

// ReserveShareBand names what SHARE of the reservable ceiling a reservation
// holds, as a prose fragment. Player-facing text never shows the raw number.
//
// It takes the pool max rather than the cap because every caller has the pool
// max to hand and none of them should be computing the ceiling themselves.
func ReserveShareBand(reserve, maxPool int) string {
	return reserveLadder[reserveRungOf(reserve, maxPool, reservationCapFor(maxPool))].prose
}

// ReservationBandName returns the short status-sheet word for a pool's current
// reservation. `pool` is a plain string so the template can call it directly.
func (c *Character) ReservationBandName(pool string) string {
	p := Pool(pool)
	max := c.poolMax(p)
	return reservationBand(c.GetPoolReservation(pool, max), max, c.ReservationCap(p))
}

// reservationBand is the short half of the ladder, taking the cap explicitly so
// tests can pin an edge without reaching through the config.
func reservationBand(reserve, maxPool, cap int) string {
	return reserveLadder[reserveRungOf(reserve, maxPool, cap)].short
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

// shareShrank is shareGrew with the comparison and both zero guards mirrored.
// Written out rather than expressed as !shareGrew because "did not grow" is not
// "shrank": an unchanged share is neither, and negating would make the remove
// line fire on every ordinary remove.
func shareShrank(beforeRes, beforeMax, afterRes, afterMax int) bool {
	if beforeRes <= 0 {
		return false
	}
	if beforeMax <= 0 || afterMax <= 0 {
		return afterRes <= 0
	}
	return afterRes*beforeMax < beforeRes*afterMax
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
	parts, sources := c.movedPoolShares(before, shareGrew)
	if len(parts) == 0 {
		return ``
	}
	return `Putting that on sets part of you aside. ` + sources.subject() + ` now ` +
		sources.verb() + ` ` + joinWithAnd(parts) + ` in reserve.`
}

// ReservationDecreaseNotice is the mirror image, for a player who has just taken
// something off. Same empty-string contract: callers must treat empty as "say
// nothing".
//
// Without it the feature was half built. The player was told when capacity was
// taken and never when it came back, so `remove` looked like it did nothing to
// the ceiling and the one lesson the disclosure exists to teach never landed.
//
// It shares movedPoolShares with the equip line ON PURPOSE, so the two cannot
// answer the same question differently: same SHARE test (a plain +Vitality item
// coming off shrinks the pool and the reserved points together, at an unchanged
// share, and stays silent), same band vocabulary, same joining. Only the
// direction of the comparison and the opening clause differ.
func (c *Character) ReservationDecreaseNotice(before ReservationTotals) string {
	parts, sources := c.movedPoolShares(before, shareShrank)
	if len(parts) == 0 {
		return ``
	}

	// Fully clear is the commonest shape of this line, and reciting "no part of
	// your health, no part of your stamina and no part of your conviction" is a
	// mouthful that buries the one thing the player wants to hear.
	if !c.holdsAnyReservation() {
		return `Taking that off gives part of you back. Nothing you carry holds ` +
			`any part of you in reserve now.`
	}

	return `Taking that off gives part of you back. ` + sources.subject() + ` now ` +
		sources.verb() + ` ` + joinWithAnd(parts) + ` in reserve.`
}

// holdsAnyReservation reports whether ANY pool is still spoken for. Asked of
// all three pools rather than only the ones that moved, so a release on one
// pool cannot announce that nothing is held while another pool still is.
func (c *Character) holdsAnyReservation() bool {
	for _, p := range []Pool{PoolHealth, PoolStamina, PoolConviction} {
		if c.GetPoolReservation(string(p), c.poolMax(p)) > 0 {
			return true
		}
	}
	return false
}

// movedPoolShares names every pool whose reserved share moved in the direction
// `moved` tests for, as "<band> of your <pool>", in a fixed pool order. It also
// returns the union of what is holding those pools NOW, so the sentence's
// subject names only sources the player really has.
//
// The band quoted is the pool's NEW total share, not the item's own
// contribution: the player's question either way is "where does this leave me",
// and the answer that lets them predict the next refusal is the total.
func (c *Character) movedPoolShares(before ReservationTotals, moved func(beforeRes, beforeMax, afterRes, afterMax int) bool) ([]string, reserveSources) {
	after := c.ReservationTotals()

	var parts []string
	var sources reserveSources
	for _, p := range []struct {
		pool                                 Pool
		beforeRes, beforeMax, afterRes, aMax int
	}{
		{PoolHealth, before.Health, before.HealthMax, after.Health, after.HealthMax},
		{PoolStamina, before.Stamina, before.StaminaMax, after.Stamina, after.StaminaMax},
		{PoolConviction, before.Conviction, before.ConvictionMax, after.Conviction, after.ConvictionMax},
	} {
		if !moved(p.beforeRes, p.beforeMax, p.afterRes, p.aMax) {
			continue
		}
		parts = append(parts, ReserveShareBand(p.afterRes, p.aMax)+` of your `+poolDisplayName(p.pool))
		sources = sources.merge(c.reserveSourcesOn(p.pool))
	}
	return parts, sources
}

// joinWithAnd renders a list as plain English: "a", "a and b", "a, b and c".
func joinWithAnd(parts []string) string {
	return joinWith(parts, ` and `)
}

// joinWithOr is joinWithAnd for a list of alternatives, which is what a remedy
// list is: the player needs to do ONE of them, not all of them.
func joinWithOr(parts []string) string {
	return joinWith(parts, ` or `)
}

func joinWith(parts []string, conjunction string) string {
	switch len(parts) {
	case 0:
		return ``
	case 1:
		return parts[0]
	case 2:
		return parts[0] + conjunction + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], `, `) + conjunction + parts[len(parts)-1]
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
// The holding case names its SOURCES rather than reciting a fixed phrase. This
// branch is shared by equip, enchant, tier-up, charm, summon, raise and the
// homunculus. On the companion paths the reservation actually blocking the
// player is very often another companion, so blaming gear alone would send them
// stripping armour when what they need to do is release a pet -- and the
// reverse is just as bad: a 2026-08-16 playtest held one companion, wore
// nothing, and was told its gear was holding a share of its conviction. See
// reserveSources.subject and .remedies.
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
	sources := c.reserveSourcesOn(p)
	return sources.subject() + ` already ` + sources.verb() + ` ` + share +
		` of your ` + pool + ` in reserve. You cannot take on more until you ` +
		`make room: ` + sources.remedies() + `.`
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
