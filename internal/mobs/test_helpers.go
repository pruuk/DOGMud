package mobs

import "github.com/GoMudEngine/GoMud/internal/state/combatphase"

// SeedMobsForTest replaces all global mob registries with the supplied test
// data and returns a cleanup function that restores the originals.
// Intended for cross-package integration tests (hooks, commands).
func SeedMobsForTest(specs map[int]*Mob, instances map[int]*Mob) func() {
	origMobs := mobs
	origAllMobNames := allMobNames
	origMobInstances := mobInstances
	origMobNameCache := mobNameCache
	origRecentlyDied := recentlyDied
	origInstanceCounter := instanceCounter

	mobs = specs
	mobInstances = instances

	// Rebuild derived caches from specs
	names := make([]string, 0, len(specs))
	cache := make(map[MobId]string, len(specs))
	for id, m := range specs {
		names = append(names, m.Character.Name)
		cache[MobId(id)] = m.Character.Name
	}
	allMobNames = names
	mobNameCache = cache

	// U12c-2: seeded fixtures are bare characters.Character{Name:...} literals,
	// so their state machines are nil -- a shape no production character has,
	// because every load path Validates. Survivable while combat state lived on
	// the Aggro struct, which needs no machine; NOT survivable now the round
	// budget lives on CombatPhase, where a nil machine silently drops writes.
	//
	// Only the combat phase machine is created, deliberately. Calling the full
	// Validate() here also seeds skills, stats and equipment, which reshapes
	// these fixtures and breaks the behaviour-tree tests that depend on their
	// bare shape.
	for _, m := range specs {
		if m != nil && m.Character.CombatPhase == nil {
			m.Character.CombatPhase = combatphase.NewMachine()
		}
	}
	for _, m := range instances {
		if m != nil && m.Character.CombatPhase == nil {
			m.Character.CombatPhase = combatphase.NewMachine()
		}
	}

	recentlyDied = map[int]int{}
	instanceCounter = 200

	return func() {
		mobs = origMobs
		allMobNames = origAllMobNames
		mobInstances = origMobInstances
		mobNameCache = origMobNameCache
		recentlyDied = origRecentlyDied
		instanceCounter = origInstanceCounter
	}
}

// SetInstanceForTest registers or removes a mob instance for tests.
// Pass nil to remove. Not safe for concurrent use.
func SetInstanceForTest(instId int, mob *Mob) {
	mobInstancesMu.Lock()
	defer mobInstancesMu.Unlock()
	if mob == nil {
		delete(mobInstances, instId)
		return
	}
	mobInstances[instId] = mob
}
