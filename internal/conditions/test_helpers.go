package conditions

// SeedConditionsForTest replaces the global conditions map with the supplied test data
// and returns a cleanup function that restores the original.
// Intended for cross-package integration tests (hooks, commands).
func SeedConditionsForTest(conditionMap map[int]*ConditionSpec) func() {
	orig := conditions
	conditions = conditionMap
	return func() {
		conditions = orig
	}
}

// SeedConditionRecordsForTest adds the condition records (79, 80, 117 to 123)
// to whatever spec map is current, with exactly the shipped mechanical shape
// AND the shipped start/trigger/end text (see each condition's
// _datafiles/world/dogmud/conditions YAML), and returns a cleanup that removes
// them again. Additive on purpose: a package fixture that already seeded its
// own conditions keeps them.
func SeedConditionRecordsForTest() func() {
	mag := EffectValue{UsesMagnitude: true}
	records := []*ConditionSpec{
		{ConditionId: ConditionIdWarcry, Name: "Warcry", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectDamageMult: mag}, EndUserText: "The fervor of the warcry fades from you."},
		{ConditionId: ConditionIdRally, Name: "Rally", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectDefenseMult: mag}, EndUserText: "The strength of the rally drains from you."},
		{ConditionId: ConditionIdOffBalance, Name: "Off Balance", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectDefenseMult: {Literal: 0.85}}},
		{ConditionId: ConditionIdRecovering, Name: "Recovering", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}},
		{ConditionId: ConditionIdConvictionWard, Name: "Conviction Ward", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: mag}, StartActorText: "Your conviction hardens into a ward around {actee}.", StartUserText: "A ward of hardened conviction settles around you.", StartRoomText: "A faint ward of conviction shimmers around {actee}.", EndUserText: "Your Conviction Ward fades.", EndRoomText: "The ward around {actee} fades."},
		{ConditionId: ConditionIdRegenerating, Name: "Regenerating", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectRegenMult: mag}, EndUserText: "The healing magic in your wounds runs its course."},
		{ConditionId: ConditionIdPoisoned, Name: "Poisoned", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{Poison, SilentStart}, TickPool: "health", TickFromMagnitude: true, TriggerUserText: `<ansi fg="green">The poison burns through your veins!</ansi>`, EndUserText: "The poison in your veins finally burns itself out."},
		{ConditionId: ConditionIdBleeding, Name: "Bleeding", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 4, Flags: []Flag{Bleeding, SilentStart, Stacking}, TickPool: "health", TickFromMagnitude: true, TriggerUserText: `<ansi fg="red">Blood seeps from your wounds!</ansi>`, EndUserText: "Your wounds stop bleeding."},
		{ConditionId: ConditionIdEnchantWithdrawal, Name: "Enchant Withdrawal", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectPoolMaxPct: mag}, StartUserText: "The severed bond leaves a hollow in you that will take time to fill.", EndUserText: "The hollow the severed bond left in you has finally closed."},
	}
	if conditions == nil {
		conditions = map[int]*ConditionSpec{}
	}
	replaced := map[int]*ConditionSpec{}
	for _, r := range records {
		if old, ok := conditions[r.ConditionId]; ok {
			replaced[r.ConditionId] = old
		}
		conditions[r.ConditionId] = r
	}
	return func() {
		for _, r := range records {
			delete(conditions, r.ConditionId)
		}
		for id, old := range replaced {
			conditions[id] = old
		}
	}
}
