package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/require"
)

// automationCaps pins small caps so the tests read plainly.
func automationCaps(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Network.MaxAutomationTicks = 2
	cfg.Network.MaxAutomationTriggers = 2
	cfg.Network.MaxAutomationFieldLength = 10
	cfg.Network.MaxAutomationConditionValues = 2
	configs.SetConfigForTest(t, cfg)
}

// A player keeps at most the cap of ticks: a new one past it is refused and
// nothing is stored, under any id, while changing one already kept still works.
func TestSetTick_CountIsCapped(t *testing.T) {
	automationCaps(t)
	u := &UserRecord{}
	a, err := u.SetTick(UserTick{Name: "a"})
	require.NoError(t, err)
	_, err = u.SetTick(UserTick{Name: "b"})
	require.NoError(t, err)

	_, err = u.SetTick(UserTick{Name: "c"})
	require.ErrorIs(t, err, ErrAutomationFull)
	_, err = u.SetTick(UserTick{Id: 999, Name: "c"})
	require.ErrorIs(t, err, ErrAutomationFull, "an unknown id is a new tick too")
	require.Len(t, u.Ticks, 2)

	a.Name = "a2"
	_, err = u.SetTick(a)
	require.NoError(t, err, "changing a kept tick is not adding one")
	require.Equal(t, "a2", u.Ticks[0].Name)
}

// A tick field longer than the cap is refused, counted in characters.
func TestSetTick_FieldLengthIsCapped(t *testing.T) {
	automationCaps(t)
	u := &UserRecord{}
	_, err := u.SetTick(UserTick{Name: "ok", Commands: strings.Repeat("x", 11)})
	require.ErrorIs(t, err, ErrAutomationTooLong)
	_, err = u.SetTick(UserTick{Name: strings.Repeat("x", 11)})
	require.ErrorIs(t, err, ErrAutomationTooLong)
	require.Empty(t, u.Ticks)

	_, err = u.SetTick(UserTick{Name: strings.Repeat("é", 10)})
	require.NoError(t, err, "ten characters fit, whatever their bytes")
}

func TestSetTrigger_CountIsCapped(t *testing.T) {
	automationCaps(t)
	u := &UserRecord{}
	for _, n := range []string{"a", "b"} {
		_, err := u.SetTrigger(UserTrigger{Name: n, Pattern: "*"})
		require.NoError(t, err)
	}
	_, err := u.SetTrigger(UserTrigger{Name: "c", Pattern: "*"})
	require.ErrorIs(t, err, ErrAutomationFull)
	require.Len(t, u.Triggers, 2)
}

// Every text field of a trigger is capped, its condition's included, and so
// is the number of condition values.
func TestSetTrigger_FieldsAreCapped(t *testing.T) {
	automationCaps(t)
	long := strings.Repeat("x", 11)
	for name, tr := range map[string]UserTrigger{
		"name":      {Name: long},
		"pattern":   {Pattern: long},
		"then":      {ThenCmds: long},
		"else":      {ElseCmds: long},
		"queueMode": {QueueMode: long},
		"kind":      {Condition: &TriggerCondition{SourceKind: long}},
		"key":       {Condition: &TriggerCondition{SourceKey: long}},
		"op":        {Condition: &TriggerCondition{Op: long}},
		"value":     {Condition: &TriggerCondition{Values: []string{"a", long}}},
		"values":    {Condition: &TriggerCondition{Values: []string{"a", "b", "c"}}},
	} {
		u := &UserRecord{}
		_, err := u.SetTrigger(tr)
		require.ErrorIs(t, err, ErrAutomationTooLong, name)
		require.Empty(t, u.Triggers, name)
	}

	u := &UserRecord{}
	_, err := u.SetTrigger(UserTrigger{Name: "fits", Pattern: "*x*",
		Condition: &TriggerCondition{SourceKind: "target", Op: "oneof", Values: []string{"a", "b"}}})
	require.NoError(t, err)
}

// A chain is counted as every sender splits it: blank pieces are not
// commands.
func TestCommandChainLength(t *testing.T) {
	require.Equal(t, 0, CommandChainLength(""))
	require.Equal(t, 0, CommandChainLength(" ; ;"))
	require.Equal(t, 3, CommandChainLength("n;e; ;s;"))
	require.Equal(t, 1, CommandChainLength("say hello there"))
}

// chainCaps pins the chain limit (the pending input cap) at 3.
func chainCaps(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Network.MaxPendingInputs = 3
	configs.SetConfigForTest(t, cfg)
}

// A tick's commands and each of a trigger's then and else commands hold at
// most MaxCommandChain, the pending input cap.
func TestSetAutomation_CommandChainIsCapped(t *testing.T) {
	chainCaps(t)
	require.Equal(t, 3, MaxCommandChain())

	u := &UserRecord{}
	_, err := u.SetTick(UserTick{Name: "ok", Commands: "n;e;s"})
	require.NoError(t, err)
	_, err = u.SetTick(UserTick{Name: "x", Commands: "n;e;s;w"})
	require.ErrorIs(t, err, ErrAutomationTooManyCommands)

	_, err = u.SetTrigger(UserTrigger{Name: "t", ThenCmds: "n;e;s;w"})
	require.ErrorIs(t, err, ErrAutomationTooManyCommands)
	_, err = u.SetTrigger(UserTrigger{Name: "t", ThenCmds: "n", ElseCmds: "n;e;s;w"})
	require.ErrorIs(t, err, ErrAutomationTooManyCommands)
	require.Len(t, u.Ticks, 1)
	require.Empty(t, u.Triggers)
}

// A chain saved before the limit stays saved and can still be switched on or
// off or renamed; only changing its commands must bring it within the limit.
func TestSetAutomation_OlderLongChainIsKeptUntilEdited(t *testing.T) {
	chainCaps(t)
	long := "n;e;s;w;n"
	u := &UserRecord{
		Ticks:    []UserTick{{Id: 1, Name: "route", Commands: long, IntervalSec: 5}},
		Triggers: []UserTrigger{{Id: 1, Name: "flee", ThenCmds: long, ElseCmds: long}},
	}

	_, err := u.SetTick(UserTick{Id: 1, Name: "route2", Commands: long, IntervalSec: 5, Enabled: true})
	require.NoError(t, err, "switching on and renaming keeps the old chain")
	_, err = u.SetTrigger(UserTrigger{Id: 1, Name: "flee", ThenCmds: long, ElseCmds: long, Enabled: true})
	require.NoError(t, err)

	_, err = u.SetTick(UserTick{Id: 1, Name: "route2", Commands: long + ";e"})
	require.ErrorIs(t, err, ErrAutomationTooManyCommands, "an edited chain must fit")
	require.Equal(t, long, u.Ticks[0].Commands, "the refused edit stores nothing")

	_, err = u.SetTick(UserTick{Id: 1, Name: "route2", Commands: "n;e"})
	require.NoError(t, err, "shortening it is fine")
}

// The shipped caps are generous: well above what a player builds by hand.
func TestAutomationCaps_ShippedDefaults(t *testing.T) {
	n := configs.Network{}
	n.Validate()
	require.Equal(t, 50, int(n.MaxAutomationTicks))
	require.Equal(t, 100, int(n.MaxAutomationTriggers))
	require.Equal(t, 1000, int(n.MaxAutomationFieldLength))
	require.Equal(t, 50, int(n.MaxAutomationConditionValues))
}
