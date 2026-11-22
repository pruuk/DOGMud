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
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	spineDewey  = 9491
	spineHadwen = 9100
	spineToke   = 9106

	spineLanding = 6467
	spinePool    = 5200
	spineSquare  = 5203
)

var spineNextUserId = 9375

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
	events.RegisterListener(events.TalkMenuAgain{}, conversation.HandleTalkMenuAgain)
	events.RegisterListener(events.RoomChange{}, hooks.QuestRoomEnter)
	events.RegisterListener(events.RoomChange{}, hooks.MenuCloseOnRoomChange)
	events.RegisterListener(events.ItemOwnership{}, hooks.CheckItemQuests)
	behaviortree.DrainAllDelayedActionsForTest(t)

	// A fresh id per walk: Dewey's per-player `leaving` state is held on his mob
	// instance, and a reload of the world can hand a later walk the same one.
	spineNextUserId++
	id := spineNextUserId
	w.user = users.NewTestUser(id, "spinewalker", "Spinewalker", uint64(id)+90000)
	w.user.Muted = true
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{id: w.user}))
	w.user.Character.RoomId = 6258
	first := rooms.LoadRoom(6258)
	if first == nil {
		t.Fatal("room 6258 is not loaded")
	}
	first.AddPlayer(id)
	t.Cleanup(func() {
		if r := rooms.LoadRoom(w.user.Character.RoomId); r != nil {
			r.RemovePlayer(id)
		}
	})
	return w
}

// pump runs what the game loop would run next: the event queue, and every
// delayed tree action at once, until both have had time to feed each other.
func (w *spineWalk) pump() {
	w.t.Helper()
	for range 4 {
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
	if err := rooms.MoveToRoom(w.user.UserId, roomId); err != nil {
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
		UserId: w.user.UserId, RoomId: matchRoom, Command: command, Noun: noun,
	}, bridge, bridge)
	w.pump()
}

// succeeded is the command event a command raises once it has worked.
func (w *spineWalk) succeeded(command string, targets ...questengine.Target) {
	w.t.Helper()
	questengine.NotifyCommand(w.user, w.user.Character.RoomId, command, targets...)
	w.pump()
}

// typed runs a line the player typed, through the command dispatcher: the room
// tree's room_command, then the command itself.
func (w *spineWalk) typed(command, rest string) {
	w.t.Helper()
	if _, err := usercommands.TryCommand(command, rest, w.user.UserId, 0); err != nil {
		w.t.Fatalf("%s %s: %v", command, rest, err)
	}
	w.pump()
}

// fleeToLanding is a flee that worked, in the order handlePlayerFlee runs it:
// the move, the flee's command event from the room fled into, then the room
// tree's room_enter. Dewey stands in The Landing, as its spawn list has him.
func (w *spineWalk) fleeToLanding() {
	w.t.Helper()
	if err := rooms.MoveToRoom(w.user.UserId, spineLanding); err != nil {
		w.t.Fatalf("flee to %d: %v", spineLanding, err)
	}
	w.npc(spineDewey)
	questengine.NotifyCommand(w.user, spineLanding, "flee")
	behaviortree.TryRoomBehavior(spineLanding, behaviortree.EventContext{
		EventType: "room_enter", UserId: w.user.UserId, RoomId: spineLanding,
	})
	w.pump()
}

// lessonsTo takes Waking to Gaius through its lessons, each by its own event,
// and stops once the quest stands on the step named last. The quest must have
// been taken.
func (w *spineWalk) lessonsTo(last string) {
	w.t.Helper()
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
		if l.want == last {
			return
		}
	}
	w.t.Fatalf("no lesson ends on %q", last)
}

// handInAtLanding types talk dewey, chooses the [?] line and follows Dewey's
// send-off up to the pool.
func (w *spineWalk) handInAtLanding() {
	w.t.Helper()
	w.typed("talk", "dewey")
	w.choose("ready")
	w.pump()
	w.requireHeld("28-end")
	if got := w.user.Character.RoomId; got != spinePool {
		w.t.Fatalf("Dewey's send-off left the player in room %d, want the pool %d", got, spinePool)
	}
}

