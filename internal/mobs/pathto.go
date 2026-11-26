package mobs

import "strings"

// PathtoFoundOnly is the word `pathto` reads as "take only the secret exits
// this mob has found" (#781, owner ruling 13). A planner's errand route
// carries it (behaviortree.RunGoalPlanner). A schedule's, a patrol's, a
// behaviour tree's, an idle command's or an admin's pathto does not: that
// route is the mob's regular way, and it knows it.
const PathtoFoundOnly = `found`

// PathtoCommand is the `pathto` line to targets (room ids or `home`), marked
// PathtoFoundOnly when foundOnly: the one builder beside the one reader,
// mobcommands' pathtoTargets. A planner's errand (behaviortree.RunGoalPlanner)
// and the path walker's re-path (hooks.advanceMobPath) build theirs here.
func PathtoCommand(foundOnly bool, targets ...string) string {
	var b strings.Builder
	b.WriteString(`pathto`)
	if foundOnly {
		b.WriteString(` ` + PathtoFoundOnly)
	}
	for _, t := range targets {
		b.WriteString(` ` + t)
	}
	return b.String()
}
