package behaviortree

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/scavenger"
)

const (
	scavMobId      = 9820
	scavInstanceId = 8400
	scavHome       = 20
	scavNorth      = 21
	scavFar        = 22
)

var scavT0 = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// scavengerFixture seeds a three-room lit street (20 - 21 - 22), a scavenger
// profile over it, and the scavenger standing at home. The pathfinder seam
// walks the line: from 20 north to 21, from 21 north to 22 or south to 20.
func scavengerFixture(t *testing.T) (*mobs.Mob, map[int]*rooms.Room) {
	t.Helper()

	lamp := rooms.LampPtr(90)
	rs := map[int]*rooms.Room{
		scavHome:  {RoomId: scavHome, Zone: `test`, Lamp: lamp},
		scavNorth: {RoomId: scavNorth, Zone: `test`, Lamp: lamp},
		scavFar:   {RoomId: scavFar, Zone: `test`, Lamp: lamp},
	}
	t.Cleanup(rooms.SeedRoomsForTest(rs, map[string]*rooms.ZoneConfig{}))

	p := &scavenger.Profile{
		MobId:       scavMobId,
		City:        `Test Town`,
		HomeRoom:    scavHome,
		PickupLines: []string{`{actor} bags the {item}.`},
		GoldLines:   []string{`{actor} pockets {gold}.`},
		ResetLines:  []string{`{actor} carts the day's haul away.`},
	}
	p.SetPoolForTest([]int{scavHome, scavNorth, scavFar})
	t.Cleanup(scavenger.SetProfilesForTest(p))

	spec := &mobs.Mob{MobId: scavMobId}
	spec.Character.Name = `Old Mags`
	spec.Character.Gold = 3

	m := &mobs.Mob{MobId: scavMobId, InstanceId: scavInstanceId, HomeRoomId: scavHome, ActivityLevel: 0}
	m.Character.Name = `Old Mags`
	m.Character.RoomId = scavHome
	m.Character.Gold = 3
	m.Character.Conditions = conditions.New()
	m.Character.Stats.Strength.ValueAdj = 100
	m.Character.Stats.Perception.ValueAdj = 100
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{scavMobId: spec}, map[int]*mobs.Mob{scavInstanceId: m}))

	prevNow, prevRand, prevStep := scavengerNow, scavengerRandn, scavengerNextStep
	scavengerNow = func() time.Time { return scavT0 }
	scavengerRandn = func(int) int { return 0 }
	scavengerNextStep = func(from, to int) (string, int, bool) {
		switch {
		case to > from:
			return `north`, from + 1, true
		case to < from:
			return `south`, from - 1, true
		}
		return ``, 0, false
	}
	t.Cleanup(func() { scavengerNow, scavengerRandn, scavengerNextStep = prevNow, prevRand, prevStep })

	// The first tick ever only records the day; settle it so tests start
	// from a scavenger already on its rounds.
	m.Character.SetMiscData(scavengerDayKey, int(rooms.DecayDay(scavT0)))
	queuedCmds(scavInstanceId)
	return m, rs
}

func scavTick(t *testing.T) Result {
	t.Helper()
	return LookupAction(`scavenger_step`)(nil, &EvalContext{InstanceId: scavInstanceId})
}

func namedItem(id int, name string) items.Item {
	return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: name}}
}

func carries(m *mobs.Mob, id int) bool {
	for _, it := range m.Character.Items {
		if it.ItemId == id {
			return true
		}
	}
	return false
}

// A mob that is not a scavenger gets Failure, so its own tree carries on.
func TestScavengerStep_NotAScavenger(t *testing.T) {
	scavengerFixture(t)
	other := &mobs.Mob{MobId: 1, InstanceId: 8401}
	mobs.SetInstanceForTest(8401, other)
	t.Cleanup(func() { mobs.SetInstanceForTest(8401, nil) })

	if got := LookupAction(`scavenger_step`)(nil, &EvalContext{InstanceId: 8401}); got != Failure {
		t.Fatalf("got %v, want Failure", got)
	}
}

// Litter is picked up one item a tick, before any step; what the world put
// there is left alone.
func TestScavengerStep_PicksUpLitterOneAtATime(t *testing.T) {
	m, rs := scavengerFixture(t)
	home := rs[scavHome]
	home.SpawnInfo = []rooms.SpawnInfo{{ItemId: 96302}} // a respawning prop
	home.Items = []items.Item{namedItem(96300, `bent spoon`), namedItem(96301, `torn glove`), namedItem(96302, `posy`)}

	if scavTick(t) != Success || !carries(m, 96300) || len(home.Items) != 2 {
		t.Fatalf("first tick: carried=%v floor=%d", carries(m, 96300), len(home.Items))
	}
	if cmds := queuedCmds(scavInstanceId); len(cmds) != 0 {
		t.Fatalf("stepped while there was litter: %v", cmds)
	}
	scavTick(t)
	if !carries(m, 96301) || len(home.Items) != 1 || home.Items[0].ItemId != 96302 {
		t.Fatalf("second tick: %+v", home.Items)
	}

	// Only the prop is left: the scavenger moves on.
	scavTick(t)
	if carries(m, 96302) {
		t.Fatal("picked up an authored spawn item")
	}
	if cmds := queuedCmds(scavInstanceId); !contains(cmds, `north`) {
		t.Fatalf("expected a step once the floor was clear, got %v", cmds)
	}
}

// Floor gold goes in its purse, where a pickpocket can find it.
func TestScavengerStep_PicksUpGold(t *testing.T) {
	m, rs := scavengerFixture(t)
	rs[scavHome].Gold = 12
	scavTick(t)
	if m.Character.Gold != 15 || rs[scavHome].Gold != 0 {
		t.Fatalf("gold carried %d, floor %d", m.Character.Gold, rs[scavHome].Gold)
	}
}

