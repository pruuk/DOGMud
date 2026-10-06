package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/itemvoices"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The voice parity record (item behaviour slice 2, spec "Slice 2" gates).
// testdata/item_voice_parity.golden was recorded on the Pinnacle tick's
// voice path (tickVoices, tryEmitVoice, emitVoiceLine) BEFORE the voices
// moved into item trees, and is frozen: the tree path must reproduce it,
// the same lines on the same rounds to the bearer, under one seeded random
// source (util.SetRandForTest) that the chance roll and the line pick both
// draw from. Two scenarios, one per shipped voiced item, each 200 rounds:
// idle, a hunger warning (the Blackrazor only), combat taunts, hungry
// feeding, a kill, idle again.
//
// Only the bearer's own lines are recorded: the room line changed on
// purpose (spec X17, it is heard and hides the bearer's name).

const (
	voiceParityUserId   = 1
	voiceParityRoomId   = 9971
	voiceParityFirst    = 1000 // the first scenario round
	voiceParityRounds   = 200
	voiceParityFightAt  = 60  // combat from this round offset
	voiceParityFightEnd = 110 // to this one
	voiceParityKillAt   = 165 // the kill: both items' chatter cooldowns are open here under the seed
	voiceParitySeed     = 5150
)

// voiceParityScenarios are the shipped voiced items, the slot each is worn
// in, and the round offset from which the tree path may differ from the
// record on purpose (-1: never). The Aegis's kill lines were dead on the
// Pinnacle path, which voiced only the weapon's; owner ruling S3 gives every
// worn voiced item the kill, so from the kill on the Aegis speaks and draws
// where the record has nothing.
var voiceParityScenarios = []struct {
	name     string
	itemId   int
	slot     string
	divertAt int
}{
	{"blackrazor", 40183, "weapon", -1},
	{"aegis", 40185, "offhand", voiceParityKillAt},
}

// loadVoiceParityWorld loads the shipped conditions and items once for the
// test, points the engine at the shipped item trees, and seeds one room.
func loadVoiceParityWorld(t *testing.T) *rooms.Room {
	t.Helper()
	// The overlay first: AddOverlayOverrides rebuilds the live config, so
	// it would drop a data path set before it.
	setPinnacleEnabled(t, true)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses a 0 trigger count
	configs.SetConfigForTest(t, cfg)
	// items.LoadDataFiles also replaces the combat and defence message
	// stores; snapshot them too, or later tests in the package read the
	// shipped pools instead of their seeds.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(items.SeedAttackMessagesForTest(nil))
	t.Cleanup(items.SeedDefenseMessagesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	for _, name := range []string{"blackrazor", "aegis"} {
		behaviortree.GetEngine().EvictItemTree(name)
		t.Cleanup(func() { behaviortree.GetEngine().EvictItemTree(name) })
	}

	room := rooms.NewRoom("voiceparity")
	room.RoomId = voiceParityRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{voiceParityRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return room
}

// newParityBearer seeds a fresh user 1 in room, wearing a fresh instance of
// itemId in slot, with fresh item state and listener caps.
func newParityBearer(t *testing.T, room *rooms.Room, itemId int, slot string) *users.UserRecord {
	t.Helper()
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	t.Cleanup(behaviortree.ResetItemListenerCapsForTest())
	u := users.NewTestUser(voiceParityUserId, "bearer", "Bearer", 0)
	u.Character.RoomId = room.RoomId
	u.Character.HealthMax.Value = 100000 // the hunger drain never reaches its floor
	u.Character.Health = 100000
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{voiceParityUserId: u}))
	room.AddPlayer(voiceParityUserId)

	it := items.New(itemId)
	switch slot {
	case "weapon":
		u.Character.Equipment.Weapon = it
	case "offhand":
		u.Character.Equipment.Offhand = it
	default:
		t.Fatalf("newParityBearer: unknown slot %q", slot)
	}
	return u
}

// voiceParityRound runs one round of the voice machinery for the bearer in
// the server's order: the Pinnacle tick (hunger and its feeding line), the
// item tick's visit to the bearer (ambient lines), then a kill when kill
// is set.
func voiceParityRound(u *users.UserRecord, room *rooms.Room, kill bool) {
	pinnacleUserTick(u, room)
	tickHeldItems(u.Character, u.UserId, 0)
	if kill {
		MobDeathItemProcs(events.MobDeath{MobId: 1, PlayerDamage: map[int]int{u.UserId: 1}})
	}
}

