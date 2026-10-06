package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// The item behaviour foundation's content guards (lighting 5e, item
// behaviour slice 1, spec Rule 15). New item behaviour goes in a tree under
// _datafiles/world/dogmud/behaviors/items, not in another ItemSpec field that
// a hook reads.

// itemSpecBehaviourFields are the ItemSpec fields that make an item DO
// something: per-item reactions a behaviour tree now owns, or the tree
// machinery itself. Each carries the reason it is behaviour. A hook may read
// one only at a site in itemSpecBehaviourReadSites. Every other exported
// field is plain data (itemSpecDataFields), which hooks read freely. The
// pattern is conditions' TestEveryEffectKindIsClassifiedExactlyOnce: every
// field is classified exactly once, so a new field fails until someone says
// which it is.
var itemSpecBehaviourFields = map[string]string{
	"Procs":                "combat procs; slice 3 moves them into proc nodes",
	"ReserveHealthPct":     "Pinnacle reserve held while worn",
	"ReserveStaminaPct":    "Pinnacle reserve held while worn",
	"ReserveConvictionPct": "Pinnacle reserve held while worn",
	"PreservesContents":    "bandolier: contents never age",
	"AmbientPotions":       "bandolier: slotted potions tick while worn",
	"MutationTickInterval": "Pinnacle mutation drip",
	"MutationTickChance":   "Pinnacle mutation drip",
	"MutationRarityFloor":  "Pinnacle mutation drip",
	"VoiceId":              "sentient voice; slice 2 moves it into speak",
	"HungerRounds":         "the Blackrazor's hunger",
	"HungerDrainPct":       "the Blackrazor's hunger",
	"TauntPull":            "the Aegis's taunt pull; slice 2 makes it a tree action",
	"Behavior":             "the item's tree: hooks reach it through behaviortree.TryItemBehavior",
	"Fixture":              "fixed to a floor: read through items.Item.IsFixture",
	"OnUseTrainSkill":      "use effect, the YAML replacement for JS onUse",
	"OnUseTrainAmount":     "use effect, the YAML replacement for JS onUse",
	"OnUseUserText":        "use effect, the YAML replacement for JS onUse",
	"OnUseRoomText":        "use effect, the YAML replacement for JS onUse",
}

// itemSpecDataFields are the ItemSpec fields that describe an item: what it
// is, weighs, costs, protects, and how it is named and stored.
var itemSpecDataFields = []string{
	"ItemId", "Value", "Uses", "ConditionIds", "WornConditionIds", "Nouns",
	"PhysicalMitigation", "MagicalMitigation", "ConvictionMitigation",
	"DamageMultiplier", "SpellDamageMultiplier", "ParryRating", "BlockRating",
	"AmmoTag", "MinStrength", "WaitRounds", "StaminaCost", "SpeedMultiplier",
	"Weight", "GrappleModifier", "EscapeModifier", "Reach", "Hands", "Name",
	"DisplayName", "NameSimple", "Description", "QuestToken", "Type",
	"Subtype", "Damage", "Element", "StatMods", "BreakChance", "Cursed",
	"KeyLockId", "ComponentTag", "IsComponent", "WeightReduction",
	"BagCapacity", "Aging", "BottleAgingMultiplier", "Toxicity", "Magnitude",
	"IsBandolier", "BandolierCapacity", "SalvageReturns", "RarityTier",
	"MaterialTier", "VendorCategories", "NotSalable", "NeverDrops",
	"Restricted",
}

// itemSpecBehaviourMethods maps an ItemSpec method to the behaviour field it
// reads, so calling it counts as reading that field.
var itemSpecBehaviourMethods = map[string]string{
	"ProcsFor": "Procs",
}

// itemSpecBehaviourReadSites are the only places non-test internal/hooks
// reads a behaviour field: "file|field". These are today's Pinnacle
// mechanics. Slice 2 retired the VoiceId and TauntPull sites (voices are
// item trees), slice 3 retires the Procs ones; an entry nothing reads any
// more fails, so the list only shrinks. A new site fails: put the
// behaviour in a tree.
var itemSpecBehaviourReadSites = map[string]bool{
	"PlayerSpawn_HandleJoin.go|PreservesContents": true,
	"item_procs.go|Procs":                         true,
	"pinnacle_tick.go|AmbientPotions":             true,
	"pinnacle_tick.go|HungerDrainPct":             true,
	"pinnacle_tick.go|HungerRounds":               true,
	"pinnacle_tick.go|MutationRarityFloor":        true,
	"pinnacle_tick.go|MutationTickChance":         true,
	"pinnacle_tick.go|MutationTickInterval":       true,
	"pinnacle_tick.go|PreservesContents":          true,
}

