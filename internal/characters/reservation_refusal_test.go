package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// ReservationRefusal has to tell two different refusals apart. An adversarial
// playtest of the U7b ceiling hit the second one first and was told a flat
// falsehood: with `Reserved: none none none` the game said its gear and bonds
// "already hold a small part" of a conviction pool it held none of, and then
// told it to "set something else aside" -- which help reservation defines as
// RESERVING, the exact opposite of the remedy.

// Case 1: the character holds NOTHING and the single addition exceeds the
// ceiling on its own. The message must not claim any existing holding, because
// there is none, and must not send the player off to shed gear that would not
// help.
func TestReservationRefusal_SingleDemandExceedsCeiling_ClaimsNoHolding(t *testing.T) {
	c := New()
	c.ConvictionMax.Base = 100
	c.Validate()

	if res := c.GetPoolReservation("conviction", c.ConvictionMax.Value); res != 0 {
		t.Fatalf("fixture: reservation = %d, want 0 (this test is about holding nothing)", res)
	}
	cap := c.ReservationCap(PoolConviction)
	added := cap + 1

	msg := c.ReservationRefusal(PoolConviction, added)

	// Every prose rung of the shared ladder, so adding a rung and forgetting
	// this list cannot quietly reintroduce the falsehood.
	lies := []string{"already hold", "already holds"}
	for _, rung := range reserveLadder {
		lies = append(lies, rung.prose)
	}
	for _, lie := range lies {
		if strings.Contains(msg, lie) {
			t.Errorf("refusal at ZERO reservation claims a holding (%q):\n%s", lie, msg)
		}
	}
	if !strings.Contains(msg, "That alone") {
		t.Errorf("the refusal must name the DEMAND, not the holding:\n%s", msg)
	}
	if !strings.Contains(msg, "reserve") {
		t.Errorf("the refusal must still name reservation as the cause:\n%s", msg)
	}
	assertPlayerCopyClean(t, msg)
}

// Case 2: the character IS holding a lot and this addition tips them over. The
// original wording is correct here and must survive -- the arithmetic
// guarantees a real holding (current + added > cap >= added implies current > 0).
func TestReservationRefusal_AlreadyHolding_KeepsTheHoldingWording(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999931: {ItemId: 999931, Name: "greedy torc", Type: items.Neck, ReserveConvictionPct: 0.60},
	})()

	c := New()
	c.ConvictionMax.Base = 100
	c.Equipment.Neck = items.New(999931)
	c.Validate()

	max := c.ConvictionMax.Value
	if res := c.GetPoolReservation("conviction", max); res != 60 {
		t.Fatalf("fixture: reservation = %d, want 60", res)
	}
	// Small enough to fit under the ceiling on its own, so the refusal is
	// genuinely caused by what is already held.
	added := 10
	if added > c.ReservationCap(PoolConviction) {
		t.Fatalf("fixture: added %d must be inside the cap %d", added, c.ReservationCap(PoolConviction))
	}
	if !c.WouldBreachReservationCap(PoolConviction, added) {
		t.Fatalf("fixture: this addition must breach, or the test proves nothing")
	}

	msg := c.ReservationRefusal(PoolConviction, added)

	// 60 of a ceiling of 66 is the `near limit` rung, which the refusal spells
	// out in prose. Same rung the status sheet would be showing at that instant.
	//
	// This character wears a draining item and has NO companion, so the subject
	// is "Your gear" alone and the verb agrees with it.
	if !strings.Contains(msg, "Your gear already holds nearly all you can set aside of your conviction in reserve") {
		t.Errorf("the holding wording must survive for a genuinely full pool:\n%s", msg)
	}
	// The remedy must agree with help reservation's "Making Room" section
	// rather than contradict it.
	if strings.Contains(msg, "set something else aside") {
		t.Errorf("the remedy still says \"set aside\", which the helpfile defines as RESERVING:\n%s", msg)
	}
	if !strings.Contains(msg, "remove a draining item") {
		t.Errorf("the remedy must name the one action that would help:\n%s", msg)
	}
	// And ONLY that action. This character has no companion to dismiss and no
	// enchantment to strip, so offering either sends it hunting for something
	// it does not have.
	for _, useless := range []string{"dismiss a companion", "disenchant"} {
		if strings.Contains(msg, useless) {
			t.Errorf("the remedy offers %q to a character that cannot do it:\n%s", useless, msg)
		}
	}
	assertPlayerCopyClean(t, msg)
}

// The boundary between the two branches. Exactly-at-cap is NOT a
// single-demand-too-large refusal: WouldBreachReservationCap allows an addition
// equal to the cap on an empty pool, so anything refused at that size was
// refused because of what is already held.
func TestReservationRefusal_BranchBoundary(t *testing.T) {
	c := New()
	c.ConvictionMax.Base = 100
	c.Validate()
	cap := c.ReservationCap(PoolConviction)

	if strings.Contains(c.ReservationRefusal(PoolConviction, cap), "That alone") {
		t.Errorf("an addition of exactly the cap is affordable on an empty pool; it must not be called too large on its own")
	}
	if !strings.Contains(c.ReservationRefusal(PoolConviction, cap+1), "That alone") {
		t.Errorf("one point past the cap can never fit, whatever is put down")
	}
}

func assertPlayerCopyClean(t *testing.T, msg string) {
	t.Helper()
	for _, digit := range "0123456789" {
		if strings.ContainsRune(msg, digit) {
			t.Errorf("player copy carries a raw number (%q):\n%s", digit, msg)
		}
	}
	if strings.ContainsAny(msg, "–—") {
		t.Errorf("no en or em dashes in player copy:\n%s", msg)
	}
	if strings.Contains(msg, "%") {
		t.Errorf("player copy carries a percentage:\n%s", msg)
	}
}
