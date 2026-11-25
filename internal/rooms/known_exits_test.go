package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// knownExitsRoom is seedRegistry's room 1 (north, south, east locked at 10,
// west secret) made a sky-less cave lit by a lamp, with a viewer who has found
// nothing. Pass lit=false for a pitch-dark room. The one-letter compass
// aliases are seeded so a guess like "w" takes the alias path.
func knownExitsRoom(t *testing.T, lit bool) (*Room, *characters.Character) {
	t.Helper()
	t.Cleanup(keywords.SeedKeywordsForTest(keywords.Aliases{DirectionAliases: map[string]string{
		"n": "north", "s": "south", "e": "east", "w": "west", "u": "up", "d": "down",
	}}))
	t.Cleanup(seedRegistry())
	t.Cleanup(SeedBiomesForTest(map[string]*BiomeInfo{
		"cave":    {BiomeId: "cave", SkyLight: SkyLightPtr(0.0)},
		"city":    {BiomeId: "city"},
		"default": {BiomeId: "default"},
	}))
	r := roomManager.rooms[1]
	r.Biome = "cave"
	if lit {
		r.Lamp = LampPtr(60)
	}
	viewer := users.NewTestUser(7801, "knower", "Knower", 97801)
	want := messaging.SightFull
	if !lit {
		want = messaging.SightNone
	}
	if got := messaging.ParticipantSight(viewer.Character, r); got != want {
		t.Fatalf("fixture premise: sight %v, want %v", got, want)
	}
	return r, viewer.Character
}

func knownNames(l KnownExitList) []string {
	out := []string{}
	for _, ke := range l.Exits {
		out = append(out, ke.Name)
	}
	return out
}

func knownExit(t *testing.T, l KnownExitList, name string) KnownExit {
	t.Helper()
	for _, ke := range l.Exits {
		if ke.Name == name {
			return ke
		}
	}
	t.Fatalf("exit %q not in %v", name, knownNames(l))
	return KnownExit{}
}

// Ruling 1: a secret exit is known only once found.
func TestKnownExits_SecretExitUnknownUntilFound(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	got := KnownExits(r, c)
	if got.Dark {
		t.Fatal("a lit room must not read dark")
	}
	if want := []string{"north", "east", "south"}; !equalStrings(knownNames(got), want) {
		t.Fatalf("known exits %v, want %v (west is an unfound secret)", knownNames(got), want)
	}
	c.LearnSecretExit(1, "west")
	if want := []string{"north", "east", "south", "west"}; !equalStrings(knownNames(KnownExits(r, c)), want) {
		t.Fatalf("after the find: %v, want %v", knownNames(KnownExits(r, c)), want)
	}
}

// Ruling 3: locked means locked now, not "has a lock".
func TestKnownExits_LockedNowVersusHasALock(t *testing.T) {
	// Test binaries load Go timing defaults, not config.yaml; pin the shipped
	// timing so the relock hour is a real span of rounds (gamelock_test.go
	// does the same), and start past RoundCountMinimum as the game does.
	c0 := configs.GetConfig()
	c0.Timing.RoundsPerDay = 900
	c0.Timing.RoundSeconds = 4
	c0.Timing.Validate()
	configs.SetConfigForTest(t, c0)
	util.SetRoundCountForTest(uint64(util.RoundCountMinimum) + 1000)
	t.Cleanup(util.ResetRoundCountForTest)
	r, c := knownExitsRoom(t, true)
	east := knownExit(t, KnownExits(r, c), "east")
	if !east.LockedNow || !east.Exit.HasLock() {
		t.Fatalf("east: lockedNow %v hasLock %v, want both true", east.LockedNow, east.Exit.HasLock())
	}
	r.SetExitLock("east", false)
	east = knownExit(t, KnownExits(r, c), "east")
	if east.LockedNow {
		t.Error("an unlocked door must not read locked now")
	}
	if !east.Exit.HasLock() {
		t.Error("an unlocked door still has its lock")
	}
	if knownExit(t, KnownExits(r, c), "north").LockedNow {
		t.Error("a door with no lock is never locked")
	}
}

