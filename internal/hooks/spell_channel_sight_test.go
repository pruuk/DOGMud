package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242: a mob's spell-channel lines went out on plain Room.SendText, the
// unfiltered audio channel, so a shapes-only or blind observer read the
// caster's name. Owner ruling R4 (2026-10-08): the disruptions (concentration
// breaks, fizzles, falters) have a sound line for a reader who sees nothing;
// the quiet weave and focus-shift lines are sight-only.
//
// seedFallbackRoom (dark_room_fallback_sight_test.go) puts users 1 and 2 in
// cave room 2 at a pinned lamp and gives user 1 the named eyes. At lamp 10
// user 1 with heat eyes reads shapes and user 2 reads nothing; at lamp 60
// both read faces.

func spellChannelMob(t *testing.T) *mobs.Mob {
	t.Helper()
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	return m
}

func TestMobSpellDisruption_ShapesReadAFigure_BlindHearTheSound(t *testing.T) {
	cases := []struct {
		name  string
		send  func(*mobs.Mob, *rooms.Room)
		seen  string
		sound string
	}{
		{"concentration breaks", sendMobConcentrationBroke, "concentration breaks", messaging.SoundChantBreaksOff},
		{"spell fizzles", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "fizzles") }, "spell fizzles", messaging.SoundSpellSputtersOut},
		{"spell falters", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "falters") }, "spell falters", messaging.SoundSpellSputtersOut},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, 10, heatEyesConditionId)
			require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
			require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(2).Character, room))

			tc.send(spellChannelMob(t), room)

			shapes, blind := drainPlain(1), drainPlain(2)
			require.Equal(t, 1, countContaining(shapes, tc.seen), "shapes reader sees it: %v", shapes)
			require.Zero(t, countContaining(shapes, "Skeleton"), "but not whose: %v", shapes)
			require.Zero(t, countContaining(shapes, tc.sound))
			require.Equal(t, 1, countContaining(blind, tc.sound), "blind reader hears it: %v", blind)
			require.Zero(t, countContaining(blind, "Skeleton"), "%v", blind)
		})
	}
}

func TestMobSpellChannel_WeaveIsSightOnly(t *testing.T) {
	room := seedFallbackRoom(t, 10, heatEyesConditionId)
	sendMobWeaving(spellChannelMob(t), room)

	shapes, blind := drainPlain(1), drainPlain(2)
	require.Equal(t, 1, countContaining(shapes, "weaves magic"), "%v", shapes)
	require.Zero(t, countContaining(shapes, "Skeleton"), "%v", shapes)
	require.Empty(t, blind, "a reader who sees nothing gets nothing for a quiet weave")
}

func TestMobSpellChannel_LitRoomNamesTheCaster(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	m := spellChannelMob(t)
	sendMobConcentrationBroke(m, room)
	sendMobWeaving(m, room)
	got := drainPlain(2)
	require.Equal(t, 1, countContaining(got, "Skeleton's concentration breaks."), "%v", got)
	require.Equal(t, 1, countContaining(got, "Skeleton weaves magic"), "%v", got)
}

// The player caster's break (prone, grapple, and the pain of a hit) gets the
// same treatment as the mob's, so mob and player casters read alike. The
// hit path used plain Room.SendText and named the caster to everyone.
func TestPlayerConcentrationBroke_FollowsTheObserversSight(t *testing.T) {
	t.Run("shapes reads a figure", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerConcentrationBroke(caster, room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "concentration breaks"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
		require.Empty(t, drainPlain(2), "the caster reads its own line, not the room's")
	})

	t.Run("sees nothing, hears the chant break off", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerConcentrationBroke(caster, room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundChantBreaksOff), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})
}

func TestMobShiftsFocus_IsSightOnlyAndHidesBothNames(t *testing.T) {
	room := seedFallbackRoom(t, 10, heatEyesConditionId)
	target := users.GetByUserId(2)
	sendMobShiftsFocus(spellChannelMob(t), room, target)

	shapes, blind := drainPlain(1), drainPlain(2)
	require.Equal(t, 1, countContaining(shapes, "shifts focus"), "%v", shapes)
	require.Zero(t, countContaining(shapes, "Skeleton"), "%v", shapes)
	require.Zero(t, countContaining(shapes, target.Character.Name), "%v", shapes)
	require.Empty(t, blind, "a reader who sees nothing gets nothing for a focus shift")
}

