package roomlife

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
)

type fakeGen struct {
	chance   int
	allow    map[int]bool
	res      Result
	err      error
	reserved []int
	gens     int
	req      Request
	user     int
}

func (g *fakeGen) Chance() int { return g.chance }
func (g *fakeGen) Reserve(userId int) bool {
	g.reserved = append(g.reserved, userId)
	return g.allow[userId]
}
func (g *fakeGen) Generate(ctx context.Context, userId int, req Request) (Result, error) {
	g.gens++
	g.req, g.user = req, userId
	return g.res, g.err
}

const (
	testRoom   = 1
	keyholder  = 7
	bystander  = 8
	playerName = `Zyxwaldo`
	setLine    = `A magpie chatters from the eaves, then falls silent.`
	seenEvent  = `A rat noses out of a gap in the boards and darts along the wall.`
	heardEvent = `You hear a woman behind the bakery door asking where the good knife went.`
)

// street seeds a lamplit street holding a keyholder, a bystander and a
// carter, runs deliveries in line, and makes every roll hit.
func street(t *testing.T, g Generator) *rooms.Room {
	t.Helper()
	carter := &mobs.Mob{MobId: 50, InstanceId: 100, Character: *characters.New()}
	carter.Character.Name = `a weary carter`
	carter.Character.RoomId = testRoom
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{50: carter}, map[int]*mobs.Mob{100: carter}))

	seed := map[int]*users.UserRecord{}
	for _, id := range []int{keyholder, bystander} {
		u := users.NewUserRecord(id, 0)
		u.Character = characters.New()
		u.Character.Name = playerName
		u.Character.RoomId = testRoom
		u.Character.SetUserId(id)
		seed[id] = u
	}
	t.Cleanup(users.SeedUsersForTest(seed))

	none, lamp := 0.0, 100
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		testRoom: {RoomId: testRoom, Zone: `Testville`, Title: `Baker's Row`,
			Description: `A narrow street of shuttered shopfronts.`, SkyLight: &none, Lamp: &lamp},
	}, map[string]*rooms.ZoneConfig{}))
	room := rooms.LoadRoom(testRoom)
	room.AddPlayer(keyholder)
	room.AddPlayer(bystander)
	room.AddMob(100)

	oldRoll, oldShuffle, oldStart, oldSee := roll, shuffle, startDelivery, canSee
	roll = func() int { return 0 }
	shuffle = func([]int) {}
	startDelivery = func(d *delivery) { d.run(false) }
	SetGenerator(g)
	t.Cleanup(func() {
		roll, shuffle, startDelivery, canSee = oldRoll, oldShuffle, oldStart, oldSee
		SetGenerator(nil)
		drain()
	})
	drain()
	return room
}

func drain() {
	events.DrainQueuedMessagesForTest(keyholder)
	events.DrainQueuedMessagesForTest(bystander)
}

func read(id int) string { return strings.Join(events.DrainQueuedMessagesForTest(id), ``) }

func dark(room *rooms.Room) { room.Lamp = nil }

func TestNoGeneratorNoEvent(t *testing.T) {
	room := street(t, nil)
	if TryReplace(room, setLine, nil) {
		t.Fatal("with no generator installed every ambient line is the set one")
	}
}

func TestASeenEventReplacesTheSetLine(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, res: Result{Kind: KindSeen, Text: seenEvent}}
	room := street(t, g)
	if !TryReplace(room, setLine, []string{setLine}) {
		t.Fatal("taken")
	}
	if g.user != keyholder || !g.req.CanSee {
		t.Fatalf("written on the keyholder's key, who sees the lit street (%d %v)", g.user, g.req.CanSee)
	}
	for _, id := range []int{keyholder, bystander} {
		got := read(id)
		if !strings.Contains(got, `rat noses`) || strings.Contains(got, `magpie`) {
			t.Fatalf("everyone sees the event, not the set line: %q", got)
		}
	}
	if Pending() != 0 {
		t.Fatal("nothing left pending")
	}
}

func TestTheRequestCarriesThePlaceButNoPlayer(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, res: Result{Kind: KindSeen, Text: seenEvent}}
	room := street(t, g)
	TryReplace(room, setLine, []string{setLine, ``, setLine, `Wind worries a loose shutter.`})
	r := g.req
	if r.Title != `Baker's Row` || !strings.Contains(r.Description, `shuttered`) {
		t.Fatalf("the room's text: %+v", r)
	}
	if len(r.NPCs) != 1 || r.NPCs[0] != `a weary carter` || r.Travellers != 2 {
		t.Fatalf("NPCs by name, players counted: %+v", r)
	}
	if len(r.Examples) != 2 {
		t.Fatalf("its own set lines, once each, for its voice: %v", r.Examples)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), playerName) {
		t.Fatal("no player is ever named in a request")
	}
}

