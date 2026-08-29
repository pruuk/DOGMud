package behaviortree

// actions_combat.go — combat actions:
// actAttack, actFlee, actCast, actAddBuff, actRemoveBuff,
// actionCancelActivity

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func actAttack(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	targetUserId := ctx.Event.UserId
	targetMobId := ctx.Event.MobId
	// If the event carries a specific attacker (player OR mob), use it
	// directly. Only fall back to "random player in room" when neither
	// is set (e.g., a heuristic trigger with no specific attacker).
	//
	// Regression guard: pre-2026-05-26, this fell back to the random-
	// player picker whenever UserId==0, even when MobId was set. That
	// caused caravan leaders (Ketil) to aggro any player following them
	// the moment a hostile mob (bandit lookout) ambushed the crew.
	// Loaded once and shared with the EngageAggroType call below. The
	// random-player fallback treats a missing room as fatal; the aggro-typing
	// pass below treats it as merely un-typeable (see its comment).
	room := rooms.LoadRoom(ctx.RoomId)
	if targetUserId == 0 && targetMobId == 0 {
		if room == nil {
			return Failure
		}
		// This was a third inline copy of the random-player picker, beside
		// target_random_player_in_room and (until U12a) nothing that knew the
		// two were the same thing.
		ref, ok := targeting.Select(
			targeting.Criteria{Kind: targeting.RandomPlayer},
			targeting.Scope{Room: room, Self: &mob.Character, SelfMobInstanceId: ctx.InstanceId})
		if !ok {
			return Failure
		}
		targetUserId = ref.UserId
	}
	// U10d: through EngageAggroType so a btree ambush respects the special-move
	// cooldown exactly as the player and mobcommands paths do. Setting
	// SurpriseAttack straight from IsHidden() let btree mobs ambush on a
	// cooldown the other two honoured. The promotion makes the opening strike
	// of the ordinary combat round resolve as a surprise — there is no
	// separate backstab crit any more.
	//
	// Second behaviour change, deliberate: when the room fails to load or the
	// target id resolves to nothing, a hidden mob now degrades to
	// DefaultAttack where the old IsHidden() read would still have said
	// SurpriseAttack. Aggro at a target that cannot be resolved is already
	// degenerate, and typing it as a surprise would charge nothing and gate
	// nothing — but it IS a change, not just the cooldown fix.
	aggroType := characters.DefaultAttack
	if room != nil {
		var target actions.Actor
		if targetUserId > 0 {
			if u := users.GetByUserId(targetUserId); u != nil {
				target = actions.NewUserActorInRoom(u, room)
			}
		} else if targetMobId > 0 {
			if m := mobs.GetInstance(targetMobId); m != nil {
				target = actions.NewMobActorInRoom(m, room)
			}
		}
		if target != nil {
			// The refusal signal is discarded on purpose: a behaviour-tree mob
			// has no one to tell. Only the player-facing paths speak it.
			aggroType, _ = actions.EngageAggroType(actions.NewMobActorInRoom(mob, room), target)
		}
	}
	targeting.Commit(&mob.Character,
		state.ActorRef{UserId: targetUserId, MobInstanceId: targetMobId},
		targeting.ReasonForAggroType(aggroType))
	return Success
}

func actFlee(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	mob.Command("flee")
	return Success
}

// actCast issues a `cast <spell>` command for the acting mob. An optional
// `target` param names another mob (or player) in the room to cast at —
// e.g. a Repair Frame add healing the boss by name — and is forwarded as
// `cast <spell> <target>`. The engine's mob HelpSingle targeting
// (actions/cast.go, room.FindByName) resolves the named target; leaving
// target unset preserves the existing self/default-target behavior.
//
// params: spell (string, required), target (string, optional)
func actCast(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	spell := getStringParam(params, "spell")
	if spell == "" {
		return Failure
	}
	target := getStringParam(params, "target")
	if target != "" {
		mob.Command("cast " + spell + " " + target)
	} else {
		mob.Command("cast " + spell)
	}
	return Success
}

// actAddBuff applies a buff to the acting mob.
// params: buff_id (int)
func actAddBuff(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	buffId := getIntParam(params, "buff_id")
	if buffId == 0 {
		return Failure
	}
	mob.AddBuff(buffId, "behaviortree")
	return Success
}

// actRemoveBuff removes a buff from the triggering player by buff ID.
// params: buff_id (int)
func actRemoveBuff(params map[string]any, ctx *EvalContext) Result {
	user := users.GetByUserId(ctx.Event.UserId)
	if user == nil {
		return Failure
	}
	buffId := getIntParam(params, "buff_id")
	user.Character.RemoveBuff(buffId)
	return Success
}