// A hidden mob caster is unseen in its own spell-channel lines by every
// reader, even one who reads faces in a lit room: the line names
// "Something", never the mob, as sendSpoken does for a speaker still hidden
// and as SendSeen's silence does for a hidden emote (#274, owner R3).
func TestMobSpellChannel_HiddenCasterIsNeverNamed(t *testing.T) {
	cases := []struct {
		name string
		send func(*mobs.Mob, *rooms.Room)
		want string
	}{
		{"concentration breaks", sendMobConcentrationBroke, "Something's concentration breaks."},
		{"spell fizzles", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "fizzles") }, "Something's spell fizzles."},
		{"spell falters", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "falters") }, "Something's spell falters."},
		{"weave", sendMobWeaving, "Something weaves magic with focused intent."},
		{"focus shift", func(m *mobs.Mob, r *rooms.Room) { sendMobShiftsFocus(m, r, users.GetByUserId(1)) }, "Something shifts focus to"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, 60, heatEyesConditionId)
			require.Equal(t, messaging.SightFull, messaging.ParticipantSight(users.GetByUserId(2).Character, room))
			m := spellChannelMob(t)
			m.Character.Validate()
			reason := state.TransitionReason{Trigger: "spell_channel_hidden_test"}
			require.NoError(t, m.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
			m.Character.Awareness.ResolveConcealment(true, reason)
			require.True(t, m.Character.IsHidden())
			events.DrainQueuedMessagesForTest(1)
			events.DrainQueuedMessagesForTest(2)

			tc.send(m, room)

			got := drainPlain(2)
			require.Zero(t, countContaining(got, "Skeleton"), "a faces reader must not see a hidden caster's name: %v", got)
			require.Equal(t, 1, countContaining(got, tc.want), "%v", got)
		})
	}
}

// testHarmSpell is a single-target harm spell with nothing authored beyond
// its axes, enough for resolveSpell to reach its no-target lines.
func testHarmSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "test-bolt", Name: "Test Bolt", AttackType: combatvocab.AttackSpell,
		DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle, EffectType: "damage"}
}

// #242 residue: the target-gone line named the target raw, to a caster who
// could not make out their own room.
func TestPlayerSpellTargetGone_NamedOnlyAsFarAsTheCasterSees(t *testing.T) {
	room := seedFallbackRoom(t, 0, nightEyesConditionId)
	caster, target := users.GetByUserId(2), users.GetByUserId(1)
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(caster.Character, room))
	room.RemovePlayer(target.UserId)
	target.Character.RoomId = 1
	rooms.LoadRoom(1).AddPlayer(target.UserId)
	drainPlain(caster.UserId)

	resolveSpell(caster, activity.CastingData{SpellId: "test-bolt", TargetUserIds: []int{target.UserId}}, testHarmSpell(), room)

	got := drainPlain(caster.UserId)
	require.Equal(t, 1, countContaining(got, "Your spell dissipates, unspent. Something is no longer here."), "%v", got)
	require.Zero(t, countContaining(got, target.Character.Name), "%v", got)
}

