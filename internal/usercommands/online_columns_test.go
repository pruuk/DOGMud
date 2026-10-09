package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// #449: the admin view of `online` adds UserId, Zone and RoomId to the
// player columns and ran to 102 columns. It drops Title; players keep it.
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

// renderedOnlineWidth is the width tables/generic.template prints for a
// table: each column is its widest cell plus one space of padding on each
// side, with a border before every column and one after the last.
func renderedOnlineWidth(columns []string, entries []onlineEntry, isAdmin bool) int {
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = language.T(column)
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i], _ = e.cells(columns)
	}
	table := templates.GetTable(`title`, headers, rows)
	width := len(columns) + 1
	for _, w := range table.ColumnWidths {
		width += w + 2
	}
	return width
}

// #449: with realistic values (the longest shipped zone name, a name at
// NameSizeMax tagged [AI], an afk marker) the admin table ran past 80
// columns. It fits, trimming the zone (which RoomId still pins down)
// before anything else.
func TestOnline_AdminTableFitsEightyColumns(t *testing.T) {
	zone := longestShippedZoneName(t)
	worst := onlineEntry{
		userId: `12345`, name: strings.Repeat(`W`, 32), isAI: true,
		online: `123h59m`, isAFK: true, role: `admin`, permClass: `admin`,
		zone: zone, roomId: `60557`,
	}
	typical := onlineEntry{
		userId: `7`, name: `Halix`, isAI: true, online: `2h15m`, isAFK: true,
		role: `user`, permClass: `user`, zone: zone, roomId: `6055`,
	}
	columns := onlineColumns(true)

	entries := fitOnlineEntries(columns, []onlineEntry{worst, typical}, true)
	if w := renderedOnlineWidth(columns, entries, true); w > 80 {
		t.Fatalf("the admin online table is %d columns, want <= 80", w)
	}

	alone := fitOnlineEntries(columns, []onlineEntry{typical}, true)
	require.Equal(t, zone, alone[0].zone, "a typical row lost its zone name")
	require.Equal(t, `Halix`, alone[0].name)
	if w := renderedOnlineWidth(columns, alone, true); w > 80 {
		t.Fatalf("the typical admin online table is %d columns, want <= 80", w)
	}
}

// The player view is not fitted: its rows come out as built.
func TestOnline_PlayerViewIsUnchanged(t *testing.T) {
	e := onlineEntry{name: strings.Repeat(`W`, 32), title: strings.Repeat(`T`, 40), online: `1h2m`, role: `user`, permClass: `user`}
	columns := onlineColumns(false)
	got := fitOnlineEntries(columns, []onlineEntry{e}, false)
	require.Equal(t, []onlineEntry{e}, got)
}
