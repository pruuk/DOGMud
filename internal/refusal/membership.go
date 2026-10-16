package refusal

// NoCompanionLine answers an AI companion command with no companion.
const NoCompanionLine = `You have no companion travelling with you.`

func NotInGuildText() string {
	return `You aren't in a guild.` + hint(`guild create <TAG> <name>`)
}

// NotPartyLeaderText answers a leader-only party command; action completes
// "Only the party leader can ...", empty reads "do that".
func NotPartyLeaderText(action string) string {
	return `Only the party leader can ` + orDoThat(action) + `.`
}

func NotInGuild(to Recipient)                    { send(to, NotInGuildText()) }
func NoCompanion(to Recipient)                   { send(to, NoCompanionLine) }
func NotPartyLeader(to Recipient, action string) { send(to, NotPartyLeaderText(action)) }

// NoPartyInviteLine answers `party accept` or `party decline` from a player
// with no invitation waiting (a member, or one whose invite is gone).
const NoPartyInviteLine = `You have no party invitation to answer.`

func NoPartyInvite(to Recipient) { send(to, NoPartyInviteLine) }

func NotInPartyText() string { return `You aren't in a party.` + hint(`party create`) }

func NotInParty(to Recipient) { send(to, NotInPartyText()) }

// NotGuildLeaderText answers a leader-only guild command; action completes
// "Only the guild leader can ...", empty reads "do that".
func NotGuildLeaderText(action string) string {
	return `Only the guild leader can ` + orDoThat(action) + `.`
}

// NotGuildOfficerText answers an officer-or-leader guild command.
func NotGuildOfficerText(action string) string {
	return `Only guild officers and the leader can ` + orDoThat(action) + `.`
}

func NotGuildLeader(to Recipient, action string)  { send(to, NotGuildLeaderText(action)) }
func NotGuildOfficer(to Recipient, action string) { send(to, NotGuildOfficerText(action)) }

// CompanionsFullLine answers a summon or charm past the companion limit.
const CompanionsFullLine = `You are already sustaining as many companions as your will can hold.`

func CompanionsFull(to Recipient) { send(to, CompanionsFullLine) }

// NotPartyMemberText answers a party command naming someone not in the
// player's party. It echoes: the player's own party is a name they know.
func NotPartyMemberText(typed string) string {
	return `No one in your party is called ` + quoted(typed) + `.`
}

func NotPartyMember(to Recipient, typed string) { send(to, NotPartyMemberText(typed)) }

func orDoThat(action string) string {
	if action == `` {
		return `do that`
	}
	return action
}
