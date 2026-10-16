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
