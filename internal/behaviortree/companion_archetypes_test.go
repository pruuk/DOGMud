package behaviortree

import (
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// The six AI companions (modules/aicompanion) each fight with an archetype
// of their own. These tests drive each tree through mob_combat_round with
// the real archetype file and check the command it reaches for.

const companionArchetypeDir = "../../_datafiles/world/dogmud/behaviors/archetypes/"

var companionArchetypes = []string{
	"companion_archer", "companion_guardian", "companion_healer",
	"companion_skirmisher", "companion_battlemage", "companion_brawler",
}

// companionOwnerId is the player the companion travels with.
const companionOwnerId = 42

// companionFight seeds a humanoid companion with the named archetype,
// fighting a standing foe. foeOnOwner puts the foe on the companion's
// owner (a player); otherwise the foe is fighting the companion.
func companionFight(t *testing.T, archetype string, instanceId int, foeOnOwner bool) (*mobs.Mob, *mobs.Mob) {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", BodyParts: []string{"arms", "hands", "legs"}},
	}))
	LoadArchetypeForTest(t, archetype, companionArchetypeDir+archetype+".yaml")

	me := &mobs.Mob{MobId: mobs.MobId(500 + instanceId), InstanceId: instanceId, BehaviorArchetype: archetype}
	me.Character.Name = "companion"
	me.Character.Health = 100
	me.Character.HealthMax.Base = 100
	me.Character.HealthMax.Value = 100
	me.Character.Conviction = 500
	me.Character.ConvictionMax.Base = 500
	me.Character.ConvictionMax.Value = 500
	me.Character.Conditions = conditions.New()
	me.Character.Cooldowns = characters.Cooldowns{}

	foe := &mobs.Mob{MobId: mobs.MobId(600 + instanceId), InstanceId: instanceId + 1}
	foe.Character.Name = "a grey wolf"
	foe.Character.Health = 100
	foe.Character.HealthMax.Value = 100
	setCombatPositionParallel(&foe.Character, position.Standing)
	if foeOnOwner {
		foe.Character.SetAggro(companionOwnerId, 0, characters.DefaultAttack)
	} else {
		foe.Character.SetAggro(0, me.InstanceId, characters.DefaultAttack)
	}
	me.Character.SetAggro(0, foe.InstanceId, characters.DefaultAttack)

	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{int(me.MobId): me, int(foe.MobId): foe},
		map[int]*mobs.Mob{me.InstanceId: me, foe.InstanceId: foe},
	))
	events.DrainQueuedInputsForTest(me.InstanceId)
	t.Cleanup(func() { events.DrainQueuedInputsForTest(me.InstanceId) })
	return me, foe
}

// companionRound runs one combat round and returns the command it queued,
// or "" when the tree left the round to the ordinary attack.
func companionRound(t *testing.T, me *mobs.Mob) string {
	t.Helper()
	if !TryMobBehavior(me.InstanceId, EventContext{EventType: "mob_combat_round"}) {
		return ""
	}
	DrainAllDelayedActionsForTest(t)
	q := events.DrainQueuedInputsForTest(me.InstanceId)
	if len(q) == 0 {
		t.Fatal("the tree claimed the round but queued nothing")
	}
	return q[0]
}

// Running, protecting the owner, calling for help and idle time all belong
// to the aicompanion module. A companion tree that fled on its own (by a
// fixed health line rather than the companion's nerve), backed out of the
// room (keep_distance), shouted for a pack it does not have, or stole from
// passers-by would fight the module rather than serve it.
func TestCompanionArchetypes_LoadAndLeaveTheRestToTheModule(t *testing.T) {
	for _, name := range companionArchetypes {
		LoadArchetypeForTest(t, name, companionArchetypeDir+name+".yaml")
		raw, err := os.ReadFile(companionArchetypeDir + name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		var body []string
		for _, l := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(l), "#") {
				body = append(body, l)
			}
		}
		text := strings.Join(body, "\n")
		for _, banned := range []string{"do: flee", "keep_distance", "callforhelp", "go_to_caller_room",
			"try_steal", "try_plant", "event: mob_idle", "event: packmate_hurt", "event: mob_hurt"} {
			if strings.Contains(text, banned) {
				t.Errorf("%s must not use %q", name, banned)
			}
		}
		if !strings.Contains(text, "event: mob_combat_round") {
			t.Errorf("%s answers no combat round", name)
		}
	}
}

func TestCompanionGuardian_DrawsAFoeOffTheOwner(t *testing.T) {
	me, _ := companionFight(t, "companion_guardian", 93001, true)
	if cmd := companionRound(t, me); cmd != "taunt" {
		t.Fatalf("a foe on his companion should be taunted onto him, got %q", cmd)
	}
}

