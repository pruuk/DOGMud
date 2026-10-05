// Package merchantchests keeps a locked chest in every merchant's room and
// restocks it on a timer. The chests are ordinary room containers
// (rooms.Container): picklock, look, get and put already work on them. This
// package owns what makes one a merchant's chest: which merchant and room it
// belongs to, its name and description, a lock whose pins scale with the
// merchant's average stock value, and the restock that clears it and fills
// it with gold and goods from the merchant's own shop list.
//
// Goods drawn into a chest are marked as the merchant's property
// (items.Item.StolenFrom, StolenFromMob), so anything lifted out of one is
// stolen goods on the stolen-bauble rules: hot for a few days in the area it
// was taken in, recognisable on the thief by the merchant and that area's
// guards, and bought by a fence anywhere (actions/sell_stolen.go,
// actions/stolen_bauble.go).
//
// Everything tunable lives in DataFiles/merchant_chests.yaml (settings,
// the generic chest and one entry per merchant and room). See context.md.
package merchantchests

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"gopkg.in/yaml.v2"
)

// FileName is the catalog under DataFiles.
const FileName = "merchant_chests.yaml"

// Settings are the catalog's tunables (the `settings:` block).
type Settings struct {
	RestockInterval        string  `yaml:"restock_interval"`         // gametime period between restocks, e.g. "1 day"
	RelockInterval         string  `yaml:"relock_interval"`          // gametime period a picked chest stays open
	GoldValueRatio         float64 `yaml:"gold_value_ratio"`         // gold stocked = average stock value x this ...
	GoldSpread             float64 `yaml:"gold_spread"`              // ... x (1 +/- this)
	ItemsMin               int     `yaml:"items_min"`                // goods stocked per restock, at least
	ItemsMax               int     `yaml:"items_max"`                // and at most
	LockBase               float64 `yaml:"lock_base"`                // pins = LockBase + LockPerDoubling x log2(1 + avg)
	LockPerDoubling        float64 `yaml:"lock_per_doubling"`        //
	LockMin                int     `yaml:"lock_min"`                 // pins clamp
	LockMax                int     `yaml:"lock_max"`                 //
	PerceptionBase         float64 `yaml:"perception_base"`          // perception = PerceptionBase + PerceptionPerDoubling x log2(1 + avg)
	PerceptionPerDoubling  float64 `yaml:"perception_per_doubling"`  //
	SleepingPerceptionMult float64 `yaml:"sleeping_perception_mult"` // a sleeping merchant's Perception counts at this fraction
	LookoutMobs            []int   `yaml:"lookout_mobs"`             // mob templates that recognise hot chest goods in their own heat area, besides the `guard` group
}

// Chest is one chest: the generic chest, or the one geared to a merchant in
// a room. Name and Description fall back to the generic chest's when empty.
// Difficulty 0 derives the lock's pins from the merchant's stock value.
type Chest struct {
	MobId       int    `yaml:"mobid,omitempty"`
	RoomId      int    `yaml:"roomid,omitempty"`
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
	Difficulty  int    `yaml:"difficulty,omitempty"`
}

// Catalog is the whole file.
type Catalog struct {
	Settings Settings `yaml:"settings"`
	Generic  Chest    `yaml:"generic"`
	Chests   []Chest  `yaml:"chests"`
}

var (
	mu      sync.RWMutex
	catalog Catalog
	byRoom  = map[int][]Chest{}
)

// Load reads DataFiles/merchant_chests.yaml strictly (an unknown key is an
// error, not a silently ignored value). A world with no such file has no
// merchant chests. A read, parse or Validate failure panics, as the other
// data loaders do.
func Load() {
	start := time.Now()
	path := string(configs.GetFilePathsConfig().DataFiles) + "/" + FileName

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			set(Catalog{})
			mudlog.Info("merchantchests.Load()", "loadedCount", 0, "note", "this world has no "+FileName)
			return
		}
		panic(fmt.Errorf("merchantchests: read %s: %w", path, err))
	}

	var c Catalog
	if err := yaml.UnmarshalStrict(data, &c); err != nil {
		panic(fmt.Errorf("merchantchests: parse %s: %w", path, err))
	}
	if err := Validate(c); err != nil {
		panic(fmt.Errorf("merchantchests: %s: %w", path, err))
	}
	set(c)
	mudlog.Info("merchantchests.Load()", "loadedCount", len(c.Chests), "Time Taken", time.Since(start))
}