// Temporary and mutator exits are listed, each with its kind.
func TestKnownExits_TemporaryAndMutatorExitsAreListed(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: "test-known-bridge",
		Exits:     map[string]exit.RoomExit{"bridge": {RoomId: 2}, "northeast": {RoomId: 3}},
	}))
	if !GetZoneConfig("TestZone").Mutators.Add("test-known-bridge") {
		t.Fatal("fixture premise: the mutator must attach")
	}
	if !r.AddTemporaryExit("gangplank", exit.TemporaryRoomExit{RoomId: 4}) {
		t.Fatal("fixture premise: the temporary exit must attach")
	}
	got := KnownExits(r, c)
	want := []string{"north", "east", "south", "northeast", "bridge", "gangplank"}
	if !equalStrings(knownNames(got), want) {
		t.Fatalf("known exits %v, want %v", knownNames(got), want)
	}
	for name, kind := range map[string]ExitKind{
		"north": ExitCompass, "northeast": ExitDiagonal, "bridge": ExitNamed, "gangplank": ExitTemporary,
	} {
		if got := knownExit(t, got, name).Kind; got != kind {
			t.Errorf("%s kind %q, want %q", name, got, kind)
		}
	}
	if gp := knownExit(t, got, "gangplank"); gp.RoomId != 4 || gp.Temp.Title != "gangplank" {
		t.Errorf("gangplank: room %d title %q, want 4 and its title", gp.RoomId, gp.Temp.Title)
	}
}

// A temporary exit named like a named room exit sorts after it: Room.Info
// keeps the first of a name, and that name reaches the room exit
// (FindExitByName), so the room exit must come first.
func TestKnownExits_RoomExitBeforeATemporaryExitOfTheSameName(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	r.Exits["out"] = exit.RoomExit{RoomId: 2}
	r.ExitsTemp = map[string]exit.TemporaryRoomExit{"out": {RoomId: 4, Title: "out"}}
	got := KnownExits(r, c)
	if want := []string{"north", "east", "south", "out", "out"}; !equalStrings(knownNames(got), want) {
		t.Fatalf("known exits %v, want %v", knownNames(got), want)
	}
	if got.Exits[3].Kind == ExitTemporary || got.Exits[4].Kind != ExitTemporary {
		t.Fatalf("the room exit out must come before the temporary out: %v then %v", got.Exits[3].Kind, got.Exits[4].Kind)
	}
}

// At no sight nothing is listed and the list says dark.
func TestKnownExits_DarkListsNothing(t *testing.T) {
	r, c := knownExitsRoom(t, false)
	c.LearnSecretExit(1, "west")
	got := KnownExits(r, c)
	if !got.Dark {
		t.Fatal("a pitch-dark room must read dark")
	}
	if len(got.Exits) != 0 {
		t.Fatalf("the dark listed %v", knownNames(got))
	}
}

func TestExitKindOf(t *testing.T) {
	for name, want := range map[string]ExitKind{
		"north": ExitCompass, "west": ExitCompass, "up": ExitVertical, "down": ExitVertical,
		"southwest": ExitDiagonal, "out": ExitNamed, "crack": ExitNamed,
	} {
		if got := ExitKindOf(name, false); got != want {
			t.Errorf("ExitKindOf(%q) = %q, want %q", name, got, want)
		}
	}
	if got := ExitKindOf("north", true); got != ExitTemporary {
		t.Errorf("a temporary exit is temporary whatever its name, got %q", got)
	}
}

