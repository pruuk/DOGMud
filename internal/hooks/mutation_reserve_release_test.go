package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/stretchr/testify/assert"
)

func seedTitan(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7, MaxRank: 4,
			ReserveUnits: []int{1, 1, 1, 1}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			ReserveUnits: []int{1, 1, 1, 2}},
		"thick-hide": {MutationId: "thick-hide", Name: "Thick Hide", Rarity: 3, MaxRank: 4},
	}))
}

func TestReleaseNewestCompanionsUntilUnderCeiling(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.Mutations = map[string]int{"titan-growth": 3, "extra-arms": 2} // 5 units = 220
	ch.Companions = []characters.CompanionInfo{
		{Name: "old wolf", InstanceId: 1, ConvictionReserve: 30},
		{Name: "new wolf", InstanceId: 2, ConvictionReserve: 30},
	}
	ch.ConvictionMax.Value = 400 // ceiling 264
	// A save from before the rule (held 0): the whole body share is growth.
	released := releaseCompanionsOverCeiling(ch)
	// 220 + 60 = 280 > 264: drop the newest (250 fits).
	if assert.Len(t, released, 1) {
		assert.Equal(t, "new wolf", released[0].Name)
	}
	if assert.Len(t, ch.Companions, 1) {
		assert.Equal(t, "old wolf", ch.Companions[0].Name)
	}
	assert.Equal(t, 5, ch.MutationReserveHeld, "the growth is acknowledged")
}

func TestNoReleaseWithoutReservingMutations(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.Companions = []characters.CompanionInfo{{Name: "wolf", InstanceId: 1, ConvictionReserve: 90}}
	ch.ConvictionMax.Value = 100
	assert.Empty(t, releaseCompanionsOverCeiling(ch), "D4: an overage with no body share is grandfathered")
	assert.Len(t, ch.Companions, 1)
}

// wearConvictionGear puts a worn item on ch that reserves pct of Conviction.
func wearConvictionGear(ch *characters.Character, pct float64) {
	ch.Equipment.Neck = items.Item{ItemId: 68010, Spec: &items.ItemSpec{
		ItemId: 68010, Name: "heavy torc", Type: items.Neck, ReserveConvictionPct: pct}}
}

// Gaining a zero-unit mutation over an existing overage releases nothing: the
// body did not grow (ruling 6).
func TestZeroUnitGainOverAnOverageReleasesNothing(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.ConvictionMax.Value = 400 // ceiling 264
	wearConvictionGear(ch, 0.70) // 280: gear alone is 16 over
	ch.Mutations = map[string]int{"titan-growth": 1}
	ch.Companions = []characters.CompanionInfo{{Name: "wolf", InstanceId: 1, ConvictionReserve: 30}}
	ch.MutationReserveHeld = 1
	ch.Mutations["thick-hide"] = 1
	assert.Empty(t, releaseCompanionsOverCeiling(ch))
	assert.Len(t, ch.Companions, 1)
	assert.Equal(t, 1, ch.MutationReserveHeld)
}

// A login re-price with the body's units unchanged releases nothing (D4).
func TestLoginWithUnchangedUnitsReleasesNothing(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.ConvictionMax.Value = 400
	ch.Mutations = map[string]int{"titan-growth": 3} // 132
	ch.Companions = []characters.CompanionInfo{
		{Name: "old wolf", InstanceId: 1, ConvictionReserve: 90},
		{Name: "new wolf", InstanceId: 2, ConvictionReserve: 90},
	} // a re-price left them 48 over
	ch.MutationReserveHeld = 3
	assert.Empty(t, releaseCompanionsOverCeiling(ch))
	assert.Len(t, ch.Companions, 2)
}

// Gear alone over the ceiling, then a body gain: the overage that was already
// there stays grandfathered and only the growth's own share is released,
// newest first. Gear is never touched.
func TestBodyGainOverGearOverageReleasesOnlyTheIncrease(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.ConvictionMax.Value = 400                     // ceiling 264
	wearConvictionGear(ch, 0.70)                     // 280: gear alone is 16 over
	ch.Mutations = map[string]int{"titan-growth": 1} // 44
	ch.Companions = []characters.CompanionInfo{
		{Name: "old wolf", InstanceId: 1, ConvictionReserve: 50},
		{Name: "mid wolf", InstanceId: 2, ConvictionReserve: 30},
		{Name: "new wolf", InstanceId: 3, ConvictionReserve: 20},
	} // 280 + 44 + 100 = 424: 160 over before the gain
	ch.MutationReserveHeld = 1
	ch.Mutations["titan-growth"] = 2 // +44: 204 over, 160 grandfathered

	released := releaseCompanionsOverCeiling(ch)
	// new wolf (20) leaves 184 over, past 160; mid wolf (30) leaves 154.
	if assert.Len(t, released, 2) {
		assert.Equal(t, "new wolf", released[0].Name)
		assert.Equal(t, "mid wolf", released[1].Name)
	}
	if assert.Len(t, ch.Companions, 1) {
		assert.Equal(t, "old wolf", ch.Companions[0].Name)
	}
	assert.Equal(t, 68010, ch.Equipment.Neck.ItemId, "gear is never removed")
	assert.Equal(t, 2, ch.MutationReserveHeld)
}

// A scour lowers the held units without releasing anyone.
func TestUnitDecreaseLowersHeld(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.ConvictionMax.Value = 400
	wearConvictionGear(ch, 0.70)
	ch.MutationReserveHeld = 4
	ch.Mutations = map[string]int{"titan-growth": 1}
	ch.Companions = []characters.CompanionInfo{{Name: "wolf", InstanceId: 1, ConvictionReserve: 30}}
	assert.Empty(t, releaseCompanionsOverCeiling(ch))
	assert.Len(t, ch.Companions, 1)
	assert.Equal(t, 1, ch.MutationReserveHeld)
}

func TestBondedCompanionsAreNeverReleased(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.Mutations = map[string]int{"titan-growth": 4, "extra-arms": 3} // 7 units, clamped to 264
	ch.Companions = []characters.CompanionInfo{{Name: "Jev", InstanceId: 1, SourceType: characters.CompanionBonded}}
	ch.ConvictionMax.Value = 400
	assert.Empty(t, releaseCompanionsOverCeiling(ch))
	assert.Len(t, ch.Companions, 1)
}