// #242 residue: "crackles through the air harmlessly" went out visual only,
// so a reader who saw nothing heard nothing of a spell spent in the room.
func TestPlayerSpellFindsNothing_HeardByAReaderWhoSeesNothing(t *testing.T) {
	t.Run("sees nothing, hears it sputter out", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		resolveSpell(caster, activity.CastingData{SpellId: "test-bolt"}, testHarmSpell(), room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
		require.Zero(t, countContaining(got, "crackles"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})
	t.Run("shapes reader sees a figure, hears no sound line", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		resolveSpell(caster, activity.CastingData{SpellId: "test-bolt"}, testHarmSpell(), room)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "crackles through the air harmlessly"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
		require.Zero(t, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
	})
}

// #242: the drain-area "finding no one to drain" line went out visual only, so
// a reader who saw nothing heard nothing of the spell spent in the room.
func TestMobDrainAreaFindsNoOne_HeardByAReaderWhoSeesNothing(t *testing.T) {
	// Downed players are skipped by the drain, so with everyone here at 0
	// health it finds no one.
	downAll := func(t *testing.T) {
		t.Helper()
		for _, uid := range []int{1, 2} {
			c := users.GetByUserId(uid).Character
			saved := c.Health
			c.Health = 0
			t.Cleanup(func() { c.Health = saved })
		}
		events.DrainQueuedMessagesForTest(1)
		events.DrainQueuedMessagesForTest(2)
	}
	spell := &spells.SpellData{SpellId: "test-drain", Name: "Core Recharge", EffectType: "drain_area"}

	t.Run("sees nothing, hears it sputter out", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		downAll(t)
		resolveMobDrainArea(spellChannelMob(t), room, spell)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
		require.Zero(t, countContaining(got, "finding no one"), "%v", got)
		require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
	})
	t.Run("shapes reader sees a figure, hears no sound line", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		downAll(t)
		resolveMobDrainArea(spellChannelMob(t), room, spell)
		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "finding no one to drain"), "%v", got)
		require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
		require.Zero(t, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
	})
}

// A hidden player caster is unseen by every reader, even one who reads faces
// in a lit room (#274, owner R3): the harmless line says "Something's".
func TestPlayerSpellFindsNothing_HiddenCasterIsNeverNamed(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	reader, caster := users.GetByUserId(1), users.GetByUserId(2)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(reader.Character, room))
	caster.Character.Validate()
	reason := state.TransitionReason{Trigger: "spell_channel_hidden_test"}
	require.NoError(t, caster.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	caster.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, caster.Character.IsHidden())
	events.DrainQueuedMessagesForTest(1)

	resolveSpell(caster, activity.CastingData{SpellId: "test-bolt"}, testHarmSpell(), room)

	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "Something's spell crackles through the air harmlessly."), "%v", got)
	require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
}

// foldingMobInRoom puts the test mob in room, mid-fold on a harm spell aimed
// at targetUserId, and in combat with that player, as a mob caster stands
// when its target walks out.
func foldingMobInRoom(t *testing.T, room *rooms.Room, targetUserId int) *mobs.Mob {
	t.Helper()
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-bolt": testHarmSpell()}))
	m := spellChannelMob(t)
	rooms.LoadRoom(m.Character.RoomId).RemoveMob(m.InstanceId)
	room.AddMob(m.InstanceId)
	m.Character.Validate()
	m.Character.SetAggro(targetUserId, 0, characters.DefaultAttack)
	require.True(t, m.Character.IsInCombat(), "fixture: the mob fights its target")
	require.NoError(t, m.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-bolt", FoldsNeeded: 3, TargetUserIds: []int{targetUserId}},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	return m
}

// walkOut moves a player from room to room 1, as a target leaving mid-fold.
func walkOut(t *testing.T, room *rooms.Room, userId int) {
	t.Helper()
	room.RemovePlayer(userId)
	users.GetByUserId(userId).Character.RoomId = 1
	rooms.LoadRoom(1).AddPlayer(userId)
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
}

// #242: a mob's target who walked out is not "gone" to the fold step (only a
// dead or logged-out one is), so IdleMobs released the mob mid-fold: it left
// combat, the fold step never ran again, and the spell ended with no line at
// all. The fold now ends first, told as the TargetGone fizzle is: by sight to
// a reader who sees (a figure at shapes), by sound to one who sees nothing.
func TestMobFold_TargetWalksOut_FizzlesAtEachReadersSight(t *testing.T) {
	cases := []struct {
		name  string
		lamp  int
		eyes  int
		sight messaging.SightDecision
		want  string
	}{
		{"faces read the caster", 60, heatEyesConditionId, messaging.SightFull, "Skeleton's spell fizzles."},
		{"shapes read a figure", 10, heatEyesConditionId, messaging.SightShapes, "spell fizzles."},
		{"sees nothing, hears it", 0, nightEyesConditionId, messaging.SightNone, messaging.SoundSpellSputtersOut},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, tc.lamp, tc.eyes)
			m := foldingMobInRoom(t, room, 2)
			walkOut(t, room, 2)
			require.Equal(t, tc.sight, messaging.ParticipantSight(users.GetByUserId(1).Character, room))

			IdleMobs(events.NewRound{RoundNumber: 1})

			require.False(t, m.Character.IsCasting(), "the fold ends with the release")
			require.False(t, m.Character.IsInCombat(), "and the mob is released")
			got := drainPlain(1)
			require.Equal(t, 1, countContaining(got, tc.want), "%v", got)
			if tc.sight != messaging.SightFull {
				require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
			}
			require.Empty(t, drainPlain(2), "the player who left reads nothing of the room")
		})
	}
}

