package refusal

// WhileBusyText answers a command refused while the player is casting,
// crafting or salvaging. cancel ends any of them, so the hint names it.
func WhileBusyText(doing string) string {
	return `You can't ` + doing + ` while you're busy.` + hint(`cancel`)
}

func WhileBusy(to Recipient, doing string) { send(to, WhileBusyText(doing)) }

// WhileUnderAttackText answers an action refused while something attacks the
// player, before they are in combat with it (the in-combat gate is
// WhileFighting).
func WhileUnderAttackText(doing string) string {
	return `You can't ` + doing + ` while under attack.`
}

func WhileUnderAttack(to Recipient, doing string) { send(to, WhileUnderAttackText(doing)) }

// TargetAsleepText answers a trade or talk with a sleeper. It names the
// sleeper; actions.RefuseIfAsleep hides the name at the player's sight
// before sending, so this helper has no send of its own. aim is what the
// player types after wake to reach the sleeper at their sight: the name in
// lower case at clear sight, `figure` or `2.figure` at shapes. The caller
// picks it, because HideNames hides only the name as spelled and a lowered
// name would slip past it. An empty aim (no sight, so wake reaches no one)
// hints shout, which wakes the whole room.
func TargetAsleepText(name, aim string) string {
	line := `<ansi fg="mobname">` + name + `</ansi> is fast asleep.`
	if aim == `` {
		return line + ` You could make some noise.` + hint(`shout wake up`)
	}
	return line + hint(`wake `+aim)
}
