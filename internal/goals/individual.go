package goals

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// A goal one live mob holds for itself, beside the goals its type shares from
// the goal file. It lives in the mob's temp data, so it ends with that mob: a
// death, a despawn or a restart, and a respawn is a new mob without it. Nothing
// reaches a goal file or an instance save.
//
// Interim (owner ruling 2026-11-01): the one user is a generic mob's theft
// grudge. Who should remember what, an individual, a type or a faction, is
// pruuk/DOGMud#752 under the behaviour arc #374. Do not grow this into that.

const individualGoalsKey = "individualGoals"

// AddIndividual gives this one mob a goal and returns it, or nil when the mob
// already holds that goal (the same dedup as Add, with no priority escalation
// on a repeat, matching what the seeders do for a type's goals).
func AddIndividual(mob *mobs.Mob, g *Goal) (*Goal, error) {
	if mob == nil || g == nil {
		return nil, nil
	}
	meta, _ := lookupMeta(g.Type)
	if err := ValidateParams(g, meta.Params); err != nil {
		return nil, err
	}

	key := ""
	if meta.DedupKey != nil {
		key = invokeDedupKey(meta.DedupKey, g)
	}
	own := IndividualGoalsOf(mob)
	for _, e := range own {
		if e.Type != g.Type {
			continue
		}
		if !meta.AllowMultiple || (meta.DedupKey != nil && invokeDedupKey(meta.DedupKey, e) == key) {
			return nil, nil
		}
	}

	g.Id = "individual:" + g.Type + ":" + key
	g.OwnerMobId = int(mob.MobId)
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	mob.SetTempData(individualGoalsKey, append(own, g))
	return g, nil
}

// IndividualGoalsOf returns the goals this mob holds for itself, dropping any
// its type's predicate says is over.
func IndividualGoalsOf(mob *mobs.Mob) []*Goal {
	if mob == nil {
		return nil
	}
	own, _ := mob.GetTempData(individualGoalsKey).([]*Goal)
	if len(own) == 0 {
		return nil
	}
	live := make([]*Goal, 0, len(own))
	for _, g := range own {
		if !IsSatisfied(g, mob) {
			live = append(live, g)
		}
	}
	if len(live) == len(own) {
		return own
	}
	if len(live) == 0 {
		mob.SetTempData(individualGoalsKey, nil)
		return nil
	}
	mob.SetTempData(individualGoalsKey, live)
	return live
}

// CurrentGoalFor is the goal this mob acts on now: its type's current goal
// unless a goal of its own outscores it, on the same score Select uses. The
// type's goal keeps a tie, and a goal of its own scoring zero is never chosen.
func CurrentGoalFor(mob *mobs.Mob) *Goal {
	if mob == nil {
		return nil
	}
	best := CurrentGoalOf(int(mob.MobId), util.ConvertForFilename(mob.Character.Name))
	own := IndividualGoalsOf(mob)
	if len(own) == 0 {
		return best
	}
	weights := resolveWeights(mob)
	bestScore := 0.0
	if best != nil {
		bestScore = effectiveScore(best, weights, mob)
	}
	for _, g := range own {
		if score := effectiveScore(g, weights, mob); score > bestScore {
			best, bestScore = g, score
		}
	}
	return best
}
