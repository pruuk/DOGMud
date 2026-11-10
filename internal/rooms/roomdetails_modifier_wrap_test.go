package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The shipped sanctuary text (mutators/sanctuary.yaml), 117 columns on one
// line.
const modifierWrapSanctuaryText = "A peace older than the stones themselves settles over you here. Wounds close more easily and breath comes more deeply."

// modifierWrapRoom is room 1 of seedRegistry under an appended description
// modifier, with a viewer standing in it.
func modifierWrapRoom(t *testing.T) (*Room, *users.UserRecord) {
	t.Helper()
	t.Cleanup(seedRegistry())
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: "test-wrap-sanctuary",
		DescriptionModifier: &mutators.TextModifier{
			Behavior: mutators.TextAppend,
			Text:     modifierWrapSanctuaryText,
		},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		7491: users.NewTestUser(7491, "wrapper", "Wrapper", 97491),
	}))
	GetZoneConfig("TestZone").Mutators.Add("test-wrap-sanctuary")
	return roomManager.rooms[1], users.GetByUserId(7491)
}

// A mutator's alert is centered for the reader. The widths at the top of the
// range keep the 65-column centering they always had; below that the alert
// centers between the room template's 6-column margins.
func TestGetDetails_AlertCentersToTheReadersWidth(t *testing.T) {
	const short = "Quake now!"          // 10 columns
	const fits = "The floors are dusty" // 20 columns
	for _, tc := range []struct {
		name    string
		width   int
		alert   string
		padding int
	}{
		{"short at 80 is unchanged", 80, short, 27},
		{"longer at 80 is unchanged", 80, fits, 22},
		{"short at 77 is unchanged", 77, short, 27},
		{"short at 60", 60, short, 19},
		{"short at 30", 30, short, 4},
		{"longer at 30 has no room to pad", 30, fits, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(seedRegistry())
			t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
				MutatorId:     "test-wrap-alert",
				AlertModifier: &mutators.TextModifier{Text: tc.alert},
			}))
			viewer := users.NewTestUser(7492, "alerted", "Alerted", 97492)
			viewer.LineWidth = tc.width
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7492: viewer}))
			GetZoneConfig("TestZone").Mutators.Add("test-wrap-alert")

			alerts := GetDetails(roomManager.rooms[1], viewer).RoomAlerts
			if len(alerts) != 1 {
				t.Fatalf("want the one alert, got %q", alerts)
			}
			want := strings.Repeat(" ", tc.padding) + tc.alert
			if got := alerts[0]; got != want {
				t.Fatalf("alert %q, want %q", got, want)
			}
			if w := 6 + util.VisibleWidth(alerts[0]); w > tc.width {
				t.Errorf("the alert ends at column %d, past the reader's %d", w, tc.width)
			}
		})
	}
}

// #455: a mutator's description modifier was appended after the
// description had been wrapped, so the Mending Hut printed its sanctuary
// line as one 117-column line. It now joins the description before the wrap.
func TestGetDetails_DescriptionModifierIsWrapped(t *testing.T) {
	r, viewer := modifierWrapRoom(t)

	desc := GetDetails(r, viewer).Description
	if !strings.Contains(desc, "A peace older than the stones") || !strings.Contains(desc, "comes more deeply.") {
		t.Fatalf("the modifier text is missing from the description: %q", desc)
	}
	for _, line := range strings.Split(desc, "\n") {
		if w := util.VisibleWidth(strings.TrimRight(line, "\r")); w > 80 {
			t.Errorf("description line is %d columns, want <= 80: %q", w, line)
		}
	}
}

// With the minimap beside it, the modifier wraps to the description column
// like the rest, so no line runs past the map.
func TestGetDetails_DescriptionModifierWrapsBesideTheMinimap(t *testing.T) {
	r, viewer := modifierWrapRoom(t)
	tinymap := []string{"╔═════╗", "║.....║", "║.....║", "║..@..║", "║.....║", "║.....║", "╚═════╝"}

	desc := GetDetails(r, viewer, tinymap).Description
	if !strings.Contains(desc, "A peace older than the stones") {
		t.Fatalf("the modifier text is missing from the description: %q", desc)
	}
	for _, line := range strings.Split(desc, "\n") {
		if w := util.VisibleWidth(strings.TrimRight(line, "\r")); w > 80 {
			t.Errorf("description line is %d columns, want <= 80: %q", w, line)
		}
	}
}
