package lookdetail

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

type fakeGen struct {
	allow map[int]bool
	res   Result
	err   error
	gens  int
	req   Request
	hold  bool // leave the delivery for the test to run
}

func (g *fakeGen) Reserve(userId int) bool { return g.allow[userId] }
func (g *fakeGen) Generate(ctx context.Context, userId int, req Request) (Result, error) {
	g.gens++
	g.req = req
	return g.res, g.err
}

const (
	testRoom   = 1
	otherRoom  = 2
	looker     = 7
	onlooker   = 8
	playerName = `Zyxwaldo`
	roomText   = `A dry <ansi fg="cyan">fountain</ansi> stands in the square, its basin
choked with leaves. Pigeons crowd the eaves of the old granary. A cartwright's
sign creaks overhead.`
	detail = `Moss has crept into every crack of the basin, and someone has scratched a lopsided heart into the rim.`
)

var held *delivery

func square(t *testing.T, g Generator) *rooms.Room {
	t.Helper()
	seed := map[int]*users.UserRecord{}
	for _, id := range []int{looker, onlooker} {
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
		testRoom:  {RoomId: testRoom, Zone: `Testville`, Title: `Market Square`, Description: roomText, SkyLight: &none, Lamp: &lamp},
		otherRoom: {RoomId: otherRoom, Zone: `Testville`, Title: `Lane`, SkyLight: &none, Lamp: &lamp},
	}, map[string]*rooms.ZoneConfig{}))
	room := rooms.LoadRoom(testRoom)
	room.AddPlayer(looker)
	room.AddPlayer(onlooker)

	ResetCacheForTest()
	restore := InlineForTest()
	if gg, ok := g.(*fakeGen); ok && gg.hold {
		restore()
		prev := startDelivery
		startDelivery = func(d *delivery) { held = d }
		restore = func() { startDelivery = prev }
	}
	SetGenerator(g)
	t.Cleanup(func() {
		restore()
		SetGenerator(nil)
		ResetCacheForTest()
		drain()
	})
	drain()
	return room
}

func drain() {
	events.DrainQueuedMessagesForTest(looker)
	events.DrainQueuedMessagesForTest(onlooker)
}

func read(id int) string { return strings.Join(events.DrainQueuedMessagesForTest(id), ``) }

func user(id int) *users.UserRecord { return users.GetByUserId(id) }

