package refusal

// NotCarryingText answers a name the player looked for in what they carry.
// It echoes typed (the player's own belongings, spec section 2); an empty
// typed reads "that".
func NotCarryingText(typed string) string {
	return `You aren't carrying ` + quoted(typed) + `.` + hint(`inventory`)
}

// NotWearingText answers a name the player looked for in what they wear.
func NotWearingText(typed string) string {
	return `You aren't wearing ` + quoted(typed) + `.` + hint(`inventory`)
}

func NotWearing(to Recipient, typed string) { send(to, NotWearingText(typed)) }

// NotInStorageText answers a name the player looked for in their storage.
func NotInStorageText(typed string) string {
	return `You don't have ` + quoted(typed) + ` in storage.` + hint(`storage`)
}

func NotInStorage(to Recipient, typed string) { send(to, NotInStorageText(typed)) }

func NotCarrying(to Recipient, typed string) { send(to, NotCarryingText(typed)) }

// RecipientFullLine answers a give to someone who cannot carry any more.
const RecipientFullLine = `They can't carry any more.`

func RecipientFull(to Recipient) { send(to, RecipientFullLine) }

// QuaffPotionText answers drink or eat aimed at a potion: nothing is
// consumed, and the line names quaff (#375 phase 7, owner ruling
// 2026-10-30). It echoes typed, the player's own item.
func QuaffPotionText(typed string) string {
	return `That is a potion, and potions are drunk with quaff.` + hint(`quaff `+echo(typed))
}

func QuaffPotion(to Recipient, typed string) { send(to, QuaffPotionText(typed)) }

// NotAPotionText answers quaff aimed at food or a cooked drink, naming the
// verb that takes it ("eat" or "drink").
func NotAPotionText(typed, verb string) string {
	return `That is not a potion.` + hint(verb+` `+echo(typed))
}

func NotAPotion(to Recipient, typed, verb string) { send(to, NotAPotionText(typed, verb)) }
