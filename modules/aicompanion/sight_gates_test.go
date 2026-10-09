package aicompanion

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Her `remove` refuses a cursed worn item up front, as `get` refuses a
// household's bauble, so she does not record a futile attempt (spec R8).
func TestCompanionRemoveRefusesACursedItem(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	ring := items.Item{ItemId: 96501, Spec: &items.ItemSpec{ItemId: 96501, Name: "hexed ring", Type: items.Ring, Subtype: items.Wearable, Cursed: true}}
	her.Character.Equipment.Ring = ring
	m, c, _ := strangerModule()
	sc := &scene{RoomId: room.RoomId, byRef: map[string]*thing{}}
	sc.byRef[`w1`] = &thing{Ref: `w1`, Kind: `worn`, Name: `Hexed Ring`, Item: ring, HasItem: true}
	out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `remove`, Ref: `w1`},
		[]stimulus{{Kind: `heard`, FromOwner: true}}, 0, 0)
	if out.Issued || out.Refused != `it will not come off` {
		t.Fatalf("want a refusal before any command, got %+v", out)
	}
}

// askForScene is an owner with a companion module enabled in harmWorld's room,
// with a Smith there or not, lit or pitch dark.
func askForScene(t *testing.T, smithHere, lit bool) (*AICompanionModule, *controller, *users.UserRecord, *rooms.Room) {
	t.Helper()
	owner, _, room, _ := harmWorld(t, configs.PVPDisabled)
	if smithHere {
		harmMob(t, room, 300, `Smith`)
	}
	if !lit {
		room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
	}
	m, c, _ := strangerModule()
	m.cfg.Enabled = true
	m.ctrls = map[int]*controller{owner.UserId: c}
	events.DrainQueuedMessagesForTest(owner.UserId)
	return m, c, owner, room
}

// #454 review F6: companion-ask finds its NPC through the owner's sight rule.
// In the dark a Smith who is there and one who is not read the same refusal,
// and the companion is sent to neither.
func TestCompanionAskSight_DarkReadsTheSameWhetherPresentOrNot(t *testing.T) {
	told := map[bool]string{}
	for _, here := range []bool{true, false} {
		m, c, owner, room := askForScene(t, here, false)
		_, _ = m.cmdAskFor(`smith about the ore`, owner, room, events.EventFlag(0))
		told[here] = strings.Join(events.DrainQueuedMessagesForTest(owner.UserId), ``)
		if c.askAuth != nil {
			t.Fatalf("here=%v: the companion was sent to ask someone the owner cannot see", here)
		}
	}
	if told[true] != told[false] {
		t.Fatalf("the refusal told a present Smith from an absent one:\n here: %q\n gone: %q", told[true], told[false])
	}
}

// The lit control: the owner can see the Smith, so the companion is sent.
func TestCompanionAskSight_LitSendsTheCompanion(t *testing.T) {
	m, c, owner, room := askForScene(t, true, true)
	_, _ = m.cmdAskFor(`smith about the ore`, owner, room, events.EventFlag(0))
	if c.askAuth == nil || c.askAuth.MobInstanceId != 300 {
		t.Fatalf("a lit owner could not send the companion to the Smith: %+v", c.askAuth)
	}
}

// She neither offers nor starts a recipe she cannot see to make (spec C5).
func TestCraftableHereIsEmptyInTheDark(t *testing.T) {
	_, _, room, her := harmWorld(t, configs.PVPDisabled)
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: `sg-twine`, Name: `Twine`, Skill: `tailoring`})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(`sg-twine`) })
	her.Character.KnownRecipes = map[string]int{`sg-twine`: 1}
	p := &Profile{Crafts: []string{`tailoring`}}

	if got := craftableHere(her, p, room); len(got) != 1 {
		t.Fatalf("control: lit, she can make twine, got %+v", got)
	}
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
	if got := craftableHere(her, p, room); len(got) != 0 {
		t.Fatalf("dark: nothing is craftable here, got %+v", got)
	}
}

// A sleeping companion in a lit room offers no recipes: `craftableHere` must
// not disagree with `actions.TooDarkToCraft`, which the actual craft attempt
// asks (InitiateCraft). Before this, `craftableHere` used `cannotSee`, which
// does not consult sleep, so a sleeping companion listed recipes that the
// craft itself then silently refused.
func TestCraftableHereIsEmptyWhenSleepingInALitRoom(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		15: {ConditionId: 15, Name: "Sleeping", Flags: []conditions.Flag{conditions.Sleeping}, TriggerCount: 1000000},
	}))
	_, _, room, her := harmWorld(t, configs.PVPDisabled)
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: `sg-twine-sleep`, Name: `Twine`, Skill: `tailoring`})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(`sg-twine-sleep`) })
	her.Character.KnownRecipes = map[string]int{`sg-twine-sleep`: 1}
	p := &Profile{Crafts: []string{`tailoring`}}

	if got := craftableHere(her, p, room); len(got) != 1 {
		t.Fatalf("control: lit and awake, she can make twine, got %+v", got)
	}
	if err := her.Character.AddCondition(15, false); err != nil {
		t.Fatalf("could not put her to sleep: %v", err)
	}
	if got := craftableHere(her, p, room); len(got) != 0 {
		t.Fatalf("asleep in a lit room: nothing is craftable here, got %+v", got)
	}
}
