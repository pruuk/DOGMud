package characters

import "testing"

func TestMigrateNewbieAwakening_Veteran(t *testing.T) {
	c := &Character{QuestProgress: map[int]string{8: "end"}} // old-world progress, no q30
	c.MigrateNewbieAwakening()
	if c.QuestProgress[30] != "end" {
		t.Errorf("veteran: want 30=end, got %q", c.QuestProgress[30])
	}
}

func TestMigrateNewbieAwakening_FreshChar(t *testing.T) {
	c := &Character{} // brand-new: empty progress
	c.MigrateNewbieAwakening()
	if _, ok := c.QuestProgress[30]; ok {
		t.Errorf("fresh char: 30 should be unset, got %q", c.QuestProgress[30])
	}
}

func TestMigrateNewbieAwakening_MidRite(t *testing.T) {
	c := &Character{QuestProgress: map[int]string{30: "start"}} // doing the rite
	c.MigrateNewbieAwakening()
	if c.QuestProgress[30] != "start" {
		t.Errorf("mid-rite: must not overwrite, got %q", c.QuestProgress[30])
	}
}

func TestMigrateNewbieAwakening_RunsOnce(t *testing.T) {
	c := &Character{QuestProgress: map[int]string{8: "end"}}
	c.MigrateNewbieAwakening()
	delete(c.QuestProgress, 30) // simulate a later quest-30 reset
	c.MigrateNewbieAwakening()  // must NOT re-grant (guard set)
	if _, ok := c.QuestProgress[30]; ok {
		t.Errorf("run-once: should not re-grant after guard set")
	}
}

// A quest retired whole (#375 phase 7: quest 29 Two Roads) is dropped from
// progress, the shelf and the focus, so hint and the quest panel never point
// at a quest that no longer exists. Other quests are untouched, and a second
// run drops nothing.
func TestDropRetiredQuests(t *testing.T) {
	c := &Character{
		QuestProgress: map[int]string{29: "start", 30: "end", 31: "start"},
		ShelvedQuests: map[int]string{29: "start"},
		LastQuestId:   29,
	}
	got := c.DropRetiredQuests(map[int]bool{29: true, 77: true})
	if len(got) != 1 || got[0] != 29 {
		t.Fatalf("dropped %v, want [29]", got)
	}
	if _, ok := c.QuestProgress[29]; ok {
		t.Errorf("quest 29 is still in progress: %v", c.QuestProgress)
	}
	if c.IsShelved(29) {
		t.Errorf("quest 29 is still on the shelf: %v", c.ShelvedQuests)
	}
	if c.LastQuestId != 0 {
		t.Errorf("focus still on %d, want 0", c.LastQuestId)
	}
	if c.QuestProgress[30] != "end" || c.QuestProgress[31] != "start" {
		t.Errorf("other quests changed: %v", c.QuestProgress)
	}
	if again := c.DropRetiredQuests(map[int]bool{29: true}); len(again) != 0 {
		t.Errorf("second run dropped %v, want nothing", again)
	}
}
