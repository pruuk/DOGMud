package actions

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The sight gates 5b per-listener table. Five listeners, each verdict forced
// exactly (a lamp and the perception machine, never a roll):
//
//	clear   lit room, plain eyes            SightFull
//	blind   lit room, Perception Blinded    SightNone
//	deaf    lit room, plain eyes, Deafened  SightFull
//	shapes  dark room, infrared             SightShapes
//	dark    dark room, plain eyes           SightNone
//
// The speaker is Kesh (a player) or Grel (a mob). Every line is sent twice,
// once from the lit room and once from the dark room.
const (
	speechSpeakerId = 8851
	speechClearId   = 8852
	speechBlindId   = 8853
	speechDeafId    = 8854
	speechShapesId  = 8855
	speechDarkId    = 8856
	speechNextId    = 8857
	speechMobInst   = 98851
	speechMobInst2  = 98858
	speechInfraCond = 8861
	speechSleepCond = 8862
	speechLitRoom   = 8870
	speechDarkRoom  = 8871
	speechNextRoom  = 8872
)

var speechTag = regexp.MustCompile(`<[^>]*>`)

type speechScene struct {
	lit, dark, next *rooms.Room
	player          *users.UserRecord
	mob             *mobs.Mob
}

func newSpeechScene(t *testing.T) *speechScene {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		speechInfraCond: {ConditionId: speechInfraCond, Name: "Test Heat Eyes",
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		speechSleepCond: {ConditionId: speechSleepCond, Name: "Test Sleep", RoundInterval: 1, TriggerCount: 50,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	player := users.NewTestUser(speechSpeakerId, "kesh", "Kesh", 98851)
	people := map[int]*users.UserRecord{
		speechSpeakerId: player,
		speechClearId:   users.NewTestUser(speechClearId, "clara", "Clara", 98852),
		speechBlindId:   users.NewTestUser(speechBlindId, "bram", "Bram", 98853),
		speechDeafId:    users.NewTestUser(speechDeafId, "dena", "Dena", 98854),
		speechShapesId:  users.NewTestUser(speechShapesId, "sorn", "Sorn", 98855),
		speechDarkId:    users.NewTestUser(speechDarkId, "dusk", "Dusk", 98856),
		speechNextId:    users.NewTestUser(speechNextId, "nell", "Nell", 98857),
	}
	t.Cleanup(users.SeedUsersForTest(people))

	people[speechDeafId].Deafened = true
	blind := people[speechBlindId].Character
	blind.Perception = characters.New().Perception
	require.NoError(t, blind.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.True(t, people[speechShapesId].Character.Conditions.AddCondition(speechInfraCond, true))

	lit := &rooms.Room{RoomId: speechLitRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60),
		Exits: map[string]exit.RoomExit{"north": {RoomId: speechNextRoom}}}
	dark := &rooms.Room{RoomId: speechDarkRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(0),
		Exits: map[string]exit.RoomExit{"up": {RoomId: speechNextRoom}}}
	next := &rooms.Room{RoomId: speechNextRoom, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60),
		Exits: map[string]exit.RoomExit{"south": {RoomId: speechLitRoom}, "down": {RoomId: speechDarkRoom}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{speechLitRoom: lit, speechDarkRoom: dark, speechNextRoom: next},
		map[string]*rooms.ZoneConfig{}))
	place := func(r *rooms.Room, ids ...int) {
		for _, id := range ids {
			people[id].Character.RoomId = r.RoomId
			r.AddPlayer(id)
		}
	}
	place(lit, speechClearId, speechBlindId, speechDeafId)
	place(dark, speechShapesId, speechDarkId)
	place(next, speechNextId)

	// Forced, not rolled: pin every verdict before any line is sent.
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(people[speechClearId].Character, lit))
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(people[speechDeafId].Character, lit))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(blind, lit))
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(people[speechShapesId].Character, dark))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(people[speechDarkId].Character, dark))

	mobChar := characters.New()
	mobChar.Name = "Grel"
	sc := &speechScene{lit: lit, dark: dark, next: next, player: player,
		mob: &mobs.Mob{InstanceId: speechMobInst, Character: *mobChar}}
	sc.drain()
	return sc
}

// speaker stands the player or the mob in r and returns it as an Actor.
func (sc *speechScene) speaker(player bool, r *rooms.Room) Actor {
	if player {
		for _, old := range []*rooms.Room{sc.lit, sc.dark, sc.next} {
			old.RemovePlayer(speechSpeakerId)
		}
		sc.player.Character.RoomId = r.RoomId
		r.AddPlayer(speechSpeakerId)
		return &UserActor{User: sc.player, Room: r}
	}
	sc.mob.Character.RoomId = r.RoomId
	return &MobActor{Mob: sc.mob, Room: r}
}

