package refusal

import "strconv"

// InputQueueFullLine answers a command refused because the player already
// has as many commands waiting as the game holds (Network.MaxPendingInputs).
// A client that reads Char.Input.Queue greys its controls instead; any other
// gets this line once for each time the queue fills (modules/gmcp).
const InputQueueFullLine = `You're already doing as much as you can. Wait a moment.`

// InputQueueFull sends InputQueueFullLine.
func InputQueueFull(to Recipient) { send(to, InputQueueFullLine) }

// AutomationLimitText answers a new tick or trigger past the most a player
// can keep (Network.MaxAutomationTicks, MaxAutomationTriggers). kind is
// "ticks" or "triggers".
func AutomationLimitText(kind string, most int) string {
	return `You can keep at most ` + strconv.Itoa(most) + ` ` + kind + `. Remove one first.`
}

// AutomationLimit sends AutomationLimitText.
func AutomationLimit(to Recipient, kind string, most int) {
	send(to, AutomationLimitText(kind, most))
}

// CommandChainTooLongText answers a macro, tick or trigger holding more
// commands than one chain may (users.MaxCommandChain, the pending command
// cap). kind is "macro", "tick" or "trigger". The common long chain is a
// walking route, so the line suggests splitting it.
func CommandChainTooLongText(kind string, most int) string {
	return `A ` + kind + ` can hold at most ` + strconv.Itoa(most) + ` commands. Split a long route into two.`
}

// CommandChainTooLong sends CommandChainTooLongText.
func CommandChainTooLong(to Recipient, kind string, most int) {
	send(to, CommandChainTooLongText(kind, most))
}

// AutomationTooLongText answers a tick or trigger with a field longer than
// Network.MaxAutomationFieldLength, or a condition with more values than
// MaxAutomationConditionValues. kind is "tick" or "trigger".
func AutomationTooLongText(kind string) string {
	return `That ` + kind + ` is too long to save. Shorten it and try again.`
}

// AutomationTooLong sends AutomationTooLongText.
func AutomationTooLong(to Recipient, kind string) { send(to, AutomationTooLongText(kind)) }