// (a) Rule 15, part one: every exported ItemSpec field is classified
// exactly once, behaviour or data, and no classification names a field that
// does not exist.
func TestEveryItemSpecFieldIsClassifiedExactlyOnce(t *testing.T) {
	data := map[string]bool{}
	for _, f := range itemSpecDataFields {
		if data[f] {
			t.Errorf("ItemSpec field %s is listed twice as data", f)
		}
		data[f] = true
	}
	fields := map[string]bool{}
	st := reflect.TypeOf(items.ItemSpec{})
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if !f.IsExported() {
			continue
		}
		fields[f.Name] = true
		_, behaviour := itemSpecBehaviourFields[f.Name]
		switch {
		case behaviour && data[f.Name]:
			t.Errorf("ItemSpec field %s is classified as both behaviour and data", f.Name)
		case !behaviour && !data[f.Name]:
			t.Errorf("ItemSpec field %s is not classified. If it makes an item DO something, it belongs "+
				"in a behaviour tree (behaviors/items); if it only describes the item, list it in "+
				"itemSpecDataFields", f.Name)
		}
	}
	for name, reason := range itemSpecBehaviourFields {
		if !fields[name] {
			t.Errorf("itemSpecBehaviourFields names %s, which ItemSpec does not have", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("behaviour field %s carries no reason", name)
		}
	}
	for name := range data {
		if !fields[name] {
			t.Errorf("itemSpecDataFields names %s, which ItemSpec does not have", name)
		}
	}
	for method, field := range itemSpecBehaviourMethods {
		if _, ok := itemSpecBehaviourFields[field]; !ok {
			t.Errorf("method %s maps to %s, which is not a behaviour field", method, field)
		}
	}
}

// (a) Rule 15, part two: internal/hooks reads a behaviour field only at an
// allowlisted site. Data fields are free.
func TestHooksReadItemBehaviourFieldsOnlyAtAllowlistedSites(t *testing.T) {
	reads := itemSpecReadsIn(t, "./internal/hooks")
	if len(reads["HungerRounds"]) == 0 {
		t.Fatalf("the scan found no HungerRounds read: it is not seeing internal/hooks (got %v)", reads)
	}
	seen := map[string]bool{}
	var problems []string
	for member, files := range reads {
		field := member
		if f, ok := itemSpecBehaviourMethods[member]; ok {
			field = f
		}
		if _, behaviour := itemSpecBehaviourFields[field]; !behaviour {
			continue
		}
		for file := range files {
			site := file + "|" + field
			seen[site] = true
			if !itemSpecBehaviourReadSites[site] {
				problems = append(problems, fmt.Sprintf("%s reads behaviour field %s (via %s), not an allowlisted site", file, field, member))
			}
		}
	}
	for site := range itemSpecBehaviourReadSites {
		if !seen[site] {
			problems = append(problems, fmt.Sprintf("allowlisted site %s is no longer read: drop it", site))
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("internal/hooks reads ItemSpec behaviour fields off the allowlist. New item behaviour goes "+
			"in a tree (behaviors/items), not in a field a hook reads; a retired read drops its site.\n%s",
			strings.Join(problems, "\n"))
	}
}

// itemSpecReadsIn type-checks one package's non-test files and returns, for
// every ItemSpec member a selector reads (field or method, through a value
// or a pointer), the files that read it. Dependencies come from the build
// cache's export data (go list -export), so nothing is type-checked twice.
func itemSpecReadsIn(t *testing.T, pkg string) map[string]map[string]bool {
	t.Helper()
	out, err := exec.Command("go", "list", "-export", "-deps", "-f", "{{.ImportPath}}={{.Export}}", pkg).Output()
	if err != nil {
		t.Fatalf("go list -export %s: %v", pkg, err)
	}
	exports := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if path, file, ok := strings.Cut(sc.Text(), "="); ok && file != "" {
			exports[path] = file
		}
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file)
	})

	dir := filepath.FromSlash(strings.TrimPrefix(pkg, "./"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatal(perr)
		}
		files = append(files, f)
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: imp}
	if _, err := conf.Check(pkg, fset, files, info); err != nil {
		t.Fatalf("type-checking %s: %v", pkg, err)
	}

	got := map[string]map[string]bool{}
	for expr, sel := range info.Selections {
		recv := sel.Recv()
		if p, ok := recv.(*types.Pointer); ok {
			recv = p.Elem()
		}
		named, ok := recv.(*types.Named)
		if !ok || named.Obj().Name() != "ItemSpec" || named.Obj().Pkg() == nil ||
			named.Obj().Pkg().Path() != "github.com/GoMudEngine/GoMud/internal/items" {
			continue
		}
		member := sel.Obj().Name()
		if got[member] == nil {
			got[member] = map[string]bool{}
		}
		got[member][filepath.Base(fset.Position(expr.Pos()).Filename)] = true
	}
	return got
}

