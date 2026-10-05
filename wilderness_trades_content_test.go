package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mining"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/timber"
)

// TestWildernessTradesContent pins the shipped wilderness-trades data
// (docs/economy/wilderness-trades.md) against the real item, species, mob and
// recipe files, so a typo in a harvest table or a recipe tag fails here rather
// than as a boot panic or a carcass that silently yields nothing:
//   - every species and mob harvest entry names a real item or tag, a known
//     tool and a sane quantity and chance;
//   - the game animals that used to leave an empty corpse now have a table;
//   - every item a harvest entry produces is sellable somewhere and every
//     spoiling one names a parseable period;
//   - every processing chain closes: each recipe ingredient tag is supplied by
//     some item.
func TestWildernessTradesContent(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(items.SeedItemsForTest(nil))
	items.LoadDataFiles()
	species.LoadForTest(t)
	crafting.LoadRecipeFiles()

	tagExists := func(tag string) bool { return items.FindSpecByComponentTag(tag) != nil }
	itemExists := func(id int) bool { return items.GetItemSpec(id) != nil }

	checkEntryOutputs := func(owner string, h *species.HarvestTable) {
		if h == nil {
			return
		}
		for _, e := range append(append([]species.HarvestEntry{}, h.Skin...), h.Butcher...) {
			var spec *items.ItemSpec
			if e.ItemId > 0 {
				spec = items.GetItemSpec(e.ItemId)
			} else {
				spec = items.FindSpecByComponentTag(e.Item)
			}
			if spec == nil {
				continue // Validate reports it
			}
			if len(spec.VendorCategories) == 0 {
				t.Errorf("%s: %s (%d) has no vendor_categories, so no merchant buys it", owner, spec.Name, spec.ItemId)
			}
		}
	}

	withTable := map[int]bool{}
	for _, sp := range species.GetAllSpecies() {
		sp := sp
		if err := sp.Harvest.Validate(tagExists, itemExists); err != nil {
			t.Errorf("species %s: %v", sp.Name, err)
		}
		checkEntryOutputs("species "+sp.Name, sp.Harvest)
		if !sp.Harvest.Empty() {
			withTable[sp.SpeciesId] = true
		}
	}

	if len(withTable) < 15 {
		t.Fatalf("only %d species carry a harvest table; did the species data load?", len(withTable))
	}

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mobs: %v", err)
	}
	for id, m := range templates {
		if err := m.Harvest.Validate(tagExists, itemExists); err != nil {
			t.Errorf("mob %d %s: %v", id, m.Character.Name, err)
		}
		checkEntryOutputs(m.Character.Name, m.Harvest)
	}

	// The game animals phase 0 found with empty corpses: each must now have
	// something to skin or butcher, from its species or its own table.
	for _, id := range []int{205, 206, 215, 223, 207, 208, 216, 9139, 9138, 9140, 9141, 9142, 9143} {
		m, ok := templates[id]
		if !ok {
			t.Errorf("mob %d missing from the shipped files", id)
			continue
		}
		if !withTable[m.Character.SpeciesId] && m.Harvest.Empty() {
			t.Errorf("mob %d %s still has nothing to skin or butcher", id, m.Character.Name)
		}
	}

	// Spoiling items name a period the game clock understands.
	for _, spec := range items.GetAllItemSpecs() {
		if spec.SpoilAfter == `` {
			continue
		}
		probe := items.Item{ItemId: spec.ItemId, CraftedRound: 1000}
		if probe.SpoilRound() <= 1000 {
			t.Errorf("item %d %s: spoil_after %q does not parse to a future round", spec.ItemId, spec.Name, spec.SpoilAfter)
		}
	}

	// Every processing chain closes.
	for _, r := range crafting.GetAll() {
		for _, ing := range r.Ingredients {
			if !tagExists(ing.ItemTag) {
				t.Errorf("recipe %s: no item supplies ingredient tag %q", r.RecipeId, ing.ItemTag)
			}
		}
		if r.Tool != `` && !items.IsKnownToolType(r.Tool) {
			t.Errorf("recipe %s: unknown tool %q", r.RecipeId, r.Tool)
		}
	}
}

