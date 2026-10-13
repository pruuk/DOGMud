package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 slice 4: every caster's own spell lines go through one body.

const (
	phaseCasterId    = 7351
	phaseTargetId    = 7352
	phaseBystanderId = 7353
	phaseMobId       = 98351
	phaseInfraredId  = 7361
	phaseSeeHiddenId = 7362
)

func phaseSpell(withActee bool) *spells.SpellData {
	s := &spells.SpellData{
		SpellId: "test-spike", Name: "Test Spike", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle,
		CastUserText: "You focus a spike.", CastRoomText: "{actor} stares intently.",
		WaitUserText: "You hold the spike.", WaitRoomText: "{actor} trembles with effort.",
	}
	if withActee {
		s.CastActeeText = "{actor} stares at you."
	}
	return s
}

// bareSpell authors no text at all, so every room line is the generic pool.
func bareSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "test-bare", Name: "Test Bare", PrimaryStat: "willpower",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle}
}

// phaseScene stands a caster (7351), a target (7352) and a bystander (7353)
// in one room. Infrared, when asked, goes to the target and the bystander,
// so a dark cave leaves them at shapes.
func phaseScene(t *testing.T, biome string, infrared bool) *rooms.Room {
	t.Helper()
	seedParityWorld(t)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		phaseInfraredId: {ConditionId: phaseInfraredId, Name: "Test Heat Eyes",
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		phaseSeeHiddenId: {ConditionId: phaseSeeHiddenId, Name: "Test Keen Eyes",
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
		"city": {BiomeId: "city", Lamp: rooms.LampPtr(90)},
	}))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		phaseCasterId:    users.NewTestUser(phaseCasterId, "kael", "Kael", 97351),
		phaseTargetId:    users.NewTestUser(phaseTargetId, "mira", "Mira", 97352),
		phaseBystanderId: users.NewTestUser(phaseBystanderId, "tob", "Tob", 97353),
	}))
	if infrared {
		for _, id := range []int{phaseTargetId, phaseBystanderId} {
			if !users.GetByUserId(id).Character.Conditions.AddCondition(phaseInfraredId, true) {
				t.Fatal("precondition: infrared")
			}
		}
	}
	room := &rooms.Room{RoomId: 7350, Biome: biome}
	for _, id := range []int{phaseCasterId, phaseTargetId, phaseBystanderId} {
		room.AddPlayer(id)
		users.GetByUserId(id).Character.RoomId = 7350
		events.DrainQueuedMessagesForTest(id)
	}
	return room
}

func phaseHide(t *testing.T, c *characters.Character) {
	t.Helper()
	c.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "spell_phase_lines_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	if !c.IsHidden() {
		t.Fatal("fixture is not hidden")
	}
}

func phaseMob(t *testing.T, room *rooms.Room, hidden bool) *mobs.Mob {
	t.Helper()
	m := &mobs.Mob{InstanceId: phaseMobId}
	m.Character.Name = "Shaman"
	m.Character.Conditions = conditions.New()
	m.Character.RoomId = room.RoomId
	if hidden {
		phaseHide(t, &m.Character)
	}
	mobs.SetInstanceForTest(phaseMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(phaseMobId, nil) })
	room.AddMob(phaseMobId)
	return m
}

func phaseLines(uid int) []string { return drinkSightPlain(events.DrainQueuedMessagesForTest(uid)) }

func castAt(caster Actor, s *spells.SpellData, phase spells.Phase, room *rooms.Room, targets ...int) {
	NarrateSpellPhase(caster, s, phase, targets, nil, room, messaging.CategorySpellFold)
}

func TestSpellPhase_CastTargetReadsActeeBystanderReadsObserver(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)

	requireOneLine(t, "caster", phaseLines(phaseCasterId), "You focus a spike.")
	requireOneLine(t, "target", phaseLines(phaseTargetId), "Kael stares at you.")
	requireOneLine(t, "bystander", phaseLines(phaseBystanderId), "Kael stares intently.")
}

func TestSpellPhase_UnauthoredActeeLeavesTheTargetTheRoomLine(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(false), spells.PhaseCast, room, phaseTargetId)

	requireOneLine(t, "target", phaseLines(phaseTargetId), "Kael stares intently.")
}

// R2: cast_actee is a single-target line; an area spell that authors one
// still leaves the target reading the room line.
func TestSpellPhase_AreaSpellSendsNoActee(t *testing.T) {
	room := phaseScene(t, "city", false)
	s := phaseSpell(true)
	s.Targeting = combatvocab.TargetArea
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), s, spells.PhaseCast, room, phaseTargetId)

	requireOneLine(t, "target", phaseLines(phaseTargetId), "Kael stares intently.")
}

func TestSpellPhase_SelfCastSendsNoActee(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast, room, phaseCasterId)

	requireOneLine(t, "caster", phaseLines(phaseCasterId), "You focus a spike.")
	requireOneLine(t, "other player", phaseLines(phaseTargetId), "Kael stares intently.")
}

// R1: darkness hides the caster's name in the target's line; it does not
// silence the line.
func TestSpellPhase_TargetLineHidesTheCasterBySight(t *testing.T) {
	for _, tc := range []struct {
		name     string
		infrared bool
		want     string
	}{{"shapes", true, "A figure stares at you."}, {"none", false, "Something stares at you."}} {
		t.Run(tc.name, func(t *testing.T) {
			room := phaseScene(t, "cave", tc.infrared)
			castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)
			requireOneLine(t, "target", phaseLines(phaseTargetId), tc.want)
		})
	}
}