// loadItemBehaviourWorld loads the shipped world the way the lighting
// goldens do, with the shipped day pinned.
func loadItemBehaviourWorld(t *testing.T) {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.NightHours = 8
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	mutators.LoadDataFiles()
	originalRound := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCount(originalRound)
		gametime.ClearDateCacheForTest()
		gametime.ClearCelestialMemoForTest()
	})
}

// treedItems returns every shipped item that names a tree, with the tree's
// definition, in item id order.
func treedItems(t *testing.T) ([]*items.ItemSpec, map[string]behaviortree.TreeDef) {
	t.Helper()
	var out []*items.ItemSpec
	defs := map[string]behaviortree.TreeDef{}
	for _, spec := range items.GetAllItemSpecsMap() {
		if spec.Behavior == "" {
			continue
		}
		out = append(out, spec)
		if _, ok := defs[spec.Behavior]; !ok {
			def, err := behaviortree.LoadTreeDef(behaviortree.GetItemTreePath(spec.Behavior))
			if err != nil {
				t.Fatalf("item %d: behavior %q: %v", spec.ItemId, spec.Behavior, err)
			}
			defs[spec.Behavior] = def
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemId < out[j].ItemId })
	if len(out) < 4 {
		t.Fatalf("found only %d treed items: the scan is not seeing the world", len(out))
	}
	return out, defs
}

// (b) and (e) Rule 15: every behavior: resolves and compiles, and no tree
// schedules an adjustable light (the boot check, run over the shipped world).
func TestEveryShippedItemBehaviorResolves(t *testing.T) {
	loadItemBehaviourWorld(t)
	if err := behaviortree.ValidateItemBehaviors(); err != nil {
		t.Fatal(err)
	}
	treed, defs := treedItems(t)
	for _, spec := range treed {
		if !behaviortree.TreeWritesLight(defs[spec.Behavior].Tree) {
			continue
		}
		for _, cid := range spec.WornConditionIds {
			c := conditions.GetConditionSpec(cid)
			for _, f := range c.Flags {
				if f == conditions.Adjustable {
					t.Errorf("item %d (%s): tree %q writes light, but worn condition %d is adjustable", spec.ItemId, spec.Name, spec.Behavior, cid)
				}
			}
		}
	}
}

// spawnedFloorItems returns, per room, the ids of the items its spawninfo
// places on the floor.
func spawnedFloorItems(t *testing.T) map[int][]int {
	t.Helper()
	ids := rooms.GetAllRoomIds()
	if len(ids) < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world", len(ids))
	}
	sort.Ints(ids)
	out := map[int][]int{}
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		for _, si := range r.SpawnInfo {
			if si.ItemId > 0 && si.Container == "" {
				out[id] = append(out[id], si.ItemId)
			}
		}
	}
	return out
}

