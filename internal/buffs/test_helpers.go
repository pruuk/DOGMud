package buffs

// SeedBuffsForTest replaces the global buffs map with the supplied test data
// and returns a cleanup function that restores the original.
// Intended for cross-package integration tests (hooks, commands).
func SeedBuffsForTest(buffMap map[int]*BuffSpec) func() {
	orig := buffs
	buffs = buffMap
	return func() {
		buffs = orig
	}
}

// SeedConditionRecordsForTest adds the condition records (79, 80, 117 to 123)
// to whatever spec map is current, with exactly the shipped mechanical shape
// AND the shipped end text (see each buff's _datafiles/world/dogmud/buffs
// YAML), and returns a cleanup that removes them again. Additive on purpose:
// a package fixture that already seeded its own buffs keeps them.
func SeedConditionRecordsForTest() func() {
	mag := EffectValue{UsesMagnitude: true}
	records := []*BuffSpec{
		{BuffId: BuffIdWarcry, Name: "Warcry", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectDamageMult: mag}, EndUserText: "The fervor of the warcry fades from you."},
		{BuffId: BuffIdRally, Name: "Rally", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectDefenseMult: mag}, EndUserText: "The strength of the rally drains from you."},
		{BuffId: BuffIdOffBalance, Name: "Off Balance", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectDefenseMult: {Literal: 0.85}}},
		{BuffId: BuffIdRecovering, Name: "Recovering", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}},
		{BuffId: BuffIdMinorShield, Name: "Minor Shield", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: mag}, EndUserText: "Your Minor Shield dissipates.", EndRoomText: "{source}'s Minor Shield dissipates."},
		{BuffId: BuffIdRegenerating, Name: "Regenerating", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectRegenMult: mag}, EndUserText: "The healing magic in your wounds runs its course."},
		{BuffId: BuffIdPoisoned, Name: "Poisoned", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{Poison, SilentStart}, TickPool: "health", TickFromMagnitude: true, EndUserText: "The poison in your veins finally burns itself out."},
		{BuffId: BuffIdBleeding, Name: "Bleeding", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{Bleeding, SilentStart}, TickPool: "health", TickFromMagnitude: true, EndUserText: "Your wounds stop bleeding."},
		{BuffId: BuffIdEnchantWithdrawal, Name: "Enchant Withdrawal", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectPoolMaxPct: mag}, EndUserText: "The hollow the severed bond left in you has finally closed."},
	}
	if buffs == nil {
		buffs = map[int]*BuffSpec{}
	}
	replaced := map[int]*BuffSpec{}
	for _, r := range records {
		if old, ok := buffs[r.BuffId]; ok {
			replaced[r.BuffId] = old
		}
		buffs[r.BuffId] = r
	}
	return func() {
		for _, r := range records {
			delete(buffs, r.BuffId)
		}
		for id, old := range replaced {
			buffs[id] = old
		}
	}
}
