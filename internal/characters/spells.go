package characters

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"

	"maps"
)

/*
All spells should have a 10% minimum chance of success.
*/
func (c *Character) GetBaseCastSuccessChance(spellId string) int {

	sp := spells.GetSpell(spellId)
	if sp == nil {
		return -1
	}

	// start with 100% chance of success
	targetNumber := 100

	// subtract spell difficulty
	// 1-100
	targetNumber -= sp.GetDifficulty()

	// add spell level bonus
	// 10-30
	skillLevel := c.GetSkillLevel(skills.Spellcasting)
	//targetNumber += (skillLevel * 5)
	//targetNumber -= 5 // cancel out the first level

	// add the proficiency of the spell (more casts == better)
	// 0-20
	profFactor := 1.0
	if skillLevel == 2 {
		profFactor = 1.25 // .25 more than lvl 1
	} else if skillLevel == 3 {
		profFactor = 1.75 // .50 more than lvl 2
	} else if skillLevel == 4 {
		profFactor = 2.50 // .75 more than lvl 3
	}
	casts := c.SpellBook[spellId]
	castsPerPoint := float64(configs.GetBalanceConfig().SpellProficiencyCastsPerPoint)
	proficiency := int(math.Floor((float64(casts) / castsPerPoint * profFactor)))
	if proficiency < 0 {
		proficiency = 0
	} else if proficiency > 20 {
		proficiency = 20
	}
	targetNumber += proficiency

	targetNumber += int(math.Floor(float64(c.Stats.Willpower.ValueAdj) / 5))

	// add by any stat mods for casting, or casting school
	// 0-xx
	targetNumber += c.StatMod(string("casting"))
	// Add stat mods for each school the spell belongs to
	for _, school := range sp.Schools {
		targetNumber += c.StatMod(string("casting-") + school)
	}

	if targetNumber < 0 {
		targetNumber = 0
	} else if targetNumber > 100 {
		targetNumber = 100
	}

	return targetNumber
}

func (c *Character) GetSpells() map[string]int {
	ret := make(map[string]int)
	maps.Copy(ret, c.SpellBook)
	return ret
}

// IsCasting returns true if the Activity machine is in the Casting state.
func (c *Character) IsCasting() bool {
	if c.Activity == nil {
		return false
	}
	return c.Activity.IsCasting()
}

// IsCrafting returns true if the Activity machine is in the Crafting state.
func (c *Character) IsCrafting() bool {
	if c.Activity == nil {
		return false
	}
	return c.Activity.IsCrafting()
}

// IsSalvaging returns true if the Activity machine is in the Salvaging state.
func (c *Character) IsSalvaging() bool {
	if c.Activity == nil {
		return false
	}
	return c.Activity.IsSalvaging()
}

// IsFree returns true if the character has no active Activity (nil machine or
// machine in the Free state). Safe to call on characters constructed outside
// New() before Validate() has run.
func (c *Character) IsFree() bool {
	if c.Activity == nil {
		return true
	}
	return c.Activity.IsFree()
}

// IsActing returns true if the Activity machine is in any non-Free state.
func (c *Character) IsActing() bool {
	if c.Activity == nil {
		return false
	}
	return c.Activity.IsActing()
}

func (c *Character) HasSpell(spellName string) bool {
	if intVal, ok := c.SpellBook[spellName]; ok {
		return intVal > 0
	}
	return false
}

func (c *Character) DisableSpell(spellName string) bool {
	if intVal, ok := c.SpellBook[spellName]; ok {
		if intVal > 0 {
			c.SpellBook[spellName] = intVal * -1
		}
	}
	return false
}

func (c *Character) EnableSpell(spellName string) bool {
	if intVal, ok := c.SpellBook[spellName]; ok {
		if intVal < 0 {
			c.SpellBook[spellName] = intVal * -1
		}
	}
	return false
}

func (c *Character) TrackSpellCast(spellName string) bool {
	if intVal, ok := c.SpellBook[spellName]; ok {
		if intVal > 0 {
			intVal++
			c.SpellBook[spellName] = intVal
		}
	}
	return false
}

// pairedSpells maps spells that are always learned together.
// Learning either spell automatically grants the other.
var pairedSpells = map[string]string{
	"fold-anchor": "fold-recall",
	"fold-recall": "fold-anchor",
}

func (c *Character) LearnSpell(spellName string) bool {
	if _, ok := c.SpellBook[spellName]; ok {
		return false
	}
	if c.SpellBook == nil {
		// Characters constructed directly (tests, tooling) skip Validate(),
		// which normally initializes this map on load.
		c.SpellBook = make(map[string]int)
	}
	c.SpellBook[spellName] = 1

	// Grant paired spell if one exists
	if paired, ok := pairedSpells[spellName]; ok {
		if _, known := c.SpellBook[paired]; !known {
			c.SpellBook[paired] = 1
		}
	}

	return true
}

func (c *Character) HasRecipe(recipeId string) bool {
	if c.KnownRecipes == nil {
		return false
	}
	if intVal, ok := c.KnownRecipes[recipeId]; ok {
		return intVal > 0
	}
	return false
}

func (c *Character) LearnRecipe(recipeId string) bool {
	if c.KnownRecipes == nil {
		c.KnownRecipes = crafting.GetStarterRecipes()
	}
	if _, ok := c.KnownRecipes[recipeId]; !ok {
		c.KnownRecipes[recipeId] = 1
		return true
	}
	return false
}

// SetCast records a pending spell cast.
//
// U12c-2: this was the LAST writer outside the targeting seam. It assigned
// c.Aggro directly and never touched any state machine, so calling it over a
// live engagement left the two stores disagreeing -- Aggro dropped to zero ids
// while CombatPhase kept the old target. It now records the cast on the
// Activity machine, which is where every other cast in the game is recorded,
// and sets the round budget on the combat phase machine.
//
// Its one production caller is mobcommands.Aid, a heal on a downed player in a
// calm room. Nothing resolves this cast into a spell effect; the record exists
// so Death_InboundAggroCleanup can abort an in-flight aid when its target dies,
// and so IsAggro reports the caster as engaged with its aim.
//
// Returns whether the cast was recorded. A refused transition (the actor is
// already busy) records nothing, which is what a caller should check before
// narrating a cast that did not start.
func (c *Character) SetCast(roundsWaitTime int, sInfo SpellAggroInfo) bool {
	if c.Activity == nil {
		return false
	}
	if err := c.Activity.TransitionToCasting(activity.CastingData{
		SpellId:              sInfo.SpellId,
		SpellRest:            sInfo.SpellRest,
		TargetUserIds:        sInfo.TargetUserIds,
		TargetMobInstanceIds: sInfo.TargetMobInstanceIds,
	}, state.TransitionReason{
		Trigger: activity.TriggerCastBegin,
	}); err != nil {
		return false
	}
	c.SetRoundsWaiting(roundsWaitTime)
	return true
}
