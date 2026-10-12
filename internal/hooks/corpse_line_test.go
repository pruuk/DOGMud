package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Ledger row 15: the body line is judged per reader by sight.
func TestAnnounceCorpse_ReadsByTheReadersSight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()

	reader := users.GetByUserId(1)
	require.NotNil(t, reader)
	identity := `<ansi fg="mobname">Skeleton</ansi>`

	cases := []struct {
		name  string
		sight messaging.SightDecision
		want  []string
	}{
		{"full sight reads the name", messaging.SightFull,
			[]string{"Skeleton's body lies where it fell."}},
		{"shapes reads an anonymous body", messaging.SightShapes,
			[]string{"A body lies where it fell."}},
		{"no sight reads nothing", messaging.SightNone, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.sight != messaging.SightFull {
				rooms.LoadRoom(1).Lamp = nil
				darken(t, 1)
			} else {
				rooms.LoadRoom(1).Lamp = rooms.LampPtr(60)
			}
			setDarkSight(reader.Character, tc.sight)
			require.Equal(t, tc.sight, messaging.ReaderSight(reader.Character, rooms.LoadRoom(1)),
				"fixture: the reader must be judged at this sight")
			drainPlain(1)

			announceCorpse(rooms.LoadRoom(1), identity)

			got := drainPlain(1)
			assert.Equal(t, len(tc.want), len(got))
			for i := range tc.want {
				assert.Equal(t, tc.want[i], got[i])
			}
		})
	}
}

// The dead player is still listed in the room; they get no body line.
func TestAnnounceCorpse_ExcludesTheDeadPlayer(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(60)
	drainPlain(1)
	drainPlain(2)

	announceCorpse(rooms.LoadRoom(1), `<ansi fg="username">Aliceia</ansi>`, 1)

	assert.Empty(t, drainPlain(1))
	assert.Equal(t, []string{"Aliceia's body lies where it fell."}, drainPlain(2))
}

func TestDropMobLootAndSetCorpse_BodyLineOnlyWhenACorpseIsLeft(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(60)
	defer configs.AddOverlayOverrides(map[string]any{
		"GamePlay.Death.CorpsesEnabled": false,
	})

	for _, enabled := range []bool{true, false} {
		require.NoError(t, configs.AddOverlayOverrides(map[string]any{
			"GamePlay.Death.CorpsesEnabled": enabled,
		}))
		m := &mobs.Mob{}
		m.Character = characters.Character{Name: "Skeleton"}
		room := rooms.LoadRoom(1)
		drainPlain(1)

		dropMobLootAndSetCorpse(m, room)

		got := drainPlain(1)
		if enabled {
			assert.Equal(t, []string{"Skeleton's body lies where it fell."}, got)
		} else {
			assert.Empty(t, got, "no corpse, no body line")
		}
	}
}