func TestCompanionGuardian_RalliesWhenTheFoeIsOnHim(t *testing.T) {
	me, _ := companionFight(t, "companion_guardian", 93003, false)
	if cmd := companionRound(t, me); cmd != "rally" {
		t.Fatalf("with the foe already on him (and no shield to bash with) he rallies, got %q", cmd)
	}
}

func TestCompanionSkirmisher_TripsThenPutsTheBootIn(t *testing.T) {
	me, foe := companionFight(t, "companion_skirmisher", 93005, true)
	if cmd := companionRound(t, me); cmd != "trip" {
		t.Fatalf("a standing foe gets its legs taken, got %q", cmd)
	}
	setCombatPositionParallel(&foe.Character, position.Prone)
	if cmd := companionRound(t, me); cmd != "kick" {
		t.Fatalf("a downed foe gets the boot, got %q", cmd)
	}
}

func TestCompanionBrawler_TakesHoldOfALoneFoe(t *testing.T) {
	me, foe := companionFight(t, "companion_brawler", 93007, false)
	if cmd := companionRound(t, me); cmd != "grapple" {
		t.Fatalf("a lone standing foe is grappled, got %q", cmd)
	}
	setCombatPositionParallel(&foe.Character, position.Prone)
	if cmd := companionRound(t, me); cmd != "kick" {
		t.Fatalf("one already down is kicked, got %q", cmd)
	}
}

func TestCompanionArcher_NoGrapplingAndAKickForTheFallen(t *testing.T) {
	me, foe := companionFight(t, "companion_archer", 93009, true)
	if cmd := companionRound(t, me); cmd != "" {
		t.Fatalf("with no shot chambered and the foe on its feet she leaves it to the ordinary attack, got %q", cmd)
	}
	setCombatPositionParallel(&foe.Character, position.Prone)
	if cmd := companionRound(t, me); cmd != "kick" {
		t.Fatalf("a downed foe gets a kick, got %q", cmd)
	}
}

func seedCompanionSpells(t *testing.T) {
	t.Helper()
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"mend-wounds": {SpellId: "mend-wounds", Name: "Mend Wounds", Cost: 40, BaseFolds: 4,
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
			EffectType: "heal", Categories: []string{"self_heal"}},
		"conviction-ward": {SpellId: "conviction-ward", Name: "Conviction Ward", Cost: 30, BaseFolds: 4,
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
			EffectType: "shield", EffectMagnitude: 75, Categories: []string{"self_defense"}},
		"kinetic-hurl": {SpellId: "kinetic-hurl", Name: "Kinetic Hurl", Cost: 15, BaseFolds: 4,
			AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle,
			EffectType: "damage", Categories: []string{"harm_single"}},
		"conviction-spike": {SpellId: "conviction-spike", Name: "Conviction Spike", Cost: 50, BaseFolds: 4,
			AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle,
			EffectType: "damage", Categories: []string{"harm_single"}},
	}))
}

func TestCompanionHealer_KeepsHerselfStandingAndLeavesTheBlows(t *testing.T) {
	seedCompanionSpells(t)
	me, _ := companionFight(t, "companion_healer", 93011, true)
	me.Character.SpellBook = map[string]int{"mend-wounds": 1, "conviction-ward": 1}
	if cmd := companionRound(t, me); cmd != "" {
		t.Fatalf("unhurt and nobody on her, she does not cast for herself, got %q", cmd)
	}
	me.Character.Health = 30
	if cmd := companionRound(t, me); !strings.HasPrefix(cmd, "cast mend-wounds") {
		t.Fatalf("badly hurt, she mends herself, got %q", cmd)
	}
}

func TestCompanionHealer_WardsHerselfOnceSomethingIsOnHer(t *testing.T) {
	seedCompanionSpells(t)
	me, _ := companionFight(t, "companion_healer", 93013, false)
	me.Character.SpellBook = map[string]int{"mend-wounds": 1, "conviction-ward": 1}
	if cmd := companionRound(t, me); !strings.HasPrefix(cmd, "cast conviction-ward") {
		t.Fatalf("with the foe on her she wards herself, got %q", cmd)
	}
}

func TestCompanionBattlemage_StrikesWithHerStrongestThenTheCheaper(t *testing.T) {
	seedCompanionSpells(t)
	me, _ := companionFight(t, "companion_battlemage", 93015, true)
	me.Character.SpellBook = map[string]int{"kinetic-hurl": 1, "conviction-spike": 1}
	if cmd := companionRound(t, me); !strings.HasPrefix(cmd, "cast conviction-spike") {
		t.Fatalf("with conviction to spare she reaches for the spike, got %q", cmd)
	}
	me.Character.Conviction = 20
	if cmd := companionRound(t, me); !strings.HasPrefix(cmd, "cast kinetic-hurl") {
		t.Fatalf("running thin, she hurls instead, got %q", cmd)
	}
}
