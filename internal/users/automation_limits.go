package users

import (
	"errors"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// A tick or trigger comes from the player's client (GMCP Char.Automation.Set)
// and is kept in their save, so what one may hold is capped in config.yaml:
// Network.MaxAutomationTicks, MaxAutomationTriggers, MaxAutomationFieldLength
// and MaxAutomationConditionValues. SetTick and SetTrigger refuse past a cap
// with one of these errors and store nothing.
var (
	// ErrAutomationFull: a new tick or trigger past the most a player keeps.
	// Changing one already kept is never refused for this.
	ErrAutomationFull = errors.New(`automation: no room for another`)
	// ErrAutomationTooLong: a text field longer than the cap, or a condition
	// with more values than the cap.
	ErrAutomationTooLong = errors.New(`automation: too long to save`)
)

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

// tickFits checks t against the caps; isNew is whether storing it adds one.
func (u *UserRecord) tickFits(t UserTick, isNew bool) error {
	if !automationFieldsFit(t.Name, t.Commands) {
		return ErrAutomationTooLong
	}
	if isNew && len(u.Ticks) >= int(configs.GetNetworkConfig().MaxAutomationTicks) {
		return ErrAutomationFull
	}
	return nil
}

// triggerFits checks t against the caps; isNew is whether storing it adds one.
func (u *UserRecord) triggerFits(t UserTrigger, isNew bool) error {
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
	if isNew && len(u.Triggers) >= int(configs.GetNetworkConfig().MaxAutomationTriggers) {
		return ErrAutomationFull
	}
	return nil
}
