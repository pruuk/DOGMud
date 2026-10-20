package usercommands

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/prompt"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 0, Task 14: every admin builder list is a menu. Each test here
// drives a builder the way world.go does (the command runs, a typed line goes
// through RoutePromptInput, the command runs again) and answers by number.

// builderAdmin returns test user 1 as an admin with no open prompt, and the
// room it stands in.
func builderAdmin(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	admin, room := getTestUserAndRoom(t)
	oldRole := admin.Role
	admin.Role = users.RoleAdmin
	admin.ClearPrompt()
	t.Cleanup(func() {
		admin.ClearPrompt()
		admin.Role = oldRole
	})
	events.DrainQueuedMessagesForTest(admin.UserId)
	return admin, room
}

// drive runs the builder once, then answers each line as a typed line and
// runs it again, returning everything the admin was sent after the last run.
func drive(t *testing.T, u *users.UserRecord, run func(), answers ...string) string {
	t.Helper()
	run()
	out := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
	for _, a := range answers {
		u.RoutePromptInput(a, true)
		run()
		out = strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
	}
	return out
}

// questionFor finds the open prompt's question by its text or menu heading.
func questionFor(t *testing.T, u *users.UserRecord, heading string) *prompt.Question {
	t.Helper()
	p := u.GetPrompt()
	require.NotNil(t, p, "prompt must still be open")
	for _, q := range p.Questions {
		if q.Question == heading {
			return q
		}
	}
	t.Fatalf("no question %q", heading)
	return nil
}

// lastQuestion is the question the builder is waiting on now.
func lastQuestion(t *testing.T, u *users.UserRecord) string {
	t.Helper()
	p := u.GetPrompt()
	require.NotNil(t, p, "prompt must still be open")
	require.NotEmpty(t, p.Questions)
	return p.Questions[len(p.Questions)-1].Question
}

func TestBuilderMenu_ItemTypeByNumber(t *testing.T) {
	admin, room := builderAdmin(t)
	run := func() { _, _ = Item("create", admin, room, 0) }

	out := drive(t, admin, run, "Test Thing")
	assert.Contains(t, out, "What Type of item will it be?")
	assert.Contains(t, out, "2.", "the type list is numbered")

	drive(t, admin, run, "2")

	q := questionFor(t, admin, `What Type of item will it be?`)
	assert.True(t, q.Done)
	assert.Equal(t, items.ItemTypes()[1].Type, q.Response)
	// Type 2 is armor, so the builder moved on to the mitigation questions.
	assert.Equal(t, `Physical mitigation % (plate:12-18, leather:4-8, cloth:1-3)?`, lastQuestion(t, admin))
}

func TestBuilderMenu_ItemSubtypeByNumberAndCancel(t *testing.T) {
	admin, room := builderAdmin(t)
	run := func() { _, _ = Item("create", admin, room, 0) }

	// Name, type 2 (armor), three mitigation defaults, then the subtype menu.
	out := drive(t, admin, run, "Test Thing", "2", "", "", "")
	assert.Contains(t, out, "What Subtype of item will it be?")
	assert.Contains(t, out, "Cancel", "the subtype menu offers Cancel")

	drive(t, admin, run, "3")
	q := questionFor(t, admin, `What Subtype of item will it be?`)
	assert.True(t, q.Done)
	assert.Equal(t, items.ItemSubtypes()[2].Type, q.Response)
	assert.Equal(t, `Will this item have a maximum number of uses?`, lastQuestion(t, admin))

	// A fresh build: Enter on the subtype menu (no current subtype) is Cancel.
	admin.ClearPrompt()
	events.DrainQueuedMessagesForTest(admin.UserId)
	out = drive(t, admin, run, "Test Thing", "2", "", "", "", "")
	assert.Contains(t, out, "Aborting...")
	assert.Nil(t, admin.GetPrompt(), "Cancel ends the build")
}

func TestBuilderMenu_ContainerByNumber(t *testing.T) {
	admin, room := builderAdmin(t)
	oldContainers := room.Containers
	room.Containers = map[string]rooms.Container{"chest": {}}
	t.Cleanup(func() { room.Containers = oldContainers })
	run := func() { _, _ = room_Edit_Containers("", admin, room, 0) }

	out := drive(t, admin, run)
	assert.Contains(t, out, "new")
	assert.Contains(t, out, "chest")
	assert.Contains(t, out, "press Enter for 1", "Enter picks new")

	drive(t, admin, run, "2")
	assert.Equal(t, `Delete this container?`, lastQuestion(t, admin), "2 picked the chest")

	admin.ClearPrompt()
	drive(t, admin, run, "")
	assert.Equal(t, `Choose a name for this container:`, lastQuestion(t, admin), "Enter picked new")
}