// R7: a phase with no authored room line reads the generic pool line, for a
// player caster as for a mob.
func TestSpellPhase_GenericRoomLineForAnyCaster(t *testing.T) {
	room := phaseScene(t, "city", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), bareSpell(), spells.PhaseCast, room, phaseTargetId)
	requireOneLine(t, "bystander, player cast", phaseLines(phaseBystanderId), "Kael begins weaving a spell.")

	m := phaseMob(t, room, false)
	castAt(NewMobActorInRoom(m, room), bareSpell(), spells.PhaseWait, room)
	got := phaseLines(phaseBystanderId)
	if len(got) != 1 || !strings.Contains(got[0], "Shaman") || !strings.HasSuffix(got[0], "weaves magic with focused intent.") {
		t.Fatalf("bystander, mob fold round: %q", got)
	}
}

// A fold round's room line is sight-only: the audio leak at the old player
// wait sites is gone.
func TestSpellPhase_WaitLineIsSightOnly(t *testing.T) {
	room := phaseScene(t, "cave", false)
	castAt(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(false), spells.PhaseWait, room)
	if got := phaseLines(phaseBystanderId); len(got) != 0 {
		t.Fatalf("a bystander who sees nothing read %q", got)
	}
	requireOneLine(t, "caster", phaseLines(phaseCasterId), "You hold the spike.")
}

// The magic phase has no generic room line: a spell with no magic_* text
// sends nothing, and an authored magic_observer reaches the room by sight.
func TestSpellPhase_MagicLineOnlyWhenAuthored(t *testing.T) {
	room := phaseScene(t, "city", false)
	caster := NewUserActorInRoom(users.GetByUserId(phaseCasterId), room)

	castAt(caster, phaseSpell(false), spells.PhaseMagic, room, phaseTargetId)
	if got := phaseLines(phaseBystanderId); len(got) != 0 {
		t.Fatalf("no magic text authored, yet the room read %q", got)
	}

	s := phaseSpell(false)
	s.MagicUserText, s.MagicRoomText = "The spike flies.", "{actor}'s spike flies."
	castAt(caster, s, spells.PhaseMagic, room, phaseTargetId)
	requireOneLine(t, "caster", phaseLines(phaseCasterId), "The spike flies.")
	requireOneLine(t, "bystander", phaseLines(phaseBystanderId), "Kael's spike flies.")
}

// R3 + R5: a hidden caster's lines are silent to a reader who does not
// perceive them, who instead hears one unnamed line at cast start only.
func TestSpellPhase_HiddenCasterIsHeardNotSeenAtCastStart(t *testing.T) {
	room := phaseScene(t, "city", false)
	caster := users.GetByUserId(phaseCasterId)
	phaseHide(t, caster.Character)

	castAt(NewUserActorInRoom(caster, room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)

	unseen := spells.CastStartedUnseenLine()
	for _, id := range []int{phaseTargetId, phaseBystanderId} {
		got := phaseLines(id)
		if len(got) != 1 || got[0] != unseen {
			t.Fatalf("player %d read %q, want only the unseen line", id, got)
		}
	}

	castAt(NewUserActorInRoom(caster, room), phaseSpell(true), spells.PhaseWait, room, phaseTargetId)
	if got := phaseLines(phaseBystanderId); len(got) != 0 {
		t.Fatalf("a fold round of a hidden caster must be silent, got %q", got)
	}
}

func TestSpellPhase_SeeHiddenReaderReadsTheNamedLines(t *testing.T) {
	room := phaseScene(t, "city", false)
	m := phaseMob(t, room, true)
	if !users.GetByUserId(phaseTargetId).Character.Conditions.AddCondition(phaseSeeHiddenId, true) {
		t.Fatal("precondition: see hidden")
	}
	if !users.GetByUserId(phaseTargetId).Character.Perceives(&m.Character) {
		t.Fatal("precondition: the target should perceive the hidden mob")
	}

	castAt(NewMobActorInRoom(m, room), phaseSpell(true), spells.PhaseCast, room, phaseTargetId)

	got := phaseLines(phaseTargetId)
	if len(got) != 1 || !strings.Contains(got[0], "Shaman") || !strings.HasSuffix(got[0], "stares at you.") {
		t.Fatalf("a perceiving target reads the named actee line, got %q", got)
	}
	if got := phaseLines(phaseBystanderId); len(got) != 1 || got[0] != spells.CastStartedUnseenLine() {
		t.Fatalf("a non-perceiving bystander hears the unseen line, got %q", got)
	}
}

func TestSpellPhase_MobTargetReadsNothingAndTheRoomReadsTheRoomLine(t *testing.T) {
	room := phaseScene(t, "city", false)
	m := phaseMob(t, room, false)
	NarrateSpellPhase(NewUserActorInRoom(users.GetByUserId(phaseCasterId), room), phaseSpell(true), spells.PhaseCast,
		nil, []int{m.InstanceId}, room, messaging.CategorySpellFold)
	requireOneLine(t, "bystander", phaseLines(phaseTargetId), "Kael stares intently.")
}
