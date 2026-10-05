package species

import (
	"fmt"
	"math"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// HarvestEntry is one thing a carcass can give: a material named by its
// component_tag, how many for a MEDIUM body, which tool is needed to take it,
// and whether it is a rare part that only a sharp eye notices.
//
//   - {item: raw-meat, qty: 2, tool: knife}
//   - {item: bone,     qty: 2, tool: cleaver}
//   - {item: fang,     qty: 1, tool: bone_saw, rare: true}
//
// Tool defaults to knife when empty: every carcass job needs a blade.
type HarvestEntry struct {
	// Item is a component tag. ItemId, when set, names one exact item and
	// wins over the tag: several hides share the `hide` tag so one tanning
	// recipe takes them all, and a deer must still give a deer hide, not
	// whichever hide is cheapest.
	Item   string         `yaml:"item,omitempty"`
	ItemId int            `yaml:"itemid,omitempty"`
	Qty    int            `yaml:"qty"`
	Tool   items.ToolType `yaml:"tool,omitempty"`
	Rare   bool           `yaml:"rare,omitempty"`
	// Chance is a rare entry's base chance to be noticed at all, before the
	// gatherer's Perception scales it. 0 means Balance.GatherRareBaseChance.
	// Ignored on entries that are not rare.
	Chance float64 `yaml:"chance,omitempty"`
	// MinTool is the poorest tool tier that can take this part at all
	// (2 iron, 3 steel, 4 masterwork; 0 = any). A trophy pelt or an intact
	// set of antlers needs an edge good enough not to ruin it.
	MinTool items.ToolTier `yaml:"min_tool,omitempty"`
}

// ToolOrDefault is the entry's tool, or knife when none is authored.
func (e HarvestEntry) ToolOrDefault() items.ToolType {
	if e.Tool == `` {
		return items.ToolKnife
	}
	return e.Tool
}

// HarvestTable is what a carcass gives to `skin` and to `butcher`
// (wilderness-trades plan, phase 2). It is authored on a species, and a mob
// may author its own to override the species per section: a mob's non-empty
// Skin replaces the species Skin, and likewise Butcher. That is how a unique
// pelt (the Cascade Pass predators' thick pelt) replaces a plain wolf hide
// without restating the meat and bone.
type HarvestTable struct {
	Skin    []HarvestEntry `yaml:"skin,omitempty"`
	Butcher []HarvestEntry `yaml:"butcher,omitempty"`
}

// Empty reports whether the table gives nothing at all. A nil table is empty.
func (h *HarvestTable) Empty() bool {
	return h == nil || (len(h.Skin) == 0 && len(h.Butcher) == 0)
}

// MergeHarvest resolves a mob's harvest: each section of override that is
// non-empty replaces the same section of base. Either argument may be nil.
// The result is a fresh value; neither input is modified.
func MergeHarvest(base, override *HarvestTable) HarvestTable {
	out := HarvestTable{}
	if base != nil {
		out.Skin = append([]HarvestEntry(nil), base.Skin...)
		out.Butcher = append([]HarvestEntry(nil), base.Butcher...)
	}
	if override != nil {
		if len(override.Skin) > 0 {
			out.Skin = append([]HarvestEntry(nil), override.Skin...)
		}
		if len(override.Butcher) > 0 {
			out.Butcher = append([]HarvestEntry(nil), override.Butcher...)
		}
	}
	return out
}

// ScaleHarvestQty scales an authored (medium-body) quantity by body size:
// small halves it, large doubles it. A positive quantity never scales below
// one: a hare still has a pelt.
func ScaleHarvestQty(qty int, size Size) int {
	if qty <= 0 {
		return 0
	}
	f := float64(qty)
	switch size {
	case Small:
		f *= 0.5
	case Large:
		f *= 2
	}
	n := int(math.Round(f))
	if n < 1 {
		n = 1
	}
	return n
}

// Key names the entry for matching and bookkeeping: its tag, or "#<id>" for
// an entry authored by item id alone.
func (e HarvestEntry) Key() string {
	if e.Item != `` {
		return e.Item
	}
	return fmt.Sprintf("#%d", e.ItemId)
}

// Validate checks every entry: a known material tag or item id, a positive
// quantity, a known tool type and a sane chance. tagExists and itemExists are
// callbacks so this package does not need the item registry loaded; either
// may be nil to skip that check.
func (h *HarvestTable) Validate(tagExists func(tag string) bool, itemExists func(itemId int) bool) error {
	if h == nil {
		return nil
	}
	for section, entries := range map[string][]HarvestEntry{`skin`: h.Skin, `butcher`: h.Butcher} {
		for i, e := range entries {
			if e.Item == `` && e.ItemId == 0 {
				return fmt.Errorf("harvest %s[%d]: needs an item tag or an itemid", section, i)
			}
			if e.ItemId != 0 && itemExists != nil && !itemExists(e.ItemId) {
				return fmt.Errorf("harvest %s[%d]: item %d does not exist", section, i, e.ItemId)
			}
			if e.Qty <= 0 {
				return fmt.Errorf("harvest %s[%d] %q: qty must be positive, got %d", section, i, e.Item, e.Qty)
			}
			if e.Tool != `` && !items.IsKnownToolType(e.Tool) {
				return fmt.Errorf("harvest %s[%d] %q: unknown tool %q", section, i, e.Item, e.Tool)
			}
			if e.MinTool != items.ToolTierNone && !e.MinTool.Valid() {
				return fmt.Errorf("harvest %s[%d] %q: min_tool must be 1..4, got %d", section, i, e.Item, e.MinTool)
			}
			if e.Chance < 0 || e.Chance > 1 {
				return fmt.Errorf("harvest %s[%d] %q: chance must be 0..1, got %v", section, i, e.Item, e.Chance)
			}
			if e.ItemId == 0 && tagExists != nil && !tagExists(e.Item) {
				return fmt.Errorf("harvest %s[%d]: no item carries component_tag %q", section, i, e.Item)
			}
		}
	}
	return nil
}

// ValidateSpeciesHarvest panics on any species whose harvest table names an
// unknown material, tool or quantity. Called from main after species and items
// are loaded, the same shape as ValidateSpeciesConditionIds. Panicking at boot
// is the point: a harvest entry that silently yields nothing is the bug the
// phase 0 corpse fix was cleaning up.
func ValidateSpeciesHarvest(tagExists func(tag string) bool, itemExists func(itemId int) bool) {
	for _, sp := range allSpecies {
		if err := sp.Harvest.Validate(tagExists, itemExists); err != nil {
			panic(fmt.Sprintf("species %q (id %d): %v", sp.Name, sp.SpeciesId, err))
		}
	}
}
