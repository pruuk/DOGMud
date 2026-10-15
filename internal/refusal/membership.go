package refusal

// NoCompanionLine answers an AI companion command with no companion.
const NoCompanionLine = `You have no companion travelling with you.`

func NotInGuildText() string {
	return `You aren't in a guild.` + hint(`guild create <TAG> <name>`)
}

// NotPartyLeaderText answers a leader-only party command; action completes
// "Only the party leader can ...", empty reads "do that".
func NotPartyLeaderText(action string) string {
	if action == `` {
		action = `do that`
	}
	return `Only the party leader can ` + action + `.`
}

func NotInGuild(to Recipient)                    { send(to, NotInGuildText()) }
func NoCompanion(to Recipient)                   { send(to, NoCompanionLine) }
func NotPartyLeader(to Recipient, action string) { send(to, NotPartyLeaderText(action)) }

// NoPartyInviteLine answers `party accept` or `party decline` from a player
// with no invitation waiting (a member, or one whose invite is gone).
const NoPartyInviteLine = `You have no party invitation to answer.`

func NoPartyInvite(to Recipient) { send(to, NoPartyInviteLine) }
