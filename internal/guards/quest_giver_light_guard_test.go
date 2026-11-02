package guards

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestEveryQuestGiverIsSeenByFace holds the owner ruling on #616 (2026-10-17):
// a quest giver must be addressable by name, so its room is lit to the faces
// band at every hour and/or it carries a light. A quest giver is any mob whose
// dialogue file has an offer or hand-in topic.
//
// Talk refuses below full sight (conversation.Talk, seesByName), so a giver
// below the faces band cannot be spoken to at all, even by a player who holds
// the quest. Each giver is spawned for real, so a worn lantern reaches the room
// through the live path, and is tried alone in every room it can be awake in
// (wanderReach, or its schedule's segment target) at every hour of midwinter,
// the equinox and midsummer. A sleeping segment is skipped: a sleeper cannot be
// asked anything and its lantern is dark. A giver on patrol has no one room to
// check, so a patrol fails loudly until the guard learns the route.
//
// The fix for a failure is a never_drops light in the `light` slot, the Keeper's
// Lantern (20134), or room light (`skylight`, `lamp`) for a room that should be
// lit for everyone in it.
func TestEveryQuestGiverIsSeenByFace(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.NightHours = 8
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()

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

	type placement struct {
		room      *rooms.Room
		maxWander int
	}
	spawnedIn := map[int][]placement{}
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
			if !ok || seen[si.MobId] {
				continue
			}
			seen[si.MobId] = true
			mw := tmpl.MaxWander
			if si.MaxWander != 0 {
				mw = si.MaxWander
			}
			spawnedIn[si.MobId] = append(spawnedIn[si.MobId], placement{room: r, maxWander: mw})
		}
	}

	// Today's count is 60; this floor stops a loader that finds nothing from
	// passing by checking nothing.
	var giverIds []int
	for id, tmpl := range templates {
		df := dialogue.Load(id, tmpl.Zone)
		if df == nil {
			continue
		}
		for i := range df.Topics {
			if k := df.Topics[i].Kind; k == dialogue.KindOffer || k == dialogue.KindHandIn {
				giverIds = append(giverIds, id)
				break
			}
		}
	}
	sort.Ints(giverIds)
	if len(giverIds) < 50 {
		t.Fatalf("found only %d quest givers: the scope rule is not seeing them", len(giverIds))
	}

	lighting := configs.GetLightingConfig()
	observer := &characters.Character{}
	var unplaced, patrolling, dark []string
	samples, nightSamples := 0, 0

	for _, id := range giverIds {
		tmpl := templates[id]
		label := fmt.Sprintf("mob %d %s [%s]", id, tmpl.Character.Name, tmpl.Zone)
		places := spawnedIn[id]
		if len(places) == 0 {
			unplaced = append(unplaced, label)
			continue
		}
		sched := mobs.GetSchedule(tmpl.ScheduleId)
		if tmpl.ScheduleId != "" && sched == nil {
			t.Fatalf("%s schedule %q did not load", label, tmpl.ScheduleId)
		}
		if tmpl.PatrolId != "" {
			patrolling = append(patrolling, label+" patrol "+tmpl.PatrolId)
		}

		for _, p := range places {
			var roam []*rooms.Room
			if sched == nil {
				roam = append([]*rooms.Room{p.room}, wanderReach(t, p.room, p.maxWander)...)
			}
			inst := mobs.NewMobByIdFresh(tmpl.MobId, p.room.RoomId)
			if inst == nil {
				t.Fatalf("%s failed to spawn", label)
			}
			darkByRoom := map[int][]string{}
			var darkOrder []int

			for _, d := range nightTradeSampleDays {
				for hour := 0; hour < 24; hour++ {
					round := uint64(d.Doy-1)*900 + uint64(math.Ceil(float64(hour)*37.5))
					util.SetRoundCount(round)
					gd := gametime.GetDate(round)
					night := math.IsInf(gametime.SunLight(lighting, gd.Day, float64(gd.Hour24)+gd.MinuteFloat/60), -1)

					here := roam
					if sched != nil {
						seg := sched.CurrentSegment(hour)
						if seg == nil {
							t.Fatalf("%s schedule %q has no segment at hour %d", label, sched.Id, hour)
						}
						if seg.Activity == "sleeping" {
							continue
						}
						if seg.Activity == "patrol" {
							patrolling = append(patrolling, fmt.Sprintf("%s schedule %s hour %d", label, sched.Id, hour))
							continue
						}
						r := rooms.LoadRoom(seg.TargetRoom)
						if r == nil {
							t.Fatalf("%s schedule hour %d targets room %d, which does not load", label, hour, seg.TargetRoom)
						}
						here = []*rooms.Room{r}
					}

					inst.Character.Conditions.RemoveCondition(15) // Sleeping
					if lamp := inst.Character.Equipment.Light; lamp.ItemId > 0 && lamp.HasBehavior() {
						behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{
							UUID: lamp.UUID, ItemId: lamp.ItemId, MobInstanceId: inst.InstanceId, Slot: "light"})
					}

					for _, r := range here {
						inst.Character.RoomId = r.RoomId
						r.AddMob(inst.InstanceId)
						band := messaging.LightBand(observer, r)
						level := r.LightLevel()
						r.RemoveMob(inst.InstanceId)
						samples++
						if night {
							nightSamples++
						}
						if band < messaging.BandFaces {
							if _, ok := darkByRoom[r.RoomId]; !ok {
								darkOrder = append(darkOrder, r.RoomId)
							}
							darkByRoom[r.RoomId] = append(darkByRoom[r.RoomId],
								fmt.Sprintf("%s %02d:00 light %d (%s)", d.Name, hour, level, band))
						}
					}
				}
			}
			mobs.DestroyInstance(inst.InstanceId)
			for _, rid := range darkOrder {
				dark = append(dark, fmt.Sprintf("%s (spawns in %d, wander %d, sched %q) in room %d: dark at %d samples, first %s",
					label, p.room.RoomId, p.maxWander, tmpl.ScheduleId, rid, len(darkByRoom[rid]), darkByRoom[rid][0]))
			}
		}
	}

	if nightSamples == 0 {
		t.Fatal("no awake night sample was taken: the guard checked nothing in the dark")
	}
	t.Logf("checked %d quest givers across %d awake samples, %d of them at night", len(giverIds), samples, nightSamples)
	if len(unplaced) > 0 {
		t.Errorf("%d quest givers are spawned by no room, so this guard cannot place them:\n\n%s",
			len(unplaced), strings.Join(unplaced, "\n"))
	}
	if len(patrolling) > 0 {
		t.Errorf("%d quest givers patrol, and this guard has no room to check them in:\n\n%s",
			len(patrolling), strings.Join(patrolling, "\n"))
	}
	if len(dark) > 0 {
		t.Errorf("%d quest giver rooms are too dark to address the giver by name (owner ruling\n"+
			"on #616). Give the giver a Keeper's Lantern (20134) in the `light` equipment\n"+
			"slot, or light the room.\n\n%s", len(dark), strings.Join(dark, "\n"))
	}
}
