package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Owner rule, 2026-10-05: a line sent to a snapshot is judged by the room as
// it was when the snapshot was taken, and reaches only those who were there
// then and are still there now.
func TestSendTextVisualToSnapshot_JudgesTheRoomBeforeTheChange(t *testing.T) {
	r := sightTestRoom(t, "city")
	r.SkyLight, r.Lamp = SkyLightPtr(0), LampPtr(60)
	r.RemovePlayer(7413) // not here when the snapshot is taken

	snap := r.VisualSnapshot()
	require.Equal(t, VisualSnapshot{7411: messaging.SightFull, 7412: messaging.SightFull}, snap)

	// The change: the room goes dark, Bobrick leaves, Ordel arrives.
	r.Lamp = nil
	require.Equal(t, messaging.SightNone, r.ParticipantSight(7411), "fixture: the room must now be dark")
	r.RemovePlayer(7412)
	r.AddPlayer(7413)
	for _, id := range []int{7411, 7412, 7413} {
		events.DrainQueuedMessagesForTest(id)
	}

	r.SendTextVisualToSnapshot(snap, messaging.CategoryEquipment,
		`<ansi fg="username">Kesh</ansi> puts on a lantern.`, []string{"Kesh"})

	stayed := events.DrainQueuedMessageEventsForTest(7411)
	require.Len(t, stayed, 1, "a player who saw the moment and is still here reads the line")
	require.Equal(t, "Kesh puts on a lantern.", hidingSenderText(stayed[0]))
	require.Empty(t, events.DrainQueuedMessageEventsForTest(7412), "a player who has left reads nothing")
	require.Empty(t, events.DrainQueuedMessageEventsForTest(7413), "a player who arrived after the moment reads nothing")
}

// A player recorded at shapes has the names hidden, even if the room has since
// lit up, and one recorded at nothing reads nothing.
func TestSendTextVisualToSnapshot_ShapesHideNamesAndNoneIsSilent(t *testing.T) {
	r := hidingSenderDark(t)
	snap := r.VisualSnapshot()
	require.Equal(t, messaging.SightShapes, snap[7412])
	require.Equal(t, messaging.SightNone, snap[7413])

	r.Lamp = LampPtr(60)
	require.Equal(t, messaging.SightFull, r.ParticipantSight(7413), "fixture: the room must now be lit")

	r.SendTextVisualToSnapshot(snap, messaging.CategoryConditionApply,
		`A pall gathers around Kesh.`, []string{"Kesh"})

	shapes := events.DrainQueuedMessageEventsForTest(7412)
	require.Len(t, shapes, 1)
	require.Equal(t, "A pall gathers around a figure.", hidingSenderText(shapes[0]))
	require.Empty(t, events.DrainQueuedMessageEventsForTest(7413), "a player blind at the moment reads nothing")
}

// #456: SendTextVisualWithAudioToSnapshot is the sound twin. A departure is
// judged by the room before the mover left with their light: a reader who
// saw them then reads the line, one who saw nothing then hears the sound,
// and one who was not there then reads nothing.
func TestSendTextVisualWithAudioToSnapshot_SightThenSoundAtTheMoment(t *testing.T) {
	r := sightTestRoom(t, "cave")
	r.Lamp = LampPtr(60) // the mover's light, still here
	ordel := users.GetByUserId(7413).Character
	ordel.Perception = characters.New().Perception
	require.NoError(t, ordel.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	r.RemovePlayer(7412) // not here when the snapshot is taken

	snap := r.VisualSnapshot()
	require.Equal(t, VisualSnapshot{7411: messaging.SightFull, 7413: messaging.SightNone}, snap)

	// The change: the light leaves with its bearer; Bobrick arrives.
	r.Lamp = nil
	require.Equal(t, messaging.SightNone, r.ParticipantSight(7411), "fixture: the room must now be dark")
	r.AddPlayer(7412)
	for _, id := range []int{7411, 7412, 7413} {
		events.DrainQueuedMessagesForTest(id)
	}

	r.SendTextVisualWithAudioToSnapshot(snap, messaging.CategoryRoomExit,
		`<ansi fg="username">Kesh</ansi> leaves to the north.`, `You hear someone leave the room.`)

	saw := events.DrainQueuedMessageEventsForTest(7411)
	require.Len(t, saw, 1, "a reader who saw the mover then reads the line")
	require.Equal(t, "Kesh leaves to the north.", hidingSenderText(saw[0]))

	heard := events.DrainQueuedMessageEventsForTest(7413)
	require.Len(t, heard, 1, "a reader who saw nothing then hears it")
	require.Equal(t, "You hear someone leave the room.", hidingSenderText(heard[0]))

	require.Empty(t, events.DrainQueuedMessageEventsForTest(7412), "a reader who arrived after reads nothing")
}
