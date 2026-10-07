package ferry

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #428 review: the factor's boarding and landing lines went out on plain
// Room.SendText with its bare name, so even a viewer who sees nothing read
// "<Name> trundles ashore". They describe something seen, so they now take
// the room's sight path with the factor's name hidden: a figure at shapes,
// nothing at all to a viewer who sees nothing, the name at clear sight.

const (
	factorSightDarkRoom = 9801
	factorSightLitRoom  = 9802
	factorSightInfraId  = 9811
)

var factorSightTag = regexp.MustCompile(`<[^>]*>`)

func factorSightLines(userId int) []string {
	raw := events.DrainQueuedMessagesForTest(userId)
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		out = append(out, strings.TrimSpace(factorSightTag.ReplaceAllString(l, "")))
	}
	return out
}

// factorSightScene: a dark room holding a heat-sighted viewer (1) and a viewer
// with no special sight (2), and a lamp-lit room holding a viewer (3).
func factorSightScene(t *testing.T) *mobs.Mob {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		factorSightInfraId: {
			ConditionId: factorSightInfraId,
			Name:        "Test Heat Eyes",
			Flags:       []conditions.Flag{conditions.InfraredVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}},
		},
	}))
	dark := &rooms.Room{RoomId: factorSightDarkRoom, Zone: "FerryZone", Biome: "cave"}
	lit := &rooms.Room{RoomId: factorSightLitRoom, Zone: "FerryZone", Biome: "cave", Lamp: rooms.LampPtr(90)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{factorSightDarkRoom: dark, factorSightLitRoom: lit},
		map[string]*rooms.ZoneConfig{"FerryZone": {Name: "FerryZone", RoomId: factorSightDarkRoom,
			RoomIds: map[int]struct{}{factorSightDarkRoom: {}, factorSightLitRoom: {}}}}))

	heat := users.NewTestUser(1, "heat", "Heateye", 1001)
	plain := users.NewTestUser(2, "plain", "Plainsight", 1002)
	clear := users.NewTestUser(3, "clear", "Clearsight", 1003)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: heat, 2: plain, 3: clear}))
	require.True(t, heat.Character.Conditions.AddCondition(factorSightInfraId, true))
	for _, u := range []*users.UserRecord{heat, plain} {
		u.Character.RoomId = factorSightDarkRoom
		dark.AddPlayer(u.UserId)
	}
	clear.Character.RoomId = factorSightLitRoom
	lit.AddPlayer(clear.UserId)
	for id := 1; id <= 3; id++ {
		events.DrainQueuedMessagesForTest(id)
	}

	factor := &mobs.Mob{InstanceId: 98801}
	factor.Character.Name = "Tally Factor"
	factor.Character.RoomId = factorSightDarkRoom
	dark.AddMob(factor.InstanceId)
	return factor
}

func TestTransportFactor_LinesFollowEachViewersSight(t *testing.T) {
	factor := factorSightScene(t)

	transportFactor(factor, factorSightDarkRoom, factorSightLitRoom,
		`Tally Factor leads a laden handcart up the gangplank.`,
		`Tally Factor wheels a laden handcart aboard and settles it against the rail.`)

	require.Equal(t, []string{"A figure leads a laden handcart up the gangplank."}, factorSightLines(1),
		"a heat-sighted viewer reads a figure")
	require.Empty(t, factorSightLines(2), "a viewer who sees nothing reads nothing")
	require.Equal(t, []string{"Tally Factor wheels a laden handcart aboard and settles it against the rail."}, factorSightLines(3),
		"a clear-sighted viewer reads the name")

	transportFactor(factor, factorSightLitRoom, factorSightDarkRoom,
		`Tally Factor wheels a laden handcart down the gangplank.`,
		`Tally Factor trundles ashore with a laden handcart, tally-slip in hand.`)

	require.Equal(t, []string{"Tally Factor wheels a laden handcart down the gangplank."}, factorSightLines(3))
	require.Equal(t, []string{"A figure trundles ashore with a laden handcart, tally-slip in hand."}, factorSightLines(1))
	require.Empty(t, factorSightLines(2))
}