// TestTimberContent pins timber.yaml against the shipped items and zones, and
// checks that every zone pool names a zone that has choppable rooms.
func TestTimberContent(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(items.SeedItemsForTest(nil))
	items.LoadDataFiles()

	path := configs.GetFilePathsConfig().DataFiles.String() + `/` + timber.DataFileName
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	d, err := timber.Parse(raw, timber.World{
		ItemExists: func(id int) bool { return items.GetItemSpec(id) != nil },
	})
	if err != nil {
		t.Fatalf("timber.yaml: %v", err)
	}
	timber.Install(d)
	t.Cleanup(func() { timber.Install(nil) })

	species := timber.AllSpecies()
	if len(species) < 10 {
		t.Fatalf("only %d species; did timber.yaml load?", len(species))
	}
	for _, sp := range species {
		log := items.GetItemSpec(sp.LogItemId)
		if log == nil {
			continue // Parse reports it
		}
		if log.ComponentTag == `` {
			t.Errorf("%s log %d has no component tag, so no recipe can saw it", sp.Name, sp.LogItemId)
		}
		if len(log.VendorCategories) == 0 {
			t.Errorf("%s log %d is not sold anywhere", sp.Name, sp.LogItemId)
		}
	}
	// Every bow wood gives its bows something, and every shaft wood its
	// arrows: the wood a bow or quiver names must mean something.
	for _, sp := range species {
		log := items.GetItemSpec(sp.LogItemId)
		if log == nil {
			continue
		}
		switch log.ComponentTag {
		case `bow-wood-log`:
			if sp.Bow == (timber.BowTraits{}) {
				t.Errorf("%s makes bow staves but has no bow traits", sp.Name)
			}
		case `softwood-log`:
			if sp.Arrow == (timber.ArrowTraits{}) {
				t.Errorf("%s makes arrow shafts but has no arrow traits", sp.Name)
			}
		}
	}
	for _, id := range []int{40419, 40420, 10057, 10041, 10058, 10059, 30062, 30063} {
		if spec := items.GetItemSpec(id); spec == nil || !spec.CarriesWood {
			t.Errorf("item %d should carry its wood", id)
		}
	}

	for _, biome := range []string{`forest`, `dense_forest`, `swamp`} {
		if !timber.IsChoppable(biome) {
			t.Errorf("biome %s should grow timber", biome)
		}
	}
	if timber.IsChoppable(`city_thoroughfare`) {
		t.Error("city streets must not grow timber")
	}
	if items.FindSpecByComponentTag(`branch`) == nil {
		t.Error("no item carries the branch tag that felling gives")
	}
}

