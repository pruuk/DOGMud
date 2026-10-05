package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func allTools(tier items.ToolTier) func(items.ToolType) (items.ToolTier, bool) {
	return func(items.ToolType) (items.ToolTier, bool) { return tier, true }
}

func knifeOnly(items.ToolType) (items.ToolTier, bool) { return items.ToolTierIron, true }

func wolfTable() []species.HarvestEntry {
	return []species.HarvestEntry{
		{Item: "raw-meat", Qty: 2},
		{Item: "bone", Qty: 2, Tool: items.ToolCleaver},
		{Item: "fang", Qty: 1, Tool: items.ToolBoneSaw, Rare: true, Chance: 0.5},
	}
}

func TestPlanHarvest_ToolGatesEntries(t *testing.T) {
	onlyKnife := func(tt items.ToolType) (items.ToolTier, bool) {
		return items.ToolTierIron, tt == items.ToolKnife
	}
	takes, missed := planHarvest(planInputs{
		Entries: wolfTable(), Size: species.Medium, Grade: items.QualityFine,
		Perception: 100, ToolTier: onlyKnife, Rand: func() float64 { return 0 },
	})
	if len(takes) != 1 || takes[0].Entry.Item != "raw-meat" {
		t.Errorf("knife alone should take only the meat, got %+v", takes)
	}
	if len(missed) != 2 {
		t.Errorf("bone and fang need other tools, got missed %+v", missed)
	}
}

func TestPlanHarvest_SizeBonusAndRare(t *testing.T) {
	takes, _ := planHarvest(planInputs{
		Entries: wolfTable(), Size: species.Large, Grade: items.QualityStandard,
		BonusUnits: 1, Perception: 100, ToolTier: allTools(items.ToolTierSteel),
		Rand: func() float64 { return 0 }, // every rare roll succeeds
	})
	got := map[string]int{}
	for _, tk := range takes {
		got[tk.Entry.Item] = tk.Qty
	}
	if got["raw-meat"] != 5 { // 2 * large(2) + 1 bonus on the first entry only
		t.Errorf("meat = %d, want 5", got["raw-meat"])
	}
	if got["bone"] != 4 {
		t.Errorf("bone = %d, want 4 (no bonus past the first entry)", got["bone"])
	}
	if got["fang"] != 2 {
		t.Errorf("fang = %d, want 2", got["fang"])
	}

	takes, _ = planHarvest(planInputs{
		Entries: wolfTable(), Size: species.Medium, Grade: items.QualityStandard,
		Perception: 100, ToolTier: allTools(items.ToolTierSteel),
		Rand: func() float64 { return 0.99 }, // every rare roll fails
	})
	for _, tk := range takes {
		if tk.Entry.Rare {
			t.Error("a missed rare roll yields no part")
		}
	}
}

func TestPlanHarvest_EntryToolCapsGrade(t *testing.T) {
	tiers := func(tt items.ToolType) (items.ToolTier, bool) {
		if tt == items.ToolCleaver {
			return items.ToolTierCrude, true
		}
		return items.ToolTierMasterwork, true
	}
	takes, _ := planHarvest(planInputs{
		Entries: wolfTable()[:2], Size: species.Medium, Grade: items.QualityPristine,
		Perception: 100, ToolTier: tiers, Rand: func() float64 { return 0 },
	})
	for _, tk := range takes {
		if tk.Entry.Item == "bone" && tk.Grade != items.QualityStandard {
			t.Errorf("bone cut with a crude cleaver caps at standard, got %v", tk.Grade)
		}
		if tk.Entry.Item == "raw-meat" && tk.Grade != items.QualityPristine {
			t.Errorf("meat cut with a masterwork knife keeps pristine, got %v", tk.Grade)
		}
	}
}

