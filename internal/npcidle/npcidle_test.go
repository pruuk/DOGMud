package npcidle

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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
	testRoom    = 1
	testOther   = 2
	testPlayer  = 7
	testMob     = 100
	testCat     = 101
	playerName  = `Zyxwaldo`
	setEmote    = `emote wipes down the counter`
	momentEmote = `flaps a rag at a sparrow that has claimed the top shelf`
)

// world seeds a room holding a player who can see, a shopkeeper and a cat,
// runs deliveries in line, and makes every roll hit.
func world(t *testing.T, g Generator) (*mobs.Mob, *users.UserRecord) {
	t.Helper()
	keeper := &mobs.Mob{MobId: 50, InstanceId: testMob, Character: *characters.New(),
		IdleCommands: []string{setEmote, ``, `wander`, `say Slow day.`}}
	keeper.Character.Name = `Old Brask`
	keeper.Character.Description = `A stooped shopkeeper with flour on his sleeves.`
	keeper.Character.RoomId = testRoom
	cat := &mobs.Mob{MobId: 51, InstanceId: testCat, Character: *characters.New()}
	cat.Character.Name = `a tabby cat`
	cat.Character.RoomId = testRoom
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{50: keeper, 51: cat},
		map[int]*mobs.Mob{testMob: keeper, testCat: cat}))

	u := users.NewUserRecord(testPlayer, 0)
	u.Character = characters.New()
	u.Character.Name = playerName
	u.Character.RoomId = testRoom
	u.Character.SetUserId(testPlayer)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{testPlayer: u}))

	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		testRoom:  {RoomId: testRoom, Zone: `Testville`, Title: `The Flour Shop`, Description: `Sacks lean against a crooked shelf.`},
		testOther: {RoomId: testOther, Zone: `Testville`, Title: `The Lane`},
	}, map[string]*rooms.ZoneConfig{}))
	room := rooms.LoadRoom(testRoom)
	room.AddPlayer(testPlayer)
	room.AddMob(testMob)
	room.AddMob(testCat)

	oldRoll, oldShuffle, oldStart, oldSee := roll, shuffle, startDelivery, canSee
	roll = func() int { return 0 }
	shuffle = func([]int) {}
	startDelivery = func(d *delivery) { d.run(false) }
	canSee = func(int, *rooms.Room) bool { return true }
	SetGenerator(g)
	t.Cleanup(func() {
		roll, shuffle, startDelivery, canSee = oldRoll, oldShuffle, oldStart, oldSee
		SetGenerator(nil)
		events.DrainQueuedInputsForTest(testMob)
	})
	events.DrainQueuedInputsForTest(testMob)
	return keeper, u
}

func TestNoGeneratorNoMoment(t *testing.T) {
	keeper, _ := world(t, nil)
	if TryReplace(keeper, setEmote) {
		t.Fatal("with no generator installed every idle line is the set one")
	}
}

func TestAMomentReplacesTheSetLine(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true}, res: Result{Kind: KindEmote, Text: momentEmote}}
	keeper, _ := world(t, g)
	if !TryReplace(keeper, setEmote) {
		t.Fatal("the roll hit and the player's key may be used: the event is taken")
	}
	if g.gens != 1 || g.user != testPlayer {
		t.Fatalf("written once, on the player's key (%d, %d)", g.gens, g.user)
	}
	got := events.DrainQueuedInputsForTest(testMob)
	if len(got) != 1 || got[0] != `emote `+momentEmote {
		t.Fatalf("the NPC acts the moment through its own emote, and not the set line: %q", got)
	}
	if Pending() != 0 {
		t.Fatal("nothing left pending")
	}
}

