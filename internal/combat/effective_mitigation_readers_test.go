package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// #465: the readers that used to see raw mitigation with no cap at all now
// read what combat applies.

// armoredChar carries raw mitigation as worn points on one body item.
func armoredChar(physical, magical, conviction int) characters.Character {
	c := *characters.New()
	c.Stats.Strength.ValueAdj = 100
	c.Stats.Dexterity.ValueAdj = 100
	c.Stats.Willpower.ValueAdj = 100
	c.Stats.Charisma.ValueAdj = 100
	c.HealthMax.Value = 100
	if physical+magical+conviction > 0 {
		c.Equipment.Body = items.Item{ItemId: 1, Spec: &items.ItemSpec{
			PhysicalMitigation:   physical,
			MagicalMitigation:    magical,
			ConvictionMitigation: conviction,
		}}
	}
	return c
}

// EffectiveMitigationFor is each channel's raw sum bent through that
// channel's own curve.
func TestEffectiveMitigationFor_BendsEachChannel(t *testing.T) {
	pinShippedCurve(t)
	c := armoredChar(40, 100, 370)
	assert.InDelta(t, 0.40, EffectiveMitigationFor(&c, ChannelPhysical), 1e-9, "below the knee: raw")
	assert.InDelta(t, 0.75, EffectiveMitigationFor(&c, ChannelMagical), 1e-9, "raw 1.00 bends to 0.75")
	assert.InDelta(t, 0.90, EffectiveMitigationFor(&c, ChannelConviction), 1e-9, "raw 3.70 reaches the cap")
}

// PowerScore weighs the average effective mitigation (x300): 370 raw points
// on one channel add 300 x 0.90 / 3 = 90, not the 370 the raw sum used to add.
func TestPowerScore_ReadsEffectiveMitigation(t *testing.T) {
	pinShippedCurve(t)
	bare := armoredChar(0, 0, 0)
	heavy := armoredChar(370, 0, 0)
	assert.InDelta(t, 90.0, PowerScore(heavy)-PowerScore(bare), 0.01)

	// Below the knee nothing changes: 40 points add 300 x 0.40 / 3 = 40.
	light := armoredChar(40, 0, 0)
	assert.InDelta(t, 40.0, PowerScore(light)-PowerScore(bare), 0.01)
}
