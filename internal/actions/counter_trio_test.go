package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const counterTrioInfraredBuffId = 7501

var counterTrioTag = regexp.MustCompile(`<[^>]*>`)

func counterTrioPlain(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(counterTrioTag.ReplaceAllString(l, "")))
	}
	return out
}

// counterTrioRoom seeds 7511 Aliceia (the counterer), 7512 Bobrick (countered)
// and 7513 Ordel (watching) in one room of the given biome.
func counterTrioRoom(t *testing.T, biome string) *rooms.Room {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", DarkArea: true},
		"city":    {BiomeId: "city", LitArea: true},
		"default": {BiomeId: "default", LitArea: true},
	}))
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		counterTrioInfraredBuffId: {BuffId: counterTrioInfraredBuffId, Name: "Test Heat Eyes", Flags: []buffs.Flag{buffs.InfraredVision}},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7511: users.NewTestUser(7511, "aliceia", "Aliceia", 97511),
		7512: users.NewTestUser(7512, "bobrick", "Bobrick", 97512),
		7513: users.NewTestUser(7513, "ordel", "Ordel", 97513),
	}))
	room := &rooms.Room{RoomId: 7510, Biome: biome}
	for _, id := range []int{7511, 7512, 7513} {
		room.AddPlayer(id)
		events.DrainQueuedMessagesForTest(id)
	}
	return room
}

func counterTrioResult() combat.CounterResult {
	return combat.CounterResult{
		Countered:       true,
		CountererUserId: 7511,
		CountererName:   "Aliceia",
		CounteredName:   "Bobrick",
		DefenderMsg:     "You strike back at Bobrick!",
		AttackerMsg:     "Aliceia turns your attack into a strike of their own!",
		RoomMsg:         "Aliceia strikes back at Bobrick!",
	}
}

func TestSendCounterTrio_InTheDarkNobodyIsNamed(t *testing.T) {
	room := counterTrioRoom(t, "cave")
	SendCounterTrio(room, counterTrioResult(), users.GetByUserId(7512), 7512)

	assert.Equal(t, []string{"You strike back at something!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7511)))
	assert.Equal(t, []string{"Something turns your attack into a strike of their own!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7512)))
	assert.Empty(t, events.DrainQueuedMessagesForTest(7513),
		"an observer who cannot see gets no counter line")
}

func TestSendCounterTrio_InfraredObserverReadsFigures(t *testing.T) {
	room := counterTrioRoom(t, "cave")
	require.True(t, users.GetByUserId(7513).Character.Buffs.AddBuff(counterTrioInfraredBuffId, true))
	SendCounterTrio(room, counterTrioResult(), users.GetByUserId(7512), 7512)

	assert.Equal(t, []string{"A figure strikes back at a figure!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7513)))
}

func TestSendCounterTrio_LitRoomIsUnchanged(t *testing.T) {
	room := counterTrioRoom(t, "city")
	SendCounterTrio(room, counterTrioResult(), users.GetByUserId(7512), 7512)

	assert.Equal(t, []string{"You strike back at Bobrick!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7511)))
	assert.Equal(t, []string{"Aliceia turns your attack into a strike of their own!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7512)),
		"the countered party gets their own line and not the room line as well")
	assert.Equal(t, []string{"Aliceia strikes back at Bobrick!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7513)))
}

// DispatchCounterMessages is how the 23 special-move wrappers speak a counter.
func TestDispatchCounterMessages_InTheDarkGoesThroughTheSeam(t *testing.T) {
	room := counterTrioRoom(t, "cave")
	DispatchCounterMessages(NewUserActorInRoom(users.GetByUserId(7512), room), counterTrioResult())

	assert.Equal(t, []string{"You strike back at something!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7511)))
	assert.Equal(t, []string{"Something turns your attack into a strike of their own!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7512)))
}

func TestDispatchCounterMessages_AMobCounteredPartyIsHiddenFromTheCounterer(t *testing.T) {
	room := counterTrioRoom(t, "cave")
	m := newSearchTestMob(7521, "Skeleton", 7510)
	mobs.SetInstanceForTest(7521, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(7521, nil) })
	room.AddMob(7521)

	res := counterTrioResult()
	res.CounteredName = "Skeleton"
	res.DefenderMsg = "You strike back at Skeleton!"
	res.AttackerMsg = "Aliceia turns your attack into a strike of their own!"
	res.RoomMsg = "Aliceia strikes back at Skeleton!"
	DispatchCounterMessages(NewMobActorInRoom(m, room), res)

	assert.Equal(t, []string{"You strike back at something!"},
		counterTrioPlain(events.DrainQueuedMessagesForTest(7511)))
	assert.Empty(t, events.DrainQueuedMessagesForTest(7513))
}

func TestSendCounterTrio_NotCounteredSendsNothing(t *testing.T) {
	room := counterTrioRoom(t, "city")
	res := counterTrioResult()
	res.Countered = false
	SendCounterTrio(room, res, users.GetByUserId(7512), 7512)

	for _, id := range []int{7511, 7512, 7513} {
		assert.Empty(t, events.DrainQueuedMessagesForTest(id))
	}
}
