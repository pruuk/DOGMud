package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Empathic Shroud is a real hide (#444, owner 2026-10-09). The Awareness
// cascade that adds record 9 lives in internal/hooks and is not wired here,
// so these tests see exactly what this package does.

// seedStealthRecords seeds the two stealth records with their shipped flags.
func seedStealthRecords(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
		conditions.ConditionIdEmpathicShroud: {ConditionId: conditions.ConditionIdEmpathicShroud, Name: "Empathic Shroud",
			TriggerCount: 16, RoundInterval: 1,
			Flags: []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat}},
	}))
}

// newHider is a visible character with a bare Awareness machine and a known
// sneak base: Dexterity dex, Skullduggery 0. Stats are set on Base, because
// every add door runs Validate, which recalculates ValueAdj from Base.
func newHider(dex int) *Character {
	c := New()
	c.Awareness = awareness.NewMachine()
	c.Stats.Dexterity.Base = dex
	_ = c.Validate()
	return c
}

// sneakHide hides c as a sneak, the way the sneak command and its cascade
// leave it: Hidden as a sneak, holding a permanent record 9.
func sneakHide(t *testing.T, c *Character) {
	t.Helper()
	r := state.TransitionReason{Trigger: "test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
	c.Awareness.ResolveConcealment(true, r)
	require.NoError(t, c.AddCondition(9, true))
	require.True(t, c.IsHidden())
	require.False(t, c.HiddenByShroud())
}

func TestShroudRecord_EntersHiddenAtTheRecordScore(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
	require.True(t, c.IsHidden(), "record 31 must hide a visible holder")
	require.True(t, c.HiddenByShroud())
	require.Equal(t, 180.0, c.HideBaseScore(), "the shroud hides at the caster's score, not Dex plus Skullduggery")
	sw := float64(configs.GetBalanceConfig().SkillWeight)
	require.Equal(t, 50+float64(c.GetSkillLevel(skills.Skullduggery))*sw, c.SneakBaseScore())
}

func TestShroudRecord_NoMagnitudeUsesTheHoldersOwnScore(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	c.Stats.Willpower.Base = 70
	c.SetSkill("spellcasting", 4)
	require.NoError(t, c.Validate())
	require.NoError(t, c.AddCondition(conditions.ConditionIdEmpathicShroud, false))
	require.True(t, c.HiddenByShroud())
	want := 70 + 4*float64(configs.GetBalanceConfig().SkillWeight)
	require.Equal(t, want, c.HideBaseScore())
	require.Equal(t, want, c.OwnShroudScore())
}

// Ending: once no live 31 remains, Validate reveals the shroud hide. A sneak
// hide with no 31 is left alone.
func TestShroudHide_EndsWhenNoLiveRecordRemains(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
	require.True(t, c.IsHidden())

	c.RemoveCondition(conditions.ConditionIdEmpathicShroud) // expires it, then validates
	require.False(t, c.IsHidden(), "a shroud hide must end with its record")

	sneaker := newHider(50)
	sneakHide(t, sneaker)
	require.NoError(t, sneaker.Validate())
	require.True(t, sneaker.IsHidden(), "a sneak hide needs no record 31")
}

// One hide at a time, the stronger (owner, 2026-10-09): the shroud landing on
// a sneak hide.
func TestShroudLanding_OnASneakHide(t *testing.T) {
	t.Run("stronger shroud replaces the sneak", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		sneakHide(t, c)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 200, "spell"))
		require.True(t, c.IsHidden(), "the hide never lapses")
		require.True(t, c.HiddenByShroud())
		require.Equal(t, 200.0, c.HideBaseScore())
		require.False(t, c.Conditions.HasCondition(9), "record 9 is discarded, not cancelled, so no end line")
	})
	t.Run("weaker shroud is dropped", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(300)
		sneakHide(t, c)
		require.Error(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"),
			"a dropped shroud is refused so no start line is told")
		require.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud))
		require.True(t, c.IsHidden())
		require.False(t, c.HiddenByShroud())
		require.True(t, c.holdsLiveStealthRecord(), "the sneak keeps its record 9")
	})
	t.Run("a tie keeps the sneak already there", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(100)
		sneakHide(t, c)
		require.Error(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, c.SneakBaseScore(), "spell"))
		require.False(t, c.HiddenByShroud())
	})
}