// actTargetRandomPlayerInRoom picks a random player in the
// caller's current room and stashes them as the EvalContext's
// SoftTarget. This is the non-combat target-picker primitive
// used by skullduggery archetypes (thief, future shadow/plant
// variants).
//
// CRITICAL: this action does NOT call SetAggro or transition
// Combat Phase. The picked player is a "soft target" — the
// caller's NEXT action (try_steal, try_plant, etc.) consumes
// SoftTarget without triggering combat. This is the structural
// fix for the chunk 2.7 thief-archetype bug; non-combat target
// picking must not silently engage combat.
//
// Returns Failure when no players are present in the room.
func actTargetRandomPlayerInRoom(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return Failure
	}
	// Select, never Commit. This archetype picks a victim for skullduggery
	// WITHOUT entering combat; committing here is the chunk-2.7 bug class
	// that ctx.SoftTarget exists to prevent. The seam makes the distinction
	// explicit rather than a comment nobody can enforce.
	ref, ok := targeting.Select(
		targeting.Criteria{Kind: targeting.RandomPlayer},
		targeting.Scope{Room: room, Self: &mob.Character, SelfMobInstanceId: ctx.InstanceId})
	if !ok {
		return Failure
	}
	ctx.SoftTarget = ref
	return Success
}

// actTargetWeakestMobInRoom scans room.GetMobs(), computes
// PowerScore(target) / PowerScore(self) for each candidate that
// passes mob.HatesMob, picks the lowest ratio strictly below the
// ratio_below ceiling (default 1.0), and sets it as Aggro.
// Returns Success on a successful target pick, Failure otherwise.
//
// Skips: self, dead mobs, non-combatant mobs, mobs the caller's
// HatesMob returns false for, and (if caller is itself charmed)
// fellow companions of the same owner.
//
// Players are NOT scanned — predation is a mob-vs-mob action.
// Player aggression continues through the standard hostile-mob
// attack chain.
func actTargetWeakestMobInRoom(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil || mob.IsNonCombatant() {
		return Failure
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}

	// The scan (self-skip, dead, non-combatant, HatesMob, the
	// companion-allegiance skip and the power-ratio ceiling) moved verbatim
	// into targeting.selectWeakestHatedMob. ratio_below still defaults to 1.0,
	// meaning "engage anyone strictly weaker".
	ref, ok := targeting.Select(
		targeting.Criteria{
			Kind:       targeting.WeakestHatedMob,
			RatioBelow: getFloatParam(params, "ratio_below", 1.0),
		},
		targeting.Scope{Room: room, Self: &mob.Character, SelfMobInstanceId: ctx.InstanceId})
	if !ok {
		return Failure
	}

	// Predation DOES commit: unlike the skullduggery picker above, this
	// archetype wants the fight.
	targeting.Commit(&mob.Character, ref, targeting.ReasonAttack)
	return Success
}

// actionCancelActivity aborts the mob's current Activity if any is
// in progress. Returns Success if an activity was cancelled, Failure
// if the mob was already Free (or has no Activity machine).
//
// Inlines the refund + cleanup logic rather than delegating to
// mobcommands.Cancel because mobcommands imports behaviortree
// (callforhelp.go, flee.go), which would create a circular import.
//
// Use cases in behavior trees:
//   - panic-flee on low HP: cancel offensive cast, then flee
//   - swap to heal mid-cast when ally is dying
//   - drop craft to defend when ambushed
func actionCancelActivity(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	a := mob.Character.Activity
	if a == nil || a.IsFree() {
		return Failure
	}

	switch a.State() {
	case activity.Casting:
		d, _ := a.CastingData()
		unspent := d.TotalConvictionCost - d.ConvictionSpent
		if unspent > 0 {
			refund := unspent / 2
			mob.Character.ApplyRestore(characters.PoolConviction, refund)
		}
		_ = a.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerCastCancel,
			Actor:   state.ActorRef{MobInstanceId: mob.InstanceId},
		})

	case activity.Crafting:
		_ = a.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerCraftCancel,
			Actor:   state.ActorRef{MobInstanceId: mob.InstanceId},
		})

	case activity.Salvaging:
		_ = a.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerSalvageCancel,
			Actor:   state.ActorRef{MobInstanceId: mob.InstanceId},
		})
	}
	return Success
}