// A newcomer can flee the effigy at any lesson of the practice ring, and the
// only exit leads to The Landing, which has none. The flee must still open
// Dewey's hand-in, or the player is stranded (#375 phase 7).
func TestSmoke_FleeFromAnEarlyLessonOpensTheHandIn(t *testing.T) {
	for _, at := range []string{
		"proving_prompt", "warcried", "saw_conditions", "checked_cooldowns", "considered", "attacked", "cast_spike",
	} {
		t.Run(at, func(t *testing.T) {
			w := newSpineWalk(t)
			w.npc(spineDewey)
			w.offer(spineDewey, "waking_offer", "28-start")
			w.lessonsTo(at)

			w.fleeToLanding()

			w.step(28, "fled", "a flee from "+at)
			w.marker(spineDewey, menu.MarkerHandIn)
			w.handInAtLanding()
		})
	}
}

// Set the quest aside in the ring, flee, and take it back up from Dewey at The
// Landing: the quest comes back on the lesson it was left on, where nothing can
// finish it, so the first thing the player types there must count the flee.
func TestSmoke_AbandonFleeResumeAtTheLandingReachesTheHandIn(t *testing.T) {
	for _, first := range []struct{ command, rest string }{
		{"talk", "dewey"}, {"say", "hello"}, {"north", ""},
	} {
		t.Run(first.command, func(t *testing.T) {
			w := newSpineWalk(t)
			w.npc(spineDewey)
			w.offer(spineDewey, "waking_offer", "28-start")
			w.lessonsTo("attacked")

			if ok, line := usercommands.AbandonQuest(w.user, 28); !ok {
				t.Fatalf("quest 28 cannot be set aside: %s", line)
			}
			w.fleeToLanding()
			if _, held := w.user.Character.QuestProgress[28]; held {
				t.Fatalf("a flee took a shelved quest back up: %v", w.user.Character.QuestProgress)
			}
			w.marker(spineDewey, menu.MarkerQuest)

			w.typed("talk", "dewey")
			w.acceptOffer("waking_offer", "28-start")
			w.leave()
			w.step(28, "attacked", "the resume")

			w.typed(first.command, first.rest)
			w.user.CloseMenu(menu.CloseLeft)
			w.step(28, "fled", first.command+" after the resume")
			w.handInAtLanding()
		})
	}
}

// The Landing is the only room a flee counts in: the trigger is room-limited,
// and no room tree but The Landing's grants the step.
func TestSmoke_FleeElsewhereCountsForNothing(t *testing.T) {
	cases := []struct {
		at   string
		room int
	}{
		{"attacked", 6262},
		{"attacked", 6261},
		{"tripped", 6262},
		{"tripped", spinePool},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s in %d", c.at, c.room), func(t *testing.T) {
			w := newSpineWalk(t)
			w.npc(spineDewey)
			w.offer(spineDewey, "waking_offer", "28-start")
			w.lessonsTo(c.at)

			if w.user.Character.RoomId != c.room {
				w.walkTo(c.room)
			}
			w.npc(spineDewey)
			w.succeeded("flee")
			w.step(28, c.at, fmt.Sprintf("a flee into room %d", c.room))
			if w.onMenu(spineDewey, "ready") {
				t.Fatal("Dewey's hand-in opened")
			}
		})
	}
}

