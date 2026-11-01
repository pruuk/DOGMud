package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The drink parity table (drink path unification). A player and a mob, built
// alike and holding the same potion, drink it through actions.Quaff; every
// row asserts the two come out the same: the result, toxicity, the item
// consumed, the queued condition events, and the row's own special effect.
// Before unification the mob drank through a 49-line copy with no toxicity,
// no aging or crafter scaling and no special potions (owner ruling
// 2026-09-28: every special potion applies fully to a mob).

const (
	parityUserId  = 7301
	parityMobId   = 98301
	parityRoomId  = 99999
	parityBrewId  = 39301
	parityAgedId  = 39302
	parityTinctId = 39303
	parityToxicId = 39304

	parityHealCond  = 9301
	parityTinctCond = 9302
	parityHeldCond  = 61
)

// parityQueued is the part of a queued condition event both actors share.
type parityQueued struct {
	ConditionId  int
	DurationMult float64
	Triggers     int
	Magnitude    float64
}

// paritySide is one drinker after the drink.
type paritySide struct {
	res    DrinkResult
	char   *characters.Character
	queued []parityQueued
}

func newParityChar() *characters.Character {
	c := characters.New()
	c.Name = "Drinker"
	c.RoomId = parityRoomId
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	c.Health = 10
	c.HealthMax.Value = 100
	c.Mutations = map[string]int{}
	return c
}

func parityQueuedOf(evs []events.Condition) []parityQueued {
	out := make([]parityQueued, 0, len(evs))
	for _, e := range evs {
		out = append(out, parityQueued{e.ConditionId, e.DurationMult, e.Triggers, e.Magnitude})
	}
	return out
}

// drinkBoth builds a player and a mob, runs setup on each character (the
// item is stored by setup), drinks rest through actions.Quaff, and returns
// both sides.
func drinkBoth(t *testing.T, rest string, setup func(c *characters.Character)) (paritySide, paritySide) {
	t.Helper()
	room := newEmptyTestRoom(t)

	uc := newParityChar()
	setup(uc)
	u := &users.UserRecord{UserId: parityUserId, Character: uc}
	events.DrainQueuedConditionsForTest(parityUserId)
	ures := Quaff(NewUserActorInRoom(u, room).(DrinkActor), rest)
	player := paritySide{ures, uc, parityQueuedOf(events.DrainQueuedConditionsForTest(parityUserId))}

	m := &mobs.Mob{InstanceId: parityMobId, Character: *newParityChar()}
	setup(&m.Character)
	events.DrainQueuedMobConditionsForTest(parityMobId)
	mres := Quaff(NewMobActorInRoom(m, room).(DrinkActor), rest)
	mob := paritySide{mres, &m.Character, parityQueuedOf(events.DrainQueuedMobConditionsForTest(parityMobId))}

	return player, mob
}

func requireParity(t *testing.T, itemId int, player, mob paritySide) {
	t.Helper()
	if player.res != mob.res {
		t.Errorf("DrinkResult: player %+v, mob %+v", player.res, mob.res)
	}
	if player.char.Toxicity != mob.char.Toxicity {
		t.Errorf("toxicity after: player %v, mob %v", player.char.Toxicity, mob.char.Toxicity)
	}
	pLeft := countDrinkItemsById(player.char.Items, itemId)
	mLeft := countDrinkItemsById(mob.char.Items, itemId)
	if pLeft != mLeft {
		t.Errorf("item %d left: player %d, mob %d", itemId, pLeft, mLeft)
	}
	if len(player.queued) != len(mob.queued) {
		t.Fatalf("queued conditions: player %+v, mob %+v", player.queued, mob.queued)
	}
	for i := range player.queued {
		if player.queued[i] != mob.queued[i] {
			t.Errorf("queued condition %d: player %+v, mob %+v", i, player.queued[i], mob.queued[i])
		}
	}
}

func queuedHas(q []parityQueued, conditionId int) (parityQueued, bool) {
	for _, e := range q {
		if e.ConditionId == conditionId {
			return e, true
		}
	}
	return parityQueued{}, false
}

