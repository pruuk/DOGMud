package characters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMutationUnitsReserveConviction(t *testing.T) {
	c := grown(t, 0, 3, false)
	// 3 units of 0.11 of 400 = 132.
	assert.Equal(t, 132, c.GetPoolReservation("conviction", 400))
	assert.Equal(t, 0, c.GetPoolReservation("health", 400), "the body reserves Conviction only")
}

func TestFullBudgetReservesTheCeilingExactly(t *testing.T) {
	c := grown(t, 1, 4, true) // 4 + 1 + 1 = 6 units
	assert.Equal(t, reservationCapFor(400), c.GetPoolReservation("conviction", 400))
}

func TestReservationNamesTheBody(t *testing.T) {
	c := grown(t, 0, 3, false)
	c.ConvictionMax.Value = 400
	subject, verb := c.ReservationHolders(PoolConviction)
	assert.Equal(t, "Your body", subject)
	assert.Equal(t, "holds", verb)

	c.Companions = append(c.Companions, CompanionInfo{Name: "a wolf", ConvictionReserve: 40})
	subject, verb = c.ReservationHolders(PoolConviction)
	assert.Equal(t, "Your body and bonds", subject)
	assert.Equal(t, "hold", verb)
}

func TestRefusalWithOnlyTheBodyHoldingOffersNoRemedy(t *testing.T) {
	c := grown(t, 3, 3, false) // six units, at the ceiling
	c.ConvictionMax.Value = 400
	msg := c.ReservationRefusal(PoolConviction, 10)
	assert.Contains(t, msg, "Your body")
	assert.NotContains(t, msg, "make room: .")
	assert.Contains(t, msg, "nothing you put down will free it")
}