// set installs a catalog, normalizing each chest's fallbacks to the generic
// chest and indexing by room.
func set(c Catalog) {
	idx := map[int][]Chest{}
	for i, ch := range c.Chests {
		ch.Name = strings.ToLower(strings.TrimSpace(ch.Name))
		if ch.Name == `` {
			ch.Name = strings.ToLower(strings.TrimSpace(c.Generic.Name))
		}
		if strings.TrimSpace(ch.Description) == `` {
			ch.Description = c.Generic.Description
		}
		ch.Description = strings.TrimRight(ch.Description, "\n")
		c.Chests[i] = ch
		idx[ch.RoomId] = append(idx[ch.RoomId], ch)
	}
	mu.Lock()
	catalog = c
	byRoom = idx
	mu.Unlock()
}

// SetForTest installs c as the live catalog and returns a restore func.
func SetForTest(c Catalog) func() {
	mu.RLock()
	old := catalog
	mu.RUnlock()
	set(c)
	return func() { set(old) }
}

// Validate checks the catalog's shape. It does not need mobs or rooms
// loaded; the world-facing checks (the merchant exists, has a shop list and
// spawns in the room) run in CheckWorld once they are.
func Validate(c Catalog) error {
	s := c.Settings
	var errs []string
	if len(c.Chests) > 0 {
		if strings.TrimSpace(c.Generic.Name) == `` {
			errs = append(errs, "generic.name is required")
		}
		if strings.ContainsAny(c.Generic.Name, " :") {
			errs = append(errs, "generic.name must be one word")
		}
		if strings.TrimSpace(c.Generic.Description) == `` {
			errs = append(errs, "generic.description is required")
		}
		if strings.TrimSpace(s.RestockInterval) == `` {
			errs = append(errs, "settings.restock_interval is required")
		}
		if s.ItemsMin < 0 || s.ItemsMax < s.ItemsMin {
			errs = append(errs, "settings.items_min/items_max must satisfy 0 <= min <= max")
		}
		if s.GoldValueRatio < 0 || s.GoldSpread < 0 || s.GoldSpread > 1 {
			errs = append(errs, "settings.gold_value_ratio must be >= 0 and gold_spread in [0,1]")
		}
		if s.LockMin < 2 || s.LockMax > 32 || s.LockMax < s.LockMin {
			errs = append(errs, "settings.lock_min/lock_max must satisfy 2 <= min <= max <= 32 (util.GetLockSequence's range)")
		}
		if s.SleepingPerceptionMult < 0 || s.SleepingPerceptionMult > 1 {
			errs = append(errs, "settings.sleeping_perception_mult must be in [0,1]")
		}
	}
	seen := map[string]int{}
	for i, ch := range c.Chests {
		where := fmt.Sprintf("chests[%d] (mobid %d, roomid %d)", i, ch.MobId, ch.RoomId)
		if ch.MobId <= 0 || ch.RoomId <= 0 {
			errs = append(errs, where+": mobid and roomid are required")
		}
		name := strings.ToLower(strings.TrimSpace(ch.Name))
		if name == `` {
			name = strings.ToLower(strings.TrimSpace(c.Generic.Name))
		}
		if strings.ContainsAny(name, " :") {
			errs = append(errs, where+": name must be one word (it is typed as a container noun and forms the lock id)")
		}
		if ch.Difficulty != 0 && (ch.Difficulty < 2 || ch.Difficulty > 32) {
			errs = append(errs, where+": difficulty must be 0 (derived) or 2-32")
		}
		key := fmt.Sprintf("%d/%s", ch.RoomId, name)
		if j, dup := seen[key]; dup {
			errs = append(errs, fmt.Sprintf("%s: room %d already has a %q chest (chests[%d])", where, ch.RoomId, name, j))
		}
		seen[key] = i
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// Current returns the live settings.
func Current() Settings {
	mu.RLock()
	defer mu.RUnlock()
	return catalog.Settings
}

// All returns every chest.
func All() []Chest {
	mu.RLock()
	defer mu.RUnlock()
	return append([]Chest(nil), catalog.Chests...)
}

// ChestAt returns the merchant chest named containerName in roomId.
func ChestAt(roomId int, containerName string) (Chest, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, ch := range byRoom[roomId] {
		if ch.Name == containerName {
			return ch, true
		}
	}
	return Chest{}, false
}

// IsLookout reports whether mob template mobId is one of the catalog's
// lookout_mobs: town law-keepers outside the `guard` group (a dock
// constable, a gate warden) who recognise hot stolen chest goods on a thief
// in their own heat area, as a guard does.
func IsLookout(mobId int) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, id := range catalog.Settings.LookoutMobs {
		if id == mobId {
			return true
		}
	}
	return false
}

// SleepingPerceptionMult is the fraction a sleeping merchant's Perception
// counts for in theft and detection contests: the catalog's
// sleeping_perception_mult. With no catalog loaded (a world without the
// file) it is 1, no change; the value comes from data, never a Go default.
func SleepingPerceptionMult() float64 {
	mu.RLock()
	defer mu.RUnlock()
	if len(catalog.Chests) == 0 {
		return 1
	}
	return catalog.Settings.SleepingPerceptionMult
}

// StockItemIds is the merchant's inventory list: the distinct item ids of
// its template's shop list, in file order.
func StockItemIds(mobId int) []int {
	spec := mobs.GetMobSpec(mobs.MobId(mobId))
	if spec == nil {
		return nil
	}
	seen := map[int]bool{}
	var ids []int
	for _, si := range spec.Character.Shop {
		if si.ItemId <= 0 || seen[si.ItemId] {
			continue
		}
		if items.GetItemSpec(si.ItemId) == nil {
			continue
		}
		seen[si.ItemId] = true
		ids = append(ids, si.ItemId)
	}
	return ids
}

// AverageStockValue is the plain mean of the item value over the
// merchant's inventory list, 0 when it has none.
func AverageStockValue(mobId int) float64 {
	ids := StockItemIds(mobId)
	if len(ids) == 0 {
		return 0
	}
	total := 0
	for _, id := range ids {
		total += items.GetItemSpec(id).Value
	}
	return float64(total) / float64(len(ids))
}

// doublings is log2(1 + avg): each doubling of stock value moves a chest's
// lock and its merchant's Perception by a fixed step.
func doublings(avg float64) float64 {
	if avg < 0 {
		avg = 0
	}
	return math.Log2(1 + avg)
}

// LockDifficulty is the lock's pins for a merchant whose average stock
// value is avg, clamped to [LockMin, LockMax].
func LockDifficulty(s Settings, avg float64) int {
	d := int(math.Floor(s.LockBase + s.LockPerDoubling*doublings(avg) + 0.5))
	if d < s.LockMin {
		d = s.LockMin
	}
	if d > s.LockMax {
		d = s.LockMax
	}
	return d
}

// TargetPerception is the Perception base a merchant whose average stock
// value is avg should carry: more valuable stock, a sharper eye.
func TargetPerception(s Settings, avg float64) int {
	return int(math.Floor(s.PerceptionBase + s.PerceptionPerDoubling*doublings(avg) + 0.5))
}

// GoldFor is one restock's gold: avg x GoldValueRatio x (1 + spread), where
// roll01 in [0,1] maps onto [-GoldSpread, +GoldSpread]. At least 1.
func GoldFor(s Settings, avg float64, roll01 float64) int {
	spread := s.GoldSpread * (2*roll01 - 1)
	g := int(math.Floor(avg*s.GoldValueRatio*(1+spread) + 0.5))
	if g < 1 {
		g = 1
	}
	return g
}

// CheckWorld runs the checks that need mobs and rooms: each chest's
// merchant exists, has a shop list and spawns in the chest's room, and its
// template Perception matches TargetPerception. spawnsIn reports whether a
// mob spawns in a room (a parameter so tests need no world). The first
// three are errors; a Perception mismatch is a warning, because the base is
// authored content that may be tuned on purpose.
func CheckWorld(spawnsIn func(mobId, roomId int) bool) (errs []string, warnings []string) {
	s := Current()
	warned := map[int]bool{}
	for _, ch := range All() {
		spec := mobs.GetMobSpec(mobs.MobId(ch.MobId))
		if spec == nil {
			errs = append(errs, fmt.Sprintf("chest %q in room %d: mob %d does not exist", ch.Name, ch.RoomId, ch.MobId))
			continue
		}
		if len(StockItemIds(ch.MobId)) == 0 {
			errs = append(errs, fmt.Sprintf("chest %q in room %d: mob %d (%s) has no shop list to stock it from", ch.Name, ch.RoomId, ch.MobId, spec.Character.Name))
			continue
		}
		if spawnsIn != nil && !spawnsIn(ch.MobId, ch.RoomId) {
			errs = append(errs, fmt.Sprintf("chest %q in room %d: mob %d (%s) does not spawn there", ch.Name, ch.RoomId, ch.MobId, spec.Character.Name))
		}
		want := TargetPerception(s, AverageStockValue(ch.MobId))
		if got := spec.Character.Stats.Perception.Base; got != want && !warned[ch.MobId] {
			warned[ch.MobId] = true
			warnings = append(warnings, fmt.Sprintf("mob %d (%s): perception base %d, stock value says %d", ch.MobId, spec.Character.Name, got, want))
		}
	}
	for _, id := range s.LookoutMobs {
		if mobs.GetMobSpec(mobs.MobId(id)) == nil {
			errs = append(errs, fmt.Sprintf("settings.lookout_mobs: mob %d does not exist", id))
		}
	}
	sort.Strings(errs)
	sort.Strings(warnings)
	return errs, warnings
}
