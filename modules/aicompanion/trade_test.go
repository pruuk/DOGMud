package aicompanion

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
)

// tradeRecipes registers a plain dish, an accomplished one and a salve,
// none needing a station or ingredients, and teaches her all three.
func tradeRecipes(t *testing.T) {
	t.Helper()
	for _, r := range []*crafting.RecipeSpec{
		{RecipeId: `tt-stew`, Name: `Stew`, Skill: `cooking`},
		{RecipeId: `tt-feast`, Name: `Feast`, Skill: `cooking`, SkillMinimum: 10},
		{RecipeId: `tt-salve`, Name: `Salve`, Skill: `alchemy`},
	} {
		crafting.RegisterRecipeForTest(r)
		id := r.RecipeId
		t.Cleanup(func() { crafting.UnregisterRecipeForTest(id) })
	}
}

// What she picks: the trade that means most to her, then the finest piece
// of work in it she can make.
func TestBestCraftFollowsHerCalling(t *testing.T) {
	made := []recipeOption{
		{Id: `tt-stew`, Name: `Stew`, Skill: `cooking`},
		{Id: `tt-feast`, Name: `Feast`, Skill: `cooking`, Min: 10},
		{Id: `tt-salve`, Name: `Salve`, Skill: `alchemy`},
	}
	cook := &Profile{Archetype: Archetype{Skills: map[string]float64{`cooking`: 0.8, `alchemy`: 0.1}}}
	if got := bestCraft(cook, made); got.Id != `tt-feast` || got.drive != 0.8 {
		t.Fatalf("a cook makes the finest dish he can: %+v", got)
	}
	healer := &Profile{Archetype: Archetype{Skills: map[string]float64{`cooking`: 0.3, `alchemy`: 0.6}}}
	if got := bestCraft(healer, made); got.Id != `tt-salve` {
		t.Fatalf("a herbalist brews before she cooks: %+v", got)
	}
}

// A born cook idle where he can cook gets on with it at once, ahead of the
// slow pastime roll; then not again until IdleTradeSeconds have passed. One
// for whom it is a passing interest leaves it to the pastime roll.
func TestACookSetsToWorkWhenIdle(t *testing.T) {
	_, _, _, her := harmWorld(t, configs.PVPDisabled)
	tradeRecipes(t)
	her.Character.KnownRecipes = map[string]int{`tt-stew`: 1, `tt-feast`: 1, `tt-salve`: 1}
	her.Character.Skills = map[string]int{`cooking`: 10}
	m, c, _ := strangerModule()
	m.cfg.IdleTradeSeconds = 60
	c.profile.Crafts = []string{`cooking`, `alchemy`}
	c.profile.Archetype.Skills = map[string]float64{`cooking`: 1.0}
	now := time.Now().Unix()

	if !m.tradeAtHand(c, her, now) {
		t.Fatal("a cook with the makings and the place cooks")
	}
	if c.pendingAct == nil || c.pendingAct.Key != `craft:tt-feast` {
		t.Fatalf("the finest dish he can make: %+v", c.pendingAct)
	}
	c.pendingAct = nil
	if m.tradeAtHand(c, her, now+30) {
		t.Fatal("not again before IdleTradeSeconds")
	}
	if !m.tradeAtHand(c, her, now+61) {
		t.Fatal("and again after")
	}

	c.pendingAct, c.lastCraft = nil, 0
	c.profile.Archetype.Skills = map[string]float64{`cooking`: 0.3}
	if m.tradeAtHand(c, her, now+200) {
		t.Fatal("a passing interest is not a calling")
	}

	c.profile.Archetype.Skills = map[string]float64{`cooking`: 1.0}
	c.lastCraft = 0
	// What she knows keeps up with her skill: recipes within her reach
	// are learned again before she looks for something to make.
	her.Character.KnownRecipes = nil
	if !m.tradeAtHand(c, her, now+400) {
		t.Fatal("recipes within her skill are known again, and she cooks")
	}
	if _, ok := her.Character.KnownRecipes[`tt-feast`]; !ok {
		t.Fatalf("the feast is within cooking 10: %v", her.Character.KnownRecipes)
	}
}

// Of the six, the cook and the healer have a calling; the rest cook now and
// then when nothing else is on.
func TestWhoHasACalling(t *testing.T) {
	profiles, _ := loadProfiles()
	called := map[string]string{}
	for id, p := range profiles {
		for _, trade := range p.Crafts {
			if tradeDrive(p, trade) >= callingDrive {
				called[id] = trade
			}
		}
	}
	if called[`hal`] != `cooking` || called[`liesl`] != `alchemy` || len(called) != 2 {
		t.Fatalf("expected Hal to cook and Liesl to brew by calling, got %v", called)
	}
}

