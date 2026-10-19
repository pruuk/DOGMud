package main

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #614: skills and inventory ended in two line breaks, so a blank line
// followed each screen. Their templates end in a newline and SendText ends
// every message with its own; a blank line is sent on purpose
// (SendBlankLine) or not at all (#610). The commands trim the template's.
// An empty pack read " Carrying: " with nothing after it; it says nothing.
//
// The live templates are loaded as in TestBoxedScreensEndWithoutAStop: a
// test binary otherwise renders a "template not found" page, which has no
// box and proves nothing.
func TestLaidOutScreensEndWithoutABlankLine(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dogmudDataFilesRoot)
	configs.SetConfigForTest(t, cfg)
	templates.RegisterFS(os.DirFS(dogmudDataFilesRoot).(fs.ReadFileFS))

	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		88140: {RoomId: 88140, Zone: "TestZone", Title: "Screen Room"},
	}, map[string]*rooms.ZoneConfig{}))
	room := rooms.LoadRoom(88140)
	// inventory reads the species' natural attack for its damage line.
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		88140: {SpeciesId: 88140, Name: "Screenfolk"},
	}))
	u := users.NewTestUser(8814, "screen", "Screenwick", 18814)
	u.Character.RoomId = room.RoomId
	u.Character.SpeciesId = 88140
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8814: u}))

	screens := map[string]func() (bool, error){
		"skills":    func() (bool, error) { return usercommands.Skills("", u, room, 0) },
		"inventory": func() (bool, error) { return usercommands.Inventory("", u, room, 0) },
	}
	for name, run := range screens {
		events.DrainQueuedMessagesForTest(u.UserId)
		if _, err := run(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		msgs := events.DrainQueuedMessagesForTest(u.UserId)
		if len(msgs) == 0 {
			t.Fatalf("%s sent nothing", name)
		}
		// The pipeline wraps a message in its category's colour tag, so a
		// doubled break can sit just inside the closing </ansi>.
		screen := boxBottomTagRE.ReplaceAllString(strings.ReplaceAll(strings.Join(msgs, ""), "\r", ""), "")
		if !strings.Contains(screen, "┘") && !strings.Contains(screen, "Carrying:") {
			t.Fatalf("%s did not render its live template, so this check proves nothing: %q", name, screen)
		}
		if strings.HasSuffix(screen, "\n\n") {
			t.Errorf("%s ends with a blank line: %q", name, screen[max(0, len(screen)-60):])
		}
		if name == "inventory" && !strings.Contains(screen, "Carrying: nothing") {
			t.Errorf("an empty pack must read \"Carrying: nothing\": %q", screen)
		}
	}
}
