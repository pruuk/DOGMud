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

// Every alert reaches the room template as bare text at every reader width:
// the template centres it between two rules as wide as the reader's screen
// (owner, 2026-11-10), so no width is baked in here.
func TestGetDetails_AlertsAreBareText(t *testing.T) {
	const quake = "Quake now! The floors are dusty."
	for _, width := range []int{30, 42, 80} {
		t.Cleanup(seedRegistry())
		t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
			MutatorId:     "test-wrap-alert",
			AlertModifier: &mutators.TextModifier{Text: quake},
		}))
		viewer := users.NewTestUser(7492, "alerted", "Alerted", 97492)
		viewer.LineWidth = width
		t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7492: viewer}))
		GetZoneConfig("TestZone").Mutators.Add("test-wrap-alert")
		r := roomManager.rooms[1]
		r.IsBank, r.IsStorage = true, true

		alerts := GetDetails(r, viewer).RoomAlerts
		want := []string{AlertBank, AlertStorage, quake}
		if strings.Join(alerts, "|") != strings.Join(want, "|") {
			t.Fatalf("at %d the alerts are %q, want %q", width, alerts, want)
		}
		r.IsBank, r.IsStorage = false, false
	}
}

// On a screen 42 columns wide or narrower the minimap goes above the prose,
// which then takes the whole width (owner, 2026-11-10); wider, it stays
// beside the prose (TestGetDetails_MinimapSitsAtTheViewersRightEdge).
func TestGetDetails_MinimapGoesAboveTheProseOnANarrowScreen(t *testing.T) {
	r, viewer := modifierWrapRoom(t)
	tinymap := []string{"╔═════╗", "║.....║", "║.....║", "║..@..║", "║.....║", "║.....║", "╚═════╝"}
	for _, width := range []int{30, 42} {
		viewer.LineWidth = width
		lines := strings.Split(GetDetails(r, viewer, tinymap).Description, "\n")
		if len(lines) <= len(tinymap) || strings.Join(lines[:len(tinymap)], "\n") != strings.Join(tinymap, "\n") {
			t.Fatalf("at %d the map is not above the prose:\n%s", width, strings.Join(lines, "\n"))
		}
		if !strings.Contains(strings.Join(lines[len(tinymap):], " "), "A peace older") {
			t.Errorf("at %d the prose is missing:\n%s", width, strings.Join(lines, "\n"))
		}
		widest := 0
		for _, line := range lines {
			widest = max(widest, util.VisibleWidth(line))
		}
		if widest > width || widest < width-12 {
			t.Errorf("at %d the prose does not take the width: widest line %d", width, widest)
		}
	}
	for _, width := range []int{43, 80} {
		viewer.LineWidth = width
		lines := strings.Split(GetDetails(r, viewer, tinymap).Description, "\n")
		if !strings.HasSuffix(lines[0], tinymap[0]) || lines[0] == tinymap[0] {
			t.Errorf("at %d the map is no longer beside the prose: %q", width, lines[0])
		}
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
