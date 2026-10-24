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

// couleeZone is the new-player area this guard covers.
const couleeZone = "Pothole Coulee"

// TestCouleeNPCsCarryALightAndAreSeenByFace is the content guard behind the
// owner ruling of 2026-10-17: in the Pothole Coulee every NPC and quest giver
// carries a light source, so a brand-new player with no light and no night
// vision can address them by name at any hour (Cleric Hadwen had none). The
// animals and hostile creatures are left in the dark. It also holds #616's
// ruling that a quest giver must be addressable by name.
//
// Who is an NPC, read from data. A mob template is in scope when it is
// authored in the zone or spawned by a room in the zone. It is an NPC when it
// is not hostile and it is any of: non_combatant, a shop, or a mob with a
// dialogue file (dialogue.Load). That takes in every quest giver, shopkeeper
// and townsperson, and Sullen Garrow (combat_passive, but he talks), and
// leaves out the hares, the pronghorn, the practice targets and every
// hostile creature, none of which is non_combatant, a shop or a talker.
//
// For each NPC it asserts:
//
//  1. The template wears a light in the `light` slot whose worn condition
//     gives light, and the item is never_drops, so the coulee is not a free
//     lantern farm (Death_MobLoot skips a NeverDrops item worn or carried).
//  2. Spawned for real (mobs.NewMobByIdFresh, so the lantern reaches the room
//     through the same worn-condition path as a live spawn) and alone in each
//     room it can be awake in, a bare observer sees it at the faces band at
//     every awake hour of midwinter, the equinox and midsummer.
//
// Awake rooms follow the night-trade guard's rules (wanderReach, the
// schedule's segment target) with one difference: an NPC does not have to be
// at home, since a player can address it wherever its schedule takes it. A
// sleeping segment is skipped: a sleeper cannot be asked anything
// (actions.RefuseMobIfAsleep) and its lantern is dark by owner ruling R3
// (behaviors/items/keeper_lantern.yaml). A patrol segment has no one room, so
// it fails loudly rather than be skipped; the coulee has none today.
func TestCouleeNPCsCarryALightAndAreSeenByFace(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// The real shipped config and the day-cycle golden's pinned timing, as in
	// TestEveryShopkeeperCanTradeAtNightWhileAwake.
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

	// Every coulee room's spawns, and every template authored in the zone.
	type placement struct {
		room      *rooms.Room
		maxWander int
	}
	spawnedIn := map[int][]placement{}
	inScope := map[int]bool{}
	roomIds := rooms.GetAllRoomIds()
	sort.Ints(roomIds)
	couleeRooms := 0
	for _, rid := range roomIds {
		r := rooms.LoadRoom(rid)
		if r == nil {
			t.Fatalf("room %d failed to load", rid)
		}
		if r.Zone != couleeZone {
			continue
		}
		couleeRooms++
		seen := map[int]bool{}
		for _, si := range r.SpawnInfo {
			tmpl, ok := templates[si.MobId]
			if !ok || seen[si.MobId] {
				continue
			}
			seen[si.MobId] = true
			inScope[si.MobId] = true
			mw := tmpl.MaxWander
			if si.MaxWander != 0 {
				mw = si.MaxWander
			}
			spawnedIn[si.MobId] = append(spawnedIn[si.MobId], placement{room: r, maxWander: mw})
		}
	}
	for id, tmpl := range templates {
		if tmpl.Zone == couleeZone {
			inScope[id] = true
		}
	}
	if couleeRooms < 100 {
		t.Fatalf("found only %d %s rooms: the walk is not seeing the zone", couleeRooms, couleeZone)
	}

	var npcIds []int
	for id := range inScope {
		tmpl := templates[id]
		if tmpl.LegacyHostile || tmpl.AutoAggro {
			continue
		}
		if tmpl.NonCombatant || tmpl.Character.NonCombatant || tmpl.HasShop() ||
			dialogue.Load(id, tmpl.Zone) != nil {
			npcIds = append(npcIds, id)
		}
	}
	sort.Ints(npcIds)
	// A floor under today's count (32), so a loader that finds nothing cannot
	// pass this guard by checking nothing.
	if len(npcIds) < 30 {
		t.Fatalf("found only %d coulee NPCs: the scope rule is not seeing them", len(npcIds))
	}

	lighting := configs.GetLightingConfig()
	observer := &characters.Character{}
	var unlit, droppable, unplaced, dark []string
	samples, nightSamples := 0, 0

	for _, id := range npcIds {
		tmpl := templates[id]
		label := fmt.Sprintf("mob %d %s", id, tmpl.Character.Name)

		lampId := tmpl.Character.Equipment.Light.ItemId
		spec := items.GetItemSpec(lampId)
		if lampId <= 0 || spec == nil || spec.Type != items.Light {
			unlit = append(unlit, label)
		} else {
			lightGiving := false
			for _, cid := range spec.WornConditionIds {
				if c := conditions.GetConditionSpec(cid); c != nil && c.IsLightSource() {
					lightGiving = true
				}
			}
			if !lightGiving {
				unlit = append(unlit, fmt.Sprintf("%s (item %d gives no light)", label, lampId))
			}
			if !spec.NeverDrops {
				droppable = append(droppable, fmt.Sprintf("%s (item %d %s)", label, lampId, spec.Name))
			}
		}

		places := spawnedIn[id]
		if len(places) == 0 {
			unplaced = append(unplaced, label)
			continue
		}
		sched := mobs.GetSchedule(tmpl.ScheduleId)
		if tmpl.ScheduleId != "" && sched == nil {
			t.Fatalf("%s schedule %q did not load", label, tmpl.ScheduleId)
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
					if gd.Hour24 != hour {
						t.Fatalf("round %d reads hour %d, want %d: the sample mapping is off", round, gd.Hour24, hour)
					}
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
							t.Fatalf("%s patrols at hour %d: this guard has no room to check it in", label, hour)
						}
						r := rooms.LoadRoom(seg.TargetRoom)
						if r == nil {
							t.Fatalf("%s schedule hour %d targets room %d, which does not load", label, hour, seg.TargetRoom)
						}
						here = []*rooms.Room{r}
					}

					// Awake: run the lantern's tree as the item tick would.
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
				dark = append(dark, fmt.Sprintf("%s (spawns in %d) in room %d: below faces at %d awake samples, first %s",
					label, p.room.RoomId, rid, len(darkByRoom[rid]), darkByRoom[rid][0]))
			}
		}
	}

	if nightSamples == 0 {
		t.Fatal("no awake night sample was taken: the guard checked nothing in the dark")
	}
	t.Logf("checked %d coulee NPCs across %d awake samples, %d of them at night", len(npcIds), samples, nightSamples)

	if len(unlit) > 0 {
		t.Errorf("%d coulee NPCs carry no light. Owner ruling 2026-10-17: every coulee NPC and\n"+
			"quest giver wears one in the `light` equipment slot.\n\n%s", len(unlit), strings.Join(unlit, "\n"))
	}
	if len(droppable) > 0 {
		t.Errorf("%d coulee NPCs carry a light that can drop. Give them a never_drops light so\n"+
			"the new-player area is not a free lantern farm.\n\n%s", len(droppable), strings.Join(droppable, "\n"))
	}
	if len(unplaced) > 0 {
		t.Errorf("%d coulee NPCs are spawned by no %s room, so this guard cannot place them:\n\n%s",
			len(unplaced), couleeZone, strings.Join(unplaced, "\n"))
	}
	if len(dark) > 0 {
		t.Errorf("%d coulee NPC rooms are too dark for a new player to make out the NPC's face\n"+
			"while it is awake there, so they cannot address it by name.\n\n%s", len(dark), strings.Join(dark, "\n"))
	}
}