// TestSmoke_NewCharacterSpine walks a brand-new character through the Pothole
// Coulee opening on shipped data (#375 phase 7, plan 2): every quest is offered
// by the NPC who took the previous hand-in, nothing is granted by itself, and
// Crier Toke and each trade's trainer start the seven trades. No token is written
// by the test: each one lands through the talk menu, a quest trigger or a
// behavior tree, and the newcomer flag is set the way the new-to-MUDs route sets it.
func TestSmoke_NewCharacterSpine(t *testing.T) {
	w := newSpineWalk(t)
	c := w.user.Character

	// The new-to-MUDs route (usercommands.grantNewcomerMarker).
	c.SetQuestFlag("30-newcomer", "new")
	w.pump()

	// Waking to Gaius: Dewey offers it, each lesson's own event moves it one
	// step, and Dewey takes it.
	if len(c.QuestProgress) != 0 {
		t.Fatalf("a new character holds no quest, not %v", c.QuestProgress)
	}
	w.npc(spineDewey)
	w.marker(spineDewey, menu.MarkerQuest)
	w.offer(spineDewey, "waking_offer", "28-start")

	w.lessonsTo("tripped")
	if w.onMenu(spineDewey, "ready") {
		t.Fatal("Dewey's hand-in is open before the flee lesson")
	}

	w.fleeToLanding()
	w.step(28, "fled", "the flee")
	w.marker(spineDewey, menu.MarkerHandIn)

	// Dewey's tree plays the send-off and moves the player to the pool.
	w.handIn(spineDewey, "ready", "28-end")
	if c.RoomId != spinePool {
		t.Fatalf("Dewey's send-off left the player in room %d, want the pool %d", c.RoomId, spinePool)
	}

	// A logout here, before Hadwen is taken up, must not Open the character.
	want := map[int]string{28: "end"}
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
	for _, key := range []string{"road_toke", "another_way"} {
		if !w.onMenu(spineHadwen, key) {
			t.Fatalf("Hadwen's %s is not open after the hand-in", key)
		}
	}
	if w.onMenu(spineHadwen, "road_esk") {
		t.Fatal("Hadwen still offers Esk's Road")
	}
	if crumbs := strings.Join(w.df(spineToke).Breadcrumbs(w.ps()), " "); !strings.Contains(crumbs, "Hadwen") {
		t.Fatalf("Toke's breadcrumbs do not point at Hadwen: %q", crumbs)
	}
	// Hadwen's hand-in opens one offer, so the game pitches it at once. Leave on
	// its card brings back the menu, which still holds the other way out.
	w.choose("no")
	w.pump()
	if w.user.Character.HasQuest("88-start") {
		t.Fatal("leaving the card granted Toke's Road")
	}
	w.choose("another_way")
	w.pump()
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

	// The seven trades: Toke offers each first quest and starts it. A trade's
	// first teacher either offers none and points back to Toke, or (door set)
	// is a second door that offers it too (owner ruling 2026-11-22, #940; all
	// seven trades have a door now).
	trades := []struct {
		offer   string
		quest   int
		teacher int
		door    string // the teacher's own offer topic, when it has one
	}{
		{"send_drill", 32, 9108, "drill_offer"}, {"send_forge", 35, 9116, "heat_start"},
		{"send_brew", 38, 9128, "brew_start"}, {"send_steppe", 41, 9136, "sign_start"},
		{"send_folding", 44, 9144, "cast_start"}, {"send_folk", 47, 9153, "words_start"},
		{"send_range", 50, 9161, "shoot_start"},
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
			if tp.GrantsQuest == start && tp.Key != tr.door {
				t.Errorf("mob %d topic %s grants %s: only Toke and the door %q may start it", tr.teacher, tp.Key, start, tr.door)
			}
		}
		if text, _ := w.df(tr.teacher).OpenerFor(w.ps()); tr.door == "" && !strings.Contains(text, "Toke") {
			t.Errorf("mob %d's opener for an Opened newcomer does not send them to Toke: %q", tr.teacher, text)
		} else if tr.door != "" && !strings.Contains(text, "ask me") {
			t.Errorf("mob %d's opener for an Opened newcomer does not invite them to ask: %q", tr.teacher, text)
		}
		if tr.door != "" && !w.onMenu(tr.teacher, tr.door) {
			t.Errorf("mob %d's door %s is not on the menu after footing", tr.teacher, tr.door)
		}
		if !w.onMenu(spineToke, tr.offer) {
			t.Fatalf("Toke's %s is not open after footing", tr.offer)
		}
		w.offer(spineToke, tr.offer, start)
		// Toke's door is in Hub Square: it does not skip the arrival talk.
		w.step(tr.quest, "start", "Toke's "+tr.offer)
	}
}

