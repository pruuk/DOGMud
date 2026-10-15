package refusal

// The sight-safe not-here lines (spec section 2, #454). They never echo:
// a name the player can only find by looking reads the same whether it is
// there unseen or matches nothing. Callers that return a line instead of
// sending it (actions.AimBySight) use the constants.
const (
	// NotHereCreatureLine answers a creature name the player cannot pick out.
	NotHereCreatureLine = `You don't see them here.`
	// NotHereThingLine answers a thing in the room (floor, room container,
	// corpse) the player cannot pick out.
	NotHereThingLine = `You don't see that here.`
	// NothingToAimLine answers a player who sees nothing and named a shape,
	// or named nothing at all.
	NothingToAimLine = `You can't see anything to aim at.`
)

func NotHereCreature(to Recipient) { send(to, NotHereCreatureLine) }
func NotHereThing(to Recipient)    { send(to, NotHereThingLine) }
func NothingToAim(to Recipient)    { send(to, NothingToAimLine) }
