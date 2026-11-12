package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A reader under 80 columns gets a map drawn to the narrow form width with
// no frame; at 80 and over the map is as before (owner 2026-11-12, plan 5).
func TestMapSizeForReader(t *testing.T) {
	cases := []struct {
		name          string
		level         int
		wide          bool
		screenW, wrap int
		w, h          int
		narrow        bool
	}{
		{"desktop", 1, false, 80, 80, 65, 21, false},
		{"wide on a big telnet", 4, true, 200, 80, 186, 34, false},
		{"phone", 1, false, 42, 42, 42, 21, true},
		{"phone asking wide", 4, true, 42, 42, 42, 21, true},
		{"between", 1, false, 60, 60, 60, 21, true},
		{"just under 80", 1, false, 79, 79, 79, 21, true},
		{"narrowest", 1, false, 30, 30, 30, 21, true},
		{"under the floor", 1, false, 25, 25, 30, 21, true},
	}
	for _, c := range cases {
		w, h, narrow := mapSizeForReader(c.level, c.wide, c.screenW, 40, c.wrap)
		if w != c.w || h != c.h || narrow != c.narrow {
			t.Errorf("%s: got %dx%d narrow=%v, want %dx%d narrow=%v", c.name, w, h, narrow, c.w, c.h, c.narrow)
		}
	}
}

// shippedMapWords reads the longest words a map screen can hold from the
// shipped world: the five longest room map legends (with the names the code
// writes) and the longest zone name.
func shippedMapWords(t *testing.T) (legend []string, zone string) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	rooms := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "rooms")

	legends := map[string]bool{"Party Member": true, "Secret": true}
	legendLine := regexp.MustCompile(`(?m)^maplegend:\s*"?([^"\r\n]+?)"?\s*$`)
	files, err := filepath.Glob(filepath.Join(rooms, "*", "*.yaml"))
	require.NoError(t, err)
	require.Greater(t, len(files), 1000, "the glob must reach the shipped rooms")
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		if m := legendLine.FindSubmatch(raw); m != nil {
			legends[string(m[1])] = true
		}
	}
	for name := range legends {
		legend = append(legend, name)
	}
	sort.Slice(legend, func(i, j int) bool {
		if len(legend[i]) != len(legend[j]) {
			return len(legend[i]) > len(legend[j])
		}
		return legend[i] < legend[j]
	})
	require.Greater(t, len(legend), 5, "the scan must find the shipped legends")
	legend = legend[:5]

	nameLine := regexp.MustCompile(`(?m)^name:\s*"?([^"\r\n]+?)"?\s*$`)
	zones, err := filepath.Glob(filepath.Join(rooms, "*", "zone-config.yaml"))
	require.NoError(t, err)
	require.Greater(t, len(zones), 20, "the glob must reach the shipped zones")
	for _, f := range zones {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		if m := nameLine.FindSubmatch(raw); m != nil && len(m[1]) > len(zone) {
			zone = string(m[1])
		}
	}
	require.NotEmpty(t, zone)
	return legend, zone
}

// The map screen fits the reader it is drawn for, at every width, built the
// way Map builds it (owner 2026-11-12, plan 5). A web `map wide` at 80 is
// held too: 66 columns plus the 14-column frame. A telnet `map wide` is sized
// by the terminal width the client reports (mapSizeFor uses screenWidth), so
// it fits that terminal and is not tested here.
func TestMapRendersFitEveryWidth(t *testing.T) {
	useDogmudTemplates(t)
	names, zone := shippedMapWords(t)
	legend := map[rune]string{}
	for i, name := range names {
		legend[rune('a'+i)] = name
	}

	cases := []struct {
		name         string
		level        int
		wide         bool
		screenW, wrp int
	}{
		{"30", 1, false, 30, 30},
		{"42", 1, false, 42, 42},
		{"60", 1, false, 60, 60},
		{"79", 1, false, 79, 79},
		{"80", 1, false, 80, 80},
		{"web map wide at 80", 4, true, 80, 80},
	}
	for _, c := range cases {
		mapWidth, mapHeight, narrow := mapSizeForReader(c.level, c.wide, c.screenW, 40, c.wrp)
		lines := make([]string, mapHeight)
		for i := range lines {
			lines[i] = strings.Repeat("*", mapWidth)
		}
		// As Map chooses the form: a narrow reader gets the form for its
		// band, anyone else the framed base.
		formWidth := templates.FullWidth
		if narrow {
			formWidth = users.RedrawnFormWidth(c.wrp)
		}
		out, err := templates.ProcessForWidth("maps/map", mapScreenData(zone, lines, mapWidth, legend), false, formWidth)
		require.NoError(t, err, c.name)
		if w := templates.WidestLine(out); w > c.wrp {
			t.Errorf("%s: the map screen is %d wide, want at most %d:\n%s", c.name, w, c.wrp, out)
		}
		if got := strings.Count(out, "*"); got != mapWidth*mapHeight {
			t.Errorf("%s: %d map cells drawn, want %d", c.name, got, mapWidth*mapHeight)
		}
	}
}
