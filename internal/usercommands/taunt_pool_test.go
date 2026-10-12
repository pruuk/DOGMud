package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	playerPoolSpeakerId = 9611
	playerPoolBeastId   = 9612
)

func playerPoolBands(marker string) map[combat.TauntIntensity]*combat.TauntMessages {
	bands := map[combat.TauntIntensity]*combat.TauntMessages{}
	for _, band := range []combat.TauntIntensity{combat.TauntHit, combat.TauntMiss, combat.TauntCritical, combat.TauntFumble} {
		bands[band] = &combat.TauntMessages{
			ToAttacker: []string{marker + " attacker"},
			ToDefender: []string{marker + " defender"},
			ToRoom:     []string{marker + " room"},
		}
	}
	return bands
}

// A speaker's taunt reads the rhetoric pool; a speechless player's (a future
// shapeshift) reads the wordless pool (M6 slice 3).
func TestPlayerTauntReadsPoolBySpeech(t *testing.T) {
	for _, tc := range []struct {
		name    string
		species int
		want    string
		other   string
	}{
		{"speaker", playerPoolSpeakerId, "RHETORIC", "WORDLESS"},
		{"speechless", playerPoolBeastId, "WORDLESS", "RHETORIC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			no := false
			species.SetSpeciesForTest(t, map[int]*species.Species{
				1:                   {SpeciesId: 1, Name: "plain"},
				playerPoolSpeakerId: {SpeciesId: playerPoolSpeakerId, Name: "speaker"},
				playerPoolBeastId:   {SpeciesId: playerPoolBeastId, Name: "beast", Speech: &no},
			})
			t.Cleanup(combat.SeedTauntPoolsForTest(map[string]map[combat.TauntIntensity]*combat.TauntMessages{
				combat.TauntPoolRhetoric: playerPoolBands("RHETORIC"),
				combat.TauntPoolWordless: playerPoolBands("WORDLESS"),
			}))
			attacker := users.GetByUserId(1)
			defender := users.GetByUserId(2)
			attacker.Character.SpeciesId = tc.species
			original := executeTauntAction
			executeTauntAction = func(actions.Actor) actions.TauntResult {
				return actions.TauntResult{
					Executed: true, Hit: true,
					Target: actions.AggroTarget{Char: defender.Character, Name: defender.Character.Name, UserId: defender.UserId, Found: true},
				}
			}
			t.Cleanup(func() { executeTauntAction = original })
			room := rooms.LoadRoom(1)
			events.DrainQueuedMessagesForTest(attacker.UserId)

			handled, err := Taunt("bobrick", attacker, room, 0)
			require.NoError(t, err)
			require.True(t, handled)

			got := strings.Join(events.DrainQueuedMessagesForTest(attacker.UserId), "\n")
			require.Contains(t, got, tc.want+" attacker")
			require.NotContains(t, got, tc.other)
		})
	}
}