func TestBuilderMenu_ExitByNumber(t *testing.T) {
	admin, room := builderAdmin(t)
	require.Contains(t, room.Exits, "north", "fixture room has a north exit")
	run := func() { _, _ = room_Edit_Exits("", admin, room, 0) }

	out := drive(t, admin, run)
	assert.Contains(t, out, "create a new exit")

	drive(t, admin, run, "2")
	assert.Equal(t, `Delete this exit?`, lastQuestion(t, admin), "2 picked north")
}

func TestBuilderMenu_TrapConditionsToggleByNumber(t *testing.T) {
	admin, _ := builderAdmin(t)
	// Two specs share a name, as two shipped "Stunned" specs do: picking
	// each by number must store a different id.
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		201: {ConditionId: 201, Name: "Alpha"},
		202: {ConditionId: 202, Name: "Stunned"},
		203: {ConditionId: 203, Name: "Stunned"},
	}))
	p, _ := admin.StartPrompt(`test trap`, ``)
	var lock gamelock.Lock
	var pending bool
	run := func() { lock, pending = editLockAndTrap(p, admin, gamelock.Lock{}, `container`) }

	heading := `Select a condition to add to the trap, or nothing to continue:`
	out := drive(t, admin, run, "yes", "5", "yes")
	assert.Contains(t, out, heading)
	assert.Contains(t, out, "Done")

	out = drive(t, admin, run, "2")
	assert.Contains(t, out, menu.MarkerChecked, "the toggled condition shows [x]")
	out = drive(t, admin, run, "3")
	assert.Equal(t, 2, strings.Count(out, menu.MarkerChecked), "both toggles show [x]")

	stored, ok := p.Recall(`trapConditions`)
	require.True(t, ok)
	ids := stored.([]int)
	require.Len(t, ids, 2)
	assert.NotEqual(t, ids[0], ids[1], "same-named conditions picked by number are distinct")
	assert.ElementsMatch(t, []int{202, 203}, ids)

	// Enter is Done: the builder stores the list on the lock and moves on.
	drive(t, admin, run, "")
	assert.True(t, pending, "the relock question is still to come")
	assert.ElementsMatch(t, []int{202, 203}, lock.TrapConditionIds)
	assert.NotEqual(t, heading, lastQuestion(t, admin))
}

func TestBuilderMenu_RecipesByNumber(t *testing.T) {
	admin, _ := builderAdmin(t)

	t.Run("existing recipe by number", func(t *testing.T) {
		admin.ClearPrompt()
		p, _ := admin.StartPrompt(`test recipes`, ``)
		var got map[int][]int
		var pending bool
		run := func() { got, pending = editContainerRecipes(p, admin, map[int][]int{10001: {20001}}) }

		out := drive(t, admin, run, "yes")
		assert.Contains(t, out, "Modify which (or new)?")
		assert.Contains(t, out, "press Enter for 3", "Enter picks skip")

		drive(t, admin, run, "1")
		assert.Equal(t, `Enter an itemId to add to the recipe, or nothing to continue:`, lastQuestion(t, admin))
		drive(t, admin, run, "30001", "")
		assert.False(t, pending)
		assert.ElementsMatch(t, []int{20001, 30001}, got[10001])
	})

	t.Run("new by number", func(t *testing.T) {
		admin.ClearPrompt()
		p, _ := admin.StartPrompt(`test recipes`, ``)
		run := func() { _, _ = editContainerRecipes(p, admin, map[int][]int{}) }
		drive(t, admin, run, "yes", "1")
		assert.Equal(t, `What itemId will be created?`, lastQuestion(t, admin))
	})

	t.Run("skip by Enter", func(t *testing.T) {
		admin.ClearPrompt()
		p, _ := admin.StartPrompt(`test recipes`, ``)
		var pending bool
		run := func() { _, pending = editContainerRecipes(p, admin, map[int][]int{}) }
		drive(t, admin, run, "yes", "")
		assert.False(t, pending, "skip finishes the recipe step")
	})
}

func seedTestMutators(t *testing.T) {
	t.Helper()
	t.Cleanup(mutators.SeedSpecsForTest(
		mutators.MutatorSpec{MutatorId: "zz-test-alpha"},
		mutators.MutatorSpec{MutatorId: "zz-test-beta"},
	))
}

// mutatorIndex is the 1-based menu number of a mutator id: the builders list
// every registered mutator sorted by id.
func mutatorIndex(t *testing.T, id string) string {
	t.Helper()
	names := mutators.GetAllMutatorIds()
	sort.Strings(names)
	for i, n := range names {
		if n == id {
			return strconv.Itoa(i + 1)
		}
	}
	t.Fatalf("mutator %q not registered", id)
	return ""
}

