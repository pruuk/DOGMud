package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A spell's room line never names a hidden mob, even when the player caster
// sees the hidden (epic #382). The spell lines render mob names for the
// caster's viewer id, so a caster who Perceives a hidden mob read its name,
// and the room line reused that same string: every observer who sees faces
// read "Skeleton". A help spell never starts a fight, and a harmful one reads
// its names before commitHarmfulSpellAggro reveals the target, so the mob is
// still hidden when the line is built.

// seeHiddenCasterOnHiddenSkeleton hides the Skeleton (instance 100) in lit
// room 1, gives Aliceia (1) a see-hidden condition, and drains both players.
// Bobrick (2) is the observer: he reads faces and does not see the hidden.
func seeHiddenCasterOnHiddenSkeleton(t *testing.T) (*users.UserRecord, *mobs.Mob, *rooms.Room) {
	t.Helper()
	m := litRoomOneWithHiddenSkeleton(t)
	const seeHiddenId = 7102
	// SeedConditionsForTest replaces the registry, so the glow a condition
	// spell names is reseeded here, and the records a dot, heal and shield
	// apply (121 Poisoned, Regenerating, Conviction Ward) are added on top.
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		seeHiddenId: {ConditionId: seeHiddenId, Name: "Test True Sight",
			Flags: []conditions.Flag{conditions.SeeHidden}},
		glowConditionId: {ConditionId: glowConditionId, Name: "Test Glow", RoundInterval: 5, TriggerCount: 3},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	caster := users.GetByUserId(1)
	require.True(t, caster.Character.Conditions.AddCondition(seeHiddenId, true))
	require.True(t, caster.Character.Perceives(&m.Character))
	require.False(t, users.GetByUserId(2).Character.Perceives(&m.Character))
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return caster, m, rooms.LoadRoom(1)
}

func TestHiddenMob_SpellRoomLinesDoNotNameIt(t *testing.T) {
	cases := []struct {
		name  string
		spell *spells.SpellData
		// veto blocks the target's own engagement, as a grace-protected
		// caster does (SetAggro's untargetable guard), so a damage spell's
		// commit cannot reveal the mob before its line is built.
		veto bool
		// want empty: the line is the condition's own, which skips a reader
		// who does not perceive the holder (#458).
		want string
	}{
		{"damage", &spells.SpellData{SpellId: "sparks", Name: "Sparks", EffectType: "damage",
			DamageMultiplier: 0.8, BaseFolds: 4}, true, "Aliceia's Sparks strikes something!"},
		{"dot", &spells.SpellData{SpellId: "test-venom", Name: "Venom", EffectType: "dot", BaseFolds: 4},
			false, "Aliceia's Venom afflicts something!"},
		{"knockdown", &spells.SpellData{SpellId: "test-slam", Name: "Slam", EffectType: "knockdown"},
			false, "Aliceia's Slam knocks something to the ground!"},
		{"condition", &spells.SpellData{SpellId: "test-glow", Name: "Glow", EffectType: "condition",
			ConditionIds: []int{glowConditionId}}, false, "Aliceia's Glow settles over something."},
		{"heal", &spells.SpellData{SpellId: "heal", Name: "Heal", EffectType: "heal", EffectMagnitude: 3},
			false, "Aliceia's Heal envelops something in healing light."},
		{"shield", &spells.SpellData{SpellId: "test-shield", Name: "Ward", EffectType: "shield"},
			false, ""},
		{"purge", &spells.SpellData{SpellId: "test-purge", Name: "Cleanse", EffectType: "purge"},
			false, "Aliceia's Cleanse cleanses something of afflictions."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caster, m, room := seeHiddenCasterOnHiddenSkeleton(t)
			if tc.veto {
				characters.SetUserUntargetableCheck(func(userId int) bool { return userId == caster.UserId })
				t.Cleanup(func() { characters.SetUserUntargetableCheck(nil) })
			}

			applySpellEffect(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
				actions.NewMobActorInRoom(m, room), room, tc.spell, 10, spellContestAttackWin()))
			landQueuedConditions() // a ward or a heal lands through the condition queue

			assert.NotZero(t, countContaining(drainPlain(1), "Skeleton"),
				"the see-hidden caster still reads the mob's name in its own line")
			if tc.want == "" {
				assert.Empty(t, drainPlain(2), "a reader who does not perceive the holder reads no condition line")
				return
			}
			requireUnnamed(t, drainPlain(2), tc.want)
		})
	}
}

func TestHiddenMob_SpellInterruptRoomLineDoesNotNameIt(t *testing.T) {
	caster, m, room := seeHiddenCasterOnHiddenSkeleton(t)
	saved := m.Character.Activity
	t.Cleanup(func() { m.Character.Activity = saved })
	setMobCastingForSpellTest(m, "core-discharge")
	spell := &spells.SpellData{SpellId: "neural-stun", Name: "Neural Stun", EffectType: "damage"}

	interruptSpellTarget(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
		actions.NewMobActorInRoom(m, room), room, spell, 10, spellContestAttackWin()))

	assert.NotZero(t, countContaining(drainPlain(1), "Skeleton"))
	requireUnnamed(t, drainPlain(2), "Something's spell collapses!")
}

// A defied charm narrated the defence triad with the name the caster's own
// line printed, so the room line named a hidden mob the caster sees. The
// fixture mob's Character carries no instance id, so this also pins
// spellDefenceIdentity's fallback, which named a hidden mob by GetMobName.
func TestHiddenMob_DefiedCharmRoomLineDoesNotNameIt(t *testing.T) {
	caster, m, room := seeHiddenCasterOnHiddenSkeleton(t)
	require.Zero(t, m.Character.MobInstanceId, "the fixture reaches the identity fallback")
	out := combat.ChannelDefenceResult{Defended: true, Defence: combatvocab.DefenceDefy}

	applySpellEffect(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
		actions.NewMobActorInRoom(m, room), room, charmTestSpellData(), 10, out))

	requireUnnamed(t, drainPlain(2), "Something withstands Aliceia's Charm.")
}