// A guess at an unfound secret exit matches nothing: by its name, by its alias
// and by a prefix. The unfiltered lookup still finds it (mob moves use it).
func TestFindKnownExitByName_AnUnfoundSecretExitMatchesNothing(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	for _, guess := range []string{"west", "w", "wes"} {
		if name, id := r.FindKnownExitByName(c, guess); name != "" || id != 0 {
			t.Errorf("guess %q found %q (%d) before the find", guess, name, id)
		}
	}
	if name, _ := r.FindExitByName("west"); name != "west" {
		t.Fatalf("the unfiltered lookup must still find west, got %q", name)
	}
	if name, _ := r.FindKnownExitByName(c, "north"); name != "north" {
		t.Errorf("an ordinary exit must be found, got %q", name)
	}
	c.LearnSecretExit(1, "west")
	if name, id := r.FindKnownExitByName(c, "west"); name != "west" || id != 5 {
		t.Errorf("after the find: %q (%d), want west (5)", name, id)
	}
	if name, _ := r.FindKnownExitByName(nil, "west"); name != "" {
		t.Errorf("a nil character knows no secret exit, found %q", name)
	}
}

func TestExitKnownTo(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	if !r.ExitKnownTo(c, "north") || r.ExitKnownTo(c, "west") || r.ExitKnownTo(c, "nowhere") {
		t.Fatal("north known, west (unfound secret) and nowhere (no exit) not known")
	}
	c.LearnSecretExit(1, "west")
	if !r.ExitKnownTo(c, "west") {
		t.Error("a found secret exit is known")
	}
}

// FindKnownExitTo: the way to a room, among the exits the mover knows.
func TestFindKnownExitTo(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	if got := r.FindKnownExitTo(c, 5); got != "" {
		t.Fatalf("the only way to room 5 is an unfound secret exit, got %q", got)
	}
	if got := r.FindExitTo(5); got != "west" {
		t.Fatalf("the unfiltered FindExitTo must still answer west, got %q", got)
	}
	c.LearnSecretExit(1, "west")
	if got := r.FindKnownExitTo(c, 5); got != "west" {
		t.Errorf("after the find: %q, want west", got)
	}
}

// Ruling 13 (an NPC's own wandering): a random exit may be a secret exit the
// mover found, never one it has not. Locked exits stay out, as before.
func TestGetRandomKnownExit(t *testing.T) {
	r := &Room{RoomId: 77, Exits: map[string]exit.RoomExit{
		"west": {RoomId: 5, Secret: true},
		"east": {RoomId: 6, Lock: gamelock.Lock{Difficulty: 10}},
	}}
	c := &characters.Character{}
	for range 50 {
		if name, _ := r.GetRandomKnownExit(c); name != "" {
			t.Fatalf("picked %q: west is unfound and east is locked", name)
		}
	}
	c.LearnSecretExit(77, "west")
	for range 50 {
		if name, id := r.GetRandomKnownExit(c); name != "west" || id != 5 {
			t.Fatalf("picked %q (%d), want the found west (5)", name, id)
		}
	}
	if name, _ := r.GetRandomExit(); name != "" {
		t.Errorf("GetRandomExit still skips every secret exit, picked %q", name)
	}
}

