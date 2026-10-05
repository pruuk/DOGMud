// Package mining is the data and the per-room vein state behind mining
// (wilderness trades). It mirrors internal/timber.
//
// mining.yaml names the ores (each with the item a load gives, a tier and
// the poorest pick that can work it), the gems a mining job may turn up, and
// the weighted pools of ore per room biome, per zone and per room. A room is
// mineable when its biome has a pool (cave, mountains, cliffs) and its zone is
// not excluded, or when the room has its own pool. A zone pool replaces the
// biome pool for that zone's mineable rooms; a room pool replaces both (the
// deep drifts of a working mine).
//
// Each mineable room holds a Vein: one ore, a number of loads that can still
// be dug and the round it last changed, in the room's long-term data. It
// refills one load per Balance.MiningRegrowRounds; once worked out, its ore is
// re-rolled when it refills, leaning toward its neighbours' ore.
package mining

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

// DataFileName is the authored ore and pool file, under the world data path.
const DataFileName = `mining.yaml`

// Ore is one kind of vein.
type Ore struct {
	Id      string `yaml:"id"`       // "silver"
	Name    string `yaml:"name"`     // how a player reads it: "silver"
	ItemId  int    `yaml:"item"`     // the item one load gives
	Tier    int    `yaml:"tier"`     // 1 common .. 4 rare; harder to work, worth more
	MinPick int    `yaml:"min_pick"` // poorest pick tier that can work it (items.ToolTier: 1 crude .. 4 masterwork)
	Note    string `yaml:"note"`     // one line for prospect
}

// Gem is one entry of the gem table a successful mining job may draw from.
type Gem struct {
	ItemId  int `yaml:"item"`
	Weight  int `yaml:"weight"`
	MinPick int `yaml:"min_pick"` // poorest pick that takes it whole (0 = any)
}

// Weighted is one entry of a pool.
type Weighted struct {
	Ore    string `yaml:"ore"`
	Weight int    `yaml:"weight"`
}

type fileFormat struct {
	Ores         []Ore                 `yaml:"ores"`
	Gems         []Gem                 `yaml:"gems"`
	Biomes       map[string][]Weighted `yaml:"biomes"`
	Zones        map[string][]Weighted `yaml:"zones"`
	Rooms        map[int][]Weighted    `yaml:"rooms"`
	ExcludeZones []string              `yaml:"exclude_zones"`
}

// Data is a parsed, validated mining.yaml.
type Data struct {
	ores    map[string]*Ore
	order   []string
	gems    []Gem
	biomes  map[string][]Weighted
	zones   map[string][]Weighted
	rooms   map[int][]Weighted
	exclude map[string]bool
}

// World is what Parse checks the file against.
type World struct {
	ItemExists func(itemId int) bool
	ZoneExists func(zone string) bool
	RoomExists func(roomId int) bool
}

var (
	mu      sync.RWMutex
	current = emptyData()
)

func emptyData() *Data {
	return &Data{
		ores:    map[string]*Ore{},
		biomes:  map[string][]Weighted{},
		zones:   map[string][]Weighted{},
		rooms:   map[int][]Weighted{},
		exclude: map[string]bool{},
	}
}

