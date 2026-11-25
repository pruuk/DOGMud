package mobs

// PathtoFoundOnly is the word `pathto` reads as "take only the secret exits
// this mob has found" (#781, owner ruling 13). A planner's errand route
// carries it (behaviortree.RunGoalPlanner). A schedule's, a patrol's, a
// behaviour tree's, an idle command's or an admin's pathto does not: that
// route is the mob's regular way, and it knows it.
const PathtoFoundOnly = `found`