func TestPlanHarvest_StaleCarcass(t *testing.T) {
	perishing := func(e species.HarvestEntry) bool { return e.Item == "raw-meat" }
	takes, _ := planHarvest(planInputs{
		Entries: wolfTable()[:2], Size: species.Medium, Grade: items.QualityFine,
		Staleness: 0.6, Perception: 100, ToolTier: allTools(items.ToolTierSteel),
		Rand: func() float64 { return 0 }, IsPerishing: perishing,
	})
	for _, tk := range takes {
		if tk.Grade != items.QualityStandard {
			t.Errorf("a half-decayed carcass gives one grade worse, got %v", tk.Grade)
		}
	}
	takes, _ = planHarvest(planInputs{
		Entries: wolfTable()[:2], Size: species.Medium, Grade: items.QualityFine,
		Staleness: 0.9, Perception: 100, ToolTier: allTools(items.ToolTierSteel),
		Rand: func() float64 { return 0 }, IsPerishing: perishing,
	})
	for _, tk := range takes {
		if tk.Entry.Item == "raw-meat" {
			t.Error("a nearly crumbled carcass has no meat")
		}
	}
}

func TestCarcassDifficulty_GrowsWithPowerAndSize(t *testing.T) {
	hare := CarcassDifficulty(20, species.Small)
	wolf := CarcassDifficulty(70, species.Medium)
	bear := CarcassDifficulty(150, species.Large)
	if !(hare < wolf && wolf < bear) {
		t.Errorf("difficulty should order hare<wolf<bear, got %v %v %v", hare, wolf, bear)
	}
}

func TestFindHarvestPart_MatchesTagAndName(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		1: {ItemId: 1, Name: "Wolf Pelt", ComponentTag: "wolf-pelt"},
		2: {ItemId: 2, Name: "Fang", ComponentTag: "fang"},
	})
	defer cleanup()
	table := species.HarvestTable{
		Skin:    []species.HarvestEntry{{ItemId: 1, Qty: 1}},
		Butcher: []species.HarvestEntry{{Item: "fang", Qty: 1, Rare: true}},
	}
	c := &rooms.Corpse{}
	if _, sec, ok := FindHarvestPart(table, c, "pelt"); !ok || sec != HarvestSkin {
		t.Errorf("'pelt' should find the wolf pelt in the skin section")
	}
	if e, sec, ok := FindHarvestPart(table, c, "fangs"); !ok || sec != HarvestButcher || e.Item != "fang" {
		t.Errorf("'fangs' should find the fang")
	}
	c.Skinned = true
	if _, _, ok := FindHarvestPart(table, c, "pelt"); ok {
		t.Error("a skinned carcass has no pelt left")
	}
}