func TestPhrase(t *testing.T) {
	for in, want := range map[string]string{
		`fountain`: `fountain`, `The Fountain`: `fountain`, `  old   granary `: `old granary`, `a sign`: `sign`,
	} {
		if got, ok := Phrase(in); !ok || got != want {
			t.Errorf("Phrase(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{``, `it`, `the`, `here`, `everything`, `x1`, `<ansi>`, `a very long phrase made of words`, strings.Repeat(`a`, 41)} {
		if _, ok := Phrase(in); ok {
			t.Errorf("Phrase(%q) should be refused", in)
		}
	}
}

func TestFind(t *testing.T) {
	cases := []struct{ phrase, shown string }{
		{`fountain`, `fountain`},
		{`fountains`, `fountain`},
		{`pigeon`, `pigeons`},
		{`old granary`, `old granary`},
		{`basin`, `basin`},
	}
	for _, c := range cases {
		shown, excerpt, ok := Find(roomText, c.phrase)
		if !ok || shown != c.shown {
			t.Errorf("Find(%q) = %q %v, want %q", c.phrase, shown, ok, c.shown)
		}
		if ok && (strings.Contains(excerpt, `<`) || excerpt == ``) {
			t.Errorf("the excerpt is the plain sentence: %q", excerpt)
		}
	}
	if _, ex, _ := Find(roomText, `pigeons`); !strings.HasPrefix(ex, `Pigeons crowd`) || strings.Contains(ex, `fountain`) {
		t.Fatalf("the excerpt is the one sentence: %q", ex)
	}
	for _, p := range []string{`cart`, `dragon`, `foun`} {
		if _, _, ok := Find(roomText, p); ok {
			t.Errorf("%q is not named as a whole word", p)
		}
	}
}

func TestACloserLookIsWrittenShownAndCached(t *testing.T) {
	g := &fakeGen{allow: map[int]bool{looker: true}, res: Result{Text: detail}}
	room := square(t, g)
	if !TryLook(user(looker), room, `fountain`) {
		t.Fatal("answered")
	}
	got := read(looker)
	if !strings.Contains(got, `You look at the <ansi fg="noun">fountain</ansi>:`) || !strings.Contains(got, `lopsided heart`) {
		t.Fatalf("shown as a room noun is: %q", got)
	}
	if seen := read(onlooker); !strings.Contains(seen, `is examining the`) || strings.Contains(seen, `lopsided`) {
		t.Fatalf("onlookers see the look, not the detail: %q", seen)
	}
	r := g.req
	if r.Thing != `fountain` || !strings.Contains(r.Excerpt, `dry fountain`) || r.Title != `Market Square` {
		t.Fatalf("the request: %+v", r)
	}
	if b, _ := json.Marshal(r); strings.Contains(string(b), playerName) {
		t.Fatal("no player is named in a request")
	}

	// Anyone else, key or not, reads the same detail from the cache.
	SetGenerator(nil)
	if !TryLook(user(onlooker), room, `fountains`) || !strings.Contains(read(onlooker), `lopsided heart`) || g.gens != 1 {
		t.Fatal("the cache answers, at no cost, whatever spelling finds it")
	}
	// A changed description starts afresh.
	room.Description = `A dry fountain, newly scrubbed, gleams in the square.`
	if TryLook(user(onlooker), room, `fountain`) {
		t.Fatal("not cached for the new description, and no generator")
	}
}

func TestNotInTheDescriptionIsNotAnswered(t *testing.T) {
	g := &fakeGen{allow: map[int]bool{looker: true}, res: Result{Text: detail}}
	room := square(t, g)
	for _, what := range []string{`dragon`, `cart`, `it`} {
		if TryLook(user(looker), room, what) {
			t.Fatalf("%q is not in the description: the look answers as always", what)
		}
	}
	if g.gens != 0 {
		t.Fatal("nothing was written")
	}
}

func TestNoKeyNoCacheIsNotAnswered(t *testing.T) {
	g := &fakeGen{allow: map[int]bool{}}
	room := square(t, g)
	if TryLook(user(looker), room, `fountain`) || Pending() != 0 {
		t.Fatal("no key it may use and nothing cached: the look answers as always")
	}
}

func TestAFailedLookSaysNothingSpecial(t *testing.T) {
	g := &fakeGen{allow: map[int]bool{looker: true}, err: errors.New(`provider said no`)}
	room := square(t, g)
	TryLook(user(looker), room, `fountain`)
	if got := read(looker); !strings.Contains(got, `You see nothing special about the`) {
		t.Fatalf("it is there, but nothing came: %q", got)
	}
	if read(onlooker) != `` {
		t.Fatal("onlookers see nothing")
	}
	g.err, g.res = nil, Result{Text: `Too short.`}
	TryLook(user(looker), room, `fountain`)
	if got := read(looker); !strings.Contains(got, `nothing special`) {
		t.Fatalf("a detail the rules refuse is no detail: %q", got)
	}
}

func TestAKeyholdersDetailIsNotCached(t *testing.T) {
	g := &fakeGen{allow: map[int]bool{looker: true}, res: Result{Text: detail, KeyholderOnly: true}}
	room := square(t, g)
	TryLook(user(looker), room, `fountain`)
	if !strings.Contains(read(looker), `lopsided heart`) {
		t.Fatal("the keyholder reads it")
	}
	SetGenerator(nil)
	if TryLook(user(onlooker), room, `fountain`) {
		t.Fatal("unmoderated text is never cached for anyone else")
	}
}

// Light, judged when the detail arrives by the look command's own rules: a
// looker in the dark by then is told they cannot see; one gone is told
// nothing.
func TestTheLightIsJudgedWhenItArrives(t *testing.T) {
	g := &fakeGen{allow: map[int]bool{looker: true}, res: Result{Text: detail}, hold: true}
	room := square(t, g)
	TryLook(user(looker), room, `fountain`)
	if TryLook(user(looker), room, `granary`) != true || g.gens != 0 {
		t.Fatal("a second look while one is on its way waits on it")
	}
	room.Lamp = nil
	held.run(false)
	if got := read(looker); !strings.Contains(got, `You can't see anything!`) || strings.Contains(got, `lopsided`) {
		t.Fatalf("dark by the time it came: %q", got)
	}
	if Pending() != 0 {
		t.Fatal("freed")
	}

	lamp := 100
	room.Lamp = &lamp
	ResetCacheForTest()
	TryLook(user(looker), room, `fountain`)
	user(looker).Character.RoomId = otherRoom
	held.run(false)
	if got := read(looker); got != `` {
		t.Fatalf("gone: nothing: %q", got)
	}
}

func TestCleanResult(t *testing.T) {
	got, err := CleanResult(Result{Text: "*You notice — faintly — a smell of <b>rain</b>.*"})
	if err != nil || got.Text != `You notice, faintly, a smell of rain.` {
		t.Fatalf("cleaned: %q %v", got.Text, err)
	}
	for _, bad := range []string{`Short.`, strings.Repeat(`word `, 150), "A sign in 中文 hangs here, faded."} {
		if _, err := CleanResult(Result{Text: bad}); !errors.Is(err, ErrUnusable) {
			t.Errorf("CleanResult(%q) should be unusable", bad)
		}
	}
}
