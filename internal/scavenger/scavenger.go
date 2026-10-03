// Package scavenger holds the city scavengers: one NPC per city (and per New
// Plymouth district) that walks to random rooms of its city, one step at a
// time, and picks up the litter it finds. It replaced the loot goblin
// (2026-09-30). The behavior-tree action that drives them each idle tick is
// scavenger_step (internal/behaviortree/actions_scavenger.go); this package
// holds the authored profiles, the resolved room pools, and the pure pieces
// of the walk.
package scavenger

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// DataFileName is the authored profile file, under the world's data path.
const DataFileName = `scavengers.yaml`

// DefaultBiomes are the biomes a scavenger's pool takes from its zones when
// the profile names none: streets and the buildings on them.
var DefaultBiomes = []string{`city_thoroughfare`, `city_backstreet`, `interior`}

// Profile is one scavenger as authored in scavengers.yaml.
type Profile struct {
	MobId int    `yaml:"mobid"`
	City  string `yaml:"city"` // for logs and staff: "New Plymouth, the Common"

	// The pool of rooms the scavenger picks targets from: every room of
	// Zones whose biome is one of Biomes (DefaultBiomes when empty), or,
	// when IncludeRooms is set, exactly those rooms. ExcludeRooms are then
	// taken out, and any room the scavenger cannot reach from HomeRoom is
	// dropped at load.
	Zones        []string `yaml:"zones"`
	Biomes       []string `yaml:"biomes,omitempty"`
	IncludeRooms []int    `yaml:"include_rooms,omitempty"`
	ExcludeRooms []int    `yaml:"exclude_rooms,omitempty"`

	// HomeRoom is where it spawns (the room's spawninfo names it) and where
	// its pool is measured from.
	HomeRoom int `yaml:"home_room"`

	// Lines in the scavenger's own voice. {actor} is the scavenger, {item}
	// the thing picked up, {gold} the coins. One is picked at random.
	PickupLines []string `yaml:"pickup_lines"`
	GoldLines   []string `yaml:"gold_lines"`
	ResetLines  []string `yaml:"reset_lines"` // the daily haul is taken away

	pool []int
}

// Pool is the resolved list of rooms the scavenger picks targets from.
func (p *Profile) Pool() []int {
	return p.pool
}

// fileFormat is the shape of scavengers.yaml.
type fileFormat struct {
	Scavengers []*Profile `yaml:"scavengers"`
}

// World is what loading needs from the rooms and the mapper, injected so the
// resolver can be tested without a world on disk.
type World struct {
	ZoneExists func(zone string) bool
	ZoneRooms  func(zone string) []int
	RoomBiome  func(roomId int) (biome string, ok bool)
	Reachable  func(from, to int) bool
	MobExists  func(mobId int) bool
	// Private reports rooms no scavenger may ever patrol (player homes).
	// They are dropped from every pool, include_rooms and home room alike.
	Private func(roomId int) bool
}

var (
	mu        sync.RWMutex
	profiles  = map[int]*Profile{}
	patrolled = map[int]bool{}
)

// ProfileFor returns the scavenger profile for a mob template id, or nil.
func ProfileFor(mobId int) *Profile {
	mu.RLock()
	defer mu.RUnlock()
	return profiles[mobId]
}

// All returns every loaded profile, ordered by mob id.
func All() []*Profile {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]*Profile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b *Profile) int { return a.MobId - b.MobId })
	return out
}

// IsPatrolledRoom reports whether any scavenger keeps this room. The daily
// floor decay (rooms.SetFloorDecayExempt) leaves those floors to them.
func IsPatrolledRoom(roomId int) bool {
	mu.RLock()
	defer mu.RUnlock()
	return patrolled[roomId]
}

// AnchorRooms are the scavengers' home rooms. The server prepares them at
// boot so every scavenger is walking from the start, not only once a player
// wanders by.
func AnchorRooms() []int {
	out := []int{}
	for _, p := range All() {
		out = append(out, p.HomeRoom)
	}
	return out
}

// LoadDataFiles reads scavengers.yaml from the world data path and resolves
// every pool against w. Authored mistakes (an unknown mob or zone, a home
// room outside the pool, a duplicate) fail the boot like any other broken
// content; a missing file loads nothing.
func LoadDataFiles(w World) {
	start := time.Now()
	path := configs.GetFilePathsConfig().DataFiles.String() + `/` + DataFileName

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			mudlog.Info(`scavenger.LoadDataFiles()`, `loadedCount`, 0, `note`, `no `+DataFileName)
			install(nil)
			return
		}
		panic(fmt.Sprintf(`scavenger.LoadDataFiles: %v`, err))
	}

	loaded, err := Parse(raw, w)
	if err != nil {
		panic(fmt.Sprintf(`scavenger.LoadDataFiles: %v`, err))
	}
	install(loaded)

	for _, p := range loaded {
		mudlog.Info(`scavenger`, `mobId`, p.MobId, `city`, p.City, `homeRoom`, p.HomeRoom, `poolRooms`, len(p.pool))
	}
	mudlog.Info(`scavenger.LoadDataFiles()`, `loadedCount`, len(loaded), `Time Taken`, time.Since(start))
}

