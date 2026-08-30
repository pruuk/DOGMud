package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func init() {
	combatphase.SetMachineResolver(resolveCombatPhaseMachine)
}

// resolveCombatPhaseMachine turns an ActorRef into the live CombatPhase
// machine for that actor, by asking the packages that authoritatively own the
// answer. internal/state cannot import users or mobs (they depend on
// internal/characters, which depends on internal/state), so the function is
// injected here instead.
//
// This replaced a hand-maintained registry map. See the "Machine resolution"
// comment in internal/state/combatphase/combatphase.go for why: the map was a
// second copy of userManager and mobInstances, and every way it drifted from
// them was a live bug.
//
// Returning nil is normal and every caller handles it: the actor has logged
// out, despawned, or never existed.
func resolveCombatPhaseMachine(ref state.ActorRef) *combatphase.Machine {
	if ref.UserId > 0 {
		if u := users.GetByUserId(ref.UserId); u != nil && u.Character != nil {
			return u.Character.CombatPhase
		}
		return nil
	}
	if ref.MobInstanceId > 0 {
		if m := mobs.GetInstance(ref.MobInstanceId); m != nil {
			return m.Character.CombatPhase
		}
	}
	return nil
}