func seedParityWorld(t *testing.T) {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "Human", Size: species.Medium, Selectable: true},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		parityHealCond: {ConditionId: parityHealCond, Name: "Test Mending", RoundInterval: 1, TriggerCount: 10,
			TickPool: "health", TickPercent: 0.10},
		parityTinctCond: {ConditionId: parityTinctCond, Name: "Test Tincture", RoundInterval: 1, TriggerCount: 400,
			Flags: []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{
				conditions.EffectNightVisionStrength: {Literal: 12}, conditions.EffectInfraReach: {UsesMagnitude: true}}},
		parityHeldCond: {ConditionId: parityHeldCond, Name: "Ironhide Brew", RoundInterval: 1, TriggerCount: 400},
		70:             {ConditionId: 70, Name: "Purging Draught", RoundInterval: 1, TriggerCount: 1},
		75:             {ConditionId: 75, Name: "Nausea", RoundInterval: 1, TriggerCount: 5},
		76:             {ConditionId: 76, Name: "Purging Weakness", RoundInterval: 1, TriggerCount: 50},
		90:             {ConditionId: 90, Name: "Communion", RoundInterval: 1, TriggerCount: 30},
		93:             {ConditionId: 93, Name: "Bloom Detox", RoundInterval: 1, TriggerCount: 400},
	}))
	aging := items.AgingThresholds{FermentRounds: 100, PeakRounds: 200, DecayRounds: 400, SpoilRounds: 800}
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		parityBrewId: {ItemId: parityBrewId, Name: "test brew", Type: items.Potion, Subtype: items.Drinkable,
			Uses: 1, Toxicity: 5, ConditionIds: []int{parityHealCond}},
		parityAgedId: {ItemId: parityAgedId, Name: "aged brew", Type: items.Potion, Subtype: items.Drinkable,
			Uses: 1, Toxicity: 5, ConditionIds: []int{parityHealCond}, Aging: aging, BottleAgingMultiplier: 1.0},
		parityTinctId: {ItemId: parityTinctId, Name: "test tincture", Type: items.Potion, Subtype: items.Drinkable,
			Uses: 1, Magnitude: 20, ConditionIds: []int{parityTinctCond}},
		parityToxicId: {ItemId: parityToxicId, Name: "strong brew", Type: items.Potion, Subtype: items.Drinkable,
			Uses: 1, Toxicity: 1000, ConditionIds: []int{parityHealCond}},
		purgingDraughtItemId: {ItemId: purgingDraughtItemId, Name: "purging draught", Type: items.Potion,
			Subtype: items.Drinkable, Uses: 1, Toxicity: 34, ConditionIds: []int{70}},
		ysoldesPurgeItemId: {ItemId: ysoldesPurgeItemId, Name: "ysoldes purge", Type: items.Potion,
			Subtype: items.Drinkable, Uses: 1, Toxicity: 26, ConditionIds: []int{93}},
		catalystOfUnmakingItemId: {ItemId: catalystOfUnmakingItemId, Name: "catalyst of unmaking",
			Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
		phialOfSecondBirthItemId: {ItemId: phialOfSecondBirthItemId, Name: "phial of second birth",
			Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
		bloomWaferItemId: {ItemId: bloomWaferItemId, Name: "bloom wafer", Type: items.Potion,
			Subtype: items.Drinkable, Uses: 1, Toxicity: 20},
	}))
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"common-1": {MutationId: "common-1", Name: "Common One", Rarity: 2},
		"rare-1":   {MutationId: "rare-1", Name: "Rare One", Rarity: 7},
		"rare-2":   {MutationId: "rare-2", Name: "Rare Two", Rarity: 7},
	}))
	util.SetRoundCountForTest(100000)
	t.Cleanup(util.ResetRoundCountForTest)
}

func storeItem(t *testing.T, c *characters.Character, it items.Item) {
	t.Helper()
	if !c.StoreItem(it) {
		t.Fatalf("could not store item %d", it.ItemId)
	}
}

