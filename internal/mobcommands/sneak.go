package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

func Sneak(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	result := actions.Sneak(&actions.MobActor{Mob: mob, Room: room})
	if result.Cost.Status == characters.CostRefused {
		return true, nil
	}

	// U10b-1 Task 18. ⚠️ THIS SITE IS NOT IN THE PLAN'S LIST OF SIXTEEN -- the
	// count was seventeen. Found by sweeping every production progression call
	// rather than trusting the enumeration.
	//
	// Awarded win or lose so a mob that was spotted trains like one that was
	// not, matching the player path in
	// usercommands/skill.skullduggery.sneak.go.
	mob.Character.AwardResolved(0, result.Success, mob.Character.CandidateFor("skullduggery"))

	return true, nil
}