// In the dark it finds nothing, like anyone else, and keeps walking.
func TestScavengerStep_DarkRoomPicksUpNothing(t *testing.T) {
	m, rs := scavengerFixture(t)
	rs[scavHome].SkyLight, rs[scavHome].Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
	rs[scavHome].Items = []items.Item{namedItem(96300, `bent spoon`)}
	scavTick(t)
	if carries(m, 96300) {
		t.Fatal("picked something up in the dark")
	}
}

// One step, then it lingers ScavengerStepMin..Max seconds before the next.
func TestScavengerStep_StepsThenLingers(t *testing.T) {
	m, _ := scavengerFixture(t)
	// randn n-1: the far room as target, and the longest linger (150 s).
	scavengerRandn = func(n int) int { return n - 1 }

	scavTick(t)
	w := scavengerWalk(m)
	if w.Target != scavFar || w.Expect != scavNorth || w.LastFrom != scavHome {
		t.Fatalf("walk after the first step: %+v", w)
	}
	if !contains(queuedCmds(scavInstanceId), `north`) {
		t.Fatal("no step issued")
	}
	if want := scavT0.Add(150 * time.Second); !w.NextMoveAt.Equal(want) {
		t.Fatalf("next move at %v, want %v", w.NextMoveAt, want)
	}

	// Arrive, and a tick inside the linger issues nothing.
	m.Character.RoomId = scavNorth
	scavengerNow = func() time.Time { return scavT0.Add(149 * time.Second) }
	scavTick(t)
	if cmds := queuedCmds(scavInstanceId); len(cmds) != 0 {
		t.Fatalf("stepped while lingering: %v", cmds)
	}

	// Past it, the next step goes on toward the same target.
	scavengerNow = func() time.Time { return scavT0.Add(151 * time.Second) }
	scavTick(t)
	if w.Fails != 0 || w.Target != scavFar || w.Expect != scavFar || w.LastFrom != scavNorth {
		t.Fatalf("second step: %+v", w)
	}
	if cmds := queuedCmds(scavInstanceId); len(cmds) != 1 || cmds[0] != `north` {
		t.Fatalf("expected exactly one step north, got %v", cmds)
	}
}

// A step that goes nowhere twice (a locked door) gives up the target.
func TestScavengerStep_GivesUpOnABlockedTarget(t *testing.T) {
	m, _ := scavengerFixture(t)
	scavengerRandn = func(n int) int { return n - 1 } // target the far room
	scavTick(t)
	w := scavengerWalk(m)
	if w.Target != scavFar {
		t.Fatalf("target %d, want %d", w.Target, scavFar)
	}

	// It never moves: the first repeat counts one failure and tries again.
	scavengerNow = func() time.Time { return scavT0.Add(time.Hour) }
	scavTick(t)
	if w.Fails != 1 || w.Target != scavFar {
		t.Fatalf("after one failed step: %+v", w)
	}

	// The second failure drops the target; the new pick (the near room this
	// time) is stepped toward at once.
	scavengerRandn = func(int) int { return 0 }
	scavengerNow = func() time.Time { return scavT0.Add(2 * time.Hour) }
	scavTick(t)
	if w.Fails != 0 || w.Target != scavNorth || w.Expect != scavNorth {
		t.Fatalf("after two failed steps it should have re-targeted: %+v", w)
	}
}

// At the day boundary the haul goes: items and any gold above its purse.
func TestScavengerStep_DailyResetClearsTheHaul(t *testing.T) {
	m, _ := scavengerFixture(t)
	m.Character.Items = []items.Item{namedItem(96300, `bent spoon`), namedItem(96301, `torn glove`)}
	m.Character.Gold = 40

	// Same day: nothing happens to the haul.
	scavTick(t)
	if len(m.Character.Items) != 2 || m.Character.Gold != 40 {
		t.Fatal("reset before the day turned")
	}

	scavengerNow = func() time.Time { return scavT0.Add(24 * time.Hour) }
	scavTick(t)
	if len(m.Character.Items) != 0 || m.Character.Gold != 3 {
		t.Fatalf("after the day turned: %d items, %d gold (purse 3)", len(m.Character.Items), m.Character.Gold)
	}
	if day, _ := miscDay(m.Character.GetMiscData(scavengerDayKey)); day != rooms.DecayDay(scavT0)+1 {
		t.Fatalf("reset day %d", day)
	}
}

// A scavenger seen for the first time only records the day: a restart does
// not throw away a haul it may have been saved with.
func TestScavengerStep_FirstTickOnlyRecordsTheDay(t *testing.T) {
	m, _ := scavengerFixture(t)
	m.Character.SetMiscData(scavengerDayKey, nil)
	m.Character.Items = []items.Item{namedItem(96300, `bent spoon`)}
	scavTick(t)
	if len(m.Character.Items) != 1 {
		t.Fatal("cleared on the first tick")
	}
	if _, known := miscDay(m.Character.GetMiscData(scavengerDayKey)); !known {
		t.Fatal("day not recorded")
	}
}

// The pickup line is in its own voice and names the item.
func TestScavengerLine_Renders(t *testing.T) {
	got := scavenger.Line([]string{`{actor} bags the {item}.`}, func(int) int { return 0 }, `Old Mags`, `bent spoon`, 0)
	if !strings.Contains(got, `Old Mags`) || !strings.Contains(got, `bent spoon`) {
		t.Fatal(got)
	}
}