// Parse decodes and validates mining.yaml.
func Parse(raw []byte, w World) (*Data, error) {
	var f fileFormat
	if err := yaml.UnmarshalStrict(raw, &f); err != nil {
		return nil, err
	}
	d := emptyData()
	if f.Biomes != nil {
		d.biomes = f.Biomes
	}
	if f.Zones != nil {
		d.zones = f.Zones
	}
	if f.Rooms != nil {
		d.rooms = f.Rooms
	}
	for i := range f.Ores {
		o := f.Ores[i]
		if o.Id == `` || o.Name == `` {
			return nil, fmt.Errorf("ore %d: id and name are required", i)
		}
		if _, dup := d.ores[o.Id]; dup {
			return nil, fmt.Errorf("ore %q listed twice", o.Id)
		}
		if o.Tier < 1 || o.Tier > 4 {
			return nil, fmt.Errorf("ore %q: tier must be 1..4, got %d", o.Id, o.Tier)
		}
		if o.MinPick < 1 || o.MinPick > 4 {
			return nil, fmt.Errorf("ore %q: min_pick must be 1..4, got %d", o.Id, o.MinPick)
		}
		if w.ItemExists != nil && !w.ItemExists(o.ItemId) {
			return nil, fmt.Errorf("ore %q: item %d does not exist", o.Id, o.ItemId)
		}
		d.ores[o.Id] = &o
		d.order = append(d.order, o.Id)
	}
	for i, g := range f.Gems {
		if g.Weight <= 0 {
			return nil, fmt.Errorf("gem %d: weight must be positive", i)
		}
		if g.MinPick < 0 || g.MinPick > 4 {
			return nil, fmt.Errorf("gem %d: min_pick must be 0..4, got %d", i, g.MinPick)
		}
		if w.ItemExists != nil && !w.ItemExists(g.ItemId) {
			return nil, fmt.Errorf("gem %d: item %d does not exist", i, g.ItemId)
		}
	}
	d.gems = f.Gems
	check := func(where string, pool []Weighted) error {
		if len(pool) == 0 {
			return fmt.Errorf("%s: empty pool", where)
		}
		for _, e := range pool {
			if _, ok := d.ores[e.Ore]; !ok {
				return fmt.Errorf("%s: unknown ore %q", where, e.Ore)
			}
			if e.Weight <= 0 {
				return fmt.Errorf("%s: ore %q weight must be positive", where, e.Ore)
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
	for roomId, pool := range d.rooms {
		if err := check(fmt.Sprintf(`room %d`, roomId), pool); err != nil {
			return nil, err
		}
		if w.RoomExists != nil && !w.RoomExists(roomId) {
			return nil, fmt.Errorf("room %d does not exist", roomId)
		}
	}
	for _, zone := range f.ExcludeZones {
		if w.ZoneExists != nil && !w.ZoneExists(zone) {
			return nil, fmt.Errorf("excluded zone %q does not exist", zone)
		}
		if _, has := d.zones[zone]; has {
			return nil, fmt.Errorf("zone %q is both excluded and given a pool", zone)
		}
		d.exclude[zone] = true
	}
	return d, nil
}

// LoadDataFiles reads mining.yaml from the world data path. Broken content
// fails the boot; a missing file loads nothing and no room is mineable.
func LoadDataFiles(w World) {
	start := time.Now()
	path := configs.GetFilePathsConfig().DataFiles.String() + `/` + DataFileName
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			mudlog.Info(`mining.LoadDataFiles()`, `loadedCount`, 0, `note`, `no `+DataFileName)
			Install(nil)
			return
		}
		panic(fmt.Sprintf(`mining.LoadDataFiles: %v`, err))
	}
	d, err := Parse(raw, w)
	if err != nil {
		panic(fmt.Sprintf(`mining.LoadDataFiles: %v`, err))
	}
	Install(d)
	mudlog.Info(`mining.LoadDataFiles()`, `ores`, len(d.ores), `biomes`, len(d.biomes), `zones`, len(d.zones), `rooms`, len(d.rooms), `Time Taken`, time.Since(start))
}

// Install makes d the live data (nil clears it). Tests use it directly.
func Install(d *Data) {
	if d == nil {
		d = emptyData()
	}
	mu.Lock()
	current = d
	mu.Unlock()
}

// GetOre returns an ore by id, or nil.
func GetOre(id string) *Ore {
	mu.RLock()
	defer mu.RUnlock()
	return current.ores[id]
}

// AllOres lists every ore in file order.
func AllOres() []Ore {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Ore, 0, len(current.order))
	for _, id := range current.order {
		out = append(out, *current.ores[id])
	}
	return out
}

// Gems lists the gem table.
func Gems() []Gem {
	mu.RLock()
	defer mu.RUnlock()
	return append([]Gem{}, current.gems...)
}

// Pool is the weighted ore pool for a room: its own pool when it has one;
// else nothing when its zone is excluded or its biome has no pool; else the
// zone's pool when the zone has one, else the biome's.
func Pool(roomId int, zone, biome string) []Weighted {
	mu.RLock()
	defer mu.RUnlock()
	if p, ok := current.rooms[roomId]; ok {
		return p
	}
	if current.exclude[zone] {
		return nil
	}
	if _, ok := current.biomes[biome]; !ok {
		return nil
	}
	if p, ok := current.zones[zone]; ok {
		return p
	}
	return current.biomes[biome]
}

// IsMineableBiome reports whether rooms of this biome hold ore by default.
func IsMineableBiome(biome string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := current.biomes[biome]
	return ok
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

// PickGem draws a gem a pick of tier pickTier can take, or ok false when the
// table is empty or nothing qualifies. rnd(n) returns 0..n-1.
func PickGem(pickTier int, rnd func(int) int) (Gem, bool) {
	gems := Gems()
	total := 0
	for _, g := range gems {
		if g.MinPick <= pickTier {
			total += g.Weight
		}
	}
	if total <= 0 {
		return Gem{}, false
	}
	pick := rnd(total)
	for _, g := range gems {
		if g.MinPick > pickTier {
			continue
		}
		if pick < g.Weight {
			return g, true
		}
		pick -= g.Weight
	}
	return Gem{}, false
}