// TestSmoke_TrainerDoorsSkipTheArrivalTalk takes each trade's first quest from
// its trainer in the trainer's room (owner ruling 2026-11-22, #940): the quest
// starts on its second step, and every item the offer carries is in the pack.
// The two earlier quests are marked done directly, because the offer reads
// only their end tokens. First Light (98) is not one of the seven trades Toke
// offers in the spine, so it is here only.
func TestSmoke_TrainerDoorsSkipTheArrivalTalk(t *testing.T) {
	doors := []struct {
		name    string
		quest   int
		trainer int
		room    int
		door    string
		arrive  string // the arrival topic the skip makes untrue
		second  string
		items   []int
	}{
		{"Vorn", 32, 9108, 5227, "drill_offer", "drill_arrive", "met", []int{10043}},
		{"Rusk", 35, 9116, 5245, "heat_start", "heat_arrive", "met", nil},
		{"Birna", 38, 9128, 5265, "brew_start", "brew_arrive", "met", nil},
		{"Tarn", 41, 9136, 5283, "sign_start", "sign_arrive", "met", nil},
		{"Grieve", 44, 9144, 5303, "cast_start", "cast_lesson", "met", nil},
		{"Wenna", 47, 9153, 5324, "words_start", "words_lesson", "met", nil},
		{"Iden", 50, 9161, 5350, "shoot_start", "shoot_lesson", "met", []int{10038, 30064}},
		{"Gethin", 98, 9125, 5255, "light_start", "first_light_lantern", "lantern", []int{20097}},
	}
	for _, d := range doors {
		t.Run(d.name, func(t *testing.T) {
			w := newSpineWalk(t)
			for _, tok := range []string{"30-start", "30-end", "31-start", "31-end"} {
				if !w.user.Character.GiveQuestToken(tok) {
					t.Fatalf("could not hold %s", tok)
				}
			}
			w.walkTo(d.room)
			w.npc(d.trainer)
			if !w.onMenu(d.trainer, d.door) {
				t.Fatalf("%s is not on the menu after Find Your Footing", d.door)
			}
			arrive := w.df(d.trainer).FindTopic(d.arrive)
			if arrive == nil {
				t.Fatalf("mob %d has no arrival topic %s", d.trainer, d.arrive)
			}
			start := fmt.Sprintf("%d-start", d.quest)
			w.talk(d.trainer)
			w.choose(d.door)
			w.pump()
			if w.user.Character.HasQuest(start) {
				t.Fatalf("topic %s granted %s before Accept", d.door, start)
			}
			w.choose("yes")
			w.pump()
			w.requireHeld(start)
			// The talk menu is open again once the queue has drained. It is
			// drawn after the quest_granted trigger has moved the step, so it
			// must not offer the arrival talk. The menu has no reader, so the
			// test types the arrival topic's label: a label on the menu is
			// chosen (true), and one that is not closes the menu as another
			// command (false).
			if !w.user.HasOpenMenu() {
				t.Fatal("the talk menu was not redrawn after Accept")
			}
			if w.user.RouteMenuInput(arrive.Label, true) {
				t.Errorf("the menu after Accept still offers the arrival topic %s (%q)", d.arrive, arrive.Label)
			}
			w.step(d.quest, d.second, d.name+"'s "+d.door)
			for _, id := range d.items {
				if !w.user.Character.CarriesItem(id) {
					t.Errorf("%s did not hand over item %d", d.name, id)
				}
			}
		})
	}
}