func TestAMissedRollOrNoKeyTakesNothing(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{}}
	room := street(t, g)
	if TryReplace(room, setLine, nil) || g.gens != 0 || Pending() != 0 {
		t.Fatal("no usable key: nothing taken, nothing pending")
	}
	g.allow[keyholder] = true
	roll = func() int { return 10 }
	if TryReplace(room, setLine, nil) {
		t.Fatal("a roll of 10 misses a 10% chance")
	}
	roll = func() int { return 0 }
	if TryReplace(room, ``, nil) {
		t.Fatal("no set line, no event")
	}
}

// Light: in a dark room the keyholder cannot see, so they are asked for a
// sound only; a sound is heard by everyone there whatever the light, the
// audio channel every "You hear" line uses.
func TestInTheDarkOnlyASoundAndItIsHeard(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, res: Result{Kind: KindHeard, Text: heardEvent}}
	room := street(t, g)
	dark(room)
	if !TryReplace(room, setLine, nil) {
		t.Fatal("taken")
	}
	if g.req.CanSee {
		t.Fatal("the keyholder in the dark is asked for a sound only")
	}
	for _, id := range []int{keyholder, bystander} {
		if got := read(id); !strings.Contains(got, `You hear a woman`) {
			t.Fatalf("heard in the dark: %q", got)
		}
	}
}

// A seen event in the dark is refused (CleanResult), and the set line,
// which is visual, is then judged by the visual sender: nobody sees it.
func TestInTheDarkASeenEventIsRefusedAndNothingIsSeen(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, res: Result{Kind: KindSeen, Text: seenEvent}}
	room := street(t, g)
	dark(room)
	TryReplace(room, setLine, nil)
	for _, id := range []int{keyholder, bystander} {
		if got := read(id); got != `` {
			t.Fatalf("dark: nothing seen, neither the event nor the set line: %q", got)
		}
	}
}

// A seen event is sight-gated per reader at delivery: a reader who has gone
// blind meanwhile sees nothing, while everyone else does.
func TestASeenEventIsJudgedPerReader(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, res: Result{Kind: KindSeen, Text: seenEvent}}
	room := street(t, g)
	blind(t, users.GetByUserId(bystander).Character)
	TryReplace(room, setLine, nil)
	if got := read(keyholder); !strings.Contains(got, `rat noses`) {
		t.Fatalf("the keyholder sees it: %q", got)
	}
	if got := read(bystander); got != `` {
		t.Fatalf("a blinded reader does not: %q", got)
	}
}

func TestASleeperIsNotAsked(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true, bystander: true}, res: Result{Kind: KindSeen, Text: seenEvent}}
	room := street(t, g)
	asleep(t, users.GetByUserId(keyholder).Character)
	TryReplace(room, setLine, nil)
	for _, id := range g.reserved {
		if id == keyholder {
			t.Fatal("a sleeping player's key is never asked")
		}
	}
	if g.user != bystander {
		t.Fatalf("the awake one's is: %d", g.user)
	}
}

func TestFailureShowsTheSetLine(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, err: errors.New(`provider said no`)}
	room := street(t, g)
	TryReplace(room, setLine, nil)
	if got := read(bystander); !strings.Contains(got, `magpie`) {
		t.Fatalf("no event came: the set line is shown after all: %q", got)
	}
	g.err, g.res = nil, Result{Kind: KindSeen, Text: `A dog sniffs at your boots.`}
	TryReplace(room, setLine, nil)
	if got := read(bystander); !strings.Contains(got, `magpie`) {
		t.Fatalf("an event the rules refuse is no event: %q", got)
	}
}

func TestAKeyholderOnlyEventReachesNobodyElse(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, res: Result{Kind: KindHeard, Text: heardEvent, KeyholderOnly: true}}
	room := street(t, g)
	TryReplace(room, setLine, nil)
	if got := read(keyholder); !strings.Contains(got, `You hear a woman`) {
		t.Fatalf("the keyholder hears it: %q", got)
	}
	if got := read(bystander); got != `` {
		t.Fatalf("nobody else gets it, nor the set line: %q", got)
	}
}

