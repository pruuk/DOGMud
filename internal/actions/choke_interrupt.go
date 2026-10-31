package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// chokeCastInterrupt is the cast interrupt a choking move carries (throttle,
// smother): an opposed contest through the concentration seam, the caster's
// hold (Willpower plus spellcasting) against the attacker's grip (Dexterity
// plus unarmed combat), floored only by ConcentrationFloor. A telegraphed
// NoDamageInterrupt cast is not contestable. The hold takes the sight ramp
// (the caster needs to see to hold the weave); the grip takes none. The
// defender's concentration award fires win or lose. Callers run it on a hit
// only, after ClaimSpecialMove, so it fires at most once per special-move
// cooldown window. Returns true when the spell was cut off.
func chokeCastInterrupt(actor Actor, char, targetChar *characters.Character, sightRoom messaging.RoomVisibility) bool {
	if !targetChar.IsCasting() {
		return false
	}
	if cs, ok := targetChar.Activity.CastingData(); ok {
		if sd := spells.GetSpell(cs.SpellId); sd != nil && sd.NoDamageInterrupt {
			return false
		}
	}
	grip := float64(char.Stats.Dexterity.ValueAdj) + configs.WeightedSkill(float64(char.GetSkillLevel(skills.UnarmedCombat)))
	hold := float64(targetChar.Stats.Willpower.ValueAdj) + configs.WeightedSkill(float64(targetChar.GetSkillLevel(skills.Spellcasting)))
	hold *= messaging.SightMult(targetChar, sightRoom)
	res := combat.RunConcentrationContest(hold, grip)
	// The DEFENDER's concentration award, win or lose. res.Success is the
	// caster's win: hold is the attack side of the contest. The attacker's
	// own progression comes from the move's hit resolution.
	targetChar.AwardResolved(targetChar.GetUserId(), res.Success, targetChar.CandidateFor(string(skills.Spellcasting)))
	if res.Success {
		return false
	}
	return InterruptTargetCast(targetChar, ActorRefOf(actor))
}
