package rifts

// roll.go: the pure decisions, kept free of the world so they can be tested.
// rng(n) returns a uniform int in [0, n); production passes util.Rand.

// RollPool picks a pool for one door by weight, among the pools allowed at
// depth and not excluded. It returns "" only if nothing can be rolled.
func RollPool(p *Profile, depth int, exclude map[Pool]bool, rng func(int) int) Pool {
	total := 0
	for _, pool := range AllPools {
		if w := poolWeight(p, pool, depth, exclude); w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return ``
	}
	pick := rng(total)
	for _, pool := range AllPools {
		w := poolWeight(p, pool, depth, exclude)
		if w <= 0 {
			continue
		}
		if pick < w {
			return pool
		}
		pick -= w
	}
	return ``
}

func poolWeight(p *Profile, pool Pool, depth int, exclude map[Pool]bool) int {
	if exclude[pool] || depth < p.MinDepth[pool] {
		return 0
	}
	return p.Weights[pool]
}

// PlanDoors rolls the destination pool for each of a room's n doors. The
// room is of pool from and sits at depth; its doors lead to depth+1.
//
// Rules, which together guarantee a room never strands a party without a key:
//   - at most one door leads to an exit (F) room, and that door is locked;
//   - since every non-exit room has at least two doors, at least one door is
//     always unlocked;
//   - a boss room never opens straight onto another boss room;
//   - an exit room does not lead to another exit room.
func PlanDoors(p *Profile, from Pool, depth int, n int, rng func(int) int) []Pool {
	out := make([]Pool, 0, n)
	exclude := map[Pool]bool{}
	if from == PoolBoss {
		exclude[PoolBoss] = true
	}
	if from == PoolExit {
		exclude[PoolExit] = true
	}
	for i := 0; i < n; i++ {
		pool := RollPool(p, depth+1, exclude, rng)
		if pool == `` {
			pool = PoolPassage
		}
		if pool == PoolExit {
			exclude[PoolExit] = true
		}
		out = append(out, pool)
	}
	// Belt and braces: a room with doors must keep one that needs no key.
	open := 0
	for _, pool := range out {
		if pool != PoolExit {
			open++
		}
	}
	if open == 0 && len(out) > 0 {
		out[0] = PoolPassage
	}
	return out
}

// RollRange returns an int in [r.Min, r.Max].
func RollRange(r IntRange, rng func(int) int) int {
	if r.Max <= r.Min {
		return r.Min
	}
	return r.Min + rng(r.Max-r.Min+1)
}

// StatPool is the stat pool a mob of tier gets at depth.
func StatPool(p *Profile, tier string, depth int) int {
	base := p.StatPools.Trash
	switch tier {
	case `elite`:
		base = p.StatPools.Elite
	case `boss`:
		base = p.StatPools.Boss
	case `hunter`:
		base = p.StatPools.Hunter
	}
	if base < 1 {
		base = 1
	}
	return base + depth*p.StatPoolPerDepth
}

// ChooseDoors picks n of the template's doors, keeping their authored order.
func ChooseDoors(doors []DoorSpec, n int, rng func(int) int) []DoorSpec {
	if n >= len(doors) {
		return append([]DoorSpec(nil), doors...)
	}
	idx := make([]int, len(doors))
	for i := range idx {
		idx[i] = i
	}
	for i := len(idx) - 1; i > 0; i-- {
		j := rng(i + 1)
		idx[i], idx[j] = idx[j], idx[i]
	}
	keep := map[int]bool{}
	for _, i := range idx[:n] {
		keep[i] = true
	}
	out := make([]DoorSpec, 0, n)
	for i, d := range doors {
		if keep[i] {
			out = append(out, d)
		}
	}
	return out
}
