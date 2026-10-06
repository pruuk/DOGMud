package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func hidingSenderText(m events.Message) string {
	return strings.TrimSpace(sightTestTag.ReplaceAllString(m.Text, ""))
}

// hidingSenderLit lights the fixture room exactly (no sky, a lamp in the faces
// band) and blinds Ordel (7413) with the perception machine.
func hidingSenderLit(t *testing.T) *Room {
	t.Helper()
	r := sightTestRoom(t, "city")
	r.SkyLight, r.Lamp = SkyLightPtr(0), LampPtr(60)
	ordel := users.GetByUserId(7413).Character
	ordel.Perception = characters.New().Perception
	require.NoError(t, ordel.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.Equal(t, messaging.SightFull, r.ParticipantSight(7412))
	require.Equal(t, messaging.SightNone, r.ParticipantSight(7413))
	return r
}

// hidingSenderDark is the unlit fixture with Bobrick (7412) on infrared.
func hidingSenderDark(t *testing.T) *Room {
	t.Helper()
	r := sightTestRoom(t, "cave")
	require.True(t, users.GetByUserId(7412).Character.Conditions.AddCondition(sightTestInfraredConditionId, true))
	require.Equal(t, messaging.SightShapes, r.ParticipantSight(7412))
	require.Equal(t, messaging.SightNone, r.ParticipantSight(7413))
	return r
}

func TestSendTextHidingNames_HeardByAllNamedByNone(t *testing.T) {
	r := hidingSenderDark(t)
	r.SendTextHidingNames(messaging.CategoryRally, `<ansi fg="username">Aliceia</ansi> lets out a roar!`,
		[]string{"Aliceia"}, messaging.HideNames, 7411)

	shapes := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, shapes, 1)
	require.Equal(t, "A figure lets out a roar!", hidingSenderText(shapes[0]))
	require.False(t, shapes[0].IsCommunication, "an authored sound is never deafen-filtered")

	none := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, none, 1, "audio reaches a listener who sees nothing")
	require.Equal(t, "Something lets out a roar!", hidingSenderText(none[0]))

	require.Empty(t, events.DrainQueuedMessageEventsForTest(7411), "the excluded maker reads nothing")
}

// The defect the deleted mobcommands.sendAudioRoomText carried: a lit room
// named the speaker to a blinded listener.
func TestSendTextHidingNames_NoLitRoomShortcut(t *testing.T) {
	r := hidingSenderLit(t)
	r.SendTextHidingNames(messaging.CategorySpeech,
		`<ansi fg="mobname">Grel</ansi> says, "<ansi fg="saytext-mob">hello</ansi>"`,
		[]string{"Grel"}, messaging.HideSpeakerNames)

	lit := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, lit, 1)
	require.Equal(t, `Grel says, "hello"`, hidingSenderText(lit[0]))
	blind := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, blind, 1)
	require.Equal(t, `Someone says, "hello"`, hidingSenderText(blind[0]))
}

func TestSendCommunicationHidingNames_IsPlayerChatter(t *testing.T) {
	r := hidingSenderDark(t)
	r.SendCommunicationHidingNames(messaging.CategorySpeech,
		`<ansi fg="username">Aliceia</ansi> says, "<ansi fg="saytext">I am Aliceia</ansi>"`,
		[]string{"Aliceia"}, 7411)

	shapes := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, shapes, 1)
	require.Equal(t, `A figure says, "I am Aliceia"`, hidingSenderText(shapes[0]), "the words arrive untouched")
	require.True(t, shapes[0].IsCommunication)
	none := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, none, 1)
	require.Equal(t, `Someone says, "I am Aliceia"`, hidingSenderText(none[0]))
	require.True(t, none[0].IsCommunication)
	require.Empty(t, events.DrainQueuedMessageEventsForTest(7411))
}

func TestSendVisualCommunicationHidingNames_SeenAndMarked(t *testing.T) {
	r := hidingSenderDark(t)
	r.SendVisualCommunicationHidingNames(messaging.CategoryEmote,
		`<ansi fg="username">Aliceia</ansi> <ansi fg="137">waves, and Aliceia grins.</ansi>`,
		[]string{"Aliceia"}, 7411)

	shapes := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, shapes, 1)
	require.Equal(t, "A figure waves, and a figure grins.", hidingSenderText(shapes[0]))
	require.True(t, shapes[0].IsCommunication)
	require.Empty(t, events.DrainQueuedMessageEventsForTest(7413), "an emote is seen, not heard")
}

func TestVisualSendersStayOffTheDeafenFilter(t *testing.T) {
	r := sightTestRoom(t, "city")
	r.SkyLight, r.Lamp = SkyLightPtr(0), LampPtr(60)
	require.Equal(t, messaging.SightFull, r.ParticipantSight(7413))
	sends := map[string]func(){
		"SendTextVisual": func() { r.SendTextVisual(messaging.CategoryEmote, "Aliceia waves.", 7411) },
		"SendTextVisualHidingNames": func() {
			r.SendTextVisualHidingNames(messaging.CategoryEmote, "Aliceia waves.", []string{"Aliceia"}, 7411)
		},
		"SendTextVisualToSnapshot": func() {
			r.SendTextVisualToSnapshot(r.VisualSnapshot(), messaging.CategoryEmote, "Aliceia waves.", []string{"Aliceia"}, 7411)
		},
	}
	for name, send := range sends {
		send()
		msgs := events.DrainQueuedMessageEventsForTest(7413)
		require.Len(t, msgs, 1, name)
		require.False(t, msgs[0].IsCommunication, "%s must not be deafen-filtered", name)
	}
}
