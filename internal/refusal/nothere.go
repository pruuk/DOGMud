package refusal

// The sight-safe not-here lines (spec section 2, #454). They never echo:
// a name the player can only find by looking reads the same whether it is
// there unseen or matches nothing. Callers that return a line instead of
// sending it (actions.AimBySight) use the constants. Both are sense-neutral
// (#609, owner ruling 2026-10-17): the same words fit every light, and a
// player who cannot see is not told they looked.
const (
	// NotHereCreatureLine answers a creature name the player cannot pick out.
	NotHereCreatureLine = `You can't find them here.`
	// NotHereThingLine answers a thing in the room (floor, room container,
	// corpse) the player cannot pick out.
	NotHereThingLine = `You can't find that here.`
	// NothingToAimLine answers a player who sees nothing and named a shape,
	// or named nothing at all.
	NothingToAimLine = `You can't see anything to aim at.`
	// NoMerchantLine answers every shop verb (list, buy, sell, offer,
	// appraise) used where no merchant is present. One wording for all of
	// them (2026-10-17 playtest found four).
	NoMerchantLine = `There's no merchant here.`
)

func NotHereCreature(to Recipient) { send(to, NotHereCreatureLine) }
func NotHereThing(to Recipient)    { send(to, NotHereThingLine) }
func NothingToAim(to Recipient)    { send(to, NothingToAimLine) }
func NoMerchant(to Recipient)      { send(to, NoMerchantLine) }
