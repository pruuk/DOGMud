package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// #491: a grapple participant's personal line names the partner, so the
// partner's name is hidden by the READER's own sight, the way combat's
// personal lines are. A room broadcast already did this per observer; the two
// participant lines went out raw and read the name of a creature the room
// listed as "a figure".
func TestGrapplePersonalLine_HidesThePartnerByTheReadersSight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()

	reader := users.GetByUserId(1)
	partner := users.GetByUserId(2)
	require.NotNil(t, reader)
	require.NotNil(t, partner)
	partnerName := partner.Character.Name
	// The tagged shape grapplemessaging.RenderTriad fills {actor}/{actee} with.
	line := "Pressure stays on. " + characterDisplayName(partner.Character) +
		" can't generate the leverage to move you."

	cases := []struct {
		name  string
		sight messaging.SightDecision
		want  string
		keep  bool
	}{
		{"full sight reads the name", messaging.SightFull, partnerName, true},
		{"shapes reads a figure", messaging.SightShapes, "A figure", false},
		{"no sight reads something", messaging.SightNone, "Something", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.sight != messaging.SightFull {
				rooms.LoadRoom(1).Lamp = nil
				darken(t, 1)
			} else {
				rooms.LoadRoom(1).Lamp = rooms.LampPtr(60) // comfortably lit
			}
			setDarkSight(reader.Character, tc.sight)
			require.Equal(t, tc.sight, messaging.ReaderSight(reader.Character, rooms.LoadRoom(1)),
				"fixture: the reader must be judged at this sight")
			drainPlain(1)

			sendToCharacter(reader.Character, partner.Character, line)

			got := drainPlain(1)
			require.Len(t, got, 1)
			assert.Contains(t, got[0], tc.want)
			if tc.keep {
				assert.Contains(t, got[0], partnerName)
			} else {
				assert.NotContains(t, got[0], partnerName)
			}
		})
	}
}

// A line that names no one (the exhaustion line) passes a nil partner and
// arrives untouched even for a reader who sees nothing.
func TestGrapplePersonalLine_NilPartnerLeavesTheLineAlone(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	reader := users.GetByUserId(1)
	setDarkSight(reader.Character, messaging.SightNone)
	drainPlain(1)

	sendToCharacter(reader.Character, nil, "Exhaustion strips the control.")

	got := drainPlain(1)
	require.Len(t, got, 1)
	assert.Equal(t, "Exhaustion strips the control.", got[0])
}
