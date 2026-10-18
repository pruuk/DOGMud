package rooms

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestNewbieQuestTargetsRespawnInTwoRealMinutes pins the 2026-10-17 newbie
// quest audit fix (S7). These spawns are the targets of newbie quests, and
// with no respawnrate the engine default (15 real minutes, the spawninfo
// default in rooms.go) left the next player waiting a quarter of an hour:
//
//   - the canyon scout (mob 9165, the Sounding Floor 5356), the single
//     target of quests 51 and 59;
//   - the marsh leeches (mob 9130, Reed Beds 5269 and Black Pool 5273);
//   - the pronghorns (mob 9139, Thornbrake 5289 and Carrion Flat 5291),
//     hunted in quests 42 and 56.
//
// The rate must PARSE as two real minutes, not only read that way.
// gametime.AddPeriod never fails: an unknown unit falls through to a count of
// rounds, which is what makes a "1h 30m" style string silently wrong. So this
// asserts the parsed length in seconds.
func TestNewbieQuestTargetsRespawnInTwoRealMinutes(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "rooms", "pothole_coulee")

	roundSeconds := uint64(configs.GetTimingConfig().RoundSeconds)
	require.Greater(t, roundSeconds, uint64(0))

	for _, tc := range []struct {
		roomId int
		mobId  int
	}{
		{5356, 9165}, // canyon scout
		{5269, 9130}, // leech
		{5273, 9130}, // leech
		{5289, 9139}, // pronghorn
		{5291, 9139}, // pronghorn
	} {
		raw, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(tc.roomId)+".yaml"))
		require.NoError(t, err)

		var room struct {
			RoomId    int `yaml:"roomid"`
			SpawnInfo []struct {
				MobId       int    `yaml:"mobid"`
				RespawnRate string `yaml:"respawnrate"`
			} `yaml:"spawninfo"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &room))
		require.Equal(t, tc.roomId, room.RoomId)

		found := false
		for _, s := range room.SpawnInfo {
			if s.MobId != tc.mobId {
				continue
			}
			found = true
			require.NotEmpty(t, s.RespawnRate, "room %d mob %d: an empty respawnrate is the 15 real minute default", tc.roomId, tc.mobId)
			require.True(t, RealPeriodOK(s.RespawnRate), "room %d: respawnrate %q is not a period the engine understands", tc.roomId, s.RespawnRate)
			const start = 1000
			rounds := gametime.GetDate(start).AddPeriod(s.RespawnRate) - start
			require.Equal(t, uint64(120), rounds*roundSeconds,
				"room %d mob %d: respawnrate %q must parse to two real minutes", tc.roomId, tc.mobId, s.RespawnRate)
		}
		require.True(t, found, "room %d no longer spawns mob %d; this test would prove nothing", tc.roomId, tc.mobId)
	}
}