// TestToolLadderContent pins the wilderness trades review: shops sell only
// crude tools, every better tool is a smith's recipe, the best are hard to
// make, every recipe's station exists in some room, and the field merchants
// can trade.
func TestToolLadderContent(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(items.SeedItemsForTest(nil))
	items.LoadDataFiles()
	crafting.LoadRecipeFiles()

	dataRoot := configs.GetFilePathsConfig().DataFiles.String()
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataRoot + `/mobs`)
	if err != nil {
		t.Fatalf("loading mobs: %v", err)
	}

	// Which tool items any merchant stocks.
	sold := map[int]bool{}
	for id, m := range templates {
		stocked := append([]int{}, m.CrafterRestockMaterials...)
		for _, si := range m.Character.Shop {
			stocked = append(stocked, si.ItemId)
		}
		for _, itemId := range stocked {
			spec := items.GetItemSpec(itemId)
			if spec == nil || spec.Tool == nil {
				continue
			}
			sold[itemId] = true
			if items.NeverResold(*spec) {
				t.Errorf("mob %d %s sells %s (%s tool): only crude tools may be bought", id, m.Character.Name, spec.Name, spec.Tool.Tier)
			}
		}
	}

	// Which items a smith (or any recipe) makes, and at what skill.
	made := map[int]*crafting.RecipeSpec{}
	for _, r := range crafting.GetAll() {
		made[r.Output.ItemId] = r
	}

	// Every tool type that has items at all has the whole ladder.
	byType := map[items.ToolType]map[items.ToolTier][]*items.ItemSpec{}
	for _, spec := range items.GetAllItemSpecs() {
		spec := spec
		if spec.Tool == nil {
			continue
		}
		if byType[spec.Tool.Type] == nil {
			byType[spec.Tool.Type] = map[items.ToolTier][]*items.ItemSpec{}
		}
		byType[spec.Tool.Type][spec.Tool.Tier] = append(byType[spec.Tool.Type][spec.Tool.Tier], &spec)
	}
	for _, tt := range items.AllToolTypes {
		tiers := byType[tt]
		if len(tiers) == 0 {
			continue // a tool type nothing uses yet (trowel)
		}
		crudeSold := false
		for _, spec := range tiers[items.ToolTierCrude] {
			if sold[spec.ItemId] {
				crudeSold = true
			}
		}
		if !crudeSold {
			t.Errorf("tool type %s: no merchant sells a crude one", tt)
		}
		for _, tier := range []items.ToolTier{items.ToolTierIron, items.ToolTierSteel, items.ToolTierMasterwork} {
			forged := false
			for _, spec := range tiers[tier] {
				r := made[spec.ItemId]
				if r == nil {
					continue
				}
				forged = true
				if r.Skill != `blacksmithing` {
					t.Errorf("%s is made by %s; tools are a smith's work", spec.Name, r.Skill)
				}
				if tier == items.ToolTierMasterwork && r.SkillMinimum < 40 {
					t.Errorf("%s needs only skill %d; the best tools should be hard to make", spec.Name, r.SkillMinimum)
				}
			}
			if !forged {
				t.Errorf("tool type %s: no recipe makes a %s one", tt, tier)
			}
		}
	}

	// Every recipe station exists in some room.
	stations := map[string]bool{}
	stationLine := regexp.MustCompile(`(?m)^station:\s*"?([a-z_]+)"?\s*$`)
	err = filepath.WalkDir(dataRoot+`/rooms`, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, `.yaml`) {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := stationLine.FindSubmatch(raw); m != nil {
			stations[string(m[1])] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking rooms: %v", err)
	}
	for _, r := range crafting.GetAll() {
		if r.Station != `` && !stations[r.Station] {
			t.Errorf("recipe %s needs a %s, and no room has one", r.RecipeId, r.Station)
		}
	}

	// Every trophy part (taken only with a steel or masterwork tool) and every
	// rare carcass part a bone saw takes feeds at least one recipe, so the
	// better tool pays off in better gear, not only in coin.
	usedTags := map[string]bool{}
	for _, r := range crafting.GetAll() {
		for _, ing := range r.Ingredients {
			usedTags[ing.ItemTag] = true
		}
	}
	for _, id := range []int{40255, 40256, 40257, 40258, 40312, 40313, 40315, 40318} {
		spec := items.GetItemSpec(id)
		if spec == nil {
			t.Errorf("rare part %d is missing", id)
			continue
		}
		if !usedTags[spec.ComponentTag] {
			t.Errorf("%s (%s) is in no recipe", spec.Name, spec.ComponentTag)
		}
	}

	// The field merchants: hunting camps, trappers and lumber camps.
	for _, id := range []int{9137, 9840, 9841, 9536, 328, 9337, 9399} {
		m, ok := templates[id]
		if !ok {
			t.Errorf("field merchant %d is missing", id)
			continue
		}
		if !m.HasShop() {
			t.Errorf("mob %d %s has no shop", id, m.Character.Name)
		}
		if m.MaxWander != 0 || !m.IsNonCombatant() {
			t.Errorf("mob %d %s must stay put and stay out of fights (maxwander %d, non_combatant %v)", id, m.Character.Name, m.MaxWander, m.IsNonCombatant())
		}
		if m.Character.Equipment.Light.ItemId == 0 {
			t.Errorf("mob %d %s carries no light, so cannot trade at night", id, m.Character.Name)
		}
	}
	for _, id := range []int{9137, 9840, 9841} {
		m, ok := templates[id]
		if !ok {
			continue
		}
		if m.ShopCraftSupport != `hunting` {
			t.Errorf("mob %d %s is a hunting merchant, craft_support %q", id, m.Character.Name, m.ShopCraftSupport)
		}
		// Every merchant that buys carcass goods sells fletching supplies.
		sells := map[int]bool{}
		for _, si := range m.Character.Shop {
			sells[si.ItemId] = true
		}
		for _, want := range []int{40316, 40421, 40357} {
			if !sells[want] {
				t.Errorf("hunting merchant %d %s does not sell item %d (feathers / arrowheads)", id, m.Character.Name, want)
			}
		}
	}
}