// The craft command finds a recipe by its name, as a player types it; an id
// like grilled-meat finds nothing. Before this a companion's every craft
// (hers by choice, or the model's) quietly came to nothing. A recipe she
// has not the skill for is not offered either, since the attempt would be
// refused.
func TestCraftIsByNameAndOnlyWhatWouldStart(t *testing.T) {
	_, _, room, her := harmWorld(t, configs.PVPDisabled)
	tradeRecipes(t)
	her.Character.KnownRecipes = map[string]int{`tt-stew`: 1, `tt-feast`: 1}
	p := &Profile{Crafts: []string{`cooking`}}
	got := craftableHere(her, p, room)
	if len(got) != 1 || got[0].Id != `tt-stew` {
		t.Fatalf("without the skill for the feast, only the stew: %+v", got)
	}
	if cmd := craftCommand(got[0]); cmd != `craft stew` {
		t.Fatalf("by name: %q", cmd)
	}
	her.Character.Skills = map[string]int{`cooking`: 10}
	if got := craftableHere(her, p, room); len(got) != 2 {
		t.Fatalf("with it, both: %+v", got)
	}
}

// Asked to cook, she can: the model's craft verb starts the recipe it was
// shown, by name, the same command the idle trade uses.
func TestTheModelCanCraftWhenAsked(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	tradeRecipes(t)
	her.Character.KnownRecipes = map[string]int{`tt-stew`: 1}
	m, c, _ := strangerModule()
	c.profile.Crafts = []string{`cooking`}
	made := craftableHere(her, c.profile, room)
	if len(made) != 1 {
		t.Fatalf("fixture: %+v", made)
	}
	events.DrainQueuedInputsForTest(her.InstanceId)
	asked := []stimulus{{Kind: `heard`, FromOwner: true, Text: `Cook this for me, would you?`}}
	out := m.performAction(c, her, owner, &scene{RoomId: room.RoomId, byRef: map[string]*thing{}},
		ActionProposal{Verb: `craft`, Ref: made[0].Ref}, asked, 0, 0)
	if !out.Issued {
		t.Fatalf("asked, she cooks: %+v", out)
	}
	if out.Pending == nil || out.Pending.Key != `craft:tt-stew` {
		t.Fatalf("the stew: %+v", out.Pending)
	}
}

// With the makings but not the place, she knows it, and says where: the
// model is shown the recipe and the station it needs, and the stations she
// has seen are on her map.
func TestShowsWhatNeedsAStationSheIsNotAt(t *testing.T) {
	_, _, room, her := harmWorld(t, configs.PVPDisabled)
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: `tt-roast`, Name: `Roast`, Skill: `cooking`, Station: `cooking_fire`})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(`tt-roast`) })
	her.Character.KnownRecipes = map[string]int{`tt-roast`: 1}
	p := &Profile{Crafts: []string{`cooking`}}
	if got := craftableHere(her, p, room); len(got) != 0 {
		t.Fatalf("no fire here: %+v", got)
	}
	away := craftableElsewhere(her, p, room)
	if len(away) != 1 || away[0] != `Roast (cooking), at a cooking fire` {
		t.Fatalf("%q", away)
	}
	room.Station = `cooking_fire`
	if got := craftableElsewhere(her, p, room); len(got) != 0 {
		t.Fatalf("at a fire it is not elsewhere: %q", got)
	}
	mind := newMind(1, p)
	mind.recordRoom(room, &scene{RoomId: room.RoomId, byRef: map[string]*thing{}}, 0, 1, 100)
	if f := mind.Map[room.RoomId].Features; len(f) == 0 || f[0] != `a cooking fire` {
		t.Fatalf("the fire is on her map: %q", f)
	}
}

// Idle, each leans to what suits them: Tobin searches, Mara and Liesl
// forage, Corvel watches the ways out.
func TestPastimesSuitEachCompanion(t *testing.T) {
	profiles, errs := loadProfiles()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	top := func(p *Profile) string {
		best, bw := ``, -1
		for _, k := range []string{`search`, `scan`, `forage`, `salvage`} {
			if w := p.pastimeWeight(k); w > bw {
				best, bw = k, w
			}
		}
		return best
	}
	for id, want := range map[string]string{`tobin`: `search`, `mara`: `forage`, `liesl`: `forage`, `corvel`: `scan`, `isaura`: `search`} {
		if got := top(profiles[id]); got != want {
			t.Errorf("%s leans to %s, want %s", id, got, want)
		}
	}
	bad := *profiles[`tobin`]
	bad.Pastimes = map[string]int{`juggle`: 3}
	if bad.validate() == nil {
		t.Fatal("an unknown pastime is refused at load")
	}
}
