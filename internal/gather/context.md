# Gather Package Context

## Overview

`internal/gather` is the shared roll behind the wilderness trades
(`docs/economy/wilderness-trades.md`): skinning, butchering, chopping,
sawing and, later, foraging. For one gathering job it answers whether it
worked, what grade came out and which tool was used.

The roll leans on the gatherer's body and tool, not their skill:

    score      = avg(StatA, StatB) * toolMult + skill * Balance.GatherSkillWeight
    difficulty = Balance.GatherBaseDifficulty + targetTier

`GatherSkillWeight` (1.5) is far below the craft `SkillWeight` (5.0).

## Files

- **gather.go**: `Job` (two stat names, an optional skill, a tool type,
  whether the tool is required), the predefined `JobSkin` and `JobButcher`,
  `Roll`, `Result`, `Inputs`, `Score`, `Difficulty`, `Rounds`, and the
  unexported `rollInputs` and `gradeFrom`.
- **tools.go**: `Tool`, `BestTool`, `TierMult` and the unexported
  `bestToolFrom`, `asTool`, `better`.

## Rules

- `Roll(c, room, job, targetTier)` finds the best tool, refuses with
  `Result.NoTool` when a required tool is missing, applies
  `messaging.SightMult` once (nil room means full light) and contests
  through `crafting.RunSalvageContest`, so the salvage mercy floor applies.
- Grade: a win is standard plus one grade per `GatherGradeStepSigma` of
  margin (in roll standard deviations, `AttackRoll.StdDev * sqrt 2`); a loss
  within half a step is crude; a floor-granted win is crude; anything else
  is nothing. The tool tier then caps it (`items.ToolTier.MaxGrade`). A job
  with no tool type is capped only at pristine; a tool job done without its
  tool is capped as crude.
- `BestTool` searches equipped items, then the backpack. It never searches
  the bandolier or component bag. Ties go to a real tool over an improvised
  one, then to the faster tool.
- `Tool` is a transient copy (listed in `transientItemHolders` in
  `bauble_sweep_guard_test.go`); the tool itself stays in the inventory.
- `rollInputs` is exempt in `sight_penalty_guard_test.go` because `Roll`
  sets `Inputs.SightMult` and `Score` multiplies it in.

## Tests

`gather_test.go` pins the grade ladder, the tool caps, floored outcomes,
that stats and tools outweigh skill, tool selection (tier, improvised,
grade nudges) and the missing-tool refusal.

## Crafted grades

`CraftGrade(cr, consumed, recipeHasTool, toolTier)` grades a crafted output:
ungraded unless an input is graded or the recipe has a tool; standard plus
one per `GatherGradeStepSigma` of margin (floored win crude, nil contest
standard); capped one above the worst graded input and by the tool tier.
Called from the multi-round craft completion in
`hooks/NewRound_UserRoundTick.go` and the instant path in `actions/craft.go`.

`JobChop` (strength and vitality, axe required, no skill) is the felling job.

## Rare finds, wear and forged tools (wilderness trades review)

- `RareMult(tier)` is Balance `RareToolMult*`: the multiplier a tool tier
  puts on rare part and rare forage odds.
- `WearTool(c, tool)` adds one job's wear to the character's own copy of the
  tool (equipped first, then carried) and removes it when it breaks,
  returning its name. Improvised weapons never wear.
- `CraftGradeOutput(cr, consumed, hasTool, toolTier, outputIsTool)` is
  `CraftGrade` with the output in view: a tool output is always graded by
  the margin, with no tool cap. `CraftGrade` calls it with false.

`JobMine` (strength and vitality, pick required, no skill) is the mining job.