func TestTheRequestCarriesTheNPCAndRoomButNoPlayer(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true}, res: Result{Kind: KindSay, Text: `Blasted shelf, stay put.`}}
	keeper, _ := world(t, g)
	TryReplace(keeper, setEmote)
	r := g.req
	if r.NPC.Name != `Old Brask` || !strings.Contains(r.NPC.Description, `flour`) {
		t.Fatalf("the NPC's own text: %+v", r.NPC)
	}
	if r.Place.Title != `The Flour Shop` || !strings.Contains(r.Place.Description, `crooked shelf`) {
		t.Fatalf("the room's text: %+v", r.Place)
	}
	if len(r.Others) != 1 || r.Others[0] != `a tabby cat` {
		t.Fatalf("others present, by name, not itself: %v", r.Others)
	}
	if r.Travellers != 1 {
		t.Fatalf("players are counted: %d", r.Travellers)
	}
	if len(r.NPC.Examples) != 2 || r.NPC.Examples[0] != setEmote {
		t.Fatalf("its own flavor lines only, for its voice: %v", r.NPC.Examples)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), playerName) {
		t.Fatal("no player is ever named in a request")
	}
	if got := events.DrainQueuedInputsForTest(testMob); len(got) != 1 || got[0] != `say Blasted shelf, stay put.` {
		t.Fatalf("a say is said: %q", got)
	}
}

func TestOnlyFlavorIsReplaced(t *testing.T) {
	g := &fakeGen{chance: 100, allow: map[int]bool{testPlayer: true}}
	keeper, _ := world(t, g)
	for _, cmd := range []string{``, `lookfortrouble`, `wander`, `emote`, `say   `, `emote waves;say hi`, `cast heal`} {
		if TryReplace(keeper, cmd) {
			t.Fatalf("%q is not flavor", cmd)
		}
	}
	if len(g.reserved) != 0 {
		t.Fatal("no key is even asked for")
	}
}

func TestAMissedRollTakesNothing(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true}}
	keeper, _ := world(t, g)
	roll = func() int { return 5 }
	if TryReplace(keeper, setEmote) || len(g.reserved) != 0 {
		t.Fatal("a roll of 5 misses a 5% chance")
	}
	g.chance = 0
	roll = func() int { return 0 }
	if TryReplace(keeper, setEmote) {
		t.Fatal("a chance of 0 never hits")
	}
}

func TestNoUsableKeyNoMoment(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{}}
	keeper, _ := world(t, g)
	if TryReplace(keeper, setEmote) || g.gens != 0 {
		t.Fatal("a player without a key the generator may use writes nothing")
	}
	if Pending() != 0 {
		t.Fatal("and nothing is left pending")
	}
}

// Light: a player who cannot see the NPC clearly (a dark room, blindness)
// never has their key write one; the set line runs under its own sight
// rules instead.
func TestAPlayerWhoCannotSeeWritesNothing(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true}}
	keeper, _ := world(t, g)
	canSee = func(int, *rooms.Room) bool { return false }
	if TryReplace(keeper, setEmote) || len(g.reserved) != 0 {
		t.Fatal("no moment, and their key is not even asked for")
	}
}

func TestFailureRunsTheSetLine(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true}, err: errors.New(`provider said no`)}
	keeper, _ := world(t, g)
	if !TryReplace(keeper, setEmote) {
		t.Fatal("taken")
	}
	if got := events.DrainQueuedInputsForTest(testMob); len(got) != 1 || got[0] != setEmote {
		t.Fatalf("no moment came: the set line runs after all (%q)", got)
	}

	g.err = nil
	g.res = Result{Kind: KindEmote, Text: `glares at you over the counter`}
	TryReplace(keeper, setEmote)
	if got := events.DrainQueuedInputsForTest(testMob); len(got) != 1 || got[0] != setEmote {
		t.Fatalf("a moment the rules refuse is no moment: the set line runs (%q)", got)
	}
}

func TestAMomentThatCannotBeModeratedIsTheKeyholdersAlone(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true},
		res: Result{Kind: KindEmote, Text: momentEmote, KeyholderOnly: true}}
	keeper, _ := world(t, g)
	if !TryReplace(keeper, setEmote) {
		t.Fatal("taken")
	}
	if got := events.DrainQueuedInputsForTest(testMob); len(got) != 0 {
		t.Fatalf("the NPC does not act it for the room, nor the set line: %q", got)
	}
}

