package aicompanion

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
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
		// companion-ask resolves what the owner typed against the NPC's info
		// topics (R13), so the Smith needs one. harmMob leaves MobId 0 and
		// no zone, which is the key dialogue.Load reads.
		t.Cleanup(dialogue.InstallForTest(&dialogue.DialogueFile{Topics: []dialogue.Topic{
			{Key: `ore`, Kind: dialogue.KindInfo, Label: `What of the ore shipment?`, Text: `Delayed.`},
			{Key: `weather`, Kind: dialogue.KindInfo, Label: `How is the weather?`, Text: `Grim.`},
			{Key: `rumor`, Kind: dialogue.KindInfo, Label: `Any rumors, friend?`, Text: `None.`},
			{Key: `delay`, Kind: dialogue.KindInfo, Label: `Why is the caravan late?`, Text: `Bandits.`},
			{Key: `ferry`, Kind: dialogue.KindInfo, Label: `Can you take me across?`, Text: `Aye.`, Behavior: true},
		}}))
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

const askInfraredCond = 9786 // infravision, reach 30

// #454 review K10: an owner who makes out the Smith only as a shape sends
// their companion to it, and the errand wrote the Smith's real name into the
// companion's memory and prompt, where she may say it aloud. Both now name
// the NPC as the owner perceives it.
func TestCompanionAskSight_AShapeIsNotNamedToTheCompanion(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		askInfraredCond: {ConditionId: askInfraredCond, Name: "Test Infrared", Secret: true,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	m, c, owner, room := askForScene(t, true, false)
	if err := owner.Character.AddCondition(askInfraredCond, true); err != nil {
		t.Fatalf("could not give the owner infravision: %v", err)
	}
	if got := room.ParticipantSight(owner.UserId); got != messaging.SightShapes {
		t.Fatalf("fixture: the owner must make out shapes only, got %v", got)
	}

	// R13: a conversation needs full sight and the NPC perceived, so a shape
	// is refused with the generic line and no leave is granted.
	_, _ = m.cmdAskFor(`#300 about the ore`, owner, room, events.EventFlag(0))
	told := strings.Join(events.DrainQueuedMessagesForTest(owner.UserId), ``)
	if c.askAuth != nil {
		t.Fatalf("a shape is not someone to question, yet a leave was granted: %+v", c.askAuth)
	}
	if strings.Contains(told, `Smith`) || strings.Contains(told, `ore shipment`) {
		t.Errorf("the refusal leaks the NPC or its topics: %q", told)
	}
	for _, line := range c.mind.RecentLines {
		if strings.Contains(line.Text, `Smith`) {
			t.Errorf("her memory names the shape: %q", line.Text)
		}
	}
	for _, s := range c.pending {
		if strings.Contains(s.Text, `Smith`) {
			t.Errorf("her prompt names the shape: %q", s.Text)
		}
	}
	if len(c.pending) != 0 {
		t.Errorf("no errand should be sent to a shape: %+v", c.pending)
	}
}

// The lit control: at full sight the errand names the NPC.
func TestCompanionAskSight_LitNamesTheNPC(t *testing.T) {
	m, c, owner, room := askForScene(t, true, true)
	_, _ = m.cmdAskFor(`smith about the ore`, owner, room, events.EventFlag(0))
	if len(c.pending) == 0 || !strings.Contains(c.pending[len(c.pending)-1].Text, `Smith`) {
		t.Fatalf("a lit errand should name the Smith: %+v", c.pending)
	}
}

const unbondLightCond = 9785 // a carried light, literal strength 60

// #456 review G3, owner ruling 2026-10-09: a mover is seen by the light they
// carry on their own way out. Mara carries the only light in the room; when
// the bond ends, Bram, left in the dark, saw her go by that light. The line
// was judged after RemoveMob, when the light had gone with her.
func TestUnbondPartingLineIsJudgedByHerOwnLight(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		unbondLightCond: {ConditionId: unbondLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	owner, bram, room, her := harmWorld(t, configs.PVPDisabled)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)
	if err := her.Character.AddCondition(unbondLightCond, false); err != nil {
		t.Fatalf("could not light her: %v", err)
	}
	if got := room.ParticipantSight(bram.UserId); got != messaging.SightFull {
		t.Fatalf("fixture: her light must light the room, got %v", got)
	}
	owner.Character.Companions = []characters.CompanionInfo{{InstanceId: her.InstanceId, Name: `Mara`, SourceType: characters.CompanionBonded}}
	m, _, _ := strangerModule()
	m.ctrls = map[int]*controller{}
	events.DrainQueuedMessagesForTest(bram.UserId)

	if _, err := m.unbond(owner, `waves once and walks away.`); err != nil {
		t.Fatalf("unbond: %v", err)
	}
	if got := room.ParticipantSight(bram.UserId); got != messaging.SightNone {
		t.Fatalf("fixture: the light must leave with her, got %v", got)
	}
	got := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(strings.Join(events.DrainQueuedMessagesForTest(bram.UserId), ``), ``)
	if !strings.Contains(got, `Mara waves once and walks away.`) {
		t.Fatalf("Bram saw her go by her own light, got %q", got)
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

// R13: the leave stores the topic key the owner's words resolved to, and
// that key, not the model's words, is what reaches the NPC.
func TestCompanionAsk_StoresTheResolvedTopicKey(t *testing.T) {
	m, c, owner, room := askForScene(t, true, true)
	_, _ = m.cmdAskFor(`smith about the ore`, owner, room, events.EventFlag(0))
	if c.askAuth == nil || c.askAuth.TopicKey != `ore` {
		t.Fatalf("the leave should carry the topic key: %+v", c.askAuth)
	}
}

// No match, or several, grants no leave and lists the NPC's info labels.
func TestCompanionAsk_UnmatchedTopicGrantsNoLeaveAndListsTheLabels(t *testing.T) {
	for _, what := range []string{`smith about gossip`, `smith about the`} {
		m, c, owner, room := askForScene(t, true, true)
		_, _ = m.cmdAskFor(what, owner, room, events.EventFlag(0))
		told := strings.Join(events.DrainQueuedMessagesForTest(owner.UserId), ``)
		if c.askAuth != nil {
			t.Fatalf("%q: no single topic matched, yet a leave was granted: %+v", what, c.askAuth)
		}
		if !strings.Contains(told, `What of the ore shipment?`) || !strings.Contains(told, `Any rumors, friend?`) {
			t.Errorf("%q: the owner should read the NPC's info labels: %q", what, told)
		}
	}
}

// A behavior topic can move the owner (ferries, stake prompts), so a
// companion is never sent to it, and the refusal does not list it.
func TestCompanionAsk_BehaviorTopicIsNotOfferedToTheCompanion(t *testing.T) {
	m, c, owner, room := askForScene(t, true, true)
	_, _ = m.cmdAskFor(`smith about ferry`, owner, room, events.EventFlag(0))
	told := strings.Join(events.DrainQueuedMessagesForTest(owner.UserId), ``)
	if c.askAuth != nil {
		t.Fatalf("a behavior topic was reachable: %+v", c.askAuth)
	}
	if strings.Contains(told, `take me across`) {
		t.Errorf("the behavior topic was listed: %q", told)
	}
}

// valid() checks the model's words against the owner's. When the owner
// typed a key, the label's words stand in, so a natural question passes.
func TestCompanionAsk_AKeyTypedByTheOwnerStillAdmitsANaturalSentence(t *testing.T) {
	m, c, owner, room := askForScene(t, true, true)
	_, _ = m.cmdAskFor(`smith about delay`, owner, room, events.EventFlag(0))
	if c.askAuth == nil || c.askAuth.TopicKey != `delay` {
		t.Fatalf("fixture: the key was not resolved: %+v", c.askAuth)
	}
	now := time.Now().Unix()
	if !c.askAuth.valid(300, `Why is the caravan so late?`, now) {
		t.Errorf("a natural sentence about the topic was refused: %q", c.askAuth.Topic)
	}
	if c.askAuth.valid(300, `How is the weather today?`, now) {
		t.Errorf("another topic was admitted: %q", c.askAuth.Topic)
	}
}

// Underscores in a typed key read as spaces.
func TestCompanionAsk_UnderscoresInTheOwnersWordsBecomeSpaces(t *testing.T) {
	m, c, owner, room := askForScene(t, true, true)
	_, _ = m.cmdAskFor(`smith about ore_shipment`, owner, room, events.EventFlag(0))
	if c.askAuth == nil || strings.Contains(c.askAuth.Topic, `_`) {
		t.Fatalf("the stored words keep an underscore: %+v", c.askAuth)
	}
}

// The authorized sayto hands AskNpc the topic key, never the model's words.
func TestCompanionAsk_SaytoHandsTheTopicKeyToTheAsker(t *testing.T) {
	m, c, owner, room := askForScene(t, true, true)
	her := mobs.GetInstance(42)
	var gotKey string
	var gotMob int
	calls := 0
	companionai.SetNpcAsker(func(ownerId, mobInstanceId int, text string, authorized bool) bool {
		calls++
		gotKey, gotMob = text, mobInstanceId
		return authorized
	})
	t.Cleanup(func() { companionai.SetNpcAsker(nil) })

	_, _ = m.cmdAskFor(`smith about the ore`, owner, room, events.EventFlag(0))
	if c.askAuth == nil {
		t.Fatal("fixture: no leave granted")
	}
	out := m.performAction(c, her, owner, harmScene(300, 2),
		ActionProposal{Verb: `sayto`, Ref: `t1`, Query: `Any news of the ore shipment, friend?`},
		[]stimulus{{Kind: `heard`, FromOwner: true}}, 0, 0)
	if !out.Issued {
		t.Fatalf("the sayto was not issued: %+v", out)
	}
	if calls != 1 || gotKey != `ore` || gotMob != 300 {
		t.Fatalf("asker got key %q for mob %d (%d calls); want ore for 300", gotKey, gotMob, calls)
	}
	if c.askAuth != nil {
		t.Error("the leave should be spent")
	}
}