func TestDrinkParity(t *testing.T) {
	seedParityWorld(t)

	t.Run("plain healing potion", func(t *testing.T) {
		p, m := drinkBoth(t, "test brew", func(c *characters.Character) { storeItem(t, c, items.New(parityBrewId)) })
		requireParity(t, parityBrewId, p, m)
		if !p.res.Drank || p.char.Toxicity != 5 {
			t.Errorf("player: %+v toxicity %v, want drank at 5", p.res, p.char.Toxicity)
		}
		if q, ok := queuedHas(m.queued, parityHealCond); !ok || q.DurationMult != 1.0 {
			t.Errorf("mob heal condition queued %+v %v, want DurationMult 1.0", q, ok)
		}
	})

	t.Run("crafted potion at peak", func(t *testing.T) {
		p, m := drinkBoth(t, "aged brew", func(c *characters.Character) {
			it := items.New(parityAgedId)
			it.CraftedRound = util.GetRoundCount() - 300 // inside peak (200 to 400) at skill 0 speed
			it.CraftSkill = 0
			storeItem(t, c, it)
		})
		requireParity(t, parityAgedId, p, m)
		q, ok := queuedHas(m.queued, parityHealCond)
		if !ok || q.DurationMult != 1.30 {
			t.Errorf("mob peak potion queued %+v %v, want DurationMult 1.30 (peak potency)", q, ok)
		}
	})

	t.Run("crafted potion with crafter skill", func(t *testing.T) {
		p, m := drinkBoth(t, "aged brew", func(c *characters.Character) {
			it := items.New(parityAgedId)
			it.CraftedRound = util.GetRoundCount() - 10 // fresh
			it.CraftSkill = 50
			storeItem(t, c, it)
		})
		requireParity(t, parityAgedId, p, m)
		q, ok := queuedHas(m.queued, parityHealCond)
		if !ok || q.DurationMult != 1.5 {
			t.Errorf("mob crafted potion queued %+v %v, want DurationMult 1.5 (skill 50)", q, ok)
		}
	})

	t.Run("magnitude potion", func(t *testing.T) {
		p, m := drinkBoth(t, "test tincture", func(c *characters.Character) { storeItem(t, c, items.New(parityTinctId)) })
		requireParity(t, parityTinctId, p, m)
		q, ok := queuedHas(m.queued, parityTinctCond)
		if !ok || q.Magnitude != 20 || q.Triggers != 400 {
			t.Errorf("mob tincture queued %+v %v, want magnitude 20 over 400 triggers", q, ok)
		}
	})

	t.Run("spoiled potion", func(t *testing.T) {
		p, m := drinkBoth(t, "aged brew", func(c *characters.Character) {
			it := items.New(parityAgedId)
			it.CraftedRound = util.GetRoundCount() - 5000
			storeItem(t, c, it)
		})
		requireParity(t, parityAgedId, p, m)
		if !m.res.Drank || !m.res.Spoiled || m.char.Toxicity != 15 {
			t.Errorf("mob: %+v toxicity %v, want drank spoiled at 15 (three times 5)", m.res, m.char.Toxicity)
		}
		if _, ok := queuedHas(m.queued, 75); !ok {
			t.Errorf("mob spoiled drink did not queue nausea 75: %+v", m.queued)
		}
	})

	t.Run("past toxicity tolerance", func(t *testing.T) {
		p, m := drinkBoth(t, "strong brew", func(c *characters.Character) { storeItem(t, c, items.New(parityToxicId)) })
		requireParity(t, parityToxicId, p, m)
		if m.res.Refusal != DrinkRefuseToxicity || m.res.Drank {
			t.Errorf("mob: %+v, want refused for toxicity", m.res)
		}
		if countDrinkItemsById(m.char.Items, parityToxicId) != 1 {
			t.Errorf("a refused potion must stay in the pack")
		}
	})

	t.Run("purging draught", func(t *testing.T) {
		p, m := drinkBoth(t, "purging draught", func(c *characters.Character) {
			storeItem(t, c, items.New(purgingDraughtItemId))
			if err := c.AddConditionScaled(parityHeldCond, 1.0); err != nil {
				t.Fatalf("setup: %v", err)
			}
			c.Toxicity = 10
		})
		requireParity(t, purgingDraughtItemId, p, m)
		for _, side := range []paritySide{p, m} {
			side.char.Conditions.Prune()
			if side.char.HasCondition(parityHeldCond) {
				t.Errorf("the draught must strip the held potion condition")
			}
			if side.char.Toxicity != 0 {
				t.Errorf("toxicity %v after the draught, want 0", side.char.Toxicity)
			}
			if _, ok := queuedHas(side.queued, 76); !ok {
				t.Errorf("the draught must queue the weakness 76: %+v", side.queued)
			}
		}
	})

	t.Run("ysoldes purge", func(t *testing.T) {
		p, m := drinkBoth(t, "ysoldes purge", func(c *characters.Character) {
			storeItem(t, c, items.New(ysoldesPurgeItemId))
			c.BloomAddiction = 8
		})
		requireParity(t, ysoldesPurgeItemId, p, m)
		if p.char.BloomAddiction != 3 || m.char.BloomAddiction != 3 {
			t.Errorf("addiction after the purge: player %d, mob %d, want 3", p.char.BloomAddiction, m.char.BloomAddiction)
		}
	})

	t.Run("catalyst of unmaking", func(t *testing.T) {
		p, m := drinkBoth(t, "catalyst", func(c *characters.Character) {
			storeItem(t, c, items.New(catalystOfUnmakingItemId))
			c.Mutations = map[string]int{"common-1": 2}
			c.MutationProgress = 0.7
		})
		requireParity(t, catalystOfUnmakingItemId, p, m)
		for name, side := range map[string]paritySide{"player": p, "mob": m} {
			if len(side.char.Mutations) != 0 || side.char.MutationRerollBonus != scourRerollCharges || side.char.MutationProgress != 0 {
				t.Errorf("%s after the catalyst: mutations %v, reroll %d, progress %v", name,
					side.char.Mutations, side.char.MutationRerollBonus, side.char.MutationProgress)
			}
		}
	})

	t.Run("phial of second birth", func(t *testing.T) {
		p, m := drinkBoth(t, "phial", func(c *characters.Character) {
			storeItem(t, c, items.New(phialOfSecondBirthItemId))
			c.Mutations = map[string]int{"common-1": 1}
		})
		requireParity(t, phialOfSecondBirthItemId, p, m)
		// Which rare mutation is granted is a roll; that one was granted and
		// the common one is gone is not.
		for name, side := range map[string]paritySide{"player": p, "mob": m} {
			if len(side.char.Mutations) != 1 || side.char.Mutations["common-1"] != 0 {
				t.Errorf("%s after the phial: mutations %v, want exactly one rare", name, side.char.Mutations)
			}
			for id := range side.char.Mutations {
				if spec := mutations.GetMutation(id); spec == nil || spec.Rarity < phialRarityFloor {
					t.Errorf("%s was granted %q, below the phial's rarity floor", name, id)
				}
			}
		}
	})

	t.Run("bloom wafer", func(t *testing.T) {
		p, m := drinkBoth(t, "bloom wafer", func(c *characters.Character) { storeItem(t, c, items.New(bloomWaferItemId)) })
		// The mutation roll is random; the rest is not.
		requireParity(t, bloomWaferItemId, p, m)
		bal := configs.GetBalanceConfig()
		wantMult := float64(bal.BloomCommunionRounds) / 30.0
		if wantMult <= 0 {
			wantMult = 1.0
		}
		for name, side := range map[string]paritySide{"player": p, "mob": m} {
			if side.char.BloomAddiction != int(bal.BloomAddictionPerDose) {
				t.Errorf("%s addiction %d after the wafer, want %d", name, side.char.BloomAddiction, int(bal.BloomAddictionPerDose))
			}
			if side.char.BloomLastDoseRound != util.GetRoundCount() {
				t.Errorf("%s dose round %d, want %d", name, side.char.BloomLastDoseRound, util.GetRoundCount())
			}
			if q, ok := queuedHas(side.queued, 90); !ok || q.DurationMult != wantMult {
				t.Errorf("%s communion 90 queued %+v %v, want DurationMult %v", name, q, ok, wantMult)
			}
		}
	})
}