func TestAMobThatMovedOnActsNothing(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true}, res: Result{Kind: KindEmote, Text: momentEmote}}
	keeper, _ := world(t, g)
	var held *delivery
	startDelivery = func(d *delivery) { held = d }
	if !TryReplace(keeper, setEmote) {
		t.Fatal("taken")
	}
	if TryReplace(keeper, setEmote) {
		t.Fatal("one moment on its way per NPC")
	}
	keeper.Character.RoomId = testOther
	held.run(false)
	if got := events.DrainQueuedInputsForTest(testMob); len(got) != 0 {
		t.Fatalf("it left the room: nothing, not even the set line (%q)", got)
	}
	if Pending() != 0 {
		t.Fatal("freed")
	}
}

func TestCompanionsAreExcluded(t *testing.T) {
	g := &fakeGen{chance: 5, allow: map[int]bool{testPlayer: true}, res: Result{Kind: KindEmote, Text: momentEmote}}
	keeper, _ := world(t, g)

	keeper.Groups = []string{mobs.HollowGroup}
	if TryReplace(keeper, setEmote) {
		t.Fatal("a companion waiting in the Hollow is the companion module's")
	}
	keeper.Groups = nil

	companionai.SetBondedCheck(func(id int) bool { return id == testMob })
	t.Cleanup(func() { companionai.SetBondedCheck(nil) })
	if TryReplace(keeper, setEmote) {
		t.Fatal("a bonded companion is the companion module's")
	}
	companionai.SetBondedCheck(nil)

	companionai.SetDrivesCheck(func(mobId int) bool { return mobId == 50 })
	t.Cleanup(func() { companionai.SetDrivesCheck(nil) })
	if TryReplace(keeper, setEmote) {
		t.Fatal("any instance of a companion's template is excluded")
	}
	if g.gens != 0 {
		t.Fatal("nothing was written")
	}
}

func TestIsFlavor(t *testing.T) {
	for cmd, want := range map[string]bool{
		`emote polishes a cup`: true, `say Cold out.`: true, `EMOTE yawns widely`: true,
		``: false, `emote`: false, `emote   `: false, `wander`: false, `say hi;emote waves`: false, `sayto bob hi`: false,
	} {
		if IsFlavor(cmd) != want {
			t.Errorf("IsFlavor(%q) = %v", cmd, !want)
		}
	}
}

func TestCleanResult(t *testing.T) {
	ok := []struct{ kind, text, name, want string }{
		{KindEmote, `swats at a moth and misses`, `Old Brask`, `swats at a moth and misses`},
		{KindEmote, `Old Brask swats at a moth and misses`, `Old Brask`, `swats at a moth and misses`},
		{KindEmote, `the miller sneezes into a cloud of flour`, `The miller`, `sneezes into a cloud of flour`},
		{KindEmote, `*stamps a boot — hard — at a rat*`, `Old Brask`, `stamps a boot, hard, at a rat`},
		{KindSay, `"Stay on the shelf, you wretched jar!"`, `Old Brask`, `Stay on the shelf, you wretched jar!`},
		{KindSay, "“Mind the step.”", `x`, `Mind the step.`},
		{KindEmote, `mutters <ansi fg="red">darkly</ansi> at the hearth`, `x`, `mutters darkly at the hearth`},
	}
	for _, c := range ok {
		got, err := CleanResult(Result{Kind: c.kind, Text: c.text}, c.name)
		if err != nil || got.Text != c.want || got.Kind != c.kind {
			t.Errorf("CleanResult(%q) = %q, %v; want %q", c.text, got.Text, err, c.want)
		}
	}
	bad := []Result{
		{Kind: `shout`, Text: `a perfectly fine line here`},
		{Kind: KindEmote, Text: `waves at you cheerfully`},
		{Kind: KindEmote, Text: `eyes your purse with interest`},
		{Kind: KindEmote, Text: `smiles`},
		{Kind: KindSay, Text: `Hm.`},
		{Kind: KindSay, Text: strings.Repeat(`word `, 60)},
		{Kind: KindEmote, Text: `mutters in 中文 at the shelf`},
	}
	for _, r := range bad {
		if _, err := CleanResult(r, `Old Brask`); !errors.Is(err, ErrUnusable) {
			t.Errorf("CleanResult(%+v) should be unusable, got %v", r, err)
		}
	}
	if got, _ := CleanResult(Result{Kind: KindEmote, Text: momentEmote, KeyholderOnly: true}, ``); !got.KeyholderOnly {
		t.Error("KeyholderOnly survives cleaning")
	}
}

