package costs

import "github.com/GoMudEngine/GoMud/internal/actionspec"

// The action registry moved to internal/actionspec in U9 so that the cost
// calculator and the progression layer read ONE table. Costs and progression
// ask the same question of an action -- which skill governs it -- and two
// tables answering it would drift the first time someone added an action to
// only one of them.
//
// These aliases exist so no cost call site changed when it moved. New code
// should import internal/actionspec directly.

type Action = actionspec.Action
type Spec = actionspec.Spec
type SkillSource = actionspec.SkillSource

const (
	SkillNone           = actionspec.SkillNone
	SkillFixed          = actionspec.SkillFixed
	SkillEquippedCombat = actionspec.SkillEquippedCombat
)

const (
	ActionAttack          = actionspec.ActionAttack
	ActionDodge           = actionspec.ActionDodge
	ActionParry           = actionspec.ActionParry
	ActionBlock           = actionspec.ActionBlock
	ActionMove            = actionspec.ActionMove
	ActionFlee            = actionspec.ActionFlee
	ActionQuell           = actionspec.ActionQuell
	ActionDefy            = actionspec.ActionDefy
	ActionShoot           = actionspec.ActionShoot
	ActionReload          = actionspec.ActionReload
	ActionBash            = actionspec.ActionBash
	ActionTrip            = actionspec.ActionTrip
	ActionKick            = actionspec.ActionKick
	ActionGrapple         = actionspec.ActionGrapple
	ActionGrappleMaintain = actionspec.ActionGrappleMaintain
	ActionHamstring       = actionspec.ActionHamstring
	ActionRake            = actionspec.ActionRake
	ActionMaul            = actionspec.ActionMaul
	ActionPounce          = actionspec.ActionPounce
	ActionGore            = actionspec.ActionGore
	ActionDrain           = actionspec.ActionDrain
	ActionThrottle        = actionspec.ActionThrottle
	ActionThrow           = actionspec.ActionThrow
	ActionSneak           = actionspec.ActionSneak
	ActionTaunt           = actionspec.ActionTaunt
	ActionRally           = actionspec.ActionRally
	ActionWarcry          = actionspec.ActionWarcry
)

// SpecFor returns the registry entry for an action. See actionspec.SpecFor for
// the unregistered-action contract.
func SpecFor(a Action) Spec { return actionspec.SpecFor(a) }
