package main

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/hooks"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	spineUserId = 9375

	spineDewey  = 9491
	spineHadwen = 9100
	spineToke   = 9106

	spineLanding = 6467
	spinePool    = 5200
	spineSquare  = 5203
)

// spineWalk is one character walking the shipped opening through the game's
// own doors: conversation.Talk and the menu's input routing for every talk,
// the event queue with the game's listeners for every grant, the shipped
// behavior trees for Dewey's send-off and Hadwen's rite, and the shipped quest
// triggers for each lesson. The one thing not run is the command behind a
// lesson (a real attack needs rounds and a target): the walk raises the event
// that command raises, through the same call it makes.
type spineWalk struct {
	t     *testing.T
	user  *users.UserRecord
	byMob map[int]*dialogue.DialogueFile
	npcs  map[int]*mobs.Mob
}

func newSpineWalk(t *testing.T) *spineWalk {
	t.Helper()
	loadShippedQuestWorld(t)
	// The loader above swallows a boot refusal and leaves the world half wired,
	// which would fail this walk somewhere unrelated. Name the refusal here.
	if err := questDoor(true); err != nil {
		t.Fatalf("the quest door refuses shipped content: %v", err)
	}
	files, problems := dialogue.LoadAllFiles()
	if len(problems) > 0 {
		t.Fatalf("dialogue problems: %v", problems)
	}
	w := &spineWalk{t: t, byMob: map[int]*dialogue.DialogueFile{}, npcs: map[int]*mobs.Mob{}}
	for _, df := range files {
		w.byMob[df.MobId] = df
	}

	// The listeners these steps need, as internal/hooks/hooks.go registers them.
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(events.SwapListenersForTest())
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.RegisterListener(events.Quest{}, hooks.HandleQuestUpdate)
	events.RegisterListener(events.TalkNextOffer{}, conversation.HandleTalkNextOffer)
	events.RegisterListener(events.RoomChange{}, hooks.QuestRoomEnter)
	events.RegisterListener(events.RoomChange{}, hooks.MenuCloseOnRoomChange)
	events.RegisterListener(events.ItemOwnership{}, hooks.CheckItemQuests)
	behaviortree.DrainAllDelayedActionsForTest(t)

	w.user = users.NewTestUser(spineUserId, "spinewalker", "Spinewalker", 99375)
	w.user.Muted = true
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{spineUserId: w.user}))
	w.user.Character.RoomId = 6258
	first := rooms.LoadRoom(6258)
	if first == nil {
		t.Fatal("room 6258 is not loaded")
	}
	first.AddPlayer(spineUserId)
	t.Cleanup(func() {
		if r := rooms.LoadRoom(w.user.Character.RoomId); r != nil {
			r.RemovePlayer(spineUserId)
		}
	})
	return w
}

// pump runs what the game loop would run next: the event queue, and every
// delayed tree action at once, until both have had time to feed each other.
func (w *spineWalk) pump() {
	w.t.Helper()
	for i := 0; i < 4; i++ {
		events.ProcessEvents()
		behaviortree.DrainAllDelayedActionsForTest(w.t)
	}
	events.ProcessEvents()
}

func (w *spineWalk) df(mobId int) *dialogue.DialogueFile {
	w.t.Helper()
	df := w.byMob[mobId]
	if df == nil {
		w.t.Fatalf("no dialogue for mob %d", mobId)
	}
	return df
}

func (w *spineWalk) ps() *dialogue.PlayerState { return conversation.PlayerState(w.user) }

// npc stands mobId in the player's room, as the room's spawn list does in play.
func (w *spineWalk) npc(mobId int) (*rooms.Room, *mobs.Mob) {
	w.t.Helper()
	room := rooms.LoadRoom(w.user.Character.RoomId)
	if room == nil {
		w.t.Fatalf("room %d is not loaded", w.user.Character.RoomId)
	}
	for _, inst := range room.GetMobs() {
		if m := mobs.GetInstance(inst); m != nil && int(m.MobId) == mobId {
			return room, m
		}
	}
	m := mobs.NewMobById(mobs.MobId(mobId), room.RoomId)
	if m == nil {
		w.t.Fatalf("mob %d did not spawn", mobId)
	}
	room.AddMob(m.InstanceId)
	w.t.Cleanup(func() { room.RemoveMob(m.InstanceId); mobs.DestroyInstance(m.InstanceId) })
	return room, m
}

