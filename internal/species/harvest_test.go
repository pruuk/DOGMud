package species

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v2"
)

func TestHarvestTable_ParsesAuthoredYAML(t *testing.T) {
	src := `
harvest:
  skin:    [{item: wolf-pelt, qty: 1}]
  butcher:
    - {item: raw-meat, qty: 2, tool: knife}
    - {item: bone,     qty: 2, tool: cleaver}
    - {item: fang,     qty: 1, tool: bone_saw, rare: true}
`
	var sp Species
	if err := yaml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatal(err)
	}
	h := sp.Harvest
	if h.Empty() || len(h.Skin) != 1 || len(h.Butcher) != 3 {
		t.Fatalf("parsed %+v", h)
	}
	if !h.Butcher[2].Rare || h.Butcher[2].Tool != items.ToolBoneSaw {
		t.Errorf("fang entry = %+v", h.Butcher[2])
	}
	if h.Skin[0].ToolOrDefault() != items.ToolKnife {
		t.Error("an entry with no tool defaults to knife")
	}
}

func TestMergeHarvest_OverridesPerSection(t *testing.T) {
	base := &HarvestTable{
		Skin:    []HarvestEntry{{Item: "wolf-pelt", Qty: 1}},
		Butcher: []HarvestEntry{{Item: "raw-meat", Qty: 2}},
	}
	over := &HarvestTable{Skin: []HarvestEntry{{Item: "cascade-hide", Qty: 1}}}
	got := MergeHarvest(base, over)
	if got.Skin[0].Item != "cascade-hide" {
		t.Errorf("mob skin should replace species skin, got %+v", got.Skin)
	}
	if len(got.Butcher) != 1 || got.Butcher[0].Item != "raw-meat" {
		t.Errorf("species butcher should survive an override with no butcher, got %+v", got.Butcher)
	}
	got.Butcher[0].Qty = 99
	if base.Butcher[0].Qty != 2 {
		t.Error("MergeHarvest must not alias its inputs")
	}
	if !(&HarvestTable{}).Empty() || !MergeHarvestEmpty(nil, nil) {
		t.Error("empty tables are empty")
	}
}

func MergeHarvestEmpty(a, b *HarvestTable) bool {
	m := MergeHarvest(a, b)
	return m.Empty()
}

func TestScaleHarvestQty(t *testing.T) {
	cases := []struct {
		qty  int
		size Size
		want int
	}{
		{2, Medium, 2}, {2, Small, 1}, {1, Small, 1}, {3, Small, 2}, {2, Large, 4}, {0, Large, 0},
	}
	for _, c := range cases {
		if got := ScaleHarvestQty(c.qty, c.size); got != c.want {
			t.Errorf("ScaleHarvestQty(%d,%s) = %d, want %d", c.qty, c.size, got, c.want)
		}
	}
}

func TestHarvestTable_Validate(t *testing.T) {
	known := func(tag string) bool { return tag == "raw-meat" || tag == "bone" }
	ok := &HarvestTable{Butcher: []HarvestEntry{{Item: "raw-meat", Qty: 1}, {Item: "bone", Qty: 2, Tool: items.ToolCleaver}}}
	if err := ok.Validate(known, nil); err != nil {
		t.Errorf("valid table rejected: %v", err)
	}
	bad := []*HarvestTable{
		{Skin: []HarvestEntry{{Item: "", Qty: 1}}},
		{Skin: []HarvestEntry{{Item: "raw-meat", Qty: 0}}},
		{Skin: []HarvestEntry{{Item: "raw-meat", Qty: 1, Tool: "spoon"}}},
		{Skin: []HarvestEntry{{Item: "unicorn-horn", Qty: 1}}},
		{Skin: []HarvestEntry{{Item: "raw-meat", Qty: 1, Rare: true, Chance: 1.5}}},
	}
	for i, h := range bad {
		if err := h.Validate(known, nil); err == nil {
			t.Errorf("bad table %d accepted: %+v", i, h)
		}
	}
	var nilTable *HarvestTable
	if err := nilTable.Validate(known, nil); err != nil {
		t.Error("a nil table is valid (it yields nothing)")
	}
}
