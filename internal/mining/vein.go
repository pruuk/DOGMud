package mining

// Vein is the ore in one room.
type Vein struct {
	Ore       string // ore id
	Stock     int    // loads that can still be dug
	Max       int    // a full vein
	Updated   uint64 // round of the last change (digging or refilling)
	WorkedOut bool   // dug to the last load; re-roll the ore on refilling
}

// Long-term room data keys. Values are plain strings and ints so they survive
// a YAML round trip through the room's instance save.
const (
	keyOre       = `mining.ore`
	keyStock     = `mining.stock`
	keyMax       = `mining.max`
	keyUpdated   = `mining.updated`
	keyWorkedOut = `mining.workedout`
)

// Store is where a vein lives: rooms.Room satisfies it.
type Store interface {
	GetLongTermData(key string) any
	SetLongTermData(key string, value any)
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

// LoadVein reads a room's vein. ok is false when the room has none yet.
func LoadVein(s Store) (Vein, bool) {
	ore, _ := s.GetLongTermData(keyOre).(string)
	if ore == `` {
		return Vein{}, false
	}
	v := Vein{Ore: ore}
	v.Stock, _ = asInt(s.GetLongTermData(keyStock))
	v.Max, _ = asInt(s.GetLongTermData(keyMax))
	u, _ := asInt(s.GetLongTermData(keyUpdated))
	v.Updated = uint64(u)
	w, _ := asInt(s.GetLongTermData(keyWorkedOut))
	v.WorkedOut = w != 0
	if v.Max < 1 {
		v.Max = 1
	}
	return v, true
}

// SaveVein writes a vein back to its room.
func SaveVein(s Store, v Vein) {
	s.SetLongTermData(keyOre, v.Ore)
	s.SetLongTermData(keyStock, v.Stock)
	s.SetLongTermData(keyMax, v.Max)
	s.SetLongTermData(keyUpdated, int(v.Updated))
	worked := 0
	if v.WorkedOut {
		worked = 1
	}
	s.SetLongTermData(keyWorkedOut, worked)
}

// Refill adds one load per regrowRounds elapsed since Updated, up to Max. It
// reports whether the vein came back from being worked out, which is when
// the caller re-rolls its ore.
func (v *Vein) Refill(now uint64, regrowRounds int) (cameBack bool) {
	if regrowRounds < 1 || now <= v.Updated || v.Stock >= v.Max {
		if v.Stock >= v.Max {
			v.Updated = now
		}
		return false
	}
	grown := int((now - v.Updated) / uint64(regrowRounds))
	if grown < 1 {
		return false
	}
	wasEmpty := v.Stock == 0
	v.Stock += grown
	if v.Stock > v.Max {
		v.Stock = v.Max
	}
	v.Updated += uint64(grown) * uint64(regrowRounds)
	if wasEmpty && v.WorkedOut {
		v.WorkedOut = false
		return true
	}
	return false
}

// RoundsToNextLoad is how long until the vein refills its next load, or 0
// when it is full.
func (v Vein) RoundsToNextLoad(now uint64, regrowRounds int) uint64 {
	if v.Stock >= v.Max || regrowRounds < 1 {
		return 0
	}
	next := v.Updated + uint64(regrowRounds)
	if next <= now {
		return 0
	}
	return next - now
}

// Dig takes one load. It reports false when there was none.
func (v *Vein) Dig(now uint64) bool {
	if v.Stock < 1 {
		return false
	}
	if v.Stock >= v.Max {
		v.Updated = now // refilling is measured from the first load taken
	}
	v.Stock--
	if v.Stock == 0 {
		v.WorkedOut = true
	}
	return true
}

// PickOre draws an ore from pool. Each neighbouring room's ore that is in the
// pool gets a bonus of a quarter of the pool's total weight, so seams run
// together. rnd(n) returns 0..n-1. Returns "" for an empty pool.
func PickOre(pool []Weighted, neighbours []string, rnd func(int) int) string {
	if len(pool) == 0 {
		return ``
	}
	total := 0
	for _, e := range pool {
		total += e.Weight
	}
	bonus := total / 4
	if bonus < 1 {
		bonus = 1
	}
	weights := make([]int, len(pool))
	sum := 0
	for i, e := range pool {
		w := e.Weight
		for _, n := range neighbours {
			if n == e.Ore {
				w += bonus
			}
		}
		weights[i] = w
		sum += w
	}
	pick := rnd(sum)
	for i, w := range weights {
		if pick < w {
			return pool[i].Ore
		}
		pick -= w
	}
	return pool[len(pool)-1].Ore
}

// NewVein seeds a full vein of ore with min..max loads.
func NewVein(ore string, min, max int, now uint64, rnd func(int) int) Vein {
	if max < min {
		max = min
	}
	n := min
	if max > min {
		n = min + rnd(max-min+1)
	}
	if n < 1 {
		n = 1
	}
	return Vein{Ore: ore, Stock: n, Max: n, Updated: now}
}
