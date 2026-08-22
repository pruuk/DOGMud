package characters

import "testing"

// reserveConvictionForTest fields a companion holding `amount` of the summoner's
// Conviction, which is the production path GetPoolReservation reads for that
// pool. Deliberately not an equipment fixture: the item path needs a species
// record and a spec, and the companion path is a plain snapshotted int.
func reserveConvictionForTest(t *testing.T, c *Character, amount int) {
	t.Helper()
	c.Companions = append(c.Companions, CompanionInfo{ConvictionReserve: amount})
	if got := c.GetPoolReservation(string(PoolConviction), c.ConvictionMax.Value); got != amount {
		t.Fatalf("fixture did not take: reservation is %d, want %d", got, amount)
	}
}

// OnRegenTick must derive its own effective pool. A caller cannot be trusted to
// subtract the reservation: six call sites did it by hand, correctly, with
// nothing pinning it. The fyttyn vitality exploit (2026-04-16) was exactly this
// faucet — a character at their reserved cap, unable to restore any higher,
// counting as permanently "depleted".
//
// These assert on the chance rather than on a stat actually gaining, because a
// progression roll is probabilistic and a test that waits for one is flaky by
// construction.
func TestRegenTickChance_FullEffectivePoolWithLargeReservationIsZero(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "Reserved"
	// Take the pool max the engine derived rather than forcing one: ConvictionMax
	// is a StatInfo whose Value comes from RecalculateStats, so assigning .Base
	// and calling Recalculate does not produce the number you asked for.
	max := c.ConvictionMax.Value
	reserveConvictionForTest(t, c, max/2)

	// At the top of what they can actually reach.
	c.Conviction = max - max/2

	if got := c.regenTickChance(PoolConviction); got != 0 {
		t.Errorf("a character at their effective cap has regen progression chance %v, want 0", got)
	}
}

// The complement: genuinely depleted must still progress, or the fix would have
// simply disabled the faucet rather than aimed it.
func TestRegenTickChance_DepletedPoolStillProgresses(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "Drained"
	max := c.ConvictionMax.Value
	reserveConvictionForTest(t, c, max/2)

	c.Conviction = 1 // deep inside the reachable half

	if got := c.regenTickChance(PoolConviction); got <= 0 {
		t.Errorf("a genuinely depleted character has chance %v, want > 0", got)
	}
}

// The trap this task exists to avoid, and the reason EffectivePoolMax is NOT
// used here despite being the obvious helper: it is floored at 1 and never
// returns 0, so at total reservation a naive implementation sees max=1,
// current=0, ratio=0 — the MAXIMUM chance, permanently. That is the same faucet
// inverted.
func TestRegenTickChance_TotallyReservedPoolIsZeroNotMaximum(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "FullyReserved"
	reserveConvictionForTest(t, c, c.ConvictionMax.Value) // the entire pool

	c.Conviction = 0

	if got := c.regenTickChance(PoolConviction); got != 0 {
		t.Errorf("a fully reserved pool has chance %v, want 0 "+
			"(EffectivePoolMax floors at 1, which would read as ratio 0 = maximum chance)", got)
	}
}

// With no reservation at all the behaviour is unchanged: this phase must not
// alter what an ordinary character experiences.
func TestRegenTickChance_UnreservedPoolIsUnaffected(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "Ordinary"
	c.Conviction = c.ConvictionMax.Value
	if got := c.regenTickChance(PoolConviction); got != 0 {
		t.Errorf("a full unreserved pool has chance %v, want 0", got)
	}

	c.Conviction = c.ConvictionMax.Value / 2
	if got := c.regenTickChance(PoolConviction); got <= 0 {
		t.Errorf("a half-empty unreserved pool has chance %v, want > 0", got)
	}
}
