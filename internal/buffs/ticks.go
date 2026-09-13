package buffs

// TickTriggers converts a duration in rounds into the trigger count for a
// record whose triggerrate is three rounds (the spell dot and bleed). The
// old AutoHeal hook landed those ticks only on every third round while the
// duration counted every round, so a duration of rounds produced rounds/3
// ticks; the record keeps that arithmetic. Never below one.
func TickTriggers(rounds int) int {
	if rounds < 3 {
		return 1
	}
	return rounds / 3
}
