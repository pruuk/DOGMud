package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lightnotice"
	"github.com/GoMudEngine/GoMud/internal/prompt"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// lightNoticeLines returns the full text of every message sent to userId
// that contains a line from any lightnotice pool (any cause, any
// transition, either setting).
func lightNoticeLines(msgs []events.Message, userId int) []string {
	var pool []string
	for _, c := range lightnotice.Causes() {
		for _, tr := range lightnotice.Transitions() {
			pool = append(pool, lightnotice.Pool(c, tr, false)...)
			pool = append(pool, lightnotice.Pool(c, tr, true)...)
		}
	}
	var out []string
	for _, m := range msgs {
		if m.UserId != userId {
			continue
		}
		text := strings.ReplaceAll(m.Text, "\n", " ")
		for _, l := range pool {
			if strings.Contains(text, l) {
				out = append(out, text)
				break
			}
		}
	}
	return out
}

// containsAny reports whether text contains any line from pool.
func containsAny(text string, pool []string) bool {
	for _, l := range pool {
		if strings.Contains(text, l) {
			return true
		}
	}
	return false
}

// captureAllMessages records every events.Message sent while the returned
// stop function has not yet been called. The caller must call stop() (which
// drains the queue once more so anything queued mid-test is captured) before
// reading the slice.
func captureAllMessages(t *testing.T) (*[]events.Message, func()) {
	t.Helper()
	captured := []events.Message{}
	events.ProcessEvents() // drop anything queued before the capture
	id := events.RegisterListener(events.Message{}, func(e events.Event) events.ListenerReturn {
		if msg, ok := e.(events.Message); ok {
			captured = append(captured, msg)
		}
		return events.Continue
	})
	return &captured, func() {
		events.ProcessEvents()
		events.UnregisterListener(events.Message{}, id)
	}
}

// TestLightNoticeEndToEndAcrossARoomChange drives a real "north" command all
// the way through TryCommand and the event queue, proving the wiring that
// spans two packages: usercommands.TryCommand's TriggerCommand check (before
// the move) and the RoomChange -> TriggerMove seam that in production lives
// in internal/hooks.LightNoticeOnMove.
//
// usercommands cannot import internal/hooks (hooks imports usercommands, so
// the reverse import would cycle), so the RoomChange listener below is a
// small local copy of LightNoticeOnMove, registered for this test only and
// unregistered on cleanup. It is not exercising hooks' own registration;
// item 5a covers that registration is wired in hooks.go.
func TestLightNoticeEndToEndAcrossARoomChange(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	require.NoError(t, lightnotice.LoadFrom("../../_datafiles/world/dogmud/narration/light-notices"))
	t.Cleanup(lightnotice.ResetForTest)

	moveListenerId := events.RegisterListener(events.RoomChange{}, func(e events.Event) events.ListenerReturn {
		evt, ok := e.(events.RoomChange)
		if !ok || evt.UserId == 0 || evt.MobInstanceId != 0 {
			return events.Continue
		}
		lightnotice.Check(users.GetByUserId(evt.UserId), lightnotice.TriggerMove)
		return events.Continue
	})
	t.Cleanup(func() { events.UnregisterListener(events.RoomChange{}, moveListenerId) })

	// Room 1 (lit) and room 2, north of it in the seed, pinned sky-less and
	// dark so the crossing is unambiguous.
	room1, room2 := rooms.LoadRoom(1), rooms.LoadRoom(2)
	require.NotNil(t, room1)
	require.NotNil(t, room2)
	room1.SkyLight, room1.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	room2.SkyLight, room2.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	u.Character.RoomId = 1
	lightnotice.Check(u, lightnotice.TriggerQuiet)

	captured, capCleanup := captureAllMessages(t)
	handled, err := TryCommand("north", "", 1, 0)
	require.True(t, handled)
	require.NoError(t, err)
	// ProcessEvents drains the queue including events it enqueues along the
	// way (its own doc comment), but a second call is cheap insurance
	// against anything left mid-flight.
	events.ProcessEvents()
	events.ProcessEvents()
	capCleanup()

	require.Equal(t, 2, u.Character.RoomId, "north must have moved the player")
	got := lightNoticeLines(*captured, 1)
	require.Len(t, got, 1, "exactly one light notice for the move, got %q", got)
	require.True(t, containsAny(got[0], lightnotice.Pool(lightnotice.CauseMovement, lightnotice.DarkerDark, false)),
		"the one notice must be from the movement darker_dark pool, got %q", got[0])

	captured2, capCleanup2 := captureAllMessages(t)
	handled, err = TryCommand("look", "", 1, 0)
	require.True(t, handled)
	require.NoError(t, err)
	capCleanup2()
	require.Empty(t, lightNoticeLines(*captured2, 1), "a second look in the same room and band is silent")
}