func (sc *speechScene) drain() {
	for _, id := range []int{speechSpeakerId, speechClearId, speechBlindId, speechDeafId, speechShapesId, speechDarkId, speechNextId} {
		events.DrainQueuedMessageEventsForTest(id)
	}
	for _, id := range []int{speechLitRoom, speechDarkRoom, speechNextRoom} {
		events.DrainQueuedRoomMessagesForTest(id)
	}
}

// speechHeard is what uid's client prints: its queued lines minus those the
// deafen filter drops (events.Message.HiddenFromDeafened, the rule
// hooks/Message_SendMessages.go applies), tags stripped.
func speechHeard(t *testing.T, uid int) []string {
	t.Helper()
	u := users.GetByUserId(uid)
	require.NotNil(t, u)
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		if m.HiddenFromDeafened(u.Deafened) {
			continue
		}
		out = append(out, strings.TrimSpace(speechTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

func speechExpectOne(t *testing.T, who string, got []string, want string) {
	t.Helper()
	if want == "" {
		require.Empty(t, got, "%s should read nothing", who)
		return
	}
	require.Equal(t, []string{want}, got, "%s", who)
}

// speechWant is what each listener reads; "" means nothing at all.
type speechWant struct {
	clear, blind, deaf, shapes, dark string
}

// checkSpeech sends from the lit room (clear, blind and deaf listen), then
// from the dark room (shapes and dark listen), and checks every listener.
// A player speaker never reads its own room line.
func checkSpeech(t *testing.T, sc *speechScene, player bool, send func(Actor), want speechWant) {
	t.Helper()
	sc.drain()
	send(sc.speaker(player, sc.lit))
	speechExpectOne(t, "clear", speechHeard(t, speechClearId), want.clear)
	speechExpectOne(t, "blind", speechHeard(t, speechBlindId), want.blind)
	speechExpectOne(t, "deaf", speechHeard(t, speechDeafId), want.deaf)
	require.Empty(t, speechHeard(t, speechSpeakerId), "the speaker reads its own room line")
	sc.drain()
	send(sc.speaker(player, sc.dark))
	speechExpectOne(t, "shapes", speechHeard(t, speechShapesId), want.shapes)
	speechExpectOne(t, "dark", speechHeard(t, speechDarkId), want.dark)
	require.Empty(t, speechHeard(t, speechSpeakerId), "the speaker reads its own room line")
	sc.drain()
}

// Rally and warcry are heard: the exact room lines the four wrappers pass
// to SendHeard (internal/guards/speech_wrapper_guard_test.go pins that they do). Authored
// text, so the deafened listener hears them from either speaker.
func TestSendHeard_RallyAndWarcry(t *testing.T) {
	cases := []struct {
		name   string
		player bool
		cat    messaging.Category
		line   string
		want   speechWant
	}{
		{"player rally", true, messaging.CategoryRally,
			fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="username">%s</ansi> rallies everyone with an inspiring shout!</ansi>`, "Kesh"),
			speechWant{"Kesh rallies everyone with an inspiring shout!", "Something rallies everyone with an inspiring shout!",
				"Kesh rallies everyone with an inspiring shout!", "A figure rallies everyone with an inspiring shout!",
				"Something rallies everyone with an inspiring shout!"}},
		{"mob rally", false, messaging.CategoryRally,
			fmt.Sprintf(`<ansi fg="cyan-bold"><ansi fg="mobname">%s</ansi> lets out a rallying roar!</ansi>`, "Grel"),
			speechWant{"Grel lets out a rallying roar!", "Something lets out a rallying roar!",
				"Grel lets out a rallying roar!", "A figure lets out a rallying roar!", "Something lets out a rallying roar!"}},
		{"player warcry", true, messaging.CategoryWarcry,
			fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="username">%s</ansi> lets out a thunderous warcry!</ansi>`, "Kesh"),
			speechWant{"Kesh lets out a thunderous warcry!", "Something lets out a thunderous warcry!",
				"Kesh lets out a thunderous warcry!", "A figure lets out a thunderous warcry!", "Something lets out a thunderous warcry!"}},
		{"mob warcry", false, messaging.CategoryWarcry,
			fmt.Sprintf(`<ansi fg="red-bold"><ansi fg="mobname">%s</ansi> lets out a bone-shaking warcry!</ansi>`, "Grel"),
			speechWant{"Grel lets out a bone-shaking warcry!", "Something lets out a bone-shaking warcry!",
				"Grel lets out a bone-shaking warcry!", "A figure lets out a bone-shaking warcry!", "Something lets out a bone-shaking warcry!"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := newSpeechScene(t)
			checkSpeech(t, sc, c.player, func(a Actor) { SendHeard(a, c.cat, c.line) }, c.want)
		})
	}
}

// Emotes are seen: nothing for a listener who cannot see, a bare own name
// hidden at shapes, and only a player's free text spared the deafened.
func TestSendSeen_Emotes(t *testing.T) {
	cases := []struct {
		name    string
		player  bool
		cat     messaging.Category
		line    string
		chatter bool
		want    speechWant
	}{
		{"player free-form", true, messaging.CategoryEmote,
			FormatEmoteText("Kesh", "waves, and Kesh grins.", "username"), true,
			speechWant{"Kesh waves, and Kesh grins.", "", "", "A figure waves, and a figure grins.", ""}},
		{"player empty", true, messaging.CategoryEmote,
			`<ansi fg="username">Kesh</ansi> emotes.`, false,
			speechWant{"Kesh emotes.", "", "Kesh emotes.", "A figure emotes.", ""}},
		{"player alias", true, messaging.CategoryEmote,
			FormatEmoteText("Kesh", EmoteAliases["beam"], "username"), false,
			speechWant{"Kesh beams with pride.", "", "Kesh beams with pride.", "A figure beams with pride.", ""}},
		// chatter true on purpose: a mob is never deafen-filtered (ruling 6).
		{"mob free-form", false, messaging.CategoryMobEmote,
			FormatEmoteText("Grel", "sniffs the air, and Grel growls.", "mobname"), true,
			speechWant{"Grel sniffs the air, and Grel growls.", "", "Grel sniffs the air, and Grel growls.",
				"A figure sniffs the air, and a figure growls.", ""}},
		{"mob empty", false, messaging.CategoryMobEmote,
			`<ansi fg="mobname">Grel</ansi> emotes.`, false,
			speechWant{"Grel emotes.", "", "Grel emotes.", "A figure emotes.", ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := newSpeechScene(t)
			checkSpeech(t, sc, c.player, func(a Actor) { SendSeen(a, c.cat, c.line, c.chatter) }, c.want)
		})
	}
}

// A speaker somehow still hidden after the reveal (spec S2) is unseen by
// every listener, whatever their sight: the line already reads "Someone"
// before it reaches SendCommunicationHidingNames, so no listener's own sight
// can recover the name.
func TestSendSpoken_AStillHiddenSpeakerIsUnseenByAll(t *testing.T) {
	sc := newSpeechScene(t)
	checkSpeech(t, sc, true, func(a Actor) {
		sendSpoken(a, a.GetRoom(), messaging.CategorySpeech,
			FormatSayText("Kesh", "psst", "username", "saytext"), true)
	}, speechWant{
		clear: `Someone says, "psst"`, blind: `Someone says, "psst"`, deaf: "",
		shapes: `Someone says, "psst"`, dark: `Someone says, "psst"`,
	})
}

func TestSay_PerListener(t *testing.T) {
	t.Run("player", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, true, func(a Actor) { Say(a, "hello there") }, speechWant{
			clear: `Kesh says, "hello there"`, blind: `Someone says, "hello there"`, deaf: "",
			shapes: `A figure says, "hello there"`, dark: `Someone says, "hello there"`})
	})
	t.Run("mob", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, false, func(a Actor) { Say(a, "hello there") }, speechWant{
			clear: `Grel says, "hello there"`, blind: `Someone says, "hello there"`, deaf: `Grel says, "hello there"`,
			shapes: `A figure says, "hello there"`, dark: `Someone says, "hello there"`})
	})
}

