package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/species"
)

func TestResolveHarvest_MobOverridesSpecies(t *testing.T) {
	restore := species.SeedSpeciesForTest(map[int]*species.Species{
		2: {SpeciesId: 2, Name: "canine", Harvest: &species.HarvestTable{
			Skin:    []species.HarvestEntry{{Item: "wolf-pelt", Qty: 1}},
			Butcher: []species.HarvestEntry{{Item: "raw-meat", Qty: 2}},
		}},
	})
	defer restore()

	plain := &Mob{}
	plain.Character.SpeciesId = 2
	if got := ResolveHarvest(plain); got.Skin[0].Item != "wolf-pelt" || got.Butcher[0].Item != "raw-meat" {
		t.Errorf("plain wolf = %+v", got)
	}

	unique := &Mob{Harvest: &species.HarvestTable{Skin: []species.HarvestEntry{{Item: "cascade-hide", Qty: 1}}}}
	unique.Character.SpeciesId = 2
	got := ResolveHarvest(unique)
	if got.Skin[0].Item != "cascade-hide" || got.Butcher[0].Item != "raw-meat" {
		t.Errorf("unique wolf = %+v, want its own pelt and the species meat", got)
	}

	if !(&species.HarvestTable{}).Empty() {
		t.Fatal("sanity")
	}
	if got := ResolveHarvest(nil); !got.Empty() {
		t.Error("nil mob yields nothing")
	}
}
