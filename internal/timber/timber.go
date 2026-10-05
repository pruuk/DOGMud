// Package timber is the data and the per-room stand state behind
// lumberjacking (wilderness trades, phase 4).
//
// timber.yaml names the tree species (each with its log item, an optional
// bark item and a tier) and, per biome and per zone, a weighted pool of the
// species that grow there. A zone pool replaces the biome pool for that
// zone's choppable rooms, which is how the Fernway grows oak and ash while the
// Cascade Pass grows pine and yew.
//
// Each choppable room holds a Stand: one species, a number of trees that can
// still be felled, and the round it last changed. The stand lives in the
// room's long-term data, so it survives a restart with the room's instance
// save. It regrows one tree per Balance.TimberRegrowRounds, and once it has
// been felled to the last tree, its species is re-rolled when it regrows,
// leaning toward what the neighbouring rooms grow so groves stay together.
package timber

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// DataFileName is the authored species and pool file, under the world data path.
const DataFileName = `timber.yaml`

// Species is one kind of tree.
type Species struct {
	Id         string `yaml:"id"`   // "yew"
	Name       string `yaml:"name"` // how a player reads it: "yew"
	LogItemId  int    `yaml:"log"`  // the log a felled tree gives
	BarkItemId int    `yaml:"bark"` // optional bark that may come off with it
	Tier       int    `yaml:"tier"` // 1 common .. 4 rare; harder to fell, worth more
	Note       string `yaml:"note"` // one line for survey: what the wood is good for
	// Bow and Arrow are what this wood gives a bow made from its staves and
	// arrows made from its shafts (wood.go). Zero values are neutral.
	Bow   BowTraits   `yaml:"bow,omitempty"`
	Arrow ArrowTraits `yaml:"arrow,omitempty"`
}

// MinAxe is the poorest axe tier (items.ToolTier: 1 crude .. 4 masterwork)
// that can fell this species at all: one below its own tier, so common woods
// take any axe, yew and walnut want iron and ironwood wants steel.
func (s *Species) MinAxe() int {
	if s.Tier <= 2 {
		return 1
	}
	return s.Tier - 1
}

// Weighted is one entry of a pool.
type Weighted struct {
	Species string `yaml:"species"`
	Weight  int    `yaml:"weight"`
}

type fileFormat struct {
	Species []Species             `yaml:"species"`
	Biomes  map[string][]Weighted `yaml:"biomes"`
	Zones   map[string][]Weighted `yaml:"zones"`
}

// Data is a parsed, validated timber.yaml.
type Data struct {
	species map[string]*Species
	order   []string
	biomes  map[string][]Weighted
	zones   map[string][]Weighted
}

// World is what Parse checks the file against.
type World struct {
	ItemExists func(itemId int) bool
	ZoneExists func(zone string) bool
}

var (
	mu      sync.RWMutex
	current = &Data{species: map[string]*Species{}, biomes: map[string][]Weighted{}, zones: map[string][]Weighted{}}
)

