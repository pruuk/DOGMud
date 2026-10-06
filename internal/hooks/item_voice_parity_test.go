package hooks

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
// moved into item trees, and the tree path must reproduce it: the same
// lines on the same rounds to the bearer, under one seeded random source
// (util.SetRandForTest) that both the chance roll and the line pick draw
// from. Two scenarios, one per shipped voiced item, each 200 rounds:
// idle, a hunger warning (the Blackrazor only), combat taunts, hungry
// feeding, a kill, idle again.
//
// Only the bearer's own lines are recorded: the room line changed on
// purpose (spec X17, it is heard and hides the bearer's name).
var updateVoiceParity = flag.Bool("update-voices", false, "rewrite testdata/item_voice_parity.golden")

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

// voiceParityScenarios are the shipped voiced items and the slot each is
// worn in.
var voiceParityScenarios = []struct {
	name   string
	itemId int
	slot   string
}{
	{"blackrazor", 40183, "weapon"},
	{"aegis", 40185, "offhand"},
}

// loadVoiceParityWorld loads the shipped conditions, items and voices once
// for the test, and seeds one room.
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
	t.Cleanup(itemvoices.SeedVoicesForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	itemvoices.LoadDataFiles()

	room := rooms.NewRoom("voiceparity")
	room.RoomId = voiceParityRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{voiceParityRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return room
}

// newParityBearer seeds a fresh user 1 in room, wearing a fresh instance of
// itemId in slot.
func newParityBearer(t *testing.T, room *rooms.Room, itemId int, slot string) *users.UserRecord {
	t.Helper()
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

// voiceParityRound runs one round of the voice machinery for the bearer:
// the per-round tick, then a kill when kill is set.
func voiceParityRound(u *users.UserRecord, room *rooms.Room, kill bool) {
	pinnacleUserTick(u, room)
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

func TestItemVoiceParity(t *testing.T) {
	room := loadVoiceParityWorld(t)
	var b strings.Builder
	for _, s := range voiceParityScenarios {
		fmt.Fprintf(&b, "## %s (%d, %s)\n", s.name, s.itemId, s.slot)
		b.WriteString(runVoiceParity(t, room, s.itemId, s.slot))
	}
	got := b.String()

	path := filepath.Join("testdata", "item_voice_parity.golden")
	if *updateVoiceParity {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (record it with -update-voices): %v", path, err)
	}
	if got != string(want) {
		t.Errorf("the voice record moved.\n--- want\n%s\n--- got\n%s", want, got)
	}
}
