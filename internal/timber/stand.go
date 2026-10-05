package timber

// Stand is the timber standing in one room.
type Stand struct {
	Species string // species id
	Stock   int    // trees that can still be felled
	Max     int    // a full stand
	Updated uint64 // round of the last change (felling or regrowth)
	Felled  bool   // felled to the last tree; re-roll the species on regrowth
}

// Long-term room data keys. Values are plain strings and ints so they survive
// a YAML round trip through the room's instance save.
const (
	keySpecies = `timber.species`
	keyStock   = `timber.stock`
	keyMax     = `timber.max`
	keyUpdated = `timber.updated`
	keyFelled  = `timber.felled`
)

// Store is where a stand lives: rooms.Room satisfies it.
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

// LoadStand reads a room's stand. ok is false when the room has none yet.
func LoadStand(s Store) (Stand, bool) {
	sp, _ := s.GetLongTermData(keySpecies).(string)
	if sp == `` {
		return Stand{}, false
	}
	st := Stand{Species: sp}
	st.Stock, _ = asInt(s.GetLongTermData(keyStock))
	st.Max, _ = asInt(s.GetLongTermData(keyMax))
	u, _ := asInt(s.GetLongTermData(keyUpdated))
	st.Updated = uint64(u)
	f, _ := asInt(s.GetLongTermData(keyFelled))
	st.Felled = f != 0
	if st.Max < 1 {
		st.Max = 1
	}
	return st, true
}

// SaveStand writes a stand back to its room.
func SaveStand(s Store, st Stand) {
	s.SetLongTermData(keySpecies, st.Species)
	s.SetLongTermData(keyStock, st.Stock)
	s.SetLongTermData(keyMax, st.Max)
	s.SetLongTermData(keyUpdated, int(st.Updated))
	felled := 0
	if st.Felled {
		felled = 1
	}
	s.SetLongTermData(keyFelled, felled)
}

// Regrow adds one tree per regrowRounds elapsed since Updated, up to Max. It
// reports whether the stand came back from being felled to nothing, which is
// when the caller re-rolls its species.
func (st *Stand) Regrow(now uint64, regrowRounds int) (cameBack bool) {
	if regrowRounds < 1 || now <= st.Updated || st.Stock >= st.Max {
		if st.Stock >= st.Max {
			st.Updated = now
		}
		return false
	}
	grown := int((now - st.Updated) / uint64(regrowRounds))
	if grown < 1 {
		return false
	}
	wasEmpty := st.Stock == 0
	st.Stock += grown
	if st.Stock > st.Max {
		st.Stock = st.Max
	}
	st.Updated += uint64(grown) * uint64(regrowRounds)
	if wasEmpty && st.Felled {
		st.Felled = false
		return true
	}
	return false
}

// RoundsToNextTree is how long until the stand grows its next tree, or 0
// when it is full.
func (st Stand) RoundsToNextTree(now uint64, regrowRounds int) uint64 {
	if st.Stock >= st.Max || regrowRounds < 1 {
		return 0
	}
	next := st.Updated + uint64(regrowRounds)
	if next <= now {
		return 0
	}
	return next - now
}

// Fell takes one tree. It reports false when there was none.
func (st *Stand) Fell(now uint64) bool {
	if st.Stock < 1 {
		return false
	}
	if st.Stock >= st.Max {
		st.Updated = now // regrowth is measured from the first felling
	}
	st.Stock--
	if st.Stock == 0 {
		st.Felled = true
	}
	return true
}

// PickSpecies draws a species from pool. Each neighbouring room's species that
// is in the pool gets a bonus of a quarter of the pool's total weight, so
// groves cluster without any species ever being excluded. rnd(n) returns
// 0..n-1. Returns "" for an empty pool.
func PickSpecies(pool []Weighted, neighbours []string, rnd func(int) int) string {
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
			if n == e.Species {
				w += bonus
			}
		}
		weights[i] = w
		sum += w
	}
	pick := rnd(sum)
	for i, w := range weights {
		if pick < w {
			return pool[i].Species
		}
		pick -= w
	}
	return pool[len(pool)-1].Species
}

// NewStand seeds a full stand of species with min..max trees.
func NewStand(species string, min, max int, now uint64, rnd func(int) int) Stand {
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
	return Stand{Species: species, Stock: n, Max: n, Updated: now}
}