// (c) Rule 15: a light-writing tree belongs to a light or a fixture, and an
// item spawninfo leaves on a floor with such a tree is a fixture (else
// anyone could pick the lamp-post up).
func TestLightWritingTreesBelongToLightsAndFixtures(t *testing.T) {
	loadItemBehaviourWorld(t)
	treed, defs := treedItems(t)
	writes := map[int]bool{}
	for _, spec := range treed {
		if !behaviortree.TreeWritesLight(defs[spec.Behavior].Tree) {
			continue
		}
		writes[spec.ItemId] = true
		if spec.Type != items.Light && spec.Fixture == "" {
			t.Errorf("item %d (%s): tree %q writes light but the item is neither type light nor a fixture", spec.ItemId, spec.Name, spec.Behavior)
		}
	}
	placed := 0
	for roomId, itemIds := range spawnedFloorItems(t) {
		for _, id := range itemIds {
			if !writes[id] {
				continue
			}
			placed++
			if spec := items.GetItemSpec(id); spec.Fixture == "" {
				t.Errorf("room %d places item %d (%s), whose tree writes light, on its floor, but it is not a fixture: anyone could take it", roomId, id, spec.Name)
			}
		}
	}
	if placed < 2 {
		t.Errorf("found %d light-writing floor items placed by spawninfo, want the arch lantern and the Rift Stone at least: the walk is broken", placed)
	}
}

// pulseNodes collects every pulse_light node's min and max.
func pulseNodes(def behaviortree.NodeDef, out *[][2]float64) {
	if def.Do == "pulse_light" {
		*out = append(*out, [2]float64{paramFloat(def.Params["min"]), paramFloat(def.Params["max"])})
	}
	for _, ch := range def.Children {
		pulseNodes(ch, out)
	}
	if def.Child != nil {
		pulseNodes(*def.Child, out)
	}
}

func paramFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case float64:
		return x
	}
	return math.NaN()
}

// (d) Rule 11: a pulse may not straddle a band edge. For every fixture
// spawninfo places whose tree pulses, a normal observer's band with the
// fixture at its min and at its max, through the real room at the shop
// guard's 72 samples, must agree at every sample, so a pulse never fires a
// notice. Nightvision moves the edges, so keep a pulse well inside a band.
func TestPulsingFixturesStayInsideOneBand(t *testing.T) {
	loadItemBehaviourWorld(t)
	t.Cleanup(itemlight.ResetForTest())
	treed, defs := treedItems(t)
	pulses := map[int][][2]float64{}
	for _, spec := range treed {
		if spec.Fixture == "" {
			continue
		}
		var nodes [][2]float64
		pulseNodes(defs[spec.Behavior].Tree, &nodes)
		if len(nodes) > 0 {
			pulses[spec.ItemId] = nodes
		}
	}
	observer := &characters.Character{}
	probe := uuid.UUID{0x5e}
	checked := 0
	for roomId, itemIds := range spawnedFloorItems(t) {
		r := rooms.LoadRoom(roomId)
		for _, id := range itemIds {
			kind := itemlight.Light
			if items.GetItemSpec(id).Fixture == items.FixtureDarkness {
				kind = itemlight.Darkness
			}
			for _, mm := range pulses[id] {
				for _, d := range nightTradeSampleDays {
					for hour := 0; hour < 24; hour++ {
						util.SetRoundCount(uint64(d.Doy-1)*900 + uint64(math.Ceil(float64(hour)*37.5)))
						itemlight.Set(roomId, probe, kind, mm[0])
						atMin := messaging.LightBand(observer, r)
						itemlight.Set(roomId, probe, kind, mm[1])
						atMax := messaging.LightBand(observer, r)
						itemlight.Clear(roomId, probe)
						checked++
						if atMin != atMax {
							t.Errorf("room %d item %d pulses %v to %v, which straddles a band edge at %s %02d:00 (%v at min, %v at max)",
								roomId, id, mm[0], mm[1], d.Name, hour, atMin, atMax)
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no pulsing fixture was checked: the Rift Stone in room 5000 should be")
	}
}

// Issue #361: the sunstone ships with a rarity_tier, so the restock fallback
// does not silently treat it as tier 50. Its value (40) sits in the armor
// band that ships at tier 40.
func TestSunstoneShipsWithARarityTier(t *testing.T) {
	loadItemBehaviourWorld(t)
	spec := items.GetItemSpec(20099)
	if spec == nil {
		t.Fatal("sunstone 20099 is not in the shipped items")
	}
	if spec.RarityTier != 40 {
		t.Errorf("sunstone rarity_tier = %d, want 40", spec.RarityTier)
	}
}
