package crafting

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// corpseSalvageEntry pairs a mob group key with the salvage returns
// players recover when they salvage a corpse from that group.
type corpseSalvageEntry struct {
	Group   string
	Returns []items.SalvageReturn
}

// corpseSalvageTable is the static lookup table for corpse salvage.
// Order matters: LookupCorpseSalvage returns the first matching entry.
// Specific entries (rodent) must appear before broader ones (animal).
// Future expansion (bird, insect, chrysalis, etc.) just appends here.
var corpseSalvageTable = []corpseSalvageEntry{
	{
		Group: "rodent", // small game (wild hare, etc.) → hare meat
		Returns: []items.SalvageReturn{
			{ItemTag: "wild-hare-meat", Quantity: 1},
			{ItemTag: "leather-strip", Quantity: 1},
		},
	},
	{
		Group: "animal", // generic game → raw meat
		Returns: []items.SalvageReturn{
			{ItemTag: "raw-meat", Quantity: 1},
			{ItemTag: "leather-strip", Quantity: 2},
			{ItemTag: "sinew", Quantity: 1},
		},
	},
	{
		Group: "humanoid",
		Returns: []items.SalvageReturn{
			{ItemTag: "cloth-strip", Quantity: 2},
			{ItemTag: "leather-strip", Quantity: 1},
		},
	},
}

// speciesCorpseSalvage is the fallback used when none of a mob's groups match
// corpseSalvageTable. It is keyed by species name (species yaml `name:`).
//
// It exists because group tags are authored per zone and drift: the Ironwind
// steppe wolves are grouped `steppe-wolf`/`canine`, the Pothole Coulee game is
// `beast`, and neither matched a table row, so 116 natural creatures left a
// corpse with nothing to recover. The species is set on every mob and does not
// drift, so it is the honest fallback.
//
// Only warm-blooded game is listed. Insects, arachnids, reptiles, fish and
// leeches stay unsalvageable until per-species harvest tables land (the
// wilderness-trades plan, phase 2), because "raw meat and leather" from a
// beetle would be wrong rather than merely thin.
var speciesCorpseSalvage = map[string][]items.SalvageReturn{
	"rodent":   corpseSalvageTable[0].Returns,
	"canine":   corpseSalvageTable[1].Returns,
	"bear":     corpseSalvageTable[1].Returns,
	"boar":     corpseSalvageTable[1].Returns,
	"deer":     corpseSalvageTable[1].Returns,
	"feline":   corpseSalvageTable[1].Returns,
	"mustelid": corpseSalvageTable[1].Returns,
	"horse":    corpseSalvageTable[1].Returns,
	"bird": {
		{ItemTag: "raw-meat", Quantity: 1},
	},
	"raptor": {
		{ItemTag: "raw-meat", Quantity: 1},
		{ItemTag: "sinew", Quantity: 1},
	},
}

// LookupCorpseSalvage returns the salvage returns for the first matching
// group in the table, or nil if no group matches. The mob's full groups
// slice is passed in; iteration order is the table's declaration order.
func LookupCorpseSalvage(groups []string) []items.SalvageReturn {
	for _, entry := range corpseSalvageTable {
		for _, g := range groups {
			if g == entry.Group {
				return entry.Returns
			}
		}
	}
	return nil
}

// LookupCorpseSalvageFor is LookupCorpseSalvage with a species fallback:
// groups win when any matches (an authored `rodent` or `humanoid` tag is a
// deliberate choice), otherwise the species name decides. speciesName is
// compared case-insensitively; "" means no fallback.
func LookupCorpseSalvageFor(groups []string, speciesName string) []items.SalvageReturn {
	if returns := LookupCorpseSalvage(groups); len(returns) > 0 {
		return returns
	}
	return speciesCorpseSalvage[strings.ToLower(speciesName)]
}

// LookupCorpseSalvageForMob resolves the species name from a species id and
// calls LookupCorpseSalvageFor. Every call site that has a mob spec should use
// this rather than LookupCorpseSalvage, so a corpse that the salvage command
// accepts is also one the mob path, the companion path and the resolver accept.
func LookupCorpseSalvageForMob(groups []string, speciesId int) []items.SalvageReturn {
	name := ""
	if sp := species.GetSpecies(speciesId); sp != nil {
		name = sp.Name
	}
	return LookupCorpseSalvageFor(groups, name)
}
