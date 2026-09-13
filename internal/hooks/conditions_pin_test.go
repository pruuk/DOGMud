package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Pin: a poisoned player at 1 health dies to the poison tick and the death
// cause reads "poison" — this is the regression case the fix exists for.
//
// TestPin_PoisonTickKillsAndNamesTheCause: a ONE-trigger poison record
// (buffs.TickTriggers(3)) has its only trigger land on the third
// UserRoundTick call (buff 121's triggerrate is three rounds), and that
// trigger is also the record's LAST: Buffs.Trigger() decrements
// TriggersLeft before returning the buff, so the record already reads
// Expired by the time deathCauseFor runs. deathCauseFor must still read
// "poison" immediately after (the expired-but-still-held record read by id),
// AND after PruneBuffs removes the expired record outright (from
// Character.LastTickCause, stamped by the tick that landed the harm) — the
// prune race the fix also covers.
func TestPin_PoisonTickKillsAndNamesTheCause(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	_ = u.Character.AddBuffMagnitude(buffs.BuffIdPoisoned, buffs.TickTriggers(3), -5, "pin")
	u.Character.Health = 1

	// No regen lands in the round tick: 1 - 5 <= 0.
	UserRoundTick(events.NewRound{RoundNumber: 1})
	UserRoundTick(events.NewRound{RoundNumber: 2})
	UserRoundTick(events.NewRound{RoundNumber: 3})
	require.LessOrEqual(t, u.Character.Health, 0, "the poison tick must take the last point")
	stampedCause := u.Character.LastTickCause
	u.Character.LastTickCause = "" // isolate: this assertion exercises the by-id read alone, not the LastTickCause fallback
	require.Equal(t, "poison", deathCauseFor(u.Character), "before prune: the expired-but-still-held record must still be read")
	u.Character.LastTickCause = stampedCause // restore: the after-prune assertion below exercises the fallback

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	require.Equal(t, "poison", deathCauseFor(u.Character), "after prune: LastTickCause must carry the cause once the record is gone")
}

// TestPin_BleedTickKillsAndNamesTheCause is
// TestPin_PoisonTickKillsAndNamesTheCause's sibling for Bleeding: same
// one-trigger-is-the-last-trigger hole, same before/after-prune coverage.
func TestPin_BleedTickKillsAndNamesTheCause(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	u := users.GetByUserId(1)
	_ = u.Character.AddBuffMagnitude(buffs.BuffIdBleeding, buffs.TickTriggers(3), -5, "pin")
	u.Character.Health = 1

	// No regen lands in the round tick: 1 - 5 <= 0.
	UserRoundTick(events.NewRound{RoundNumber: 1})
	UserRoundTick(events.NewRound{RoundNumber: 2})
	UserRoundTick(events.NewRound{RoundNumber: 3})
	require.LessOrEqual(t, u.Character.Health, 0, "the bleed tick must take the last point")
	stampedCause := u.Character.LastTickCause
	u.Character.LastTickCause = "" // isolate: this assertion exercises the by-id read alone, not the LastTickCause fallback
	require.Equal(t, "bleeding out", deathCauseFor(u.Character), "before prune: the expired-but-still-held record must still be read")
	u.Character.LastTickCause = stampedCause // restore: the after-prune assertion below exercises the fallback

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	require.Equal(t, "bleeding out", deathCauseFor(u.Character), "after prune: LastTickCause must carry the cause once the record is gone")
}

// TestPin_DeathCauseOrder pins deathCauseFor (extracted from
// Death_PlayerAnnouncement.go's dmgCt==0 branch, lines 117-134 originally,
// now its own function so it can be pinned without driving the Life FSM or
// capturing worldevents.EmitWorldEvent). Order matters: poisoned wins over
// bleeding because the poison check runs first.
func TestPin_DeathCauseOrder(t *testing.T) {
	defer buffs.SeedConditionRecordsForTest()()

	newChar := func() *characters.Character {
		c := &characters.Character{}
		c.Buffs.Validate(true)
		return c
	}

	t.Run("poisoned", func(t *testing.T) {
		c := newChar()
		_ = c.AddBuffMagnitude(buffs.BuffIdPoisoned, 10, -5, "pin")
		require.Equal(t, "poison", deathCauseFor(c))
	})

	t.Run("bleeding", func(t *testing.T) {
		c := newChar()
		_ = c.AddBuffMagnitude(buffs.BuffIdBleeding, 10, -3, "pin")
		require.Equal(t, "bleeding out", deathCauseFor(c))
	})

	t.Run("poisoned and bleeding, poison wins", func(t *testing.T) {
		c := newChar()
		_ = c.AddBuffMagnitude(buffs.BuffIdPoisoned, 10, -5, "pin")
		_ = c.AddBuffMagnitude(buffs.BuffIdBleeding, 10, -3, "pin")
		require.Equal(t, "poison", deathCauseFor(c))
	})
}

// TestPin_AStaleTickCauseDoesNotNameTheDeath pins the LastTickCauseRound
// bound added alongside LastTickCause (see Death_PlayerAnnouncement.go's
// deathCauseFor): a character holding no lethal-condition records and not
// in combat, whose LastTickCause was stamped five rounds ago, must NOT have
// that stale cause read out. Without the round check a tick cause from an
// earlier, unrelated fight could outlive it and misname a later death.
func TestPin_AStaleTickCauseDoesNotNameTheDeath(t *testing.T) {
	defer buffs.SeedConditionRecordsForTest()()
	defer util.ResetRoundCountForTest()

	c := &characters.Character{}
	c.Buffs.Validate(true)

	current := util.GetRoundCount()
	c.LastTickCause = "poison"
	c.LastTickCauseRound = current - 5

	require.Equal(t, "their own foolishness", deathCauseFor(c), "a tick cause five rounds stale must not name the death")
}
