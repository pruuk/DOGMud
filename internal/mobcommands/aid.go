package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/textutil"
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

	mob.Character.CancelBuffsWithFlag(buffs.Hidden)

	// Set spell Aid
	spellAggro := characters.SpellAggroInfo{
		SpellId:              `aidskill`,
		SpellRest:            ``,
		TargetUserIds:        []int{aidPlayerId},
		TargetMobInstanceIds: []int{},
	}

	spellInfo := spells.GetSpell(`aidskill`)

	// Send YAML cast text (if defined).
	if spellInfo != nil && spellInfo.Narration(spells.PhaseCast).Len() > 0 {
		castRoom := rooms.LoadRoom(mob.Character.RoomId)
		roles := spellInfo.Narrate(spells.PhaseCast, textutil.TokenContext{
			SourceName:      mob.Character.GetCharacterName(true),
			SourcePlainName: mob.Character.GetCharacterName(false),
			TargetName:      p.Character.GetCharacterName(true),
			TargetPlainName: p.Character.GetCharacterName(false),
		})
		// A mob caster has no client: its own line is rendered and dropped.
		if roles.Observer != "" && castRoom != nil {
			castRoom.SendTextVisual(messaging.CategorySpellVital, roles.Observer)
		}
	}

	mob.Character.CancelBuffsWithFlag(buffs.Hidden)
	if spellInfo != nil {
		mob.Character.SetCast(spellInfo.WaitRounds, spellAggro)
	}

	return true, nil
}