// One mutator rule everywhere (GetExitInfo's): an active mutator's exit takes
// the place of the room exit of the same name. A secret mutator west the
// mover has not found, over a plain room west, hides west from every filter
// alike; once found, every filter answers the mutator's west.
func TestKnownExitFilters_MutatorExitTakesThePlaceOfTheRoomExit(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	r.Exits["west"] = exit.RoomExit{RoomId: 5}
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: "test-known-false-wall",
		Exits:     map[string]exit.RoomExit{"west": {RoomId: 9, Secret: true}},
	}))
	if !GetZoneConfig("TestZone").Mutators.Add("test-known-false-wall") {
		t.Fatal("fixture premise: the mutator must attach")
	}

	if names := knownNames(KnownExits(r, c)); equalStrings(names, []string{"north", "east", "south", "west"}) {
		t.Errorf("KnownExits listed the shadowed room west: %v", names)
	}
	if r.ExitKnownTo(c, "west") {
		t.Error("ExitKnownTo: the unfound mutator west must not be known")
	}
	for _, guess := range []string{"west", "w", "wes"} {
		if name, id := r.FindKnownExitByName(c, guess); name != "" {
			t.Errorf("FindKnownExitByName(%q) = %q (%d), want nothing", guess, name, id)
		}
	}
	for _, roomId := range []int{9, 5} {
		if got := r.FindKnownExitTo(c, roomId); got != "" {
			t.Errorf("FindKnownExitTo(%d) = %q, want nothing", roomId, got)
		}
	}
	for range 100 {
		if name, _ := r.GetRandomKnownExit(c); name == "west" {
			t.Fatal("GetRandomKnownExit picked the shadowed room west")
		}
	}
	for name, ex := range r.PermanentExitsKnownTo(c) {
		if name == "west" {
			t.Errorf("PermanentExitsKnownTo yielded the shadowed west (to %d)", ex.RoomId)
		}
	}

	c.LearnSecretExit(1, "west")
	westTo := 0
	for name, ex := range r.PermanentExitsKnownTo(c) {
		if name == "west" {
			westTo = ex.RoomId
		}
	}
	if westTo != 9 {
		t.Errorf("PermanentExitsKnownTo west leads to %d, want the mutator's 9", westTo)
	}
	if ke := knownExit(t, KnownExits(r, c), "west"); ke.RoomId != 9 {
		t.Errorf("KnownExits west leads to %d, want the mutator's 9", ke.RoomId)
	}
	if name, id := r.FindKnownExitByName(c, "w"); name != "west" || id != 9 {
		t.Errorf("FindKnownExitByName(w) = %q (%d), want west (9)", name, id)
	}
	if name, id := r.FindKnownExitByName(c, "wes"); name != "west" || id != 9 {
		t.Errorf("FindKnownExitByName(wes) = %q (%d), want west (9)", name, id)
	}
	if got := r.FindKnownExitTo(c, 9); got != "west" {
		t.Errorf("FindKnownExitTo(9) = %q, want west", got)
	}
	if got := r.FindKnownExitTo(c, 5); got != "" {
		t.Errorf("FindKnownExitTo(5) = %q, want nothing (the room west is shadowed)", got)
	}
	for range 100 {
		if name, id := r.GetRandomKnownExit(c); name == "west" && id != 9 {
			t.Fatalf("GetRandomKnownExit picked west to %d, want the mutator's 9", id)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The exits line lists exactly what KnownExits lists: the found secret exit
// joins it, a temporary exit sits in TemporaryExits, and the dark lists none.
func TestGetDetails_ExitsLineReadsKnownExits(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{}))
	viewer := users.NewTestUser(7802, "liner", "Liner", 97802)
	viewer.Character = c
	if !r.AddTemporaryExit("gangplank", exit.TemporaryRoomExit{RoomId: 4}) {
		t.Fatal("fixture premise: the temporary exit must attach")
	}
	keys := func(d RoomTemplateDetails) map[string]bool {
		out := map[string]bool{}
		for k := range d.VisibleExits {
			out[k] = true
		}
		return out
	}
	d := GetDetails(r, viewer)
	if got := keys(d); len(got) != 3 || got["west"] {
		t.Fatalf("visible exits %v, want north, south, east (west unfound)", got)
	}
	if _, ok := d.TemporaryExits["gangplank"]; !ok {
		t.Errorf("the gangplank must be in TemporaryExits, got %v", d.TemporaryExits)
	}
	c.LearnSecretExit(1, "west")
	if got := keys(GetDetails(r, viewer)); !got["west"] {
		t.Errorf("the found west must be listed, got %v", got)
	}
	r.Lamp = nil
	dark := GetDetails(r, viewer)
	if len(dark.VisibleExits) != 0 || len(dark.TemporaryExits) != 0 {
		t.Errorf("the dark listed %v and %v", dark.VisibleExits, dark.TemporaryExits)
	}
	if !dark.ExitsDark {
		t.Error("ExitsDark must tell the dark apart from a room with no exits")
	}
	if d.ExitsDark {
		t.Error("a lit room's exits are not dark")
	}
}