// #242 review G2: a mob folding at a MOB that walks or flees out is released
// by actions.ClearRoomAggroOnDeparture, not by IdleMobs' player-target check,
// so it left combat still casting, the fold step never ran for it again, and
// the spell hung unspoken. IdleMobs now ends every fold a mob out of combat
// still holds.
func TestMobFold_MobTargetRelocatesOut_Fizzles(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-bolt": testHarmSpell()}))
	caster := spellChannelMob(t)
	rooms.LoadRoom(caster.Character.RoomId).RemoveMob(caster.InstanceId)
	room.AddMob(caster.InstanceId)
	caster.Character.RoomId = room.RoomId

	target := &mobs.Mob{MobId: 2, InstanceId: 101, Character: characters.Character{
		Name: "Rat", RoomId: room.RoomId, Health: 10, Conditions: conditions.New()}}
	target.Character.HealthMax.Value = 10
	mobs.SetInstanceForTest(101, target)
	t.Cleanup(func() { mobs.SetInstanceForTest(101, nil) })
	room.AddMob(101)

	caster.Character.Validate()
	caster.Character.SetAggro(0, 101, characters.DefaultAttack)
	require.True(t, caster.Character.IsInCombat(), "fixture: the caster fights the rat")
	require.NoError(t, caster.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-bolt", FoldsNeeded: 3, TargetMobInstanceIds: []int{101}},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))

	// The rat walks out: the departure releases whoever was fighting it.
	room.RemoveMob(101)
	target.Character.RoomId = 1
	rooms.LoadRoom(1).AddMob(101)
	actions.ClearRoomAggroOnDeparture(room, 101)
	require.False(t, caster.Character.IsInCombat(), "fixture: the departure released the caster")
	require.True(t, caster.Character.IsCasting(), "fixture: still folding after the release")
	events.DrainQueuedMessagesForTest(1)

	IdleMobs(events.NewRound{RoundNumber: 1})

	require.False(t, caster.Character.IsCasting(), "the stranded fold ends")
	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "Skeleton's spell fizzles."), "%v", got)
}

// #242 review G5: a fold that completes on a room its target left fizzles in
// resolveMobSpell, but handleMobFoldCasting still paid the cast's
// progression award (and rolled spell discovery) as for a resolved cast. A
// fizzle resolves nothing, as the fold step's TargetGone fizzle does, and
// awards neither.
func TestMobFold_CompletingOnAnEmptyRoom_AwardsNothing(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-bolt": testHarmSpell()}))
	m := spellChannelMob(t)
	rooms.LoadRoom(m.Character.RoomId).RemoveMob(m.InstanceId)
	room.AddMob(m.InstanceId)
	m.Character.RoomId = room.RoomId
	m.Character.Validate()
	m.Character.ConvictionMax.Value, m.Character.Conviction = 1000, 1000
	m.Character.SetAggro(2, 0, characters.DefaultAttack)
	require.NoError(t, m.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-bolt", FoldsNeeded: 1, FoldsPerRound: 1, TargetUserIds: []int{2}},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	walkOut(t, room, 2)
	before := m.Character.SkillUseCount[string(skills.Spellcasting)]

	handleMobFoldCasting(m, room)

	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "Skeleton's spell fizzles."), "fixture: the fold completed and fizzled: %v", got)
	require.Equal(t, before, m.Character.SkillUseCount[string(skills.Spellcasting)], "a fizzled cast paid a progression award")
}

