package refusal

// NoTargetText answers a combat move with no target; verb is the command.
func NoTargetText(verb string) string {
	return `You have no target.` + hint(verb+` <name>`)
}

// CooldownText answers the shared special-move timer. doing names what the
// player tried ("kick", "consider anyone"); never a duration.
func CooldownText(doing string) string {
	return `You need a moment to recover before you can ` + doing + `.`
}

// CastSharesSpecialMoveText follows CooldownText("cast") when the cast was
// blocked by the shared special-move timer, so the player learns why a
// recent bash or taunt stops a spell (#264).
const CastSharesSpecialMoveText = `Casting counts as a special move, so it shares their recovery time.`

// WhileFightingText answers an action combat forbids.
func WhileFightingText(doing string) string {
	return `You can't ` + doing + ` while fighting.`
}

func NoTarget(to Recipient, verb string)       { send(to, NoTargetText(verb)) }
func Cooldown(to Recipient, doing string)      { send(to, CooldownText(doing)) }
func WhileFighting(to Recipient, doing string) { send(to, WhileFightingText(doing)) }
