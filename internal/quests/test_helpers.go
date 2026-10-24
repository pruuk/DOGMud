package quests

// SeedQuestsForTest adds qs to the quest registry, replacing any quest with
// the same id, and returns a function that restores the registry as it was.
// Test binaries load no quest data, so a test whose subject reads step order
// (Character.HasQuest on a step past start) seeds the quest it needs.
//
// FOR TEST USE ONLY.
func SeedQuestsForTest(qs ...*Quest) func() {
	saved := make(map[int]*Quest, len(quests))
	for id, q := range quests {
		saved[id] = q
	}
	for _, q := range qs {
		quests[q.QuestId] = q
	}
	return func() { quests = saved }
}
