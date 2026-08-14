package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// deathProtectionBuffId is the buff carrying the ReviveOnDeath flag,
// _datafiles/world/default/buffs/35-death_protection.yaml.
const deathProtectionBuffId = 35

// newRouteDeathTestMob builds a mob and registers it in the instance registry
// so mobs.GetInstance can resolve it, restoring the registry on cleanup. A mob
// that is NOT registered cannot be resolved, and the test would pass for the
// wrong reason.
func newRouteDeathTestMob(t *testing.T, health int) *mobs.Mob {
	t.Helper()

	m := &mobs.Mob{
		MobId:      1,
		InstanceId: 90001,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:      "Route-Death-Dummy",
			RoomId:    1,
			Health:    health,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	m.Character.HealthMax.Base = 100
	m.Character.HealthMax.Recalculate()

	cleanup := mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{m.InstanceId: m})
	t.Cleanup(cleanup)

	return m
}

// seedReviveBuff registers a buff spec carrying the ReviveOnDeath flag.
func seedReviveBuff(t *testing.T) {
	t.Helper()
	cleanup := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		deathProtectionBuffId: {
			BuffId:       deathProtectionBuffId,
			Name:         "Death Protection",
			TriggerCount: 1000000,
			Flags:        []buffs.Flag{buffs.ReviveOnDeath},
		},
	})
	t.Cleanup(cleanup)
}

func TestRouteAttributedDeath_WrongEventTypeIsRejected(t *testing.T) {
	if got := RouteAttributedDeath(events.Buff{}); got != events.Cancel {
		t.Errorf("got %v, want Cancel for a mismatched event type", got)
	}
}

func TestRouteAttributedDeath_UnknownVictimIsInert(t *testing.T) {
	got := RouteAttributedDeath(events.CharacterDied{MobInstanceId: 999999})
	if got != events.Continue {
		t.Errorf("got %v, want Continue when the victim is already gone", got)
	}
}

// ReviveOnDeath must heal, cancel the buff, clear DeathQueued and NOT die.
// Leaving DeathQueued set would make the character permanently unkillable;
// leaving health negative would just hand the kill to the sweep next tick.
func TestRouteAttributedDeath_ReviveHealsAndClearsQueue(t *testing.T) {
	seedReviveBuff(t)
	mob := newRouteDeathTestMob(t, -20)
	if err := mob.Character.AddBuff(deathProtectionBuffId, true); err != nil {
		t.Fatalf("AddBuff: %v", err)
	}

	if !mob.Character.HasBuffFlag(buffs.ReviveOnDeath) {
		t.Fatal("precondition: buff did not apply the ReviveOnDeath flag")
	}
	mob.Character.DeathQueued = true

	RouteAttributedDeath(events.CharacterDied{MobInstanceId: mob.InstanceId})

	if !mob.Character.IsAlive() {
		t.Error("revive did not prevent the death")
	}
	if mob.Character.Health < 1 {
		t.Errorf("health = %d, want positive — a revived character left dying is reaped by the sweep",
			mob.Character.Health)
	}
	if mob.Character.DeathQueued {
		t.Error("DeathQueued still set after a revive; the character can never be killed again")
	}
	if mob.Character.HasBuffFlag(buffs.ReviveOnDeath) {
		t.Error("revive buff was not consumed")
	}
}
