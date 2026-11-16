package main

import (
	"math"
	"os"
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// routeStops lists a routed mob's stops in walking order: its schedule's
// target rooms by start hour (a patrol segment contributes its waypoints),
// then its standalone patrol, each loop closed back to its start.
func routeStops(m *mobs.Mob) []int {
	var stops []int
	addPatrol := func(id string) {
		p := mobs.GetPatrol(id)
		if p == nil {
			return
		}
		for _, w := range p.Waypoints {
			stops = append(stops, w.Room)
		}
		if len(p.Waypoints) > 1 && p.LoopShape != "oneshot" {
			stops = append(stops, p.Waypoints[0].Room)
		}
	}
	if m.ScheduleId != "" {
		if s := mobs.GetSchedule(m.ScheduleId); s != nil {
			segs := append([]mobs.ScheduleSegment(nil), s.Segments...)
			sort.Slice(segs, func(i, j int) bool { return segs[i].Start < segs[j].Start })
			first := len(stops)
			for _, seg := range segs {
				if seg.Activity == "patrol" {
					addPatrol(seg.PatrolId)
					continue
				}
				if seg.TargetRoom > 0 {
					stops = append(stops, seg.TargetRoom)
				}
			}
			if len(stops)-first > 1 {
				stops = append(stops, stops[first])
			}
		}
	}
	if m.PatrolId != "" {
		addPatrol(m.PatrolId)
	}
	return stops
}

// worstStepPrice is the dearest stamina price actions.MovePrice quotes c for a
// step into any room of route, and that room.
func worstStepPrice(c *characters.Character, route map[int]bool) (float64, int) {
	var worst float64
	worstRoom := 0
	for rid := range route {
		r := rooms.LoadRoom(rid)
		if r == nil {
			continue
		}
		if st := actions.MovePrice(c, r); st > worst {
			worst, worstRoom = st, rid
		}
	}
	return worst, worstRoom
}

// TestShippedRoutedMobsCanPayTheirWorstStep measures, instead of inferring,
// whether movement parity 4b can strand a scheduled or patrolling mob: for
// every shipped mob with a route, the dearest single step on the paths between
// its stops, priced by actions.MovePrice for the mob as spawned (its real
// carried load, the real biome movement costs), against its reachable stamina
// pool. A mob whose plain worst step exceeds its whole pool can never walk its
// route and FAILS the test. The hidden price is measured too, by concealing
// the spawned mob and quoting again (so the hidden multiplier, the cap and any
// flight multiplier apply in the real order); a hidden step that exceeds the
// pool is LOGGED, since it bites only a mob that sneaks on that route.
//
// Knobs: unlike most tests, which see Go config defaults, this one calls
// configs.ReloadConfig, so the priced values are the shipped
// _datafiles/config.yaml ones (MovementBaseStaminaCost 0.5,
// MovementMaxStaminaCost 20.0, HiddenMoveStaminaMultiplier 3.0, StaminaBase 5,
// StaminaPerVitality 3, StaminaPerWillpower 1, StaminaPerStrength 0 at the
// time of writing). config.yaml carries skip-worktree; a local edit to it
// changes what this test measures.
//
// Loads the whole world, so it shares the boot smoke test's opt-in.
func TestShippedRoutedMobsCanPayTheirWorstStep(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the shipped-route stamina measurement (~40s)", bootSmokeEnvVar)
	}
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfigForTest(t); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	loadAllDataFiles(false)
	mapper.PreCacheMaps()

	b := configs.GetBalanceConfig()
	t.Logf("knobs as loaded: MovementBaseStaminaCost %v MovementMaxStaminaCost %v HiddenMoveStaminaMultiplier %v StaminaBase %v StaminaPerVitality %v StaminaPerWillpower %v StaminaPerStrength %v FlightMoveStaminaMult %v",
		b.MovementBaseStaminaCost, b.MovementMaxStaminaCost, b.HiddenMoveStaminaMultiplier,
		b.StaminaBase, b.StaminaPerVitality, b.StaminaPerWillpower, b.StaminaPerStrength, b.FlightMoveStaminaMult)

	type row struct {
		name             string
		mobId            int
		pool             int
		worst, hidden    float64
		worstRoom, rooms int
	}
	var rows []row
	for _, spec := range mobs.AllMobTemplates() {
		stops := routeStops(spec)
		if len(stops) < 2 {
			continue
		}
		route := map[int]bool{stops[0]: true}
		for i := 1; i < len(stops); i++ {
			if stops[i-1] == stops[i] {
				continue // two segments in the same room: no step between them
			}
			steps, err := mapper.GetPath(stops[i-1], stops[i])
			if err != nil {
				t.Logf("mob %d %s: no path %d -> %d (%v); the boot validator owns that", spec.MobId, spec.Character.Name, stops[i-1], stops[i], err)
				continue
			}
			for _, s := range steps {
				route[s.RoomId()] = true
			}
		}

		// Stat pools and carried items are randomised per spawn: take the
		// weakest pool and the dearest steps of five spawns.
		pool := math.MaxInt
		var worst, hidden float64
		worstRoom := 0
		for i := 0; i < 5; i++ {
			inst := mobs.NewMobByIdFresh(spec.MobId, stops[0])
			if inst == nil {
				t.Fatalf("mob %d did not spawn", spec.MobId)
			}
			c := &inst.Character
			if p := c.EffectivePoolMax(characters.PoolStamina); p < pool {
				pool = p
			}
			if st, rid := worstStepPrice(c, route); st > worst {
				worst, worstRoom = st, rid
			}

			orig := c.Awareness
			c.Awareness = awareness.NewMachine()
			reason := state.TransitionReason{Trigger: "move_route_stamina_test"}
			if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
				t.Fatalf("mob %d: conceal: %v", spec.MobId, err)
			}
			c.Awareness.ResolveConcealment(true, reason)
			if !c.IsHidden() {
				t.Fatalf("mob %d: concealment did not take, so the hidden price is not measured", spec.MobId)
			}
			if st, _ := worstStepPrice(c, route); st > hidden {
				hidden = st
			}
			c.Awareness = orig
			mobs.DestroyInstance(inst.InstanceId)
		}
		rows = append(rows, row{spec.Character.Name, int(spec.MobId), pool, worst, hidden, worstRoom, len(route)})
	}

	if len(rows) < 10 {
		t.Fatalf("measured only %d routed mobs; the world did not load, so a pass proves nothing", len(rows))
	}
	sort.Slice(rows, func(i, j int) bool {
		return float64(rows[i].pool)-rows[i].hidden < float64(rows[j].pool)-rows[j].hidden
	})
	hiddenOnly := 0
	var maxWorst, maxHidden float64
	minPool := math.MaxInt
	for i, r := range rows {
		maxWorst = math.Max(maxWorst, r.worst)
		maxHidden = math.Max(maxHidden, r.hidden)
		minPool = min(minPool, r.pool)
		if i < 15 {
			t.Logf("%-28s mob %5d pool %4d worst %.2f (room %d) hidden %.2f over %d rooms",
				r.name, r.mobId, r.pool, r.worst, r.worstRoom, r.hidden, r.rooms)
		}
		if r.worst > float64(r.pool) {
			t.Errorf("%s (mob %d) can never pay its worst step: %.2f stamina (room %d) against a %d pool",
				r.name, r.mobId, r.worst, r.worstRoom, r.pool)
		} else if r.hidden > float64(r.pool) {
			hiddenOnly++
			t.Logf("HIDDEN ONLY: %s (mob %d) cannot pay a hidden step of %.2f against %d", r.name, r.mobId, r.hidden, r.pool)
		}
	}
	t.Logf("measured %d routed mobs; dearest plain step %.2f, dearest hidden step %.2f, smallest pool %d; %d cannot pay a hidden step",
		len(rows), maxWorst, maxHidden, minPool, hiddenOnly)
}
