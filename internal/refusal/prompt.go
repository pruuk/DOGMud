package refusal

import "strings"

// Ask is the question a prompt ends on. The set is fixed so no caller writes
// a question literal (the phrase guard's "prompt" row holds that).
type Ask string

const (
	What        Ask = `what`
	Whom        Ask = `whom`
	HowMuch     Ask = `how much`
	HowMuchGold Ask = `how much gold`
)

// PromptText answers a command typed without what it needs:
// "<Verb> <ask>?" with the verb capitalized, then a hint naming usage when
// usage is not empty. verb carries any preposition ("talk to", "steal
// from"); an empty verb asks the bare question rather than panic.
func PromptText(verb string, ask Ask, usage string) string {
	v := strings.TrimSpace(verb) + ` ` + string(ask)
	v = strings.TrimSpace(v)
	line := strings.ToUpper(v[:1]) + v[1:] + `?`
	if usage != `` {
		line += hint(usage)
	}
	return line
}

func Prompt(to Recipient, verb string, ask Ask, usage string) {
	send(to, PromptText(verb, ask, usage))
}

// TalkToPlayerLine answers `talk` aimed at a player: talk is for creatures
// with something to say, and players hear say.
const TalkToPlayerLine = `To speak with a player, try <ansi fg="command">say <message></ansi>.`

func TalkToPlayer(to Recipient) { send(to, TalkToPlayerLine) }
