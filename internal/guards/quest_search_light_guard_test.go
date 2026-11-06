package guards

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// searchTorchItemId is the Torch every player can buy: the bare minimum light a
// player is expected to bring to a dark search site.
const searchTorchItemId = 20096

// TestEveryQuestSearchRoomIsSeenByATorch holds the owner ruling of 2026-11-02:
// a `search` where the player sees nothing at all moves no quest step
// (usercommands.Search). So every room a shipped quest asks the player to
// search must be one a bare character carrying only a torch (item 20096) can
// see in, at every hour of midwinter, the equinox and midsummer. A room that
// fails is a step the player cannot finish with the cheapest light in the game.
//
// The scope is every shipped `event: command, command: search` trigger with a
// room, plus every search_at trigger with a room (quest 76's
// spots, which the same rule covers). The observer is a real user in the room
// wearing the torch's condition, so the room's light composes exactly as live.
// The fix for a failure is room light (`lamp`, `skylight`) or a hint that
// tells the player to bring something brighter, decided by the owner.
func TestEveryQuestSearchRoomIsSeenByATorch(t *testing.T) {
	setupSearchLightWorld(t)

	torchItem := items.GetItemSpec(searchTorchItemId)
	if torchItem == nil || len(torchItem.WornConditionIds) == 0 {
		t.Fatalf("the torch (item %d) has no worn condition: the guard cannot light the observer", searchTorchItemId)
	}

	searchRooms := map[int][]string{}
	for _, q := range quests.GetAllQuests() {
		for _, tr := range q.Triggers {
			if tr.Room <= 0 {
				continue
			}
			isCmd := tr.Event == "command" && strings.EqualFold(tr.Command, "search")
			isSpot := tr.Event == quests.EventSearchAt
			if isCmd || isSpot {
				searchRooms[tr.Room] = append(searchRooms[tr.Room], fmt.Sprintf("quest %d", q.QuestId))
			}
		}
	}
	// Today's count is above 15 rooms; the floor stops a loader that finds
	// nothing from passing by checking nothing.
	if len(searchRooms) < 10 {
		t.Fatalf("found only %d quest search rooms: the quest loader is not seeing them", len(searchRooms))
	}
	// The Coulee and the other tier 2 and 3 quests named in the owner ruling.
	for _, q := range []int{49, 58, 76, 73, 74} {
		found := false
		for _, who := range searchRooms {
			for _, w := range who {
				found = found || w == fmt.Sprintf("quest %d", q)
			}
		}
		if !found {
			t.Errorf("quest %d has no search trigger with a room: the guard is not covering it", q)
		}
	}

	roomIds := make([]int, 0, len(searchRooms))
	for id := range searchRooms {
		roomIds = append(roomIds, id)
	}
	sort.Ints(roomIds)

	var failures []string
	samples := 0
	for _, rid := range roomIds {
		fails := torchSightFailures(t, rid, torchItem.WornConditionIds)
		samples += len(sampleHours())
		if len(fails) > 0 {
			sort.Strings(searchRooms[rid])
			failures = append(failures, fmt.Sprintf("room %d (%s): no sight with a torch at %d hours, first %s",
				rid, strings.Join(uniqueStrings(searchRooms[rid]), ", "), len(fails), fails[0]))
		}
	}
	t.Logf("checked %d search rooms at %d samples", len(roomIds), samples)
	if len(failures) > 0 {
		t.Errorf("%d quest search rooms leave a torch bearer unable to see (a search there moves no step):\n\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
}

// The guard above can fail: a bare character in an unlit room reads SightNone,
// and the same room with a torch does not, so the check tells them apart.
func TestQuestSearchLightGuard_CanFail(t *testing.T) {
	setupSearchLightWorld(t)
	torchItem := items.GetItemSpec(searchTorchItemId)
	if torchItem == nil || len(torchItem.WornConditionIds) == 0 {
		t.Fatalf("the torch (item %d) has no worn condition", searchTorchItemId)
	}

	ids := rooms.GetAllRoomIds()
	sort.Ints(ids)
	r := rooms.LoadRoom(ids[0])
	if r == nil {
		t.Fatalf("room %d did not load", ids[0])
	}
	oldSky, oldLamp := r.SkyLight, r.Lamp
	t.Cleanup(func() { r.SkyLight, r.Lamp = oldSky, oldLamp })
	r.SkyLight, r.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)

	bare := bearerIn(t, r, nil)
	if got := messaging.ParticipantSight(bare, r); got != messaging.SightNone {
		t.Fatalf("a bare character in an unlit room reads %v, want SightNone: the guard could never fail", got)
	}
	if fails := torchSightFailures(t, r.RoomId, nil); len(fails) == 0 {
		t.Fatal("torchSightFailures passed a bearer with no light in an unlit room")
	}
	if fails := torchSightFailures(t, r.RoomId, torchItem.WornConditionIds); len(fails) != 0 {
		t.Fatalf("a torch bearer in an unlit room was still unable to see: %v", fails)
	}
}

func setupSearchLightWorld(t *testing.T) {
	t.Helper()
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
	quests.LoadDataFiles()

	originalRound := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCount(originalRound)
		gametime.ClearDateCacheForTest()
		gametime.ClearCelestialMemoForTest()
	})
	if n := len(rooms.GetAllRoomIds()); n < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world", n)
	}
}

type sampleHour struct {
	name string
	doy  int
	hour int
}

func sampleHours() []sampleHour {
	var out []sampleHour
	for _, d := range nightTradeSampleDays {
		for hour := 0; hour < 24; hour++ {
			out = append(out, sampleHour{d.Name, d.Doy, hour})
		}
	}
	return out
}

// bearerIn puts a fresh user in r carrying the given worn conditions and
// returns their character. The user leaves the room when the test ends.
func bearerIn(t *testing.T, r *rooms.Room, wornConditions []int) *characters.Character {
	t.Helper()
	u := users.NewTestUser(9501, "searchlightprobe", "Search Light Probe", 0)
	u.Character.RoomId = r.RoomId
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{u.UserId: u})
	r.AddPlayer(u.UserId)
	t.Cleanup(func() {
		r.RemovePlayer(u.UserId)
		restore()
	})
	for _, id := range wornConditions {
		if err := u.Character.AddCondition(id, true); err != nil {
			t.Fatalf("giving the probe condition %d: %v", id, err)
		}
	}
	return u.Character
}

// torchSightFailures lists the sample hours at which a bearer of the given
// conditions, alone in room rid, is at SightNone.
func torchSightFailures(t *testing.T, rid int, wornConditions []int) []string {
	t.Helper()
	r := rooms.LoadRoom(rid)
	if r == nil {
		t.Fatalf("room %d did not load", rid)
	}
	observer := bearerIn(t, r, wornConditions)
	lighting := configs.GetLightingConfig()
	var fails []string
	for _, s := range sampleHours() {
		round := uint64(s.doy-1)*900 + uint64(math.Ceil(float64(s.hour)*37.5))
		util.SetRoundCount(round)
		gd := gametime.GetDate(round)
		if gd.Hour24 != s.hour {
			t.Fatalf("round %d reads hour %d, want %d: the sample mapping is off", round, gd.Hour24, s.hour)
		}
		if messaging.ParticipantSight(observer, r) == messaging.SightNone {
			night := math.IsInf(gametime.SunLight(lighting, gd.Day, float64(gd.Hour24)+gd.MinuteFloat/60), -1)
			fails = append(fails, fmt.Sprintf("%s %02d:00 light %d night=%v", s.name, s.hour, r.LightLevel(), night))
		}
	}
	return fails
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