// The other direction through the record door: a 9 landing on a shroud hide.
func TestStealthRecordLanding_OnAShroudHide(t *testing.T) {
	t.Run("stronger sneak base takes over", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(300)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"))
		require.NoError(t, c.AddCondition(9, true))
		require.True(t, c.IsHidden())
		require.False(t, c.HiddenByShroud())
		require.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud), "31 is discarded, not cancelled")
		require.NoError(t, c.Validate())
		require.True(t, c.IsHidden(), "with 31 gone the sneak hide holds")
	})
	t.Run("weaker sneak base is dropped", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 200, "spell"))
		require.Error(t, c.AddCondition(9, true))
		require.True(t, c.HiddenByShroud())
		require.False(t, c.Conditions.HasCondition(9))
	})
}

func TestSneakOverShroud_SwapsWithoutAReveal(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(300)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"))
	require.True(t, c.SneakOverShroud())
	require.True(t, c.IsHidden(), "no reveal")
	require.False(t, c.HiddenByShroud())
	require.Equal(t, c.SneakBaseScore(), c.HideBaseScore(), "the hide now scores as a sneak")
	require.False(t, c.Conditions.HasCondition(conditions.ConditionIdEmpathicShroud), "31 is discarded, so its end line is not told")
	require.True(t, c.holdsLiveStealthRecord(), "record 9 carries the sneak hide")
	require.False(t, c.SneakOverShroud(), "nothing left to take over")
}

// A re-cast while shroud-hidden keeps the stronger of the two scores (owner,
// 2026-10-09: one hide, the strongest), and the record carries it, so a
// reload reads the same score.
func TestShroudRecast_KeepsTheStrongerScore(t *testing.T) {
	t.Run("weaker recast keeps the held score", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 120, "spell"))
		require.True(t, c.HiddenByShroud())
		require.Equal(t, 180.0, c.HideBaseScore())
		require.Equal(t, 180.0, c.shroudRecordScore(), "the record keeps the stronger score too")
	})
	t.Run("stronger recast takes the new score", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 120, "spell"))
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
		require.Equal(t, 180.0, c.HideBaseScore())
	})
}

// A record 9 that wins a hide (or lands on a sneak hide) holds it as a
// permanent record, the way the Hidden cascade adds it. The event path adds 9
// with isPermanent false (an admin setcondition, a mob's add_condition 9),
// and 9 has no triggercount, so as authored it would be pruned at once while
// the holder stays Hidden: "emerges from the shadows" told of a holder who
// never left them.
func TestStealthRecord_WinningEventNineIsPermanent(t *testing.T) {
	t.Run("over a weaker shroud", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(300)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"))
		require.NoError(t, c.AddCondition(9, false))
		require.False(t, c.HiddenByShroud(), "fixture: the stronger sneak base took the hide")

		pruned := c.Conditions.Prune()
		require.NoError(t, c.Validate())
		for _, p := range pruned {
			require.NotEqual(t, 9, p.ConditionId, "the winning 9 must not be pruned")
		}
		require.True(t, c.IsHidden())
		require.True(t, c.holdsLiveStealthRecord(), "a live record 9 carries the hide")
	})
	t.Run("on a sneak hide", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		sneakHide(t, c)
		require.NoError(t, c.AddCondition(9, false))

		pruned := c.Conditions.Prune()
		require.NoError(t, c.Validate())
		for _, p := range pruned {
			require.NotEqual(t, 9, p.ConditionId, "a re-added 9 must not downgrade the held one")
		}
		require.True(t, c.IsHidden())
		require.True(t, c.holdsLiveStealthRecord())
	})
	// The permanent rebuild (Validate(true): a CancelConditionsWithFlag of
	// any held flag, a spawn) removes a permanent record nothing sources. A
	// live 9 on a sneak-hidden holder is sourced by the hide itself.
	t.Run("survives the permanent rebuild", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(300)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 100, "spell"))
		require.NoError(t, c.AddCondition(9, false))
		require.NoError(t, c.Validate(true))
		require.True(t, c.holdsLiveStealthRecord(), "the event 9 that won the hide")

		sneaker := newHider(50)
		sneakHide(t, sneaker)
		require.NoError(t, sneaker.Validate(true))
		require.True(t, sneaker.IsHidden())
		require.True(t, sneaker.holdsLiveStealthRecord(), "the sneak's own 9")
	})
}

