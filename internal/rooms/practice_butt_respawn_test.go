package rooms

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestPracticeButtRespawnsInFiveRounds pins the 2026-10-17 pre-deploy fix
// (owner: 5 rounds). The First Shot practice butt (mob 9163, the Long Terrace,
// room 5351) dies to a stone or two by design, and with no respawnrate the
// engine default (15 real minutes, see the spawninfo default in rooms.go) left
// a second student waiting a quarter of an hour to finish the quest.
//
// The rate must also PARSE as 5 rounds. gametime.AddPeriod never fails: an
// unknown unit falls through to "the quantity is a count of rounds", which is
// what makes "5 rounds" work and what makes a typo or a "1h 30m" style string
// silently wrong. So this asserts the parsed length, not only the text.
func TestPracticeButtRespawnsInFiveRounds(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "rooms", "pothole_coulee", "5351.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var room struct {
		RoomId    int `yaml:"roomid"`
		SpawnInfo []struct {
			MobId       int    `yaml:"mobid"`
			RespawnRate string `yaml:"respawnrate"`
		} `yaml:"spawninfo"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &room))
	require.Equal(t, 5351, room.RoomId)

	found := false
	for _, s := range room.SpawnInfo {
		if s.MobId != 9163 {
			continue
		}
		found = true
		require.NotEmpty(t, s.RespawnRate, "an empty respawnrate is the 15 real minute default")
		require.True(t, RealPeriodOK(s.RespawnRate), "respawnrate %q is not a period the engine understands", s.RespawnRate)
		const start = 1000
		got := gametime.GetDate(start).AddPeriod(s.RespawnRate) - start
		require.Equal(t, uint64(5), got, "respawnrate %q must parse to 5 rounds", s.RespawnRate)
	}
	require.True(t, found, "room 5351 no longer spawns the practice butt (9163); this test would prove nothing")
}
