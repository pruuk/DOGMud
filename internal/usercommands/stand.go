package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Stand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// Chunk 3.3: stand wakes a sleeping player. Cancel BEFORE the
	// "already standing" bail so a standing-but-sleeping player can
	// still wake via stand.
	if user.Character.HasBuffFlag(buffs.Sleeping) {
		user.Character.CancelBuffsWithFlag(buffs.Sleeping)
		mobs.OnSleeperWoken(user.Character)
	}

	// Gate on the Position FSM. Only Prone/Supine can "stand"; a grapple lock
	// must be escaped, not stood out of (previously any non-prone/supine state
	// — including grapples — wrongly reported "already standing").
	if user.Character.IsStanding() {
		user.SendText(messaging.CategorySystem, "You're already standing.")
		return true, nil
	}
	if user.Character.IsGrappling() {
		user.SendText(messaging.CategorySystem, "You're locked in a grapple — you'll need to break free first.")
		return true, nil
	}
	if !user.Character.IsProne() && !user.Character.IsSupine() {
		user.SendText(messaging.CategorySystem, "You can't stand up right now.")
		return true, nil
	}

	cfg := configs.GetBalanceConfig()

	// Calculate stamina cost and requirement.
	//
	// U7 Task 11: BOTH fractions are taken off the pool the character can
	// actually reach, not the raw StaminaMax. RecalculateStats clamps current
	// stamina to max - reservation every round, so measuring a percentage-of-max
	// threshold against the raw max charged the reserve twice: at the shipped
	// 0.15 knobs the gate demanded 21.4% of a 30%-reserved character's reachable
	// pool, and past 85% reservation it demanded more stamina than the pool could
	// ever hold, permanently locking that character on the floor.
	effMax := user.Character.EffectivePoolMax(characters.PoolStamina)
	staminaCost := int(float64(effMax) * float64(cfg.StandStaminaCost))
	minStamina := int(float64(effMax) * float64(cfg.StandMinStamina))

	// Check if player has enough stamina.
	//
	// U5b-2: stand REFUSES, and it is deliberately not a single ApplyCost.
	// StandMinStamina gates and StandStaminaCost charges; they are independent
	// knobs that merely happen to ship at the same fraction today. Collapsing
	// them into one call would weld a tuning coincidence into code.
	if !user.Character.CanAfford(characters.PoolStamina, minStamina) {
		// Name the reservation when there is one. Blaming exhaustion alone was
		// misleading: rest cannot return stamina the character's gear is holding
		// out of the pool, and the player has no other way to learn that. Bands,
		// never raw numbers -- same disclosure style as assess.
		msg := `You're too winded to get up. Catch your breath first.`
		if reserved := user.Character.StaminaMax.Value - effMax; reserved > 0 {
			msg += ` Your gear is holding ` + reserveShareBand(reserved, user.Character.StaminaMax.Value) +
				` of your stamina in reserve, so you have less to draw on than you might expect.`
		}
		user.SendText(messaging.CategorySystem, msg)
		return true, nil
	}

	// Fire the FSM transition first — bail without charging stamina if it fails
	// (shouldn't happen since Prone→Standing and Supine→Standing are valid edges).
	if err := user.Character.Position.TransitionToStanding(state.TransitionReason{Trigger: position.TriggerStandCommand}); err != nil {
		mudlog.Warn("Stand: TransitionToStanding failed", "user", user.UserId, "err", err)
		user.SendText(messaging.CategorySystem, "Something prevents you from standing.")
		return true, nil
	}

	// Charge stamina. Partial, not full-or-refuse: the FSM transition above has
	// already fired, so a refusal here would let the player stand for FREE. That
	// is unreachable at shipped values (both knobs are 0.15, so the amounts are
	// identical), but it is the correct shape if they are ever tuned apart.
	// ApplyCostPartial also makes the old manual sub-zero clamp redundant.
	_ = user.Character.ApplyCostPartial(characters.PoolStamina, staminaCost)

	// Send messages
	user.SendText(messaging.CategorySystem, "You struggle to your feet!")

	room.SendTextVisual(messaging.CategoryMobEmote,
		fmt.Sprintf(`<ansi fg="username">%s</ansi> struggles to their feet.`, user.Character.Name),
		user.UserId,
	)

	// If in combat, standing costs the current round
	if user.Character.Aggro != nil {
		user.Character.Aggro.RoundsWaiting = 1
	}

	return true, nil
}