func TestReplySchemaIsStrict(t *testing.T) {
	s := ReplySchema()
	if s[`additionalProperties`] != false {
		t.Fatal("strict schemas forbid extra properties")
	}
	req, _ := s[`required`].([]string)
	if strings.Join(req, `,`) != `kind,text` {
		t.Fatalf("every property is required: %v", req)
	}
	r, err := ParseReply(`{"kind":"emote","text":"yawns behind a floury hand"}`)
	if err != nil || r.Kind != KindEmote {
		t.Fatalf("parses: %+v %v", r, err)
	}
	if _, err := ParseReply(`not json`); err == nil {
		t.Fatal("refuses what is not JSON")
	}
}

// light is a room's light level, for the engine's own sight judgement.
type light int

func (l light) LightLevel() int { return int(l) }

// The real light rules, not a stand-in: a player in a pitch-dark room does
// not see the NPC clearly, so their key writes nothing; in a lit room they
// do.
func TestOnlyAPlayerWhoSeesClearlyIsAsked(t *testing.T) {
	world(t, nil)
	if seesClearly(testPlayer, light(0)) {
		t.Fatal("pitch dark: not clearly")
	}
	if !seesClearly(testPlayer, light(100)) {
		t.Fatal("lit: clearly")
	}
	if seesClearly(999, light(100)) {
		t.Fatal("nobody there sees nothing")
	}
}

// The keyholder-only fallback is judged by the same light rules as any NPC
// emote, by the room's own sight-gated sender at the moment it is shown:
// seen and named in a lit room, nothing at all in the dark (a say too: it
// is shown as a seen mutter), and nothing to anyone else in the room.
func TestAKeyholdersMomentFollowsTheLightRules(t *testing.T) {
	world(t, nil)
	room := rooms.LoadRoom(testRoom)
	none, lamp := 0.0, 100
	room.SkyLight = &none
	room.Lamp = &lamp

	other := users.NewUserRecord(8, 0)
	other.Character = characters.New()
	other.Character.RoomId = testRoom
	other.Character.SetUserId(8)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{testPlayer: users.GetByUserId(testPlayer), 8: other}))
	room.AddPlayer(8)
	events.DrainQueuedMessagesForTest(testPlayer)
	events.DrainQueuedMessagesForTest(8)
	emote := Result{Kind: KindEmote, Text: momentEmote}
	say := Result{Kind: KindSay, Text: `Blasted shelf.`}

	sendToKeyholder(testPlayer, room, `Old Brask`, emote)
	got := strings.Join(events.DrainQueuedMessagesForTest(testPlayer), ``)
	if !strings.Contains(got, `Old Brask`) || !strings.Contains(got, `sparrow`) {
		t.Fatalf("lit: seen, named: %q", got)
	}
	sendToKeyholder(testPlayer, room, `Old Brask`, say)
	if got := strings.Join(events.DrainQueuedMessagesForTest(testPlayer), ``); !strings.Contains(got, `Blasted shelf.`) {
		t.Fatalf("lit: a say is seen muttered: %q", got)
	}
	if got := events.DrainQueuedMessagesForTest(8); len(got) != 0 {
		t.Fatalf("nobody else in the room reads it: %q", got)
	}

	room.Lamp = nil
	if room.IsLit() {
		t.Fatal("fixture: the room is dark without its lamp")
	}
	sendToKeyholder(testPlayer, room, `Old Brask`, emote)
	sendToKeyholder(testPlayer, room, `Old Brask`, say)
	if got := events.DrainQueuedMessagesForTest(testPlayer); len(got) != 0 {
		t.Fatalf("dark: not seen at all: %q", got)
	}

	room.Lamp = &lamp
	other.Character.RoomId = testOther
	sendToKeyholder(8, room, `Old Brask`, emote)
	if got := events.DrainQueuedMessagesForTest(8); len(got) != 0 {
		t.Fatalf("a keyholder who has left the room is shown nothing: %q", got)
	}
}