func TestOneEventPerRoomAtATime(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true, bystander: true}, res: Result{Kind: KindSeen, Text: seenEvent}}
	room := street(t, g)
	var held *delivery
	startDelivery = func(d *delivery) { held = d }
	if !TryReplace(room, setLine, nil) || TryReplace(room, setLine, nil) {
		t.Fatal("one event on its way per room")
	}
	room.RemovePlayer(keyholder)
	room.RemovePlayer(bystander)
	held.run(false)
	if Pending() != 0 {
		t.Fatal("freed")
	}
}

func TestCleanResult(t *testing.T) {
	ok := []struct {
		kind, text string
		canSee     bool
		want       string
	}{
		{KindSeen, seenEvent, true, seenEvent},
		{KindHeard, heardEvent, false, heardEvent},
		{KindHeard, `you hear a cart wheel squeal somewhere out of sight.`, true, `You hear a cart wheel squeal somewhere out of sight.`},
		{KindSeen, "*A shutter bangs — twice — in the wind.*", true, `A shutter bangs, twice, in the wind.`},
	}
	for _, c := range ok {
		got, err := CleanResult(Result{Kind: c.kind, Text: c.text}, c.canSee)
		if err != nil || got.Text != c.want {
			t.Errorf("CleanResult(%q) = %q, %v; want %q", c.text, got.Text, err, c.want)
		}
	}
	bad := []struct {
		r      Result
		canSee bool
	}{
		{Result{Kind: KindSeen, Text: seenEvent}, false},
		{Result{Kind: `smelled`, Text: seenEvent}, true},
		{Result{Kind: KindSeen, Text: `A dog trots up and licks your hand.`}, true},
		{Result{Kind: KindHeard, Text: `A dog barks somewhere behind the houses.`}, true},
		{Result{Kind: KindHeard, Text: `You hear someone call your name.`}, true},
		{Result{Kind: KindSeen, Text: `Rain.`}, true},
		{Result{Kind: KindSeen, Text: strings.Repeat(`word `, 70)}, true},
		{Result{Kind: KindSeen, Text: "A sign creaks: 中文 painted on it."}, true},
	}
	for _, b := range bad {
		if _, err := CleanResult(b.r, b.canSee); !errors.Is(err, ErrUnusable) {
			t.Errorf("CleanResult(%+v, %v) should be unusable, got %v", b.r, b.canSee, err)
		}
	}
}

// blind makes a character blind, the way the engine's perception machine
// does.
func blind(t *testing.T, c *characters.Character) {
	t.Helper()
	if c.Perception == nil {
		c.Perception = perception.NewMachine()
	}
	if err := c.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}); err != nil {
		t.Fatalf("blinding: %v", err)
	}
}

// asleep puts a character to sleep with a condition carrying the Sleeping
// flag.
func asleep(t *testing.T, c *characters.Character) {
	t.Helper()
	const sleepId = 99001
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sleepId: {ConditionId: sleepId, Name: "Test Sleep", Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	if err := c.AddCondition(sleepId, true); err != nil {
		t.Fatalf("sleeping: %v", err)
	}
}

// A place hook (a rift) sets its own chance, so long as the module's is on,
// and its setting; a timeless place has no time of day.
func TestAPlaceHookSetsChanceAndSetting(t *testing.T) {
	g := &fakeGen{chance: 10, allow: map[int]bool{keyholder: true}, res: Result{Kind: KindSeen, Text: seenEvent}}
	room := street(t, g)
	roll = func() int { return 15 } // above the world's 10, below the rift's 20
	t.Cleanup(func() { SetPlaceHook(nil) })

	if TryReplace(room, setLine, []string{setLine}) {
		t.Fatal("an ordinary room keeps the module's chance")
	}
	SetPlaceHook(func(r *rooms.Room) (Place, bool) {
		return Place{Chance: 20, Setting: `A maze of black crystal.`, Timeless: true}, r.RoomId == testRoom
	})
	if !TryReplace(room, setLine, []string{setLine}) {
		t.Fatal("the place's own chance is used")
	}
	if g.req.Setting != `A maze of black crystal.` || g.req.TimeOfDay != `` {
		t.Fatalf("setting sent, no time of day: %+v", g.req)
	}

	g.chance = 0
	if TryReplace(room, setLine, []string{setLine}) {
		t.Fatal("turning generation off turns it off in the place too")
	}
}
