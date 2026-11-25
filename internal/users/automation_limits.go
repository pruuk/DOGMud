package users

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// A tick or trigger comes from the player's client (GMCP Char.Automation.Set)
// and is kept in their save, so what one may hold is capped in config.yaml:
// Network.MaxAutomationTicks, MaxAutomationTriggers, MaxAutomationFieldLength
// and MaxAutomationConditionValues, and each command chain at
// MaxCommandChain. SetTick and SetTrigger refuse past a cap with one of these
// errors and store nothing.
var (
	// ErrAutomationFull: a new tick or trigger past the most a player keeps.
	// Changing one already kept is never refused for this.
	ErrAutomationFull = errors.New(`automation: no room for another`)
	// ErrAutomationTooLong: a text field longer than the cap, or a condition
	// with more values than the cap.
	ErrAutomationTooLong = errors.New(`automation: too long to save`)
	// ErrAutomationTooManyCommands: a command chain (a tick's commands, a
	// trigger's then or else commands) holding more than MaxCommandChain.
	ErrAutomationTooManyCommands = errors.New(`automation: too many commands in a chain`)
)

// MaxCommandChain is the most commands one chain may hold: a macro, a tick,
// a trigger's then commands or its else commands. It is the cap on a
// player's pending client commands (Network.MaxPendingInputs), since a
// longer chain sent at once would lose its last commands there. One knob,
// so the two can never disagree.
func MaxCommandChain() int {
	return int(configs.GetNetworkConfig().MaxPendingInputs)
}

// CommandChainLength counts the commands in a ";"-separated chain the way
// every sender splits it (the web client trims each piece and skips a blank
// one; world.processInput skips an empty macro piece): blank pieces are not
// commands.
func CommandChainLength(chain string) int {
	n := 0
	for piece := range strings.SplitSeq(chain, `;`) {
		if strings.TrimSpace(piece) != `` {
			n++
		}
	}
	return n
}

// chainFits reports whether chain fits MaxCommandChain. A chain unchanged
// from the one already saved (was) always fits: a save from before the limit
// keeps working, and switching it on or off or renaming it is not refused;
// only editing its commands must bring it within the limit.
func chainFits(chain string, was *string) bool {
	if was != nil && *was == chain {
		return true
	}
	return CommandChainLength(chain) <= MaxCommandChain()
}

// automationFieldsFit reports whether every field is at most the cap, in
// characters.
func automationFieldsFit(fields ...string) bool {
	most := int(configs.GetNetworkConfig().MaxAutomationFieldLength)
	for _, f := range fields {
		if utf8.RuneCountInString(f) > most {
			return false
		}
	}
	return true
}

// tickFits checks t against the caps; prev is the tick it replaces, nil
// when storing it adds one.
func (u *UserRecord) tickFits(t UserTick, prev *UserTick) error {
	if !automationFieldsFit(t.Name, t.Commands) {
		return ErrAutomationTooLong
	}
	var was *string
	if prev != nil {
		was = &prev.Commands
	}
	if !chainFits(t.Commands, was) {
		return ErrAutomationTooManyCommands
	}
	if prev == nil && len(u.Ticks) >= int(configs.GetNetworkConfig().MaxAutomationTicks) {
		return ErrAutomationFull
	}
	return nil
}

// triggerFits checks t against the caps; prev is the trigger it replaces,
// nil when storing it adds one.
func (u *UserRecord) triggerFits(t UserTrigger, prev *UserTrigger) error {
	if !automationFieldsFit(t.Name, t.Pattern, t.ThenCmds, t.ElseCmds, t.QueueMode) {
		return ErrAutomationTooLong
	}
	if c := t.Condition; c != nil {
		if len(c.Values) > int(configs.GetNetworkConfig().MaxAutomationConditionValues) {
			return ErrAutomationTooLong
		}
		if !automationFieldsFit(c.SourceKind, c.SourceKey, c.Op) || !automationFieldsFit(c.Values...) {
			return ErrAutomationTooLong
		}
	}
	var wasThen, wasElse *string
	if prev != nil {
		wasThen, wasElse = &prev.ThenCmds, &prev.ElseCmds
	}
	if !chainFits(t.ThenCmds, wasThen) || !chainFits(t.ElseCmds, wasElse) {
		return ErrAutomationTooManyCommands
	}
	if prev == nil && len(u.Triggers) >= int(configs.GetNetworkConfig().MaxAutomationTriggers) {
		return ErrAutomationFull
	}
	return nil
}
