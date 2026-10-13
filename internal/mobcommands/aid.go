package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

func Aid(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	raceInfo := species.GetSpecies(mob.Character.SpeciesId)
	if !raceInfo.KnowsFirstAid {

		mob.Command(`emote doesn't know first aid.`)

		return true, nil
	}

	if !room.IsCalm() {
		return true, nil
	}

	if rest == `` {
		return true, nil
	}

	target, err := actions.ResolveTargetActor(room, rest, actions.ResolveTargetOptions{
		FindFlags: []rooms.FindFlag{rooms.FindDowned},
	})
	if err != nil || !target.IsPlayer() {
		return true, nil
	}

	p := target.(*actions.UserActor).User

	if p.Character.Health > 0 {
		return true, nil
	}

	aidPlayerId := p.UserId

	mob.Character.CancelConditionsWithFlag(conditions.Hidden)

	// Set spell Aid
	spellAggro := characters.SpellAggroInfo{
		SpellId:              `aidskill`,
		SpellRest:            ``,
		TargetUserIds:        []int{aidPlayerId},
		TargetMobInstanceIds: []int{},
	}

	spellInfo := spells.GetSpell(`aidskill`)

	// Cast-start lines through the one body every caster shares (messaging M6
	// slice 4). The aided player is the target.
	if castRoom := rooms.LoadRoom(mob.Character.RoomId); castRoom != nil && spellInfo != nil {
		actions.NarrateSpellPhase(actions.NewMobActorInRoom(mob, castRoom), spellInfo, spells.PhaseCast,
			[]int{aidPlayerId}, nil, castRoom, messaging.CategorySpellVital)
	}

	mob.Character.CancelConditionsWithFlag(conditions.Hidden)
	if spellInfo != nil {
		mob.Character.SetCast(spellInfo.WaitRounds, spellAggro)
	}

	return true, nil
}