// Parse decodes scavengers.yaml and resolves each profile's pool.
func Parse(raw []byte, w World) ([]*Profile, error) {
	var f fileFormat
	if err := yaml.UnmarshalStrict(raw, &f); err != nil {
		return nil, err
	}
	seenMob := map[int]bool{}
	for i, p := range f.Scavengers {
		if p == nil {
			return nil, fmt.Errorf(`entry %d is empty`, i)
		}
		if p.MobId <= 0 {
			return nil, fmt.Errorf(`entry %d: mobid is required`, i)
		}
		if seenMob[p.MobId] {
			return nil, fmt.Errorf(`mob %d is listed twice`, p.MobId)
		}
		seenMob[p.MobId] = true
		if w.MobExists != nil && !w.MobExists(p.MobId) {
			return nil, fmt.Errorf(`mob %d does not exist`, p.MobId)
		}
		if len(p.PickupLines) == 0 || len(p.GoldLines) == 0 || len(p.ResetLines) == 0 {
			return nil, fmt.Errorf(`mob %d: pickup_lines, gold_lines and reset_lines are all required`, p.MobId)
		}
		for _, l := range slices.Concat(p.PickupLines, p.GoldLines, p.ResetLines) {
			if !strings.Contains(l, `{actor}`) {
				return nil, fmt.Errorf(`mob %d: line %q does not name the scavenger ({actor})`, p.MobId, l)
			}
		}
		for _, l := range p.PickupLines {
			if !strings.Contains(l, `{item}`) {
				return nil, fmt.Errorf(`mob %d: pickup line %q does not name the item ({item})`, p.MobId, l)
			}
		}
		for _, l := range p.GoldLines {
			if !strings.Contains(l, `{gold}`) {
				return nil, fmt.Errorf(`mob %d: gold line %q does not name the coins ({gold})`, p.MobId, l)
			}
		}
		if err := resolvePool(p, w); err != nil {
			return nil, fmt.Errorf(`mob %d (%s): %w`, p.MobId, p.City, err)
		}
	}
	return f.Scavengers, nil
}

// resolvePool fills p.pool from its zones, biomes, include and exclude lists,
// keeping only rooms reachable from the home room.
func resolvePool(p *Profile, w World) error {
	if len(p.Zones) == 0 && len(p.IncludeRooms) == 0 {
		return fmt.Errorf(`needs zones or include_rooms`)
	}
	for _, z := range p.Zones {
		if w.ZoneExists != nil && !w.ZoneExists(z) {
			return fmt.Errorf(`zone %q does not exist`, z)
		}
	}

	candidates := []int{}
	if len(p.IncludeRooms) > 0 {
		for _, id := range p.IncludeRooms {
			if _, ok := w.RoomBiome(id); !ok {
				return fmt.Errorf(`include_rooms: room %d does not exist`, id)
			}
			candidates = append(candidates, id)
		}
	} else {
		biomes := p.Biomes
		if len(biomes) == 0 {
			biomes = DefaultBiomes
		}
		for _, z := range p.Zones {
			for _, id := range w.ZoneRooms(z) {
				if b, ok := w.RoomBiome(id); ok && slices.Contains(biomes, b) {
					candidates = append(candidates, id)
				}
			}
		}
	}

	excluded := map[int]bool{}
	for _, id := range p.ExcludeRooms {
		excluded[id] = true
	}
	if w.Private != nil {
		for _, id := range candidates {
			if w.Private(id) {
				excluded[id] = true
			}
		}
		if w.Private(p.HomeRoom) {
			return fmt.Errorf(`home_room %d is a private room (player housing)`, p.HomeRoom)
		}
	}
	if excluded[p.HomeRoom] {
		return fmt.Errorf(`home_room %d is excluded`, p.HomeRoom)
	}
	if !slices.Contains(candidates, p.HomeRoom) {
		return fmt.Errorf(`home_room %d is not in its own pool`, p.HomeRoom)
	}

	pool := []int{}
	unreachable := []string{}
	seen := map[int]bool{}
	for _, id := range candidates {
		if excluded[id] || seen[id] {
			continue
		}
		seen[id] = true
		if id != p.HomeRoom && w.Reachable != nil && !w.Reachable(p.HomeRoom, id) {
			unreachable = append(unreachable, strconv.Itoa(id))
			continue
		}
		pool = append(pool, id)
	}
	slices.Sort(pool)
	if len(unreachable) > 0 {
		mudlog.Warn(`scavenger`, `mobId`, p.MobId, `note`, `rooms dropped from the pool, no path from home`,
			`rooms`, strings.Join(unreachable, `,`))
	}
	if len(pool) < 2 {
		return fmt.Errorf(`pool has %d room(s); a scavenger needs somewhere to walk`, len(pool))
	}
	p.pool = pool
	return nil
}

// install publishes a loaded set of profiles.
func install(loaded []*Profile) {
	next := map[int]*Profile{}
	nextPatrolled := map[int]bool{}
	for _, p := range loaded {
		next[p.MobId] = p
		for _, id := range p.pool {
			nextPatrolled[id] = true
		}
	}
	mu.Lock()
	profiles = next
	patrolled = nextPatrolled
	mu.Unlock()
}

// SetProfilesForTest installs profiles (with their pools already set via
// SetPoolForTest) and returns a function restoring the previous set.
func SetProfilesForTest(ps ...*Profile) func() {
	mu.RLock()
	prevProfiles, prevPatrolled := profiles, patrolled
	mu.RUnlock()
	install(ps)
	return func() {
		mu.Lock()
		profiles, patrolled = prevProfiles, prevPatrolled
		mu.Unlock()
	}
}

// SetPoolForTest sets a profile's resolved pool directly.
func (p *Profile) SetPoolForTest(pool []int) {
	p.pool = pool
}