// onMenu reports whether the player sees the topic in the NPC's talk menu now.
func (w *spineWalk) onMenu(mobId int, key string) bool {
	w.t.Helper()
	for _, e := range w.df(mobId).Menu("npc", w.ps()).Entries {
		if e.Id == key {
			return true
		}
	}
	return false
}

func (w *spineWalk) marker(mobId int, want string) {
	w.t.Helper()
	if got := w.df(mobId).Marker(w.ps()); got != want {
		w.t.Fatalf("mob %d shows %q, want %q at %v", mobId, got, want, w.user.Character.QuestProgress)
	}
}

// talk opens the conversation as the talk command does and requires its menu.
func (w *spineWalk) talk(mobId int) {
	w.t.Helper()
	room, mob := w.npc(mobId)
	conversation.Talk(w.user, room, mob)
	if !w.user.HasOpenMenu() {
		w.t.Fatalf("talking to mob %d in room %d opened no menu", mobId, room.RoomId)
	}
}

// choose types a menu answer. A topic the menu does not list is refused by the
// menu itself, so a closed topic cannot be chosen here.
func (w *spineWalk) choose(answer string) {
	w.t.Helper()
	if !w.user.RouteMenuInput(answer, true) {
		w.t.Fatalf("the open menu has no %q at %v", answer, w.user.Character.QuestProgress)
	}
}

func (w *spineWalk) leave() {
	w.t.Helper()
	w.pump()
	w.user.CloseMenu(menu.CloseLeft)
}

func (w *spineWalk) requireHeld(token string) {
	w.t.Helper()
	if !w.user.Character.HasQuest(token) {
		w.t.Fatalf("want %s, progress %v", token, w.user.Character.QuestProgress)
	}
}

// offer takes an offer: the pitch and its card grant nothing, Accept grants.
func (w *spineWalk) offer(mobId int, key, want string) {
	w.t.Helper()
	w.talk(mobId)
	w.acceptOffer(key, want)
	w.leave()
}

func (w *spineWalk) acceptOffer(key, want string) {
	w.t.Helper()
	w.choose(key)
	w.pump()
	if w.user.Character.HasQuest(want) {
		w.t.Fatalf("topic %s granted %s before Accept", key, want)
	}
	w.choose("yes")
	w.pump()
	w.requireHeld(want)
}

// handIn takes a hand-in. The conversation stays open for what follows it.
func (w *spineWalk) handIn(mobId int, key, want string) {
	w.t.Helper()
	w.talk(mobId)
	w.choose(key)
	w.pump()
	w.requireHeld(want)
}

func (w *spineWalk) step(questId int, want, after string) {
	w.t.Helper()
	if got := w.user.Character.QuestProgress[questId]; got != want {
		w.t.Fatalf("after %s: quest %d is at %q, want %q", after, questId, got, want)
	}
}

// walkTo moves the player as the game does. RoomChange reaches QuestRoomEnter.
func (w *spineWalk) walkTo(roomId int) {
	w.t.Helper()
	if err := rooms.MoveToRoom(spineUserId, roomId); err != nil {
		w.t.Fatalf("move to %d: %v", roomId, err)
	}
	w.pump()
}

// issued is the command_issued event usercommands.TryCommand raises after a
// command dispatches.
func (w *spineWalk) issued(command, noun string) {
	w.t.Helper()
	roomId := w.user.Character.RoomId
	matchRoom, _ := rooms.OriginalRoomId(roomId)
	bridge := questengine.NewGameBridge(w.user, roomId)
	questengine.GetEngine().Notify("command_issued", questengine.EventDetails{
		UserId: spineUserId, RoomId: matchRoom, Command: command, Noun: noun,
	}, bridge, bridge)
	w.pump()
}

// succeeded is the command event a command raises once it has worked.
func (w *spineWalk) succeeded(command string, targets ...questengine.Target) {
	w.t.Helper()
	questengine.NotifyCommand(w.user, w.user.Character.RoomId, command, targets...)
	w.pump()
}