// Parse decodes and validates timber.yaml.
func Parse(raw []byte, w World) (*Data, error) {
	var f fileFormat
	if err := yaml.UnmarshalStrict(raw, &f); err != nil {
		return nil, err
	}
	d := &Data{species: map[string]*Species{}, biomes: f.Biomes, zones: f.Zones}
	if d.biomes == nil {
		d.biomes = map[string][]Weighted{}
	}
	if d.zones == nil {
		d.zones = map[string][]Weighted{}
	}
	for i := range f.Species {
		s := f.Species[i]
		if s.Id == `` || s.Name == `` {
			return nil, fmt.Errorf("species %d: id and name are required", i)
		}
		if _, dup := d.species[s.Id]; dup {
			return nil, fmt.Errorf("species %q listed twice", s.Id)
		}
		if s.Tier < 1 || s.Tier > 4 {
			return nil, fmt.Errorf("species %q: tier must be 1..4, got %d", s.Id, s.Tier)
		}
		if err := s.Bow.validate(); err != nil {
			return nil, fmt.Errorf("species %q bow: %v", s.Id, err)
		}
		if err := s.Arrow.validate(); err != nil {
			return nil, fmt.Errorf("species %q arrow: %v", s.Id, err)
		}
		if w.ItemExists != nil {
			if !w.ItemExists(s.LogItemId) {
				return nil, fmt.Errorf("species %q: log item %d does not exist", s.Id, s.LogItemId)
			}
			if s.BarkItemId != 0 && !w.ItemExists(s.BarkItemId) {
				return nil, fmt.Errorf("species %q: bark item %d does not exist", s.Id, s.BarkItemId)
			}
		}
		d.species[s.Id] = &s
		d.order = append(d.order, s.Id)
	}
	check := func(where string, pool []Weighted) error {
		if len(pool) == 0 {
			return fmt.Errorf("%s: empty pool", where)
		}
		for _, e := range pool {
			if _, ok := d.species[e.Species]; !ok {
				return fmt.Errorf("%s: unknown species %q", where, e.Species)
			}
			if e.Weight <= 0 {
				return fmt.Errorf("%s: species %q weight must be positive", where, e.Species)
			}
		}
		return nil
	}
	for biome, pool := range d.biomes {
		if err := check(`biome `+biome, pool); err != nil {
			return nil, err
		}
	}
	for zone, pool := range d.zones {
		if err := check(`zone `+zone, pool); err != nil {
			return nil, err
		}
		if w.ZoneExists != nil && !w.ZoneExists(zone) {
			return nil, fmt.Errorf("zone %q does not exist", zone)
		}
	}
	return d, nil
}

// LoadDataFiles reads timber.yaml from the world data path. Broken content
// fails the boot; a missing file loads nothing and no room is choppable.
func LoadDataFiles(w World) {
	start := time.Now()
	path := configs.GetFilePathsConfig().DataFiles.String() + `/` + DataFileName
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			mudlog.Info(`timber.LoadDataFiles()`, `loadedCount`, 0, `note`, `no `+DataFileName)
			Install(nil)
			return
		}
		panic(fmt.Sprintf(`timber.LoadDataFiles: %v`, err))
	}
	d, err := Parse(raw, w)
	if err != nil {
		panic(fmt.Sprintf(`timber.LoadDataFiles: %v`, err))
	}
	Install(d)
	mudlog.Info(`timber.LoadDataFiles()`, `species`, len(d.species), `biomes`, len(d.biomes), `zones`, len(d.zones), `Time Taken`, time.Since(start))
}

// Install makes d the live data (nil clears it). Tests use it directly.
func Install(d *Data) {
	if d == nil {
		d = &Data{species: map[string]*Species{}, biomes: map[string][]Weighted{}, zones: map[string][]Weighted{}}
	}
	mu.Lock()
	current = d
	mu.Unlock()
}

// GetSpecies returns a species by id, or nil.
func GetSpecies(id string) *Species {
	mu.RLock()
	defer mu.RUnlock()
	return current.species[id]
}

// AllSpecies lists every species in file order.
func AllSpecies() []Species {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Species, 0, len(current.order))
	for _, id := range current.order {
		out = append(out, *current.species[id])
	}
	return out
}

// IsChoppable reports whether rooms of this biome grow fellable timber.
func IsChoppable(biome string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := current.biomes[biome]
	return ok
}

// Pool is the weighted species pool for a room: the zone's pool when the zone
// has one, else the biome's. Nil when the biome grows no timber.
func Pool(zone, biome string) []Weighted {
	mu.RLock()
	defer mu.RUnlock()
	if _, ok := current.biomes[biome]; !ok {
		return nil
	}
	if p, ok := current.zones[zone]; ok {
		return p
	}
	return current.biomes[biome]
}

// Zones lists the zones with their own pool, sorted (help text, tests).
func Zones() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(current.zones))
	for z := range current.zones {
		out = append(out, z)
	}
	sort.Strings(out)
	return out
}
