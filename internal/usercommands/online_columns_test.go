package usercommands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// #449: the admin view of `online` adds UserId, Zone and RoomId to the
// player columns and ran to 102 columns. It drops Title; players keep it.
// A list still too wide takes the table's narrow form (plan 3b-3, ruling 7;
// table_leads_test.go).
func TestOnlineColumns_AdminViewDropsTitle(t *testing.T) {
	require.Equal(t, []string{`UserId`, `User.Name`, `Online`, `Role`, `Zone`, `RoomId`}, onlineColumns(true))
	require.Equal(t, []string{`User.Name`, `Title`, `Online`, `Role`}, onlineColumns(false))
}

// longestShippedZoneName reads every dogmud room file's zone name and
// returns the longest one.
func longestShippedZoneName(t *testing.T) string {
	t.Helper()
	files, err := filepath.Glob(`../../_datafiles/world/dogmud/rooms/*/*.yaml`)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no shipped room files found")
	longest := ``
	seen := map[string]bool{}
	for _, f := range files {
		dir := filepath.Dir(f)
		if seen[dir] {
			continue
		}
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		var room struct {
			Zone string `yaml:"zone"`
		}
		if yaml.Unmarshal(b, &room) != nil || room.Zone == `` {
			continue
		}
		seen[dir] = true
		if len(room.Zone) > len(longest) {
			longest = room.Zone
		}
	}
	require.NotEmpty(t, longest)
	return longest
}