// TestSmoke_NewCharacterSpine walks a brand-new character through the Pothole
// Coulee opening on shipped data (#375 phase 7, plan 2): every quest is offered
// by the NPC who took the previous hand-in, nothing is granted by itself, and
// Crier Toke ends up the only starter of the seven trades. No token is written
// by the test: each one lands through the talk menu, a quest trigger or a
// behavior tree, except 21-start, queued the way the new-to-MUDs route does.
func TestSmoke_NewCharacterSpine(t *testing.T) {
	w := newSpineWalk(t)
	c := w.user.Character

	// The new-to-MUDs route (usercommands.grantNewcomerMarker).
	events.AddToQueue(events.Quest{UserId: spineUserId, QuestToken: "21-start"})
	w.pump()

	// Waking to Gaius: Dewey offers it, each lesson's own event moves it one
	// step, and Dewey takes it.
	if len(c.QuestProgress) != 1 {
		t.Fatalf("a new character holds only the newcomer marker, not %v", c.QuestProgress)
	}
	w.npc(spineDewey)
	w.marker(spineDewey, menu.MarkerQuest)
	w.offer(spineDewey, "waking_offer", "28-start")

	lessons := []struct {
		do   func()
		want string
	}{
		{func() { w.issued("hint", "") }, "hinted"},
		{func() { w.issued("look", "") }, "looked"},
		{func() { w.issued("look", "guide") }, "examined"},
		{func() { w.walkTo(6259) }, "status_prompt"},
		{func() { w.issued("status", "") }, "statused"},
		{func() { w.issued("skills", "") }, "skilled"},
		{func() { w.issued("spells", "") }, "spelled"},
		{func() { w.succeeded("cast", questengine.Target{Noun: "chrysalis-glow"}) }, "glowed"},
		{func() { w.walkTo(6260) }, "carry_prompt"},
		{func() { w.issued("get", "token") }, "got"},
		{func() { w.issued("inventory", "") }, "invd"},
		{func() { w.issued("help", "") }, "helped"},
		{func() { w.walkTo(6261) }, "speaks_prompt"},
		{func() { w.succeeded("say") }, "said"},
		{func() { w.talk(spineDewey); w.choose("asked_28"); w.leave() }, "asked"},
		{func() { w.walkTo(6262) }, "proving_prompt"},
		{func() { w.succeeded("warcry") }, "warcried"},
		{func() { w.issued("conditions", "") }, "saw_conditions"},
		{func() { w.issued("cooldowns", "") }, "checked_cooldowns"},
		{func() { w.issued("consider", "effigy") }, "considered"},
		{func() { w.issued("attack", "effigy") }, "attacked"},
		{func() { w.succeeded("cast", questengine.Target{Noun: "conviction-spike"}) }, "cast_spike"},
		{func() { w.succeeded("trip") }, "tripped"},
	}
	for _, l := range lessons {
		l.do()
		w.step(28, l.want, "lesson "+l.want)
	}

	// The flee moves the player to The Landing, then counts (notifyFleeSucceeded).
	w.walkTo(spineLanding)
	w.npc(spineDewey)
	if w.onMenu(spineDewey, "ready") {
		t.Fatal("Dewey's hand-in is open before the flee lesson")
	}
	w.succeeded("flee")
	w.step(28, "fled", "the flee")
	w.marker(spineDewey, menu.MarkerHandIn)

	// Dewey's tree plays the send-off and moves the player to the pool.
	w.handIn(spineDewey, "ready", "28-end")
	if c.RoomId != spinePool {
		t.Fatalf("Dewey's send-off left the player in room %d, want the pool %d", c.RoomId, spinePool)
	}

	// A logout here, before Hadwen is taken up, must not Open the character.
	want := map[int]string{21: "start", 28: "end"}
	if !maps.Equal(c.QuestProgress, want) {
		t.Fatalf("at the pool after Dewey: %v, want %v", c.QuestProgress, want)
	}
	c.MigrateNewbieAwakening() // what users.LoadUser runs on a relog
	if !maps.Equal(c.QuestProgress, want) {
		t.Fatalf("a relog at the pool changed progress to %v: the rite was skipped", c.QuestProgress)
	}

	// The Awakening: Hadwen offers it, his tree runs the rite, and he takes it.
	w.npc(spineHadwen)
	if w.onMenu(spineHadwen, "road_toke") {
		t.Fatal("Toke's Road is offered before the rite")
	}
	w.marker(spineHadwen, menu.MarkerQuest)
	w.offer(spineHadwen, "rite_offer", "30-start")
	w.talk(spineHadwen)
	w.choose("begin")
	w.step(30, "rite", "choosing begin: the step lands as the rite starts")
	w.pump()
	w.step(30, "woken", "the rite's close")
	if w.onMenu(spineHadwen, "road_toke") {
		t.Fatal("Toke's Road is offered before the hand-in")
	}
	w.marker(spineHadwen, menu.MarkerHandIn)
	w.handIn(spineHadwen, "rite_report", "30-end")
	for _, key := range []string{"road_toke", "road_esk", "another_way"} {
		if !w.onMenu(spineHadwen, key) {
			t.Fatalf("Hadwen's %s is not open after the hand-in", key)
		}
	}
	if crumbs := strings.Join(w.df(spineToke).Breadcrumbs(w.ps()), " "); !strings.Contains(crumbs, "Hadwen") {
		t.Fatalf("Toke's breadcrumbs do not point at Hadwen: %q", crumbs)
	}
	// Hadwen's hand-in opens two roads, so no offer is pitched by itself: the
	// menu comes back, and the road is chosen from it.
	w.acceptOffer("road_toke", "88-start")
	w.leave()

	// Toke's Road, then Find Your Footing, both at Toke.
	w.walkTo(spineSquare)
	w.npc(spineToke)
	if !w.onMenu(spineToke, "road_toke_end") {
		t.Fatal("Toke does not take Toke's Road")
	}
	if w.onMenu(spineToke, "footing_offer") {
		t.Fatal("Find Your Footing is offered before Toke's Road is handed in")
	}
	// Toke's hand-in opens one offer, so the game pitches it at once.
	w.handIn(spineToke, "road_toke_end", "88-end")
	w.choose("yes")
	w.leave()
	w.requireHeld("31-start")

	// The walk around the town: each stop's own event moves it one step.
	w.walkTo(5213)
	w.step(31, "school", "the school hall")
	w.walkTo(5207)
	w.succeeded("list")
	w.step(31, "list", "list at Onna's")
	if !acquire.AcquireItem(w.user, items.New(20008), acquire.AcquireBuy) {
		t.Fatal("the cotton shirt did not reach the pack")
	}
	w.pump()
	w.step(31, "shop", "the cotton shirt landing")
	w.succeeded("equip", questengine.TargetItem(20008))
	w.step(31, "equip", "equip cotton shirt")
	w.walkTo(5205)
	w.step(31, "lantern", "The Drowned Lantern")
	w.walkTo(spineSquare)

	// The seven trades: Toke offers each first quest and starts it; its first
	// teacher offers none and points back to Toke.
	trades := []struct {
		offer   string
		quest   int
		teacher int
	}{
		{"send_drill", 32, 9108}, {"send_forge", 35, 9116}, {"send_brew", 38, 9128},
		{"send_steppe", 41, 9136}, {"send_folding", 44, 9144}, {"send_folk", 47, 9153},
		{"send_range", 50, 9161},
	}
	for _, tr := range trades {
		if w.onMenu(spineToke, tr.offer) {
			t.Fatalf("Toke's %s is offered before Find Your Footing is handed in", tr.offer)
		}
	}
	w.handIn(spineToke, "footing_turnin", "31-end")
	w.leave()
	w.marker(spineToke, menu.MarkerQuest)
	for _, tr := range trades {
		start := fmt.Sprintf("%d-start", tr.quest)
		for _, tp := range w.df(tr.teacher).Topics {
			if tp.GrantsQuest == start {
				t.Errorf("mob %d topic %s still grants %s: Toke is the only starter", tr.teacher, tp.Key, start)
			}
		}
		if text, _ := w.df(tr.teacher).OpenerFor(w.ps()); !strings.Contains(text, "Toke") {
			t.Errorf("mob %d's opener for an Opened newcomer does not send them to Toke: %q", tr.teacher, text)
		}
		if !w.onMenu(spineToke, tr.offer) {
			t.Fatalf("Toke's %s is not open after footing", tr.offer)
		}
		w.offer(spineToke, tr.offer, start)
	}
}