func TestShout_PerListener(t *testing.T) {
	t.Run("player", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, true, func(a Actor) { Shout(a, "HELP") }, speechWant{
			clear: `Kesh shouts, "HELP"`, blind: `Someone shouts, "HELP"`, deaf: "",
			shapes: `A figure shouts, "HELP"`, dark: `Someone shouts, "HELP"`})
	})
	t.Run("mob", func(t *testing.T) {
		sc := newSpeechScene(t)
		checkSpeech(t, sc, false, func(a Actor) { Shout(a, "HELP") }, speechWant{
			clear: `Grel shouts, "HELP"`, blind: `Someone shouts, "HELP"`, deaf: `Grel shouts, "HELP"`,
			shapes: `A figure shouts, "HELP"`, dark: `Someone shouts, "HELP"`})
	})
}

func TestShout_NextDoorHearsTheWordsAndNoName(t *testing.T) {
	sc := newSpeechScene(t)

	Shout(sc.speaker(true, sc.lit), "HELP")
	far := events.DrainQueuedRoomMessagesForTest(speechNextRoom)
	require.Len(t, far, 1, "a player's shout reaches next door once")
	require.Equal(t, `Someone shouts from the south direction, "HELP"`,
		strings.TrimSpace(speechTag.ReplaceAllString(far[0].Text, "")))
	require.True(t, far[0].IsCommunication, "a player's shout next door is still player chatter")

	sc.drain()
	Shout(sc.speaker(false, sc.lit), "HELP")
	msgs := events.DrainQueuedMessageEventsForTest(speechNextId)
	require.Len(t, msgs, 1, "a mob's shout next door now carries its words")
	require.Equal(t, `Someone shouts from the south direction, "HELP"`,
		strings.TrimSpace(speechTag.ReplaceAllString(msgs[0].Text, "")))
	require.False(t, msgs[0].IsCommunication, "an NPC's shout is never deafen-filtered")
}

