package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// recoveryContest builds the opposed prone-recovery roll for
// AttemptRecovery. Contested only when someone is actually holding the
// recoverer down: a living, same-room actor from the recoverer's inbound
// attacker set (Character.Attackers() — the accessor context.md
// prescribes over direct Aggro reads). Opponent = the strongest such
// holder by recovery score. Returns nil — a free stand — when nobody
// qualifies. Standard ContestFloor applies, so nobody is pinned forever
// and no stand is ever certain against live opposition. (A player can
// always take the paid exit instead: the manual stand command is
// uncontested by design.)
func recoveryContest(ch *characters.Character) func() bool {
	w := float64(configs.GetBalanceConfig().SkillWeight)
	score := func(c *characters.Character) float64 {
		return float64(c.Stats.Dexterity.ValueAdj) +
			float64(c.GetSkillLevel(skills.UnarmedCombat))*w
	}

	var best *characters.Character
	for _, ref := range ch.Attackers() {
		var opp *characters.Character
		if ref.UserId > 0 {
			if u := users.GetByUserId(ref.UserId); u != nil {
				opp = u.Character
			}
		} else if ref.MobInstanceId > 0 {
			if m := mobs.GetInstance(ref.MobInstanceId); m != nil {
				opp = &m.Character
			}
		}
		if opp == nil || opp.Health < 1 || opp.RoomId != ch.RoomId {
			continue
		}
		if best == nil || score(opp) > score(best) {
			best = opp
		}
	}
	if best == nil {
		return nil
	}

	self, other := score(ch), score(best)
	return func() bool {
		return combat.RunContest(self, []contest.Entry{{Score: other}}).Success
	}
}
