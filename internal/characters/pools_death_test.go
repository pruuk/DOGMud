package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// newHarmTestChar builds a character with a known health pool.
func newHarmTestChar(health int) *Character {
	c := New()
	c.HealthMax.Base = 100
	c.HealthMax.Recalculate()
	c.Health = health
	return c
}

func TestApplyHarm_LethalHealthHarmQueuesExactlyOneDeath(t *testing.T) {
	events.DrainQueuedCharacterDiedForTest() // discard leftovers
	c := newHarmTestChar(5)

	c.ApplyHarm(PoolHealth, 25, state.ActorRef{MobInstanceId: 42})

	got := events.DrainQueuedCharacterDiedForTest()
	if len(got) != 1 {
		t.Fatalf("queued %d CharacterDied events, want 1", len(got))
	}
	if got[0].KillerMobInstanceId != 42 {
		t.Errorf("KillerMobInstanceId = %d, want 42", got[0].KillerMobInstanceId)
	}
	if got[0].Overkill != 20 {
		t.Errorf("Overkill = %d, want 20 (health 5 minus a 25 blow)", got[0].Overkill)
	}
	if !c.DeathQueued {
		t.Error("DeathQueued not set")
	}
}

func TestApplyHarm_NonLethalQueuesNothing(t *testing.T) {
	events.DrainQueuedCharacterDiedForTest()
	c := newHarmTestChar(50)

	c.ApplyHarm(PoolHealth, 10, state.ActorRef{MobInstanceId: 42})

	if got := events.DrainQueuedCharacterDiedForTest(); len(got) != 0 {
		t.Fatalf("queued %d events on non-lethal harm, want 0", len(got))
	}
	if c.DeathQueued {
		t.Error("DeathQueued set by non-lethal harm")
	}
}

// The second blow lands on an already-dying target. It must not queue a second
// death, or the victim is attributed and reaped twice.
func TestApplyHarm_SecondLethalBlowDoesNotRequeue(t *testing.T) {
	events.DrainQueuedCharacterDiedForTest()
	c := newHarmTestChar(5)

	c.ApplyHarm(PoolHealth, 25, state.ActorRef{MobInstanceId: 42})
	c.ApplyHarm(PoolHealth, 25, state.ActorRef{MobInstanceId: 99})

	got := events.DrainQueuedCharacterDiedForTest()
	if len(got) != 1 {
		t.Fatalf("queued %d events, want 1 — the first lethal blow wins", len(got))
	}
	if got[0].KillerMobInstanceId != 42 {
		t.Errorf("killer = %d, want 42 — the lethal blow, not the last swing",
			got[0].KillerMobInstanceId)
	}
}

// ApplyHealthChange wraps ApplyHarm, and every one of combat.go's eight damage
// sites goes through it. It used to pass state.ActorRef{} unconditionally,
// which made melee death — the commonest death in the game — anonymous no
// matter what U5c did at the harm site.
func TestApplyHealthChange_CarriesTheKillerThrough(t *testing.T) {
	events.DrainQueuedCharacterDiedForTest()
	c := newHarmTestChar(5)

	c.ApplyHealthChange(-25, state.ActorRef{MobInstanceId: 77})

	got := events.DrainQueuedCharacterDiedForTest()
	if len(got) != 1 {
		t.Fatalf("queued %d events, want 1", len(got))
	}
	if got[0].KillerMobInstanceId != 77 {
		t.Errorf("KillerMobInstanceId = %d, want 77 — the wrapper swallowed the killer",
			got[0].KillerMobInstanceId)
	}
}

// Stamina and conviction floor at 0 and are never lethal, at any magnitude.
func TestApplyHarm_NonHealthPoolsNeverQueue(t *testing.T) {
	for _, pool := range []Pool{PoolStamina, PoolConviction} {
		events.DrainQueuedCharacterDiedForTest()
		c := newHarmTestChar(100)
		c.StaminaMax.Base = 100
		c.StaminaMax.Recalculate()
		c.Stamina = 5
		c.ConvictionMax.Base = 100
		c.ConvictionMax.Recalculate()
		c.Conviction = 5

		c.ApplyHarm(pool, 9999, state.ActorRef{MobInstanceId: 42})

		if got := events.DrainQueuedCharacterDiedForTest(); len(got) != 0 {
			t.Errorf("pool %s queued a death", pool)
		}
	}
}