// TestMiningContent pins mining.yaml against the shipped items and recipes:
// every ore and gem exists and is sold somewhere, and every metal any recipe
// asks for can be reached from mined ore through the smelting and drawing
// recipes, so nothing a smith or jeweler needs is shop-only.
func TestMiningContent(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(items.SeedItemsForTest(nil))
	items.LoadDataFiles()
	crafting.LoadRecipeFiles()

	path := configs.GetFilePathsConfig().DataFiles.String() + `/` + mining.DataFileName
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	d, err := mining.Parse(raw, mining.World{
		ItemExists: func(id int) bool { return items.GetItemSpec(id) != nil },
	})
	if err != nil {
		t.Fatalf("mining.yaml: %v", err)
	}
	mining.Install(d)
	t.Cleanup(func() { mining.Install(nil) })

	mined := map[string]bool{}
	for _, o := range mining.AllOres() {
		spec := items.GetItemSpec(o.ItemId)
		if spec == nil {
			continue // Parse reports it
		}
		if spec.ComponentTag == `` || len(spec.VendorCategories) == 0 {
			t.Errorf("ore %s item %d needs a component tag and a vendor category", o.Id, o.ItemId)
		}
		mined[spec.ComponentTag] = true
	}
	for _, g := range mining.Gems() {
		if spec := items.GetItemSpec(g.ItemId); spec != nil {
			mined[spec.ComponentTag] = true
		}
	}
	for _, want := range []string{`coal`, `copper`, `tin`, `iron`, `silver`, `gold`, `basalt-iron`, `lake-iron`} {
		if mining.GetOre(want) == nil {
			t.Errorf("no %s ore", want)
		}
	}
	for _, biome := range []string{`cave`, `mountains`, `cliffs`} {
		if !mining.IsMineableBiome(biome) {
			t.Errorf("biome %s should hold ore", biome)
		}
	}

	// Metals: every tag below is reachable from mined ore. Other ingredients
	// (planks, leather) are taken as available.
	metals := map[string]bool{
		`iron-ore`: true, `iron-ingot`: true, `steel-ingot`: true, `coal-dust`: true,
		`copper-ore`: true, `copper-ingot`: true, `copper-wire`: true,
		`tin-ore`: true, `tin-ingot`: true, `bronze-ingot`: true,
		`silver-ore`: true, `silver-ingot`: true, `silver-wire`: true,
		`gold-ore`: true, `gold-ingot`: true, `gold-wire`: true,
		`basalt-iron`: true, `lake-iron-nodule`: true, `crucible-steel`: true,
		`raw-gem`: true, `gem-dust`: true, `polished-stone`: true, `flawless-gem`: true,
	}
	have := func(tag string) bool { return mined[tag] || !metals[tag] }
	for changed := true; changed; {
		changed = false
		for _, r := range crafting.GetAll() {
			out := items.GetItemSpec(r.Output.ItemId)
			if out == nil || out.ComponentTag == `` || mined[out.ComponentTag] {
				continue
			}
			ok := len(r.Ingredients) > 0
			for _, ing := range r.Ingredients {
				if !have(ing.ItemTag) {
					ok = false
					break
				}
			}
			if ok {
				mined[out.ComponentTag] = true
				changed = true
			}
		}
	}
	for tag := range metals {
		if items.FindSpecByComponentTag(tag) == nil {
			t.Errorf("no item carries metal tag %q", tag)
			continue
		}
		if !mined[tag] {
			t.Errorf("%s cannot be reached from mined ore", tag)
		}
	}
}