// Rider 5: a hidden mob that shouts is revealed, as a player is.
func TestShout_RevealsAHiddenMob(t *testing.T) {
	sc := newSpeechScene(t)
	hideRhetoricActor(t, &sc.mob.Character)

	Shout(sc.speaker(false, sc.lit), "HELP")
	require.False(t, sc.mob.Character.IsHidden(), "shouting reveals a hidden mob")
	speechExpectOne(t, "clear", speechHeard(t, speechClearId), `Grel shouts, "HELP"`)
}

func TestShout_WakesTheRoom(t *testing.T) {
	for _, player := range []bool{true, false} {
		sc := newSpeechScene(t)
		sleeper := users.GetByUserId(speechClearId).Character
		require.True(t, sleeper.Conditions.AddCondition(speechSleepCond, false))
		require.True(t, sleeper.HasConditionFlag(conditions.Sleeping))

		Shout(sc.speaker(player, sc.lit), "WAKE UP")
		require.False(t, sleeper.HasConditionFlag(conditions.Sleeping), "player speaker %v", player)
	}
}

// A shout wakes every sleeping mob in the room, but a mob shouter never
// wakes itself. Both the shouting mob and a bystander mob must actually be
// registered in the room's mob list (mobs.GetInstance + Room.AddMob), since
// wakeSleepers walks room.GetMobs(), not the scene's sc.mob pointer alone.
func TestShout_WakesSleepingMobsButNeverTheMobShouter(t *testing.T) {
	sc := newSpeechScene(t)

	mobs.SetInstanceForTest(speechMobInst, sc.mob)
	t.Cleanup(func() { mobs.SetInstanceForTest(speechMobInst, nil) })
	sc.lit.AddMob(speechMobInst)
	require.True(t, sc.mob.Character.Conditions.AddCondition(speechSleepCond, false))
	require.True(t, sc.mob.Character.HasConditionFlag(conditions.Sleeping))

	bystanderChar := characters.New()
	bystanderChar.Name = "Bysh"
	bystander := &mobs.Mob{InstanceId: speechMobInst2, Character: *bystanderChar}
	mobs.SetInstanceForTest(speechMobInst2, bystander)
	t.Cleanup(func() { mobs.SetInstanceForTest(speechMobInst2, nil) })
	sc.lit.AddMob(speechMobInst2)
	require.True(t, bystander.Character.Conditions.AddCondition(speechSleepCond, false))
	require.True(t, bystander.Character.HasConditionFlag(conditions.Sleeping))

	Shout(sc.speaker(false, sc.lit), "WAKE UP")

	require.False(t, bystander.Character.HasConditionFlag(conditions.Sleeping),
		"a sleeping bystander mob in the room is woken by a shout")
	require.True(t, sc.mob.Character.HasConditionFlag(conditions.Sleeping),
		"a mob shouter does not wake itself")
}

// A sleeping player who shouts is skipped by their own shout's wake loop,
// while another sleeping player in the room is still woken.
func TestShout_PlayerShouterNeverWakesItself(t *testing.T) {
	sc := newSpeechScene(t)

	speaker := sc.player.Character
	require.True(t, speaker.Conditions.AddCondition(speechSleepCond, false))
	require.True(t, speaker.HasConditionFlag(conditions.Sleeping))

	other := users.GetByUserId(speechClearId).Character
	require.True(t, other.Conditions.AddCondition(speechSleepCond, false))
	require.True(t, other.HasConditionFlag(conditions.Sleeping))

	Shout(sc.speaker(true, sc.lit), "WAKE UP")

	require.True(t, speaker.HasConditionFlag(conditions.Sleeping),
		"a player shouter does not wake itself")
	require.False(t, other.HasConditionFlag(conditions.Sleeping),
		"another sleeping player in the room is woken by the shout")
}