// Reload: a saved character comes back with its records and a fresh, Visible
// Awareness machine. A live 31 re-enters the shroud hide at the record's
// score; a cancelled one does not.
func TestShroudHide_SurvivesReload(t *testing.T) {
	t.Run("live record rehides at its score", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		require.True(t, c.Conditions.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180))
		c.Awareness = awareness.NewMachine() // what a load builds
		require.False(t, c.IsHidden(), "fixture: the fresh machine is Visible")

		require.NoError(t, c.Validate())
		require.True(t, c.IsHidden())
		require.True(t, c.HiddenByShroud())
		require.Equal(t, 180.0, c.HideBaseScore())
		require.NoError(t, c.Validate())
		require.True(t, c.HiddenByShroud(), "a second Validate changes nothing")
	})
	t.Run("cancelled record stays visible", func(t *testing.T) {
		seedStealthRecords(t)
		c := newHider(50)
		require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 180, "spell"))
		c.RevealForCombat()
		require.False(t, c.IsHidden())
		require.NoError(t, c.Validate())
		require.False(t, c.IsHidden(), "a hide combat revealed stays revealed")
	})
}

// A stronger shroud taking over a sneak hide clears the sneak's misc flag:
// left set, it would move the holder as a sneaker after the shroud ends.
func TestShroudOverSneak_ClearsTheSneakingFlag(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	sneakHide(t, c)
	c.SetMiscData(`sneaking`, true)
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdEmpathicShroud, 0, 200, "spell"))
	require.True(t, c.HiddenByShroud())
	require.Nil(t, c.GetMiscData(`sneaking`))
}

// #451: a player saved while hidden by a sneak (autosave, copyover, a
// graceful shutdown) came back Visible, and the load's permanent rebuild
// (Validate(true), users.loadUserFromPath) found no source for the saved
// permanent record 9, so it expired and the prune pass told the room
// "emerges from the shadows". A live 9 on a reloaded holder re-enters the
// sneak hide the way a live 31 re-enters the shroud hide.
func TestSneakHide_SurvivesReload(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	sneakHide(t, c)
	c.SetMiscData(`sneaking`, true)

	saved, err := yaml.Marshal(c)
	require.NoError(t, err)
	loaded := &Character{}
	require.NoError(t, yaml.Unmarshal(saved, loaded))
	require.Nil(t, loaded.Awareness, "fixture: Awareness is not saved")

	require.NoError(t, loaded.Validate(true)) // what users.loadUserFromPath runs
	require.True(t, loaded.IsHidden(), "the sneak hide must survive the reload")
	require.False(t, loaded.HiddenByShroud(), "it comes back as a sneak")
	require.True(t, loaded.holdsLiveStealthRecord(), "record 9 must stay live")
	for _, rec := range loaded.Conditions.List {
		if rec.ConditionId == conditionIdHidden {
			require.False(t, rec.Expired(), "an expired 9 is pruned with its end line")
		}
	}
	require.Equal(t, true, loaded.GetMiscData(`sneaking`), "the saved flag matches the hide")

	require.NoError(t, loaded.Validate(true))
	require.True(t, loaded.IsHidden(), "a second Validate changes nothing")
}

// The sibling: a 9 that a reveal already cancelled stays cancelled.
func TestSneakHide_CancelledRecordStaysVisibleOnReload(t *testing.T) {
	seedStealthRecords(t)
	c := newHider(50)
	sneakHide(t, c)
	c.Conditions.RemoveCondition(conditionIdHidden) // what the reveal cascade's cancel leaves
	c.Awareness = awareness.NewMachine()
	require.NoError(t, c.Validate(true))
	require.False(t, c.IsHidden())
}