// runVoiceParity plays one scenario and returns its record: one line per
// round that sent the bearer anything.
func runVoiceParity(t *testing.T, room *rooms.Room, itemId int, slot string) string {
	t.Helper()
	u := newParityBearer(t, room, itemId, slot)
	c := u.Character

	restoreRand := util.SetRandForTest(voiceParitySeed)
	defer restoreRand()
	defer util.ResetRoundCountForTest()
	_ = events.DrainQueuedMessagesForTest(u.UserId)

	var b strings.Builder
	for i := 0; i < voiceParityRounds; i++ {
		util.SetRoundCountForTest(uint64(voiceParityFirst + i))
		switch i {
		case voiceParityFightAt:
			c.SetAggro(0, 424242, characters.DefaultAttack)
		case voiceParityFightEnd:
			c.EndAggro()
		}
		voiceParityRound(u, room, i == voiceParityKillAt)
		msgs := events.DrainQueuedMessagesForTest(u.UserId)
		for j := range msgs {
			msgs[j] = strings.TrimRight(msgs[j], "\n")
		}
		if len(msgs) > 0 {
			fmt.Fprintf(&b, "%d: %s\n", i, strings.Join(msgs, " | "))
		}
	}
	return b.String()
}

// voiceRecordSections splits the record into its scenarios' lines.
func voiceRecordSections(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "item_voice_parity.golden"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	name := ``
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "## ") {
			name = strings.Fields(line)[1]
			continue
		}
		out[name] = append(out[name], line)
	}
	return out
}

// recordLinesBefore keeps the record lines whose round offset is below
// cutoff (all of them when cutoff is negative).
func recordLinesBefore(lines []string, cutoff int) []string {
	if cutoff < 0 {
		return lines
	}
	var kept []string
	for _, line := range lines {
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		round, err := strconv.Atoi(line[:colon])
		if err == nil && round < cutoff {
			kept = append(kept, line)
		}
	}
	return kept
}

func TestItemVoiceParity(t *testing.T) {
	room := loadVoiceParityWorld(t)
	record := voiceRecordSections(t)
	for _, s := range voiceParityScenarios {
		got := strings.Split(strings.TrimRight(runVoiceParity(t, room, s.itemId, s.slot), "\n"), "\n")
		want := record[s.name]
		if len(want) == 0 {
			t.Fatalf("the record has no %s scenario", s.name)
		}
		if g, w := recordLinesBefore(got, s.divertAt), recordLinesBefore(want, s.divertAt); !reflect.DeepEqual(g, w) {
			t.Errorf("%s: the tree path moved off the record.\n--- want\n%s\n--- got\n%s",
				s.name, strings.Join(w, "\n"), strings.Join(g, "\n"))
		}
		if s.divertAt >= 0 {
			// Ruling S3: where the record is silent, the shield now
			// speaks one of its kill lines.
			prefix := strconv.Itoa(s.divertAt) + ": "
			found := false
			for _, line := range got {
				if strings.HasPrefix(line, prefix) && strings.Contains(line, "Aegis of Mockery</ansi> says") {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no kill line on the kill round %d", s.name, s.divertAt)
			}
		}
	}
}

// The tree pools carry the voice files' lines byte for byte (spec Rule
// 18), checked while both exist. The retire task deletes this test with
// internal/itemvoices.
func TestItemTreePoolsMatchTheVoiceFiles(t *testing.T) {
	loadVoiceParityWorld(t)
	t.Cleanup(itemvoices.SeedVoicesForTest(nil))
	itemvoices.LoadDataFiles()
	for _, id := range itemvoices.AllVoiceIds() {
		if err := behaviortree.GetEngine().LoadItemTree(id, behaviortree.GetItemTreePath(id)); err != nil {
			t.Fatalf("item tree %s: %v", id, err)
		}
		tree := behaviortree.GetEngine().GetItemVoice(id)
		if tree == nil {
			t.Fatalf("item tree %s has no speech", id)
		}
		if !reflect.DeepEqual(tree.Speech, itemvoices.GetVoice(id).Lines) {
			t.Errorf("item tree %s's pools differ from itemvoices/%s.yaml:\n tree:  %q\n voice: %q",
				id, id, tree.Speech, itemvoices.GetVoice(id).Lines)
		}
	}
}
