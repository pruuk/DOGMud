package refusal

// Raise refusals: why a raise spell found no corpse it could animate.
// actions.RaiseCorpseRefusal picks the line from the selection's reason; both
// raise gates (cast start and resolution) send it on CategoryRefusal.

// NoRemainsLine answers a raise in a room with no raisable corpse.
const NoRemainsLine = `There are no suitable remains here to raise.`

// NoRemainsMatchingLine answers a raise whose named corpse is not here.
const NoRemainsMatchingLine = `You cannot find suitable remains matching that description.`

// RemainsHoldLootLine answers a raise whose corpse still holds loot.
const RemainsHoldLootLine = `Those remains still hold spoils. Clear the loot from them first, then raise what's left.`

// RemainsTooFaintLine answers a raise whose corpse is below the spell's
// minimum essence.
const RemainsTooFaintLine = `Those remains are too faint to hold a bound spirit.`

// RemainsAlreadyRaisedLine answers a raise whose corpse was a companion.
const RemainsAlreadyRaisedLine = `Those remains were already animated once; there is nothing left to raise.`
