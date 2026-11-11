package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The scan screen (plan 3b-3, scan in narrow-screens-3b3-mock): its 80
// layout was redrawn (a line of names ran past 80), so the names wrap at
// indent 4 at 80, 42 and 30, and `dir (Room Title):` puts the title under
// the direction when the two do not fit a line.

var scanTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

// scanPlain is text with its colour tags removed and its line ends trimmed.
func scanPlain(text string) string {
	lines := strings.Split(scanTagRE.ReplaceAllString(text, ""), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \r")
	}
	return strings.Join(lines, "\n")
}

// scanDrawing is the approved drawing (scan.w80, .w42, .w30), copied from
// _archive/notes/reports/narrow-screens-3b3-mock/forms into testdata.
func scanDrawing(t *testing.T, width int) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "drawings_3b3", fmt.Sprintf("scan.w%d.txt", width)))
	require.NoError(t, err)
	return strings.TrimRight(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
}

// scanCaptured is the capture's three directions, as scanEntryText takes
// them.
func scanCaptured(formWidth int) string {
	mob := func(names ...string) []string {
		out := []string{}
		for _, n := range names {
			out = append(out, `<ansi fg="mobname">`+n+`</ansi>`)
		}
		return out
	}
	// The heading rides CategorySystem, which the pipeline wraps.
	lines := []string{messaging.WrapAnsi("You scan the surrounding area...", formWidth), ""}
	lines = append(lines,
		scanEntryText("north", "Goblin Camp Perimeter", mob("Goblin Sentry", "Goblin Scout", "Goblin Scrapper", "Goblin Scrapper", "Goblin Scout", "Goblin Shaman"), "", formWidth),
		scanEntryText("east", "Goblin Shaman's Alcove", mob("Goblin Shaman", "Goblin Scrapper"), "", formWidth),
		scanEntryText("south", "Goblin Sleeping Den", nil, "too dark to make anything out", formWidth))
	return scanPlain(strings.Join(lines, "\n"))
}

func TestScanIsTheDrawing(t *testing.T) {
	for _, width := range []int{80, 42, 30} {
		got, want := scanCaptured(width), scanDrawing(t, width)
		if got != want {
			t.Errorf("scan at %d is not the drawing:\nwant:\n%s\ngot:\n%s", width, want, got)
		}
	}
}

// scanWideId are the rooms of a scan as wide as play makes it: a long room
// title, and more creatures than a line holds, one of them with the longest
// shipped mob name.
const (
	scanWideHereId  = 9495
	scanWideThereId = 9496
	scanWideUserId  = 9497
)

// scanWideOut runs a player's scan at width into a lit room holding n
// creatures with the given names.
func scanWideOut(t *testing.T, width int, title string, names ...string) string {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"town": {BiomeId: "town", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	here := &rooms.Room{RoomId: scanWideHereId, Zone: "ScanWide", Biome: "town", Lamp: rooms.LampPtr(90),
		Exits: map[string]exit.RoomExit{"northwest": {RoomId: scanWideThereId}}}
	there := &rooms.Room{RoomId: scanWideThereId, Zone: "ScanWide", Biome: "town", Lamp: rooms.LampPtr(90), Title: title,
		Exits: map[string]exit.RoomExit{"southeast": {RoomId: scanWideHereId}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{scanWideHereId: here, scanWideThereId: there},
		map[string]*rooms.ZoneConfig{"ScanWide": {Name: "ScanWide", RoomId: scanWideHereId,
			RoomIds: map[int]struct{}{scanWideHereId: {}, scanWideThereId: {}}}},
	))
	for i, name := range names {
		id := 9600 + i
		m := newScanTestMob(id, name, scanWideThereId)
		mobs.SetInstanceForTest(id, m)
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		there.AddMob(id)
	}
	u := users.NewTestUser(scanWideUserId, "scanner", "Scanner", 99497)
	u.LineWidth = width
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{scanWideUserId: u}))
	actor := newScanFakeActor("Scanner", here, true, scanWideUserId)
	Scan(actor, ScanOptions{})
	return strings.Join(actor.sent, "\n")
}

// Every line fits the form at every width, with the longest shipped room
// title (40) and mob name (29) whole wherever they fit the room at their
// indent: the name from 42 up, the title at 80. Narrower, each splits at a
// space and keeps the indent (ruling 3).
func TestScanFitsEveryWidth(t *testing.T) {
	const title = "The Crossing, Great Hall Over the Waters"
	// Two "Goblin Scout," side by side at indent 4 end at 31: one column
	// past a 30 line, so a packer that overfills by one is caught.
	names := []string{"Goblin Sentry", "Mistress Odell the Victualler", "Goblin Shaman", "Goblin Scout", "Goblin Scout", "Goblin Scrapper"}
	for _, width := range []int{30, 42, 60, 80, 120} {
		out := scanPlain(scanWideOut(t, width, title, names...))
		formWidth := users.RedrawnFormWidth(width)
		for _, line := range strings.Split(out, "\n") {
			if line == "You scan the surrounding area..." {
				continue // CategorySystem: the pipeline wraps it
			}
			require.LessOrEqual(t, len([]rune(line)), formWidth, "at %d: %q in\n%s", width, line, out)
			if strings.HasPrefix(line, "   ") {
				require.True(t, strings.HasPrefix(line, "    "), "at %d a continuation lost its indent: %q", width, line)
			}
		}
		flat := strings.Join(strings.Fields(out), " ")
		for _, n := range names {
			require.Contains(t, flat, n, "at %d a name went missing:\n%s", width, out)
			if formWidth >= 42 {
				require.Contains(t, out, n, "at %d a name split:\n%s", width, out)
			}
		}
		require.Contains(t, flat, "("+title+"):", "at %d", width)
		if formWidth == 80 {
			require.Contains(t, out, "("+title+"):", "at %d the title split:\n%s", width, out)
		}
	}
}

// At 80 a scan whose line fits is master's line, byte for byte.
func TestScanKeepsIts80ColumnLine(t *testing.T) {
	out := scanWideOut(t, 80, "Goblin Shaman's Alcove", "Goblin Shaman", "Goblin Scrapper")
	require.Contains(t, out, "\n"+`  <ansi fg="exit">northwest</ansi> (<ansi fg="room-title">Goblin Shaman's Alcove</ansi>): <ansi fg="mobname">Goblin Shaman</ansi>, <ansi fg="mobname">Goblin Scrapper</ansi>`+"\n")
}
