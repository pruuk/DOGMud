package housing

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// The Hollow Oak is the lodging outside the cities: it checks no standing,
// and every price is a quarter of a city's (rounded up). These read the
// shipped files, so a later edit to either building cannot drift them apart
// unnoticed.
func shippedBuilding(t *testing.T, id string) Building {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(`..`, `..`, `_datafiles`, `world`, `dogmud`, `housing_buildings`, id+`.yaml`))
	if err != nil {
		t.Fatal(err)
	}
	var b Building
	if err := yaml.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestShippedHollowOak_OpenToAllAtAQuarterOfACity(t *testing.T) {
	oak, city := shippedBuilding(t, `hollow_oak`), shippedBuilding(t, `back_court_lodgings`)
	if oak.ChecksStanding() || oak.MinRepTier != `` {
		t.Errorf("the Hollow Oak checks standing: faction %q, tier %q", oak.Faction, oak.MinRepTier)
	}
	quarter := func(p int) int { return (p + 3) / 4 }
	prices := []struct {
		name      string
		oak, city int
	}{
		{`home`, oak.Tiers[0].Price, city.Tiers[0].Price},
		{`redecorate`, oak.RedecoratePrice, city.RedecoratePrice},
		{`guest key`, oak.GuestKeyPrice, city.GuestKeyPrice},
		{`container`, oak.ContainerPrice, city.ContainerPrice},
		{`strongbox`, oak.StrongboxPrice, city.StrongboxPrice},
	}
	for _, p := range prices {
		if p.oak != quarter(p.city) {
			t.Errorf("%s: the Oak asks %d, a quarter of the city's %d is %d", p.name, p.oak, p.city, quarter(p.city))
		}
	}
	// Extensions are priced off what was paid, so the same multiplier keeps
	// every deed at a quarter too.
	if oak.ExtensionPriceMultiplier != city.ExtensionPriceMultiplier {
		t.Errorf("extension multiplier %d, the city's is %d", oak.ExtensionPriceMultiplier, city.ExtensionPriceMultiplier)
	}
}