// The sweep ends harmful folds only. An idle self-buff (shipped idle
// commands such as `cast conviction-ward`) is not fizzled every time it is
// begun; that it never resolves out of combat is a separate problem.
func TestMobFold_IdleHelpFoldIsNotSwept(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-ward": {SpellId: "test-ward", Name: "Test Ward",
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSelf}}))
	caster := spellChannelMob(t)
	rooms.LoadRoom(caster.Character.RoomId).RemoveMob(caster.InstanceId)
	room.AddMob(caster.InstanceId)
	caster.Character.RoomId = room.RoomId
	caster.Character.Validate()
	require.NoError(t, caster.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-ward", FoldsNeeded: 3, TargetMobInstanceIds: []int{caster.InstanceId}},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	require.False(t, caster.Character.IsInCombat())
	events.DrainQueuedMessagesForTest(1)

	IdleMobs(events.NewRound{RoundNumber: 1})

	require.True(t, caster.Character.IsCasting(), "an idle help fold was swept")
	require.Zero(t, countContaining(drainPlain(1), "fizzles"))
}

// idleFoldCaster puts mob 100 in the fallback room, out of combat, folding
// spellId on itself.
func idleFoldCaster(t *testing.T, room *rooms.Room, spellId string, targetMobs []int) *mobs.Mob {
	t.Helper()
	caster := spellChannelMob(t)
	rooms.LoadRoom(caster.Character.RoomId).RemoveMob(caster.InstanceId)
	room.AddMob(caster.InstanceId)
	caster.Character.RoomId = room.RoomId
	caster.Character.Validate()
	require.NoError(t, caster.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: spellId, FoldsNeeded: 2, FoldsPerRound: 1, TargetMobInstanceIds: targetMobs},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	require.False(t, caster.Character.IsInCombat(), "fixture: the caster is not fighting")
	events.DrainQueuedMessagesForTest(1)
	return caster
}

// #242 review K9: a mob's fold step ran only in the combat round, so a
// shipped idle buff (Seren's `cast conviction-ward`, Rhett's
// `conviction-armor`) never resolved and left the mob casting for good. The
// combat round now steps a help fold out of combat too, as a player's fold
// steps whether or not they fight: it weaves, then resolves, its effect
// lands and the cast clears.
func TestMobFold_IdleHelpFoldResolvesOutOfCombat(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-mend": {SpellId: "test-mend", Name: "Test Mend",
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSelf,
		EffectType: "heal", EffectMagnitude: 5}}))
	t.Cleanup(conditions.SeedConditionRecordsForTest()) // the heal's regenerating condition
	caster := idleFoldCaster(t, room, "test-mend", []int{100})

	handleMobCombat(events.NewRound{RoundNumber: 1})
	require.True(t, caster.Character.IsCasting(), "one fold of two: still weaving")
	require.False(t, caster.Character.HasCondition(conditions.ConditionIdRegenerating), "the heal landed before the fold completed")

	handleMobCombat(events.NewRound{RoundNumber: 2})
	require.False(t, caster.Character.IsCasting(), "the completed help fold clears the cast")
	require.True(t, caster.Character.HasCondition(conditions.ConditionIdRegenerating), "the heal landed")
	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "Skeleton channels restorative magic."), "%v", got)
	require.Zero(t, countContaining(got, "fizzles"), "%v", got)
}

