package mining

import "testing"

const sample = `
ores:
  - {id: copper, name: copper, item: 1, tier: 1, min_pick: 1}
  - {id: gold, name: gold, item: 2, tier: 4, min_pick: 3}
gems:
  - {item: 10, weight: 5}
  - {item: 11, weight: 1, min_pick: 3}
biomes:
  cave: [{ore: copper, weight: 1}]
zones:
  Gold Hills: [{ore: gold, weight: 1}]
rooms:
  77: [{ore: gold, weight: 1}]
exclude_zones: [Drains]
`

func install(t *testing.T) {
	t.Helper()
	d, err := Parse([]byte(sample), World{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	Install(d)
	t.Cleanup(func() { Install(nil) })
}

func TestPoolPrecedence(t *testing.T) {
	install(t)
	if p := Pool(77, `Drains`, `interior`); len(p) != 1 || p[0].Ore != `gold` {
		t.Errorf("a room pool wins over everything, got %+v", p)
	}
	if p := Pool(1, `Drains`, `cave`); p != nil {
		t.Errorf("an excluded zone has no ore, got %+v", p)
	}
	if p := Pool(1, `Gold Hills`, `interior`); p != nil {
		t.Errorf("a biome with no pool has no ore even in a zone with one, got %+v", p)
	}
	if p := Pool(1, `Gold Hills`, `cave`); len(p) != 1 || p[0].Ore != `gold` {
		t.Errorf("the zone pool replaces the biome pool, got %+v", p)
	}
	if p := Pool(1, `Elsewhere`, `cave`); len(p) != 1 || p[0].Ore != `copper` {
		t.Errorf("the biome pool, got %+v", p)
	}
}

func TestParseRefusesBadData(t *testing.T) {
	bad := []string{
		"ores:\n  - {id: x, name: x, item: 1, tier: 5, min_pick: 1}\n",
		"ores:\n  - {id: x, name: x, item: 1, tier: 1, min_pick: 0}\n",
		"ores:\n  - {id: x, name: x, item: 1, tier: 1, min_pick: 1}\nbiomes:\n  cave: [{ore: y, weight: 1}]\n",
		"ores:\n  - {id: x, name: x, item: 1, tier: 1, min_pick: 1}\nzones:\n  Z: [{ore: x, weight: 1}]\nexclude_zones: [Z]\n",
		"gems:\n  - {item: 1, weight: 0}\n",
	}
	for i, raw := range bad {
		if _, err := Parse([]byte(raw), World{}); err == nil {
			t.Errorf("case %d should be refused", i)
		}
	}
}

func TestPickGemRespectsPick(t *testing.T) {
	install(t)
	for i := 0; i < 50; i++ {
		g, ok := PickGem(1, func(n int) int { return n - 1 })
		if !ok || g.ItemId != 10 {
			t.Fatalf("a crude pick never takes the steel-only gem, got %+v", g)
		}
	}
	if g, ok := PickGem(3, func(n int) int { return n - 1 }); !ok || g.ItemId != 11 {
		t.Errorf("a steel pick can take it, got %+v", g)
	}
}

type store map[string]any

func (s store) GetLongTermData(k string) any    { return s[k] }
func (s store) SetLongTermData(k string, v any) { s[k] = v }

func TestVeinDigAndRefill(t *testing.T) {
	s := store{}
	v := NewVein(`copper`, 2, 2, 100, func(int) int { return 0 })
	SaveVein(s, v)
	got, ok := LoadVein(s)
	if !ok || got.Stock != 2 || got.Ore != `copper` {
		t.Fatalf("round trip, got %+v", got)
	}
	got.Dig(110)
	got.Dig(120)
	if got.Stock != 0 || !got.WorkedOut || got.Dig(130) {
		t.Fatalf("worked out after two loads, got %+v", got)
	}
	if came := got.Refill(110+50, 25); !came || got.Stock != 2 {
		t.Errorf("two refills later it is full and comes back, got %+v came=%v", got, came)
	}
}
