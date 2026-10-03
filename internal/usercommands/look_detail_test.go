package usercommands

import (
	"context"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/lookdetail"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// detailGen writes one fixed closer look for anyone.
type detailGen struct{ asked int }

func (g *detailGen) Reserve(int) bool { return true }
func (g *detailGen) Generate(context.Context, int, lookdetail.Request) (lookdetail.Result, error) {
	g.asked++
	return lookdetail.Result{Text: `Pigeon feathers drift around its cracked rim, and a coin glints, forever out of reach, at the bottom.`}, nil
}

// A look that finds nothing but names something in the room's description
// is answered with a closer look (internal/lookdetail), through the look
// command itself; anything else still gets "Look at what???", and a real
// room noun still answers as it always did, without asking for one.
func TestLook_ACloserLookAtSomethingTheDescriptionNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430))
	t.Cleanup(util.ResetRoundCountForTest)

	user, room := getTestUserAndRoom(t)
	origDesc, origNouns := room.Description, room.Nouns
	room.Description = `A dry fountain stands in the square under a sky of gulls.`
	room.Nouns = map[string]string{`gulls`: `They wheel and complain.`}
	t.Cleanup(func() { room.Description, room.Nouns = origDesc, origNouns })

	g := &detailGen{}
	lookdetail.SetGenerator(g)
	t.Cleanup(func() { lookdetail.SetGenerator(nil); lookdetail.ResetCacheForTest() })
	t.Cleanup(lookdetail.InlineForTest())
	lookdetail.ResetCacheForTest()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Look(`at the fountain`, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	joined := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, joined, `You look at the <ansi fg="noun">fountain</ansi>:`)
	require.Contains(t, joined, `Pigeon feathers`)
	require.NotContains(t, joined, `Look at what`)

	_, _ = Look(`dragon`, user, room, 0)
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"), `Look at what???`)

	_, _ = Look(`gulls`, user, room, 0)
	joined = strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, joined, `They wheel and complain.`, "an authored room noun still answers itself")
	require.Equal(t, 1, g.asked, "only the fountain was written")
}