// #242 final review: IdleMobs' quarry release (a player foe who left the
// room) ended any fold the mob held, so a mob mid-fold on a help spell on
// itself (Seren or Olen's `cast heal`) read "spell fizzles" when its foe
// walked out. A help fold is left to its fold step, which now runs out of
// combat: the release keeps it, and it resolves.
func TestMobFold_QuarryLeaves_HelpFoldResolvesWithoutFizzle(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-mend": {SpellId: "test-mend", Name: "Test Mend",
		AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSelf,
		EffectType: "heal", EffectMagnitude: 5}}))
	t.Cleanup(conditions.SeedConditionRecordsForTest()) // the heal's regenerating condition
	caster := idleFoldCaster(t, room, "test-mend", []int{100})
	caster.Character.SetAggro(2, 0, characters.DefaultAttack)
	require.True(t, caster.Character.IsInCombat(), "fixture: the caster fights player 2")
	walkOut(t, room, 2)

	IdleMobs(events.NewRound{RoundNumber: 1})
	require.False(t, caster.Character.IsInCombat(), "the quarry release still happens")
	require.True(t, caster.Character.IsCasting(), "the release ended a help fold")

	handleMobCombat(events.NewRound{RoundNumber: 1})
	handleMobCombat(events.NewRound{RoundNumber: 2})
	require.False(t, caster.Character.IsCasting(), "the help fold resolves and clears")
	require.True(t, caster.Character.HasCondition(conditions.ConditionIdRegenerating), "the heal landed")
	got := drainPlain(1)
	require.Zero(t, countContaining(got, "fizzles"), "%v", got)
	require.Zero(t, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
	require.Equal(t, 1, countContaining(got, "Skeleton channels restorative magic."), "%v", got)
}

// The combat round leaves a harmful fold held out of combat to IdleMobs'
// sweep, which fizzles it at once (#242 G2): its target is gone, and a
// harmful spell never resolves out of combat.
func TestMobFold_StaleHarmFoldStillFizzlesAfterTheCombatRound(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-bolt": testHarmSpell()}))
	caster := idleFoldCaster(t, room, "test-bolt", []int{101})
	target := &mobs.Mob{MobId: 2, InstanceId: 101, Character: characters.Character{
		Name: "Rat", RoomId: 1, Health: 10, Conditions: conditions.New()}}
	target.Character.HealthMax.Value = 10
	mobs.SetInstanceForTest(101, target)
	t.Cleanup(func() { mobs.SetInstanceForTest(101, nil) })
	rooms.LoadRoom(1).AddMob(101)

	handleMobCombat(events.NewRound{RoundNumber: 1})
	require.True(t, caster.Character.IsCasting(), "the combat round must not step a stale harmful fold")
	IdleMobs(events.NewRound{RoundNumber: 1})

	require.False(t, caster.Character.IsCasting(), "the stranded fold ends")
	require.Equal(t, 1, countContaining(drainPlain(1), "Skeleton's spell fizzles."))
}

// #242: the fold step can also complete the cast in the round the target
// walked out, before IdleMobs releases the mob. resolveMobSpell then found
// no target in the room, resolved nothing and said nothing.
func TestMobFold_CompletesWithNoTargetLeft_Fizzles(t *testing.T) {
	cases := []struct {
		name  string
		lamp  int
		eyes  int
		sight messaging.SightDecision
		want  string
	}{
		{"faces read the caster", 60, heatEyesConditionId, messaging.SightFull, "Skeleton's spell fizzles."},
		{"shapes read a figure", 10, heatEyesConditionId, messaging.SightShapes, "spell fizzles."},
		{"sees nothing, hears it", 0, nightEyesConditionId, messaging.SightNone, messaging.SoundSpellSputtersOut},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, tc.lamp, tc.eyes)
			m := spellChannelMob(t)
			walkOut(t, room, 2)
			require.Equal(t, tc.sight, messaging.ParticipantSight(users.GetByUserId(1).Character, room))

			landed, fizzled := resolveMobSpell(m, activity.CastingData{SpellId: "test-bolt", TargetUserIds: []int{2}}, testHarmSpell(), room)

			require.False(t, landed)
			require.True(t, fizzled, "a fold with no target left reports the fizzle")
			got := drainPlain(1)
			require.Equal(t, 1, countContaining(got, tc.want), "%v", got)
			if tc.sight != messaging.SightFull {
				require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
			}
		})
	}
}