func TestBuilderMenu_RoomMutatorsToggleByNumber(t *testing.T) {
	admin, room := builderAdmin(t)
	seedTestMutators(t)
	oldMuts := room.Mutators
	t.Cleanup(func() { room.Mutators = oldMuts })
	run := func() { _, _ = room_Edit_Mutators("", admin, room, 0) }

	out := drive(t, admin, run)
	assert.Contains(t, out, "Done")

	out = drive(t, admin, run, mutatorIndex(t, "zz-test-beta"))
	assert.Contains(t, out, menu.MarkerChecked)
	stored, ok := admin.GetPrompt().Recall(`mutators`)
	require.True(t, ok)
	assert.Contains(t, stored.([]string), "zz-test-beta")

	out = drive(t, admin, run, "")
	assert.Contains(t, out, "Changes saved.")
	found := false
	for _, m := range room.Mutators {
		if m.MutatorId == "zz-test-beta" {
			found = true
		}
	}
	assert.True(t, found, "Done saved the toggled mutator on the room")
}

func TestBuilderMenu_ZoneMutatorsToggleByNumber(t *testing.T) {
	admin, room := builderAdmin(t)
	seedTestMutators(t)
	run := func() { _, _ = Zone("edit", admin, room, 0) }

	heading := `Select a mutator to add/remove, or nothing to continue:`
	out := drive(t, admin, run)
	assert.Contains(t, out, heading)

	out = drive(t, admin, run, mutatorIndex(t, "zz-test-alpha"))
	assert.Contains(t, out, menu.MarkerChecked)
	stored, ok := admin.GetPrompt().Recall(`mutators`)
	require.True(t, ok)
	assert.Contains(t, stored.([]string), "zz-test-alpha")

	drive(t, admin, run, "")
	assert.Equal(t, `Should the zone have music?`, lastQuestion(t, admin), "Done moves on")
}

func TestBuilderMenu_ServerConfigByNumber(t *testing.T) {
	admin, room := builderAdmin(t)
	run := func() { _, _ = server_Config("", admin, room, 0) }

	top, ok := getConfigOptions("")
	require.True(t, ok)
	require.NotEmpty(t, top)

	out := drive(t, admin, run)
	assert.Contains(t, out, `Choose a config option, or "quit":`)
	assert.Contains(t, out, "Quit")

	// Find a top-level section (not a leaf) so picking it opens a level.
	pick := -1
	for i, o := range top {
		if o.Description == "..." {
			pick = i
			break
		}
	}
	require.GreaterOrEqual(t, pick, 0, "config has at least one section")

	out = drive(t, admin, run, strconv.Itoa(pick+1))
	selected, ok := admin.GetPrompt().Recall(`config-selected`)
	require.True(t, ok)
	assert.Equal(t, top[pick].Name, selected)
	assert.Contains(t, out, `[<ansi fg="6">`+top[pick].Name+`</ansi>]`)

	// The stored menu is now the section's level, so a number picks from it.
	sub, ok := getConfigOptions(strings.ToLower(top[pick].Name))
	require.True(t, ok)
	q := questionFor(t, admin, `Choose a config option, or "quit":`)
	require.NotNil(t, q.Menu)
	require.Len(t, q.Menu.Entries, len(sub))
	assert.Equal(t, sub[0].Name, q.Menu.Entries[0].Id)

	// Enter is Quit.
	out = drive(t, admin, run, "")
	assert.Contains(t, out, "Quitting...")
	assert.Nil(t, admin.GetPrompt())
}

func TestBuilderMenu_ServerConfigTypedPathStillWorks(t *testing.T) {
	admin, room := builderAdmin(t)
	run := func() { _, _ = server_Config("", admin, room, 0) }

	// A dotted path two levels down that is editable and still a section, so
	// typing it opens that level without asking for a value.
	top, _ := getConfigOptions("")
	dotted := ""
	for _, o := range top {
		if o.Description != "..." {
			continue
		}
		sub, _ := getConfigOptions(strings.ToLower(o.Name))
		for _, s := range sub {
			path := strings.ToLower(o.Name + "." + s.Name)
			if s.Description == "..." && isEditAllowed(path) {
				dotted = path
				break
			}
		}
		if dotted != "" {
			break
		}
	}
	require.NotEmpty(t, dotted, "config has an editable two-level section")

	drive(t, admin, run, dotted)
	selected, ok := admin.GetPrompt().Recall(`config-selected`)
	require.True(t, ok)
	assert.True(t, strings.EqualFold(dotted, selected.(string)), "typed dotted path %q selected %q", dotted, selected)
}

func TestOptionsMenu_IdsAndMarks(t *testing.T) {
	m := optionsMenu(`H`, []templates.NameDescription{
		{Name: "plain", Description: "d"},
		{Id: 7, Name: "seven", Marked: true},
	}, 1)
	require.Len(t, m.Entries, 2)
	assert.Equal(t, "plain", m.Entries[0].Id, "no Id: the name is the entry Id")
	assert.Equal(t, "d", m.Entries[0].Detail)
	assert.Equal(t, "7", m.Entries[1].Id, "an Id is carried as text")
	assert.Equal(t, menu.MarkerChecked, m.Entries[1].Marker)
	assert.True(t, m.Prefix)

	two := optionsMenu(`H`, []templates.NameDescription{{Name: "a", Description: "d"}}, 2)
	assert.Empty(t, two.Entries[0].Detail, "two-column menus carry no detail")
}