// TestAltSwapRecordsLightNoticeQuietly guards the alt-swap bug: SwapToAlt
// (called from cmdCharacterChange below) replaces user.Character in place,
// with no MoveToRoom, RoomChange or PlayerSpawn event, so without the
// TriggerQuiet call cmdCharacterChange makes right after a successful swap
// the stale per-user lightnotice record (still naming the OLD character's
// room and band) would make the NEXT command look like a room change and
// risk a false movement notice ("You step into darkness.") though nobody
// walked.
//
// This calls cmdCharacterChange directly (same package) with a cmdPrompt
// whose two Questions are pre-answered, bypassing the interactive
// multi-step prompt flow that Character() would otherwise drive. SwapToAlt
// itself reloads alts from disk independently of the nameToAlt map passed
// in here, so the alt fixture is written for real via characters.SaveAlts
// into the temp FilePaths.DataFiles TestMain points at.
func TestAltSwapRecordsLightNoticeQuietly(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	// useDogmudTemplates points DataFiles at the real world, and this test
	// saves the user and its alts (#826). Send those writes to a temp dir.
	cfg := configs.GetConfig()
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "users"), 0o755))
	cfg.FilePaths.DataFiles = configs.ConfigString(dataDir)
	configs.SetConfigForTest(t, cfg)
	require.NoError(t, lightnotice.LoadFrom("../../_datafiles/world/dogmud/narration/light-notices"))
	t.Cleanup(lightnotice.ResetForTest)

	room1, room2 := rooms.LoadRoom(1), rooms.LoadRoom(2)
	require.NotNil(t, room1)
	require.NotNil(t, room2)
	room1.SkyLight, room1.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	room2.SkyLight, room2.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	u.Character.RoomId = 1
	lightnotice.Check(u, lightnotice.TriggerQuiet)

	altChar := characters.Character{Name: "Bobalt", RoomId: 2}
	require.True(t, characters.SaveAlts(u.UserId, []characters.Character{altChar}))
	t.Cleanup(func() { characters.SaveAlts(u.UserId, nil) })

	nameToAlt := map[string]characters.Character{"Bobalt": altChar}
	altNames := []string{"Bobalt"}

	cmdPrompt := prompt.New("character", "")
	cmdPrompt.Questions = append(cmdPrompt.Questions,
		&prompt.Question{Question: `Enter the name of the character you wish to change to:`, Response: "Bobalt", Done: true},
		&prompt.Question{
			Question:        `<ansi fg="51">Are you SURE you want to change to <ansi fg="username">Bobalt</ansi>?</ansi>`,
			Options:         []string{"yes", "no"},
			DefaultResponse: "no",
			Response:        "yes",
			Done:            true,
		},
	)

	captured, capCleanup := captureAllMessages(t)
	handled, err := cmdCharacterChange(u, room1, cmdPrompt, altNames, nameToAlt, map[string]characters.Character{}, 0)
	require.True(t, handled)
	require.NoError(t, err)
	capCleanup()

	require.Equal(t, "Bobalt", u.Character.Name, "the swap must have happened")
	require.Equal(t, 2, u.Character.RoomId, "the alt's own room must now be current")
	require.Empty(t, lightNoticeLines(*captured, 1), "the swap's own TriggerQuiet check must not speak")

	// The next command must not announce a false movement: the swap's quiet
	// check already recorded this exact room and band as the baseline.
	captured2, capCleanup2 := captureAllMessages(t)
	handled, err = TryCommand("look", "", 1, 0)
	require.True(t, handled)
	require.NoError(t, err)
	capCleanup2()
	require.Empty(t, lightNoticeLines(*captured2, 1), "no false movement notice for the alt swap")
}

// TestLightNoticeArrivesBeforeTheCommandOutput pins the ordering: the notice is
// delivered before the command's own output, and only once.
//
// look's room title and description render through templates.Process, and
// this package's TestMain never registers a templates filesystem: per
// template_freeze_test.go's useDogmudTemplates, that leaves every lookup a
// vacuous success on an empty body, so "look" would print no title at all to
// match against. useDogmudTemplates points Process at the real shipped
// templates so the room title actually renders.
func TestLightNoticeArrivesBeforeTheCommandOutput(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	require.NoError(t, lightnotice.LoadFrom("../../_datafiles/world/dogmud/narration/light-notices"))
	t.Cleanup(lightnotice.ResetForTest)

	room1 := rooms.LoadRoom(1)
	require.NotNil(t, room1)
	room1.SkyLight, room1.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(30)
	u := users.GetByUserId(1)
	require.NotNil(t, u)
	lightnotice.Check(u, lightnotice.TriggerQuiet)
	room1.Lamp = rooms.LampPtr(60)

	pool := lightnotice.Pool(lightnotice.CauseLamp, lightnotice.LighterFaces, false)
	run := func() (noticeAt, titleAt, notices int) {
		events.ProcessEvents()
		var texts []string
		id := events.RegisterListener(events.Message{}, func(e events.Event) events.ListenerReturn {
			if m, ok := e.(events.Message); ok && m.UserId == 1 {
				texts = append(texts, strings.ReplaceAll(m.Text, "\n", " "))
			}
			return events.Continue
		})
		defer events.UnregisterListener(events.Message{}, id)
		_, err := TryCommand("look", "", 1, 0)
		require.NoError(t, err)
		events.ProcessEvents()
		noticeAt, titleAt = -1, -1
		for i, text := range texts {
			for _, l := range pool {
				if strings.Contains(text, l) {
					notices++
					if noticeAt < 0 {
						noticeAt = i
					}
				}
			}
			if titleAt < 0 && strings.Contains(text, "Town Square") {
				titleAt = i
			}
		}
		return
	}

	noticeAt, titleAt, notices := run()
	require.Equal(t, 1, notices, "exactly one notice")
	require.GreaterOrEqual(t, titleAt, 0, "look must print the room title")
	require.Less(t, noticeAt, titleAt, "the notice must precede the room description")

	_, _, notices = run()
	require.Equal(t, 0, notices, "a second look in the same band says nothing")
}
