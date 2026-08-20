package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Rake(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor, handled := stageSpecialMoveTarget(user, room, rest, actions.MeleeTargetOpts{
		Verb: "rake",
	})
	if handled {
		return true, nil
	}

	// Delegate core resolution to the shared action.
	res := actions.ExecuteRake(actor)
	if res.Cost.Status == characters.CostRefused {
		user.SendText(messaging.CategorySystem, actions.CostRefusalText(res.Cost))
		return true, nil
	}

	if res.NotClawed {
		user.SendText(messaging.CategorySystem, "You have no claws to rake with.")
		return true, nil
	}
	if res.OnCooldown {
		user.SendText(messaging.CategorySystem, "You need a moment to recover before attempting another special move.")
		return true, nil
	}
	if res.NoTarget {
		user.SendText(messaging.CategorySystem, "You have no target!")
		return true, nil
	}

	targetName := res.Target.Name

	// Resolve player target for direct messaging.
	var targetChar *users.UserRecord
	if res.Target.UserId > 0 {
		targetChar = users.GetByUserId(res.Target.UserId)
	}

	dmgDesc := combat.GetDamageDescription(res.MoveResult.Damage, res.MoveResult.TargetMaxHP)

	if res.MoveResult.Hit {
		rakeMsgs := []string{
			`Your claws rake across <ansi fg="mobname">%s</ansi>, opening raking gashes that weep! (<ansi fg="damage">%s</ansi>)`,
			`You slash <ansi fg="mobname">%s</ansi> with raking claws, leaving bleeding wounds! (<ansi fg="damage">%s</ansi>)`,
			`Your raking strike tears into <ansi fg="mobname">%s</ansi>! (<ansi fg="damage">%s</ansi>)`,
			`You drive your claws across <ansi fg="mobname">%s</ansi>'s flesh — blood wells up! (<ansi fg="damage">%s</ansi>)`,
			`A vicious rake of your claws shreds <ansi fg="mobname">%s</ansi>! (<ansi fg="damage">%s</ansi>)`,
		}
		rakeTargetMsgs := []string{
			`<ansi fg="username">%s</ansi> rakes their claws across you, opening bleeding wounds! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="username">%s</ansi> slashes you with raking claws! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="username">%s</ansi>'s raking strike tears into you! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="username">%s</ansi> drives their claws across your flesh — you bleed! (<ansi fg="damage">%s</ansi>)`,
		}
		rakeRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> rakes their claws across <ansi fg="mobname">%s</ansi>!`,
			`<ansi fg="username">%s</ansi> slashes <ansi fg="mobname">%s</ansi> with raking claws!`,
			`<ansi fg="username">%s</ansi>'s claws tear into <ansi fg="mobname">%s</ansi>!`,
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(rakeMsgs[util.Rand(len(rakeMsgs))], targetName, dmgDesc))
		if targetChar != nil {
			targetChar.SendText(messaging.CategorySystem, fmt.Sprintf(rakeTargetMsgs[util.Rand(len(rakeTargetMsgs))], user.Character.Name, dmgDesc))
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp, fmt.Sprintf(rakeRoomMsgs[util.Rand(len(rakeRoomMsgs))], user.Character.Name, targetName), user.UserId, res.Target.UserId)
	} else if res.MoveResult.Damage > 0 {
		partialMsgs := []string{
			`Your raking claws mostly miss <ansi fg="mobname">%s</ansi>, but still leave a shallow scratch! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="mobname">%s</ansi> dodges most of your swipe, but your claws still catch them! (<ansi fg="damage">%s</ansi>)`,
		}
		partialTargetMsgs := []string{
			`<ansi fg="username">%s</ansi> rakes their claws at you, and you dodge most of it, but they still scratch you! (<ansi fg="damage">%s</ansi>)`,
			`<ansi fg="username">%s</ansi> swipes at you; you dodge most of it, but not all! (<ansi fg="damage">%s</ansi>)`,
		}
		partialRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> rakes their claws at <ansi fg="mobname">%s</ansi>, who mostly dodges but still gets scratched!`,
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(partialMsgs[util.Rand(len(partialMsgs))], targetName, dmgDesc))
		if targetChar != nil {
			targetChar.SendText(messaging.CategorySystem, fmt.Sprintf(partialTargetMsgs[util.Rand(len(partialTargetMsgs))], user.Character.Name, dmgDesc))
		}
		if !sendMoveDefenceTriad(user, room, res.Target, res.MoveResult.Defence, "claw rake", messaging.CategoryHitNaturalSharp, true) {
			room.SendTextVisual(messaging.CategoryHitNaturalSharp, fmt.Sprintf(partialRoomMsgs[util.Rand(len(partialRoomMsgs))], user.Character.Name, targetName), user.UserId, res.Target.UserId)
		}
	} else if !sendMoveDefenceTriad(user, room, res.Target, res.MoveResult.Defence, "claw rake", messaging.CategoryHitNaturalSharp, false) {
		missMsgs := []string{
			`Your raking claws miss <ansi fg="mobname">%s</ansi>!`,
			`You swipe at <ansi fg="mobname">%s</ansi> but they dodge your claws!`,
			`Your rake glances off <ansi fg="mobname">%s</ansi> harmlessly!`,
		}
		missTargetMsgs := []string{
			`<ansi fg="username">%s</ansi> rakes their claws at you, but misses!`,
			`<ansi fg="username">%s</ansi> swipes at you but you dodge!`,
		}
		missRoomMsgs := []string{
			`<ansi fg="username">%s</ansi> rakes their claws at <ansi fg="mobname">%s</ansi>, but misses!`,
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(missMsgs[util.Rand(len(missMsgs))], targetName))
		if targetChar != nil {
			targetChar.SendText(messaging.CategorySystem, fmt.Sprintf(missTargetMsgs[util.Rand(len(missTargetMsgs))], user.Character.Name))
		}
		room.SendTextVisual(messaging.CategoryHitNaturalSharp, fmt.Sprintf(missRoomMsgs[util.Rand(len(missRoomMsgs))], user.Character.Name, targetName), user.UserId, res.Target.UserId)
	}

	return true, nil
}
