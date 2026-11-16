package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const moveCliffBiome = "movetestcliff"

func seedMoveBiomes(t *testing.T) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		moveCliffBiome: {BiomeId: moveCliffBiome, MovementCost: 2.5},
	}))
}

// setupMover shapes a character the same way for both actor kinds.
type moverSetup struct {
	stamina, staminaMax int
	hidden              bool
	overloaded          bool
}

func (s moverSetup) apply(t *testing.T, c *characters.Character) {
	t.Helper()
	c.Stamina = s.stamina
	c.StaminaMax.Value = s.staminaMax
	c.Stats.Strength.ValueAdj = 10
	if s.overloaded {
		c.Items = []items.Item{{ItemId: 99101, Spec: &items.ItemSpec{ItemId: 99101, Name: "anvil", Weight: 100000}}}
	}
	if s.hidden {
		c.Awareness = awareness.NewMachine()
		reason := state.TransitionReason{Trigger: "move_test"}
		require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
		c.Awareness.ResolveConcealment(true, reason)
		require.True(t, c.IsHidden())
	}
}

func newMoveUser(t *testing.T, s moverSetup) Actor {
	t.Helper()
	u := users.NewTestUser(9601, "mover", "Mover", 0)
	u.Character = characters.New()
	s.apply(t, u.Character)
	return NewUserActor(u)
}

func newMoveMob(t *testing.T, s moverSetup) Actor {
	t.Helper()
	m := &mobs.Mob{InstanceId: 9602}
	m.Character = *characters.New()
	s.apply(t, &m.Character)
	return NewMobActor(m)
}

// The parity table: the same character pays the same price whether a player
// or a mob is walking (movement parity 4b, owner ruling 2). Since #401 the
// price is stamina alone: action points were removed, and a heavy load costs
// more stamina through the encumbrance curve instead of more points.
func TestChargeMove_PlayerAndMobPayTheSame(t *testing.T) {
	seedMoveBiomes(t)
	road := &rooms.Room{RoomId: 9610}
	cliff := &rooms.Room{RoomId: 9611, Biome: moveCliffBiome}

	cases := []struct {
		name        string
		setup       moverSetup
		dest        *rooms.Room
		wantRefusal MoveRefusal
	}{
		{"plain step", moverSetup{stamina: 100, staminaMax: 100}, road, MoveOK},
		{"cliff step", moverSetup{stamina: 100, staminaMax: 100}, cliff, MoveOK},
		{"hidden step", moverSetup{stamina: 100, staminaMax: 100, hidden: true}, road, MoveOK},
		{"over capacity", moverSetup{stamina: 100, staminaMax: 100, overloaded: true}, road, MoveOK},
		{"exhausted", moverSetup{stamina: 0, staminaMax: 100}, cliff, MoveRefuseExhausted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			player := newMoveUser(t, tc.setup)
			mob := newMoveMob(t, tc.setup)

			pc := ChargeMove(player, tc.dest)
			mc := ChargeMove(mob, tc.dest)

			require.Equal(t, tc.wantRefusal, pc.Refusal, "player refusal")
			require.Equal(t, pc.Refusal, mc.Refusal, "mob refusal must match the player's")
			require.InDelta(t, pc.StaminaCost, mc.StaminaCost, 1e-12)
			require.InDelta(t, MovePrice(player.GetCharacter(), tc.dest), pc.StaminaCost, 1e-12, "ChargeMove must price through MovePrice")
			if tc.wantRefusal == MoveRefuseExhausted {
				require.Equal(t, 0, player.GetCharacter().Stamina, "a refused step takes nothing")
			}
		})
	}
}

// A heavy load is paid in stamina (#401): over capacity a step costs the
// encumbrance curve's top times an empty one, and a walker who can pay it
// loses that stamina.
func TestChargeMove_OverCapacityCostsMoreStamina(t *testing.T) {
	road := &rooms.Room{RoomId: 9610}
	light := newMoveUser(t, moverSetup{stamina: 100, staminaMax: 100})
	heavy := newMoveUser(t, moverSetup{stamina: 100, staminaMax: 100, overloaded: true})

	lightPrice := MovePrice(light.GetCharacter(), road)
	heavyPrice := MovePrice(heavy.GetCharacter(), road)
	require.Greater(t, heavyPrice, lightPrice*4, "a full load multiplies the step's stamina")

	require.True(t, ChargeMove(heavy, road).OK())
	require.Less(t, heavy.GetCharacter().Stamina, 100, "the heavy step took stamina")
}

// Terrain and hidden multiply the stamina price (V9); a cliff costs 2.5 road
// steps and hidden triples it, until the step cap.
func TestMovePrice_TerrainAndHidden(t *testing.T) {
	seedMoveBiomes(t)
	road := &rooms.Room{RoomId: 9610}
	cliff := &rooms.Room{RoomId: 9611, Biome: moveCliffBiome}
	plain := newMoveUser(t, moverSetup{stamina: 100, staminaMax: 100})
	hidden := newMoveUser(t, moverSetup{stamina: 100, staminaMax: 100, hidden: true})

	r := MovePrice(plain.GetCharacter(), road)
	c := MovePrice(plain.GetCharacter(), cliff)
	h := MovePrice(hidden.GetCharacter(), road)
	require.InDelta(t, 2.5, c/r, 1e-9, "a 2.5 biome costs 2.5 road steps")
	require.InDelta(t, 3.0, h/r, 1e-9, "hidden triples the price")
}

// A step the character can never pay, even rested, reports Never so a walker
// gives up instead of waiting forever.
func TestQuoteMove_NeverWhenThePriceExceedsTheWholePool(t *testing.T) {
	seedMoveBiomes(t)
	cliff := &rooms.Room{RoomId: 9611, Biome: moveCliffBiome}
	tired := newMoveMob(t, moverSetup{stamina: 0, staminaMax: 100})
	q := QuoteMove(tired, cliff)
	require.Equal(t, MoveRefuseExhausted, q.Refusal)
	require.False(t, q.Never, "a rested mob could pay this step")

	// Stamina 0 so the quote refuses; a pool of 1 so no rest can ever cover
	// a cliff step.
	frail := newMoveMob(t, moverSetup{stamina: 0, staminaMax: 1})
	q = QuoteMove(frail, cliff)
	require.Equal(t, MoveRefuseExhausted, q.Refusal)
	require.True(t, q.Never, "a 1-stamina mob can never pay a cliff step")
}

// A quote spends nothing.
func TestQuoteMove_SpendsNothing(t *testing.T) {
	road := &rooms.Room{RoomId: 9610}
	mob := newMoveMob(t, moverSetup{stamina: 100, staminaMax: 100})
	q := QuoteMove(mob, road)
	require.True(t, q.OK())
	require.Equal(t, 100, mob.GetCharacter().Stamina)
}
