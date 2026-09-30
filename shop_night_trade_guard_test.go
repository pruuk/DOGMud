package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
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
// same worn-condition path as a live spawn), puts it in that room alone, and
// asks the gate for a bare observer.
//
// What "night" and "awake" mean here, and why:
//
//   - Night is every hour on the three sample days when the sun is below the
//     horizon (gametime.SunLight is Absent). That is the engine's own
//     definition of a sunless sky, not a copy of it. Twilight, when a low sun
//     is above the horizon but the room still reads below faces, is not
//     asserted: it is a sliver of a few days a year, and at the sample days
//     it falls inside the sunless hours anyway at midwinter.
//   - The moons are whatever the sample round gives. Every open sky and
//     backstreet room stays below faces even under full moons (35 and 43),
//     so the moon cannot hide a dark shop from this guard.
//   - Awake in the shop means: no schedule (a keeper without one never
//     sleeps), or a schedule segment at that hour whose activity is not
//     sleeping or patrol and whose target room is the shop room (a target of
//     0 means home). A sleeping keeper is exempt, since ShopClosedForSleep
//     refuses before the sight gate ever runs. A keeper away from the shop at
//     that hour is not in it to trade.
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
		mob  *mobs.Mob
		room *rooms.Room
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
			placements = append(placements, placement{mob: tmpl, room: r})
		}
	}
	// A floor well under today's count, so a loader that silently finds no
	// spawns cannot pass this guard by checking nothing.
	if len(placements) < 90 {
		t.Fatalf("found only %d shop placements: the spawn walk is not seeing the shops", len(placements))
	}

	lighting := configs.GetLightingConfig()
	observer := &characters.Character{}
	nightSamples := 0
	var failures []string

	for _, p := range placements {
		sched := mobs.GetSchedule(p.mob.ScheduleId)
		if p.mob.ScheduleId != "" && sched == nil {
			t.Fatalf("mob %d schedule %q did not load", p.mob.MobId, p.mob.ScheduleId)
		}

		inst := mobs.NewMobByIdFresh(p.mob.MobId, p.room.RoomId)
		if inst == nil {
			t.Fatalf("mob %d failed to spawn", p.mob.MobId)
		}
		p.room.AddMob(inst.InstanceId)

		var dark []string
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
				if !math.IsInf(sun, -1) {
					continue // the sun is up: not night
				}
				if !awakeInShop(sched, hour, p.room.RoomId) {
					continue
				}
				nightSamples++
				if actions.ShopSightRefusal(observer, p.room) {
					dark = append(dark, fmt.Sprintf("%s %02d:00 light %d", d.Name, hour, p.room.LightLevel()))
				}
			}
		}

		p.room.RemoveMob(inst.InstanceId)
		mobs.DestroyInstance(inst.InstanceId)

		if len(dark) > 0 {
			failures = append(failures, fmt.Sprintf("mob %d %s in room %d: refused at %d night samples, first %s",
				p.mob.MobId, p.mob.Character.Name, p.room.RoomId, len(dark), dark[0]))
		}
	}

	if nightSamples == 0 {
		t.Fatal("no awake night sample was taken: the guard checked nothing")
	}
	if len(failures) > 0 {
		t.Errorf("%d shopkeepers cannot trade at night while awake in their shop.\n"+
			"A normal human cannot make out the goods below the faces band. Give a street\n"+
			"or stall keeper an Oil Lantern (40038) in the `light` equipment slot, or give\n"+
			"a shop inside a building room light (`skylight: 0.15`, `lamp: 50`).\n\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
}

// awakeInShop reports whether a keeper on this schedule is awake and in the
// shop room at hour. No schedule means always awake and always home.
func awakeInShop(sched *mobs.Schedule, hour int, shopRoom int) bool {
	if sched == nil {
		return true
	}
	seg := sched.CurrentSegment(hour)
	if seg == nil {
		return true // no segment: the spawn override leaves the keeper at home
	}
	if seg.Activity == "sleeping" || seg.Activity == "patrol" {
		return false
	}
	return seg.TargetRoom == 0 || seg.TargetRoom == shopRoom
}
