package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	poolTestSpeakerId  = 9601
	poolTestBeastId    = 9602
	poolTestRhetoricAt = "RHETORIC-LINE"
	poolTestWordlessAt = "WORDLESS-LINE"
)

func poolTestBands(marker string) map[combat.TauntIntensity]*combat.TauntMessages {
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

// runPoolCase runs a mob command at a player target with both pools seeded
// with distinguishable lines and returns what the player was sent.
func runPoolCase(t *testing.T, run func(string, *mobs.Mob, *rooms.Room) (bool, error), speciesId int, result actions.TauntResult) string {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	no := false
	species.SetSpeciesForTest(t, map[int]*species.Species{
		1:                 {SpeciesId: 1, Name: "plain"},
		poolTestSpeakerId: {SpeciesId: poolTestSpeakerId, Name: "speaker"},
		poolTestBeastId:   {SpeciesId: poolTestBeastId, Name: "beast", Speech: &no},
	})
	t.Cleanup(combat.SeedTauntPoolsForTest(map[string]map[combat.TauntIntensity]*combat.TauntMessages{
		combat.TauntPoolRhetoric: poolTestBands(poolTestRhetoricAt),
		combat.TauntPoolWordless: poolTestBands(poolTestWordlessAt),
	}))

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.SpeciesId = speciesId
	target := users.GetByUserId(1)
	room := rooms.LoadRoom(mob.Character.RoomId)
	oldLamp := room.Lamp
	room.Lamp = rooms.LampPtr(90)
	t.Cleanup(func() { room.Lamp = oldLamp })
	room.AddPlayer(target.UserId)
	t.Cleanup(func() { room.RemovePlayer(target.UserId) })
	mob.Character.SetAggro(target.UserId, 0, characters.DefaultAttack)

	original := executeTauntAction
	executeTauntAction = func(actions.Actor) actions.TauntResult {
		result.Executed = true
		result.Target = actions.AggroTarget{Char: target.Character, Name: target.Character.Name, UserId: target.UserId, Found: true}
		return result
	}
	t.Cleanup(func() { executeTauntAction = original })
	events.DrainQueuedMessagesForTest(target.UserId)

	handled, err := run("", mob, room)
	require.NoError(t, err)
	require.True(t, handled)
	return strings.Join(events.DrainQueuedMessagesForTest(target.UserId), "\n")
}

func TestMobTauntAndHowlReadTheRightPool(t *testing.T) {
	cases := []struct {
		name    string
		run     func(string, *mobs.Mob, *rooms.Room) (bool, error)
		species int
		want    string
	}{
		{"speaking mob taunts in rhetoric", Taunt, poolTestSpeakerId, poolTestRhetoricAt},
		{"speechless mob taunts wordless", Taunt, poolTestBeastId, poolTestWordlessAt},
		{"speaking mob howls wordless", Howl, poolTestSpeakerId, poolTestWordlessAt},
		{"speechless mob howls wordless", Howl, poolTestBeastId, poolTestWordlessAt},
	}
	for _, tc := range cases {
		for _, band := range []string{"hit", "crit", "miss", "fumble"} {
			t.Run(tc.name+"/"+band, func(t *testing.T) {
				result := actions.TauntResult{}
				switch band {
				case "hit":
					result.Hit = true
				case "crit":
					result.Hit, result.Crit = true, true
				case "fumble":
					result.Fumble = true
				}
				got := runPoolCase(t, tc.run, tc.species, result)
				if band == "fumble" {
					// A fumble speaks to the room only; the player target
					// hears the room line too, since they are in the room.
					require.Contains(t, got, tc.want)
					return
				}
				require.Contains(t, got, tc.want+" defender")
				other := poolTestRhetoricAt
				if tc.want == poolTestRhetoricAt {
					other = poolTestWordlessAt
				}
				require.NotContains(t, got, other)
			})
		}
	}
}

// The pool names characters returns are the ones combat loads.
func TestTauntPoolNamesMatchCharacters(t *testing.T) {
	no := false
	species.SetSpeciesForTest(t, map[int]*species.Species{
		poolTestSpeakerId: {SpeciesId: poolTestSpeakerId, Name: "speaker"},
		poolTestBeastId:   {SpeciesId: poolTestBeastId, Name: "beast", Speech: &no},
	})
	speaker, beast := characters.New(), characters.New()
	speaker.SpeciesId, beast.SpeciesId = poolTestSpeakerId, poolTestBeastId
	require.Equal(t, combat.TauntPoolRhetoric, speaker.TauntPool())
	require.Equal(t, combat.TauntPoolWordless, beast.TauntPool())
}
