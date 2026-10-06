package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// nightTradeSampleDays are the days of the year the guard samples, the same
// three the lighting day-cycle golden uses: midwinter has the longest night,
// midsummer the shortest, and the equinox sits between them. Every hour of
// each day is tried, so a keeper who opens only in the evening is still seen.
var nightTradeSampleDays = []struct {
	Name string
	Doy  int
}{{"midwinter", 356}, {"equinox", 81}, {"midsummer", 172}}

// TestEveryShopkeeperCanTradeAtNightWhileAwake is the content guard behind the
// lighting 5b ruling "shopkeepers bring their own light while awake, a shop in
// a separate room gets its own light". The shop sight gate
// (actions.ShopSightRefusal) refuses list, buy and sell below the faces band,
// so a keeper standing awake in an unlit shop at night cannot trade with a
// normal human, who has no night vision and no light of their own.
//
// It loads the REAL world: biomes, rooms, conditions, items, species,
// mutators, patrols, schedules and every mob template. For each template with
// a shop and each room that spawns it, it spawns the real keeper through
// mobs.NewMobByIdFresh (so an equipped light reaches the room through the
// same worn-condition path as a live spawn), puts it alone in every room it
// can be awake in, and asks the gate for a bare observer.
//
// Where a keeper can be awake, and when:
//
//   - A keeper with no schedule never sleeps. It is in its spawn room at
//     every hour, and, if it wanders, in every room its wander can reach
//     (see wanderReach).
//   - A keeper with a schedule is, at each hour, in that hour's segment's
//     target room, unless the segment is sleeping or patrol. The loader
//     rejects a target of 0 outside patrol, so every awake segment names a
//     real room. A sleeping keeper is exempt, since ShopClosedForSleep
//     refuses before the sight gate ever runs. A patrolling keeper is on the
//     road, not keeping shop. The schedule executor pins MaxWander to 0, so
//     a scheduled keeper never wanders.
//
// Every awake hour of the three sample days is asserted, day and night. The
// moons are whatever the sample round gives. Every open sky and backstreet
// room stays below faces even under full moons (35 and 43), so the moon
// cannot hide a dark shop from this guard.
//
// A night sample is an awake hour when the sun is below the horizon
// (gametime.SunLight is Absent), the engine's own definition of a sunless
// sky. Every keeper must be tested at one or more night samples, or be
// listed in nightSampleExempt with the reason it never trades at night, so
// a keeper cannot pass by never being looked at in the dark.
//
// Each keeper is tested alone in the room: another keeper's lantern must not
// cover for one who has none, because the two need not both be present.
func TestEveryShopkeeperCanTradeAtNightWhileAwake(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// The real shipped config, as the lighting goldens read it: a bare test
	// binary would otherwise walk the `default` fixture world and use Go
	// default lighting edges.
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	// Timing pinned as the day-cycle golden pins it. TRAP: the Go default
	// NightHours is 0, a world with no night in it.
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.NightHours = 8
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()

	// These loaders replace package maps with no restore of their own. The
	// seed helpers snapshot what is live now and put it back on cleanup.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))

	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	species.LoadDataFiles()
	mutators.LoadDataFiles()
	mobs.LoadPatrols()
	mobs.LoadSchedules()

	// The templates are read straight from disk and seeded, rather than
	// through mobs.LoadDataFiles, which also builds relationships, facts and
	// conversations this guard has no use for.
	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", dataPath, err)
	}
	t.Cleanup(mobs.SeedMobsForTest(templates, map[int]*mobs.Mob{}))

	originalRound := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCount(originalRound)
		gametime.ClearDateCacheForTest()
		gametime.ClearCelestialMemoForTest()
	})

	// Every room that spawns a shop mob is that mob's shop.
	type placement struct {
		mob       *mobs.Mob
		room      *rooms.Room
		maxWander int // the template's, or the spawn's override when it sets one
	}
	var placements []placement
	roomIds := rooms.GetAllRoomIds()
	if len(roomIds) < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world", len(roomIds))
	}
	sort.Ints(roomIds)
	for _, rid := range roomIds {
		r := rooms.LoadRoom(rid)
		if r == nil {
			t.Fatalf("room %d failed to load", rid)
		}
		seen := map[int]bool{}
		for _, si := range r.SpawnInfo {
			tmpl, ok := templates[si.MobId]
			if !ok || !tmpl.HasShop() || seen[si.MobId] {
				continue
			}
			seen[si.MobId] = true
			// rooms.go copies a spawn's maxwander over the template's only
			// when it is non-zero.
			mw := tmpl.MaxWander
			if si.MaxWander != 0 {
				mw = si.MaxWander
			}
			placements = append(placements, placement{mob: tmpl, room: r, maxWander: mw})
		}
	}
	// A floor well under today's count, so a loader that silently finds no
	// spawns cannot pass this guard by checking nothing.
	if len(placements) < 90 {
		t.Fatalf("found only %d shop placements: the spawn walk is not seeing the shops", len(placements))
	}

	lighting := configs.GetLightingConfig()
	observer := &characters.Character{}
	totalSamples, totalNight := 0, 0
	treedSamples, asleepSamples := 0, 0
	var litAsleep []string // keepers asleep with their lantern still lit
	nightByKeeper := map[int]int{}
	keeperNames := map[int]string{}
	var failures []string

	for _, p := range placements {
		sched := mobs.GetSchedule(p.mob.ScheduleId)
		if p.mob.ScheduleId != "" && sched == nil {
			t.Fatalf("mob %d schedule %q did not load", p.mob.MobId, p.mob.ScheduleId)
		}
		mobId := int(p.mob.MobId)
		keeperNames[mobId] = p.mob.Character.Name
		if _, ok := nightByKeeper[mobId]; !ok {
			nightByKeeper[mobId] = 0
		}

		// The rooms an unscheduled keeper stands in at every hour: home, and
		// whatever its wander reaches.
		var roamRooms []*rooms.Room
		if sched == nil {
			roamRooms = append([]*rooms.Room{p.room}, wanderReach(t, p.room, p.maxWander)...)
		}

		inst := mobs.NewMobByIdFresh(p.mob.MobId, p.room.RoomId)
		if inst == nil {
			t.Fatalf("mob %d failed to spawn", p.mob.MobId)
		}

		// dark collects refusals per room, in the order rooms are first seen.
		dark := map[int][]string{}
		var darkOrder []int
		for _, d := range nightTradeSampleDays {
			for hour := 0; hour < 24; hour++ {
				// 37.5 rounds to the hour; ceil so an odd hour lands just
				// inside the hour rather than just before it.
				round := uint64(d.Doy-1)*900 + uint64(math.Ceil(float64(hour)*37.5))
				util.SetRoundCount(round)
				gd := gametime.GetDate(round)
				if gd.Hour24 != hour {
					t.Fatalf("round %d reads hour %d, want %d: the sample mapping is off", round, gd.Hour24, hour)
				}
				sun := gametime.SunLight(lighting, gd.Day, float64(gd.Hour24)+gd.MinuteFloat/60)
				night := math.IsInf(sun, -1)

				// Lighting 5e (owner rulings R2, R3): the keeper's light-slot
				// tree runs at every sample, with the Sleeping flag set as
				// the schedule would set it. Asleep, its lantern must be
				// dark; awake, the samples below must still pass.
				asleep := false
				if sched != nil {
					if seg := sched.CurrentSegment(hour); seg != nil && seg.Activity == "sleeping" {
						asleep = true
					}
				}
				if asleep {
					inst.Character.Conditions.AddCondition(15, false) // Sleeping
				} else {
					inst.Character.Conditions.RemoveCondition(15)
				}
				if lamp := inst.Character.Equipment.Light; lamp.ItemId > 0 && lamp.HasBehavior() {
					behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{
						UUID: lamp.UUID, ItemId: lamp.ItemId, MobInstanceId: inst.InstanceId, Slot: "light"})
					treedSamples++
					if asleep {
						asleepSamples++
						if inst.Character.EmitsLight() {
							litAsleep = append(litAsleep, fmt.Sprintf("mob %d %s at %s %02d:00",
								p.mob.MobId, p.mob.Character.Name, d.Name, hour))
						}
					}
				}

				here := roamRooms
				if sched != nil {
					here = scheduledRoom(t, sched, hour, mobId)
				}
				for _, r := range here {
					inst.Character.RoomId = r.RoomId
					r.AddMob(inst.InstanceId)
					refused := actions.ShopSightRefusal(observer, r)
					level := r.LightLevel()
					r.RemoveMob(inst.InstanceId)

					totalSamples++
					if night {
						totalNight++
						nightByKeeper[mobId]++
					}
					if refused {
						if _, ok := dark[r.RoomId]; !ok {
							darkOrder = append(darkOrder, r.RoomId)
						}
						dark[r.RoomId] = append(dark[r.RoomId], fmt.Sprintf("%s %02d:00 light %d", d.Name, hour, level))
					}
				}
			}
		}

		mobs.DestroyInstance(inst.InstanceId)

		for _, rid := range darkOrder {
			failures = append(failures, fmt.Sprintf("mob %d %s (spawns in %d) in room %d: refused at %d awake samples, first %s",
				p.mob.MobId, p.mob.Character.Name, p.room.RoomId, rid, len(dark[rid]), dark[rid][0]))
		}
	}

	if totalNight == 0 {
		t.Fatal("no awake night sample was taken: the guard checked nothing")
	}
	// Nine scheduled keepers carry the Oil Lantern and sleep (spec C3).
	if treedSamples == 0 || asleepSamples == 0 {
		t.Fatalf("the keepers' lantern tree ran at %d samples, %d of them asleep: the 5e check saw nothing", treedSamples, asleepSamples)
	}
	t.Logf("checked %d shop placements across %d awake samples, %d of them at night; the lantern tree ran at %d samples, %d asleep",
		len(placements), totalSamples, totalNight, treedSamples, asleepSamples)
	if len(litAsleep) > 0 {
		t.Errorf("%d samples found a keeper asleep with its lantern still lit. The Oil Lantern's tree\n"+
			"(behaviors/items/keeper_lantern.yaml) must put it out while its holder sleeps (owner\n"+
			"ruling R3).\n\n%s", len(litAsleep), strings.Join(litAsleep, "\n"))
	}

	// Every keeper is looked at in the dark at least once, or says why not.
	var unseen []string
	keeperIds := make([]int, 0, len(nightByKeeper))
	for id := range nightByKeeper {
		keeperIds = append(keeperIds, id)
	}
	sort.Ints(keeperIds)
	for _, id := range keeperIds {
		_, exempt := nightSampleExempt[id]
		switch {
		case nightByKeeper[id] == 0 && !exempt:
			unseen = append(unseen, fmt.Sprintf("mob %d %s", id, keeperNames[id]))
		case nightByKeeper[id] > 0 && exempt:
			t.Errorf("mob %d %s is in nightSampleExempt but was tested at %d night samples: drop the exemption",
				id, keeperNames[id], nightByKeeper[id])
		}
	}
	for id := range nightSampleExempt {
		if _, ok := nightByKeeper[id]; !ok {
			t.Errorf("nightSampleExempt names mob %d, which is not a shopkeeper placed in the world", id)
		}
	}
	if len(unseen) > 0 {
		t.Errorf("%d shopkeepers were never tested at night, so this guard proves nothing about them.\n"+
			"Either they are awake somewhere after dark and the guard is not finding it, or\n"+
			"they never trade at night: then list them in nightSampleExempt with the reason.\n\n%s",
			len(unseen), strings.Join(unseen, "\n"))
	}

	if len(failures) > 0 {
		t.Errorf("%d shopkeeper rooms are too dark to trade in while the keeper is awake there.\n"+
			"A normal human cannot make out the goods below the faces band. Give a street\n"+
			"or stall keeper an Oil Lantern (40038) in the `light` equipment slot, give a\n"+
			"shop inside a building room light (`skylight: 0.15`, `lamp: 50`), or pin a\n"+
			"keeper who has no reason to roam to its counter with `maxwander: 0`.\n\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
}

// nightSampleExempt lists shopkeepers who are never awake anywhere after dark
// on the sample days, with the reason. The guard fails if a listed keeper is
// in fact tested at night, so an entry cannot outlive its reason.
var nightSampleExempt = map[int]string{}

// scheduledRoom returns the room a scheduled keeper is awake in at hour, or
// nothing when the hour's segment is sleeping or patrol.
func scheduledRoom(t *testing.T, sched *mobs.Schedule, hour int, mobId int) []*rooms.Room {
	t.Helper()
	seg := sched.CurrentSegment(hour)
	if seg == nil {
		// The loader rejects a schedule that does not cover all 24 hours.
		t.Fatalf("mob %d schedule %q has no segment at hour %d", mobId, sched.Id, hour)
	}
	if seg.Activity == "sleeping" || seg.Activity == "patrol" {
		return nil
	}
	r := rooms.LoadRoom(seg.TargetRoom)
	if r == nil {
		t.Fatalf("mob %d schedule %q hour %d targets room %d, which does not load", mobId, sched.Id, hour, seg.TargetRoom)
	}
	return []*rooms.Room{r}
}

// wanderReach returns the rooms other than home that a wandering keeper can
// stand in. The engine sends a mob home once WanderCount > MaxWander, and
// counts a step only as it is taken, so a mob with MaxWander N can take N+1
// steps before it turns back. Wander stays inside the home zone. MaxWander -1
// never turns back, so the whole zone is in reach. This walks every exit,
// including the secret and locked ones the engine's random pick skips, so it
// errs toward checking more rooms, not fewer.
func wanderReach(t *testing.T, home *rooms.Room, maxWander int) []*rooms.Room {
	t.Helper()
	if maxWander == 0 {
		return nil
	}
	depth := maxWander + 1
	if maxWander < 0 {
		depth = math.MaxInt
	}
	seen := map[int]bool{home.RoomId: true}
	var out []*rooms.Room
	frontier := []*rooms.Room{home}
	for step := 0; step < depth && len(frontier) > 0; step++ {
		var next []*rooms.Room
		for _, r := range frontier {
			exitIds := make([]int, 0, len(r.Exits))
			for _, ex := range r.Exits {
				exitIds = append(exitIds, ex.RoomId)
			}
			sort.Ints(exitIds)
			for _, id := range exitIds {
				if seen[id] {
					continue
				}
				seen[id] = true
				nr := rooms.LoadRoom(id)
				if nr == nil {
					t.Fatalf("room %d exit leads to room %d, which does not load", r.RoomId, id)
				}
				if nr.Zone != home.Zone {
					continue
				}
				out = append(out, nr)
				next = append(next, nr)
			}
		}
		frontier = next
	}
	return out
}