// End to end through ResolveHarvest: whatever the roll, the carcass is marked
// and every good taken is graded, stamped and stored.
func TestResolveHarvest_MarksCarcassAndStoresGradedGoods(t *testing.T) {
	const mobId = 8811
	wolf := &mobs.Mob{StatPool: 70}
	wolf.Character.Name = "test wolf"
	wolf.Character.SpeciesId = 2
	cleanupMobs := mobs.SeedMobsForTest(map[int]*mobs.Mob{mobId: wolf}, map[int]*mobs.Mob{})
	defer cleanupMobs()
	cleanupSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		2: {SpeciesId: 2, Name: "canine", Size: species.Medium, Harvest: &species.HarvestTable{
			Skin:    []species.HarvestEntry{{Item: "wolf-pelt", Qty: 1}},
			Butcher: []species.HarvestEntry{{Item: "raw-meat", Qty: 2}},
		}},
	})
	defer cleanupSpecies()
	cleanupItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		10: {ItemId: 10, Name: "Steel Skinning Knife", Type: items.Object, Tool: &items.ToolSpec{Type: items.ToolKnife, Tier: items.ToolTierSteel, Speed: 1}},
		20: {ItemId: 20, Name: "Wolf Pelt", ComponentTag: "wolf-pelt", IsComponent: true, SpoilAfter: "4 days"},
		21: {ItemId: 21, Name: "Raw Meat", ComponentTag: "raw-meat", IsComponent: true, SpoilAfter: "2 days"},
	})
	defer cleanupItems()

	room := newSalvageTestRoom(t, 9501)
	now := util.GetRoundCount()
	corpse := rooms.Corpse{MobId: mobId, RoundCreated: now}
	corpse.Character.Name = "test wolf"
	corpse.Character.SpeciesId = 2
	room.AddCorpse(corpse)

	actor := newSalvageFakeActor(t, "Hunter", room, true, 0)
	actor.char.Stats.Dexterity.Value = 400
	actor.char.Stats.Perception.Value = 400
	actor.char.Items = []items.Item{items.New(10)}

	res := ResolveHarvest(actor, HarvestOptions{MobId: mobId, RoundCreated: now, Section: HarvestSkin})
	if res.Reason != `` {
		t.Fatalf("unexpected refusal: %q", res.Reason)
	}
	if len(room.Corpses) != 1 || !room.Corpses[0].Skinned {
		t.Fatalf("the carcass should remain, marked skinned: %+v", room.Corpses)
	}
	for _, it := range res.Taken {
		if !it.Quality.Valid() || it.Quality > items.QualitySuperb {
			t.Errorf("a steel knife grades crude..superb, got %v", it.Quality)
		}
		if !it.Spoils() {
			t.Error("a harvested pelt is on the spoilage clock")
		}
	}
	if len(actor.awards) != 1 {
		t.Errorf("one award per job, got %d", len(actor.awards))
	}

	res = ResolveHarvest(actor, HarvestOptions{MobId: mobId, RoundCreated: now, Section: HarvestButcher})
	if res.HideRuined {
		t.Error("the carcass was already skinned, nothing to ruin")
	}
	if len(room.Corpses) != 0 {
		t.Error("a skinned and butchered carcass with no loot is removed")
	}
}

func TestPlanHarvest_MinToolGatesTrophies(t *testing.T) {
	entries := []species.HarvestEntry{
		{Item: "hide", Qty: 1},
		{Item: "trophy-pelt", Qty: 1, Rare: true, Chance: 0.5, MinTool: items.ToolTierSteel},
	}
	takes, missed := planHarvest(planInputs{
		Entries: entries, Size: species.Medium, Grade: items.QualityStandard,
		Perception: 100, ToolTier: allTools(items.ToolTierIron), Rand: func() float64 { return 0 },
	})
	if len(takes) != 1 || len(missed) != 1 || missed[0].Item != "trophy-pelt" {
		t.Fatalf("an iron knife leaves the steel-only trophy behind: takes %+v missed %+v", takes, missed)
	}
	takes, missed = planHarvest(planInputs{
		Entries: entries, Size: species.Medium, Grade: items.QualityStandard,
		Perception: 100, ToolTier: allTools(items.ToolTierSteel), Rand: func() float64 { return 0 },
	})
	if len(takes) != 2 || len(missed) != 0 {
		t.Fatalf("a steel knife takes the trophy: takes %+v missed %+v", takes, missed)
	}
}

func TestPlanHarvest_BetterToolFindsMoreRares(t *testing.T) {
	entries := []species.HarvestEntry{{Item: "fang", Qty: 1, Rare: true, Chance: 0.3}}
	// Perception 100: crude 0.15, iron 0.30, masterwork 0.60. A roll of 0.4
	// misses with iron and lands with masterwork.
	roll := func() float64 { return 0.4 }
	takes, _ := planHarvest(planInputs{Entries: entries, Size: species.Small, Grade: items.QualityStandard,
		Perception: 100, ToolTier: allTools(items.ToolTierIron), Rand: roll})
	if len(takes) != 0 {
		t.Errorf("iron: chance 0.30 should miss a 0.4 roll, got %+v", takes)
	}
	takes, _ = planHarvest(planInputs{Entries: entries, Size: species.Small, Grade: items.QualityStandard,
		Perception: 100, ToolTier: allTools(items.ToolTierMasterwork), Rand: roll})
	if len(takes) != 1 {
		t.Errorf("masterwork: chance 0.60 should land a 0.4 roll, got %+v", takes)
	}
}
