package refusal

// NotCarryingText answers a name the player looked for in what they carry.
// It echoes typed (the player's own belongings, spec section 2); an empty
// typed reads "that".
func NotCarryingText(typed string) string {
	what := `that`
	if e := echo(typed); e != `` {
		what = `'` + e + `'`
	}
	return `You aren't carrying ` + what + `.` + hint(`inventory`)
}

func NotCarrying(to Recipient, typed string) { send(to, NotCarryingText(typed)) }
