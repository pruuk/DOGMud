package actions

import (
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mining"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/timber"
)

// Gradable materials (wilderness trades review).
//
// A material is "gradable" when the world can hand it out with a grade: a
// carcass part, a log, an ore or gem, or anything a recipe makes from those
// or with a tool. Shops resell what they buy without its grade, so an
// ungraded copy of a gradable material is not a blank slate: it counts as a
// standard one when a craft caps its output one grade above its worst input.
// Without this a crude hide sold to a tailor and bought back came out of the
// tannery as good as a pristine one, and shop ingots out-forged mined ones.
//
// Materials that never carry a grade (thread, bottles, reagents) stay out of
// the set, so a recipe that needs them is not capped by them.

// gradableSets lists the gradable item ids and component tags. It is built
// on each call: crafts are rare events and the data can reload.
func gradableSets() (map[int]bool, map[string]bool) {
	ids := map[int]bool{}
	tags := map[string]bool{}
	addId := func(id int) {
		if id <= 0 {
			return
		}
		ids[id] = true
		if spec := items.GetItemSpec(id); spec != nil && spec.ComponentTag != `` {
			tags[spec.ComponentTag] = true
		}
	}
	addEntries := func(es []species.HarvestEntry) {
		for _, e := range es {
			if e.ItemId > 0 {
				addId(e.ItemId)
			} else if e.Item != `` {
				tags[e.Item] = true
			}
		}
	}
	for _, sp := range species.GetAllSpecies() {
		if sp.Harvest != nil {
			addEntries(sp.Harvest.Skin)
			addEntries(sp.Harvest.Butcher)
		}
	}
	for _, sp := range timber.AllSpecies() {
		addId(sp.LogItemId)
		addId(sp.BarkItemId)
	}
	for _, o := range mining.AllOres() {
		addId(o.ItemId)
	}
	for _, g := range mining.Gems() {
		addId(g.ItemId)
	}

	// Recipe outputs, to a fixed point: made with a tool, a tool or gear
	// itself, or made from a gradable material.
	recipes := crafting.GetAll()
	for changed := true; changed; {
		changed = false
		for _, r := range recipes {
			if r == nil || ids[r.Output.ItemId] || crafting.IsEnchantingRecipe(r) {
				continue
			}
			graded := r.Tool != ``
			if !graded {
				if spec := items.GetItemSpec(r.Output.ItemId); spec != nil && (spec.Tool != nil || items.IsGearType(spec.Type)) {
					graded = true
				}
			}
			for _, ing := range r.Ingredients {
				if graded {
					break
				}
				graded = tags[ing.ItemTag]
			}
			if graded {
				addId(r.Output.ItemId)
				changed = true
			}
		}
	}
	return ids, tags
}

// capUngradedGradable caps a craft's grade at one above standard when any
// consumed input is an ungraded copy of a gradable material. A craft that
// came out ungraded stays ungraded: this only caps, it never grades.
func capUngradedGradable(grade items.Quality, consumed []items.Item) items.Quality {
	if !grade.Valid() || grade <= items.QualityStandard+1 {
		return grade
	}
	var ids map[int]bool
	var tags map[string]bool
	for _, itm := range consumed {
		if itm.Quality.Valid() {
			continue
		}
		if ids == nil {
			ids, tags = gradableSets()
		}
		if ids[itm.ItemId] {
			return items.QualityStandard + 1
		}
		if spec := items.GetItemSpec(itm.ItemId); spec != nil && spec.ComponentTag != `` && tags[spec.ComponentTag] {
			return items.QualityStandard + 1
		}
	}
	return grade
}
