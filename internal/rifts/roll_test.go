package rifts

import (
	"math/rand"
	"testing"
)

func testProfile() *Profile {
	return &Profile{
		Id: `t`, Name: `the Test`, Zone: `Rift Test`, Biome: `cave`, ExitName: `breach`, PortalExit: `rift`,
		Weights:   map[Pool]int{PoolPassage: 50, PoolFeature: 20, PoolPuzzle: 10, PoolMonster: 10, PoolBoss: 5, PoolExit: 5},
		MinDepth:  map[Pool]int{},
		StartPool: PoolFeature,
		StatPools: TierStatPools{Trash: 1, Elite: 2, Boss: 4},
	}
}

func seeded(seed int64) func(int) int {
	r := rand.New(rand.NewSource(seed))
	return r.Intn
}

// The roll follows the authored weights.
func TestRollPool_FollowsWeights(t *testing.T) {
	p := testProfile()
	rng := seeded(1)
	counts := map[Pool]int{}
	const n = 200000
	for i := 0; i < n; i++ {
		counts[RollPool(p, 10, nil, rng)]++
	}
	for pool, w := range p.Weights {
		got := float64(counts[pool]) / n * 100
		if d := got - float64(w); d > 1 || d < -1 {
			t.Errorf(`pool %s: got %.2f%%, want about %d%%`, pool, got, w)
		}
	}
}

// A pool is never rolled shallower than its min_depth, and excluded pools
// never come up.
func TestRollPool_MinDepthAndExclude(t *testing.T) {
	p := testProfile()
	p.MinDepth = map[Pool]int{PoolBoss: 3, PoolExit: 4}
	rng := seeded(2)
	for i := 0; i < 20000; i++ {
		got := RollPool(p, 2, map[Pool]bool{PoolPassage: true}, rng)
		if got == PoolBoss || got == PoolExit || got == PoolPassage {
			t.Fatalf(`rolled %s at depth 2 with passages excluded`, got)
		}
	}
}

// The no-soft-lock rules hold for every room, whatever the dice say.
func TestPlanDoors_Rules(t *testing.T) {
	p := testProfile()
	p.Weights = map[Pool]int{PoolExit: 90, PoolBoss: 10} // hostile odds
	rng := seeded(3)
	for i := 0; i < 20000; i++ {
		from := AllPools[i%len(AllPools)]
		n := 2 + i%3
		doors := PlanDoors(p, from, 5, n, rng)
		if len(doors) != n {
			t.Fatalf(`got %d doors, want %d`, len(doors), n)
		}
		exits, open := 0, 0
		for _, d := range doors {
			if d == PoolExit {
				exits++
			} else {
				open++
			}
			if from == PoolBoss && d == PoolBoss {
				t.Fatalf(`boss room led to a boss room: %v`, doors)
			}
			if from == PoolExit && d == PoolExit {
				t.Fatalf(`exit room led to an exit room: %v`, doors)
			}
		}
		if exits > 1 {
			t.Fatalf(`more than one exit door: %v`, doors)
		}
		if open == 0 {
			t.Fatalf(`no door without a key: %v`, doors)
		}
	}
}

func TestStatPool_ScalesWithDepth(t *testing.T) {
	p := testProfile()
	p.StatPoolPerDepth = 3
	if got := StatPool(p, `trash`, 0); got != 1 {
		t.Errorf(`trash at depth 0 = %d, want 1`, got)
	}
	if got := StatPool(p, `boss`, 7); got != 4+21 {
		t.Errorf(`boss at depth 7 = %d, want 25`, got)
	}
	p.StatPools.Hunter = 50
	if got := StatPool(p, `hunter`, 2); got != 56 {
		t.Errorf(`hunter at depth 2 = %d, want 56`, got)
	}
}

func TestChooseDoors_KeepsOrderAndCount(t *testing.T) {
	doors := []DoorSpec{{Exit: `a`}, {Exit: `b`}, {Exit: `c`}, {Exit: `d`}}
	rng := seeded(4)
	for i := 0; i < 1000; i++ {
		got := ChooseDoors(doors, 2, rng)
		if len(got) != 2 {
			t.Fatalf(`got %d doors`, len(got))
		}
		if got[0].Exit >= got[1].Exit {
			t.Fatalf(`order not kept: %v`, got)
		}
	}
}
