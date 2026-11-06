package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #794, owner ruling 2026-11-06: the Vitalis Bandolier holds each potion's
// effect at peak strength, and the effects it holds read as sustained in the
// conditions panel and the `conditions` command (both show a countdown only
// for a record that is not Permanent).

const (
	peakVitalisId = 999954
	peakVigorId   = 999955 // condition 54
	peakRenewId   = 999956 // condition 55
	peakPitId     = 999958 // condition 130, infravision reach from magnitude
	peakPitMag    = 20.0
)

func seedPeakAmbient() func() {
	restoreI := items.SeedItemsForTest(map[int]*items.ItemSpec{
		peakVitalisId: {ItemId: peakVitalisId, Name: "Vitalis Bandolier", Type: items.Belt,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: true, PreservesContents: true},
		peakVigorId: {ItemId: peakVigorId, Name: "vigor potion", Type: items.Potion, ConditionIds: []int{54}},
		peakRenewId: {ItemId: peakRenewId, Name: "renewal potion", Type: items.Potion, ConditionIds: []int{55}},
		peakPitId: {ItemId: peakPitId, Name: "pitsense tincture", Type: items.Potion, ConditionIds: []int{130},
			Magnitude: peakPitMag,
			Aging:     items.AgingThresholds{FermentRounds: 1500, PeakRounds: 6000, DecayRounds: 18000, SpoilRounds: 30000}},
	})
	restoreC := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		54: {ConditionId: 54, Name: "Vigor", Description: "vigor", RoundInterval: 1, TriggerCount: 150},
		55: {ConditionId: 55, Name: "Renewal", Description: "renewal", RoundInterval: 1, TriggerCount: 150},
		130: {ConditionId: 130, Name: "Pitsense", Description: "pitsense", RoundInterval: 1, TriggerCount: 100,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {UsesMagnitude: true}}},
	})
	return func() { restoreI(); restoreC() }
}

func attunePeak(t *testing.T, id int, potionIds ...int) *users.UserRecord {
	t.Helper()
	u := users.NewTestUser(id, "peak", "Peak", uint64(7000+id))
	c := u.Character
	c.Equipment.Belt = items.New(peakVitalisId)
	for _, pid := range potionIds {
		p := items.New(pid)
		p.Preserved = true
		c.PotionItems = append(c.PotionItems, p)
	}
	tickAmbientPotions(u, 100) // attuning
	c.SetMiscData("pinnacle_bandolier_attune_round", uint64(100))
	tickAmbientPotions(u, 150) // attuned
	return u
}

// drinkLands applies a drink's effect the way Condition_ApplyConditions does
// for a scaled potion: Character.AddConditionScaled, a timed record.
func drinkLands(t *testing.T, u *users.UserRecord, conditionId int) {
	t.Helper()
	if err := u.Character.AddConditionScaled(conditionId, 1.3); err != nil {
		t.Fatalf("drink did not land: %v", err)
	}
}

func heldRecord(u *users.UserRecord, conditionId int) *conditions.Condition {
	for _, h := range u.Character.Conditions.GetConditions(conditionId) {
		return h
	}
	return nil
}

// A fresh Pitsense Tincture in the Vitalis gives its peak reach, not its
// fresh reach and not nothing (a permanent record used to carry no magnitude).
func TestAmbientBandolier_HoldsAMagnitudePotionAtPeak(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer seedPeakAmbient()()

	u := attunePeak(t, 750, peakPitId)
	h := heldRecord(u, 130)
	if h == nil || !h.Permanent {
		t.Fatal("the tincture's condition should be held as a permanent record")
	}
	want := conditions.CapScaledMagnitude(conditions.EffectInfraReach, peakPitMag*items.PeakPotency)
	if h.Magnitude != want {
		t.Fatalf("held infravision reach = %v, want the peak reach %v", h.Magnitude, want)
	}
}

// A hand-drunk copy of a held condition lands as a timed record (the
// countdown the owner saw). A contents change the same round used to return
// before holding anything, so the record stayed timed and showed a
// countdown; the held effect must read sustained again after the tick.
func TestAmbientBandolier_HeldEffectStaysSustainedThroughADrinkAndAContentsChange(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer seedPeakAmbient()()

	u := attunePeak(t, 751, peakVigorId)
	drinkLands(t, u, 54)
	if h := heldRecord(u, 54); h == nil || h.Permanent {
		t.Fatal("fixture: the drink should land as a timed record")
	}

	renew := items.New(peakRenewId)
	renew.Preserved = true
	u.Character.PotionItems = append(u.Character.PotionItems, renew)
	tickAmbientPotions(u, 151) // contents changed this round

	if h := heldRecord(u, 54); h == nil || !h.Permanent {
		t.Fatal("the bandolier's held effect must read sustained (permanent) after the tick, not count down")
	}
}

// Drinking the slotted potion consumes it. The drink's own timed effect must
// run its course; the bandolier lets go only of its permanent hold.
func TestAmbientBandolier_DrinkingASlottedPotionKeepsTheDrinksEffect(t *testing.T) {
	drainEventQueueOnCleanup(t)
	defer seedPeakAmbient()()

	u := attunePeak(t, 752, peakVigorId)
	u.Character.PotionItems = nil // drunk
	drinkLands(t, u, 54)
	tickAmbientPotions(u, 151)

	if h := heldRecord(u, 54); h == nil || h.Permanent {
		t.Fatal("the drink's timed effect must survive the potion leaving the bandolier")
	}
}
